package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"ldap-es-syncer/internal/domain/repository"
)

// SyncUserUseCase はユーザーの同期処理を行うユースケースのインターフェースです。
type SyncUserUseCase interface {
	Execute(ctx context.Context) error
}

// syncUserUseCase はSyncUserUseCaseの具象実装構造体です。
type syncUserUseCase struct {
	sourceRepo        repository.SourceRepository
	targetRepo        repository.TargetRepository
	metricsRepo       repository.MetricsRepository
	finalFilter       string
	excludedUsers     []string
	syncMinUsers      int
	dryRun            bool
	syncSecurityUsers bool
}

// NewSyncUserUseCase はsyncUserUseCaseのコンストラクタです。
func NewSyncUserUseCase(
	sourceRepo repository.SourceRepository,
	targetRepo repository.TargetRepository,
	metricsRepo repository.MetricsRepository,
	finalFilter string,
	excludedUsers []string,
	syncMinUsers int,
	dryRun bool,
	syncSecurityUsers bool,
) SyncUserUseCase {
	return &syncUserUseCase{
		sourceRepo:        sourceRepo,
		targetRepo:        targetRepo,
		metricsRepo:       metricsRepo,
		finalFilter:       finalFilter,
		excludedUsers:     excludedUsers,
		syncMinUsers:      syncMinUsers,
		dryRun:            dryRun,
		syncSecurityUsers: syncSecurityUsers,
	}
}

// Execute は同期の実行フローを制御します。
func (u *syncUserUseCase) Execute(ctx context.Context) (err error) {
	start := time.Now()
	defer func() {
		// Dry-Run時はメトリクスへの反映を行わない
		if !u.dryRun {
			u.metricsRepo.RecordSyncDuration(time.Since(start))
			u.metricsRepo.RecordSyncStatus(err == nil)
		}
	}()

	// 1. [可観測ログ] 同期開始の出力 (Dry-Run モード判定含む)
	if u.dryRun {
		slog.Info("Starting user synchronization pipeline (Dry-Run mode)", slog.String("final_filter", u.finalFilter))
	} else {
		slog.Info("Starting user synchronization pipeline", slog.String("final_filter", u.finalFilter))
	}

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

		// -- ロール名とLDAPグループ名のマッピング機能 --
		var validatedRoles []string
		for _, role := range user.Roles {
			exists, err := u.targetRepo.RoleExists(ctx, role)
			if err != nil {
				// セキュリティAPI非対応（ローカル開発時の無効設定等）や接続エラー
				slog.Warn("Elasticsearch role validation skipped (API not supported or error)",
					slog.String("role", role),
					slog.String("user_id", user.ID),
					slog.String("error", err.Error()),
				)
				continue
			}
			if exists {
				validatedRoles = append(validatedRoles, role)
			} else {
				// ロール名とグループ名が一致しない、または存在しない場合はWarn警告
				slog.Warn("Role does not exist in Elasticsearch, skipping assignment",
					slog.String("role", role),
					slog.String("user_id", user.ID),
				)
			}
		}
		user.Roles = validatedRoles

		// 保存処理
		if u.dryRun {
			slog.Info("[Dry-Run] Would upsert user",
				slog.String("id", user.ID),
				slog.String("username", user.Username),
				slog.String("email", user.Email),
				slog.Any("roles", user.Roles),
			)
			if u.syncSecurityUsers {
				slog.Info("[Dry-Run] Would upsert security user (Native Realm)",
					slog.String("id", user.ID),
					slog.String("username", user.Username),
					slog.String("password_hash_present", fmt.Sprintf("%t", user.PasswordHash != "")),
				)
			}
		} else {
			if err := u.targetRepo.SaveUser(ctx, user); err != nil {
				return fmt.Errorf("failed to save active user %s (ID: %s) to target: %w", user.Username, user.ID, err)
			}
			if u.syncSecurityUsers {
				if err := u.targetRepo.SaveSecurityUser(ctx, user); err != nil {
					return fmt.Errorf("failed to save active security user %s (ID: %s) to target: %w", user.Username, user.ID, err)
				}
			}
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
				if u.dryRun {
					slog.Info("[Dry-Run] Would deactivate user", slog.String("id", existingID))
					if u.syncSecurityUsers {
						slog.Info("[Dry-Run] Would deactivate security user (Native Realm)", slog.String("id", existingID))
					}
				} else {
					if err := u.targetRepo.SaveUser(ctx, user); err != nil {
						return fmt.Errorf("failed to deactivate user %s on target: %w", existingID, err)
					}
					if u.syncSecurityUsers {
						if err := u.targetRepo.SaveSecurityUser(ctx, user); err != nil {
							return fmt.Errorf("failed to deactivate security user %s on target: %w", existingID, err)
						}
					}
				}
				processedCount++
			}
		}
	}

	// 6. ログ統計の出力
	if u.dryRun {
		slog.Info("User synchronization process completed (Dry-Run mode)",
			slog.Int("processed_count", processedCount),
			slog.Int("total_active_users", len(users)),
		)
	} else {
		slog.Info("User synchronization process completed",
			slog.Int("processed_count", processedCount),
			slog.Int("total_active_users", len(users)),
		)

		// メトリクスの記録 (成功時かつ非Dry-Run時)
		u.metricsRepo.RecordProcessedUsers(processedCount)
		u.metricsRepo.RecordActiveUsers(len(users))
	}

	return nil
}
