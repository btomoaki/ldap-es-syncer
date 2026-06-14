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
}

// NewSyncUserUseCase はsyncUserUseCaseのコンストラクタです。
func NewSyncUserUseCase(
	sourceRepo repository.SourceRepository,
	targetRepo repository.TargetRepository,
	finalFilter string,
	excludedUsers []string,
) SyncUserUseCase {
	return &syncUserUseCase{
		sourceRepo:    sourceRepo,
		targetRepo:    targetRepo,
		finalFilter:   finalFilter,
		excludedUsers: excludedUsers,
	}
}

// Execute は同期の実行フローを制御します。
func (u *syncUserUseCase) Execute(ctx context.Context) error {
	// 1. [可視化ログ] FinalFilterを出力
	slog.Info("Starting user synchronization pipeline", slog.String("final_filter", u.finalFilter))

	// 2. [ES全件取得]
	existingIDs, err := u.targetRepo.GetAllUserIDs(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve existing user IDs from target: %w", err)
	}

	// 3. [LDAP生存者取得]
	users, err := u.sourceRepo.FetchUsers(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch users from source: %w", err)
	}

	activeMap := make(map[string]bool)
	processedCount := 0

	// 4. [生存者 Upsert]
	for _, user := range users {
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
