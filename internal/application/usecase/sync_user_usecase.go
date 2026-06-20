package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"ldap-es-syncer/internal/domain/repository"
)

// SyncUserUseCase はユーザーの同期処理を行うユースケースのインターフェースです。
type SyncUserUseCase interface {
	Execute(ctx context.Context) error
}

// syncUserUseCase はSyncUserUseCaseの具象実装構造体です。
type syncUserUseCase struct {
	sourceRepo    repository.SourceRepository
	targetRepo    repository.TargetRepository
	finalFilter   string
	excludedUsers []string
	syncMinUsers  int
}

// NewSyncUserUseCase はsyncUserUseCaseのコンストラクタです。
func NewSyncUserUseCase(
	sourceRepo repository.SourceRepository,
	targetRepo repository.TargetRepository,
	finalFilter string,
	excludedUsers []string,
	syncMinUsers int,
) SyncUserUseCase {
	return &syncUserUseCase{
		sourceRepo:    sourceRepo,
		targetRepo:    targetRepo,
		finalFilter:   finalFilter,
		excludedUsers: excludedUsers,
		syncMinUsers:  syncMinUsers,
	}
}

// Execute は同期の実行フローを制御します。
func (u *syncUserUseCase) Execute(ctx context.Context) error {
	// 1. [可視化ログ] FinalFilterを出力
	slog.Info("Starting user synchronization pipeline", slog.String("final_filter", u.finalFilter))

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("synchronization aborted before start: %w", err)
	}

	// 2. [ES全件取得]
	existingIDs, err := u.targetRepo.GetAllUserIDs(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve existing user IDs from target: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("synchronization aborted after target check: %w", err)
	}

	// 3. [LDAP生存者取得]
	users, err := u.sourceRepo.FetchUsers(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch users from source: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("synchronization aborted after source fetch: %w", err)
	}

	// 3.5 [セーフティガード] 取得したユーザー数が閾値未満の場合は同期をアボート
	if len(users) < u.syncMinUsers {
		return fmt.Errorf("safety guard triggered: LDAP user count (%d) is below the minimum threshold (%d). aborting synchronization to prevent accidental mass deletion", len(users), u.syncMinUsers)
	}

	activeMap := make(map[string]bool)
	processedCount := 0

	// 4. [生存者 Upsert]
	for _, user := range users {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("synchronization aborted during user upsert: %w", err)
		}
		user.IsActive = true // LDAP生存者は明示的に有効
		if err := u.targetRepo.SaveUser(ctx, user); err != nil {
			return fmt.Errorf("failed to save active user %s (ID: %s) to target: %w", user.Username, user.ID, err)
		}
		activeMap[user.ID] = true
		processedCount++
	}

	// 除外チェック用クロージャ
	isExcluded := func(id string) bool {
		for _, ex := range u.excludedUsers {
			if strings.EqualFold(id, ex) {
				return true
			}
		}
		return false
	}

	// 5. [論理削除処理]
	for _, existingID := range existingIDs {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("synchronization aborted during logical deactivation: %w", err)
		}
		if isExcluded(existingID) {
			continue
		}

		if !activeMap[existingID] {
			// 生存マップにいないユーザーは論理削除
			user, err := u.targetRepo.GetUser(ctx, existingID)
			if err != nil {
				return fmt.Errorf("failed to get user %s from target for logical deletion: %w", existingID, err)
			}

			// すでに非アクティブの場合は無駄な更新を避ける
			if user.IsActive {
				user.Deactivate()
				if err := u.targetRepo.SaveUser(ctx, user); err != nil {
					return fmt.Errorf("failed to deactivate user %s on target: %w", existingID, err)
				}
				processedCount++
			}
		}
	}

	// 6. ログ統計の出力
	slog.Info("User synchronization process completed",
		slog.Int("processed_count", processedCount),
		slog.Int("total_active_users", len(users)),
	)

	return nil
}
