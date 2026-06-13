package usecase

import (
	"context"
	"fmt"
	"log"

	"ldap-es-syncer/internal/domain/repository"
)

// SyncUserUseCase はユーザーの同期処理を行うユースケースのインターフェースです。
type SyncUserUseCase interface {
	Execute(ctx context.Context) error
}

// syncUserUseCase はSyncUserUseCaseの具象実装構造体です。
type syncUserUseCase struct {
	sourceRepo repository.SourceRepository
	targetRepo repository.TargetRepository
}

// NewSyncUserUseCase はsyncUserUseCaseのコンストラクタです。
func NewSyncUserUseCase(sourceRepo repository.SourceRepository, targetRepo repository.TargetRepository) SyncUserUseCase {
	return &syncUserUseCase{
		sourceRepo: sourceRepo,
		targetRepo: targetRepo,
	}
}

// Execute は同期の実行フローを制御します。
func (u *syncUserUseCase) Execute(ctx context.Context) error {
	// 1. 同期元からユーザー一覧を取得
	users, err := u.sourceRepo.FetchUsers(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch users from source: %w", err)
	}

	totalUsers := len(users)
	processedCount := 0

	// 2. ユーザーをループで同期先に一件ずつ保存
	for _, user := range users {
		if err := u.targetRepo.SaveUser(ctx, user); err != nil {
			// 途中でエラーが発生した場合は即座にエラーを返して終了（早期リターン）
			return fmt.Errorf("failed to save user %s (ID: %s) to target: %w", user.Username, user.ID, err)
		}
		processedCount++
	}

	// 3. ループ内での一人ひとりの成功ログは排除し、最後に合計件数を1行で出力する最適化 (Log Aggregation)
	log.Printf("Total processed: %d/%d users", processedCount, totalUsers)

	return nil
}
