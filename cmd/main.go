package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ldap-es-syncer/internal/application/usecase"
	"ldap-es-syncer/internal/di"
)

func main() {
	// DIコンテナの初期化（内部で構造化ロガーも初期化されます）
	container, err := di.NewContainer()
	if err != nil {
		slog.Error("Failed to initialize application dependencies", slog.Any("error", err))
		os.Exit(1)
	}

	appConfig := container.GetAppConfig()
	syncUseCase := container.GetSyncUserUseCase()

	// シグナルを検知して自動でキャンセルされるベースコンテキストを作成
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ワンオフ（バッチ）モードの処理
	if !appConfig.DaemonMode {
		slog.Info("User synchronization process started (one-off mode)")
		if err := runSingleSync(ctx, syncUseCase); err != nil {
			slog.Error("User synchronization failed", slog.Any("error", err))
			os.Exit(1)
		}
		slog.Info("User synchronization process completed")
		return
	}

	// デーモン（常駐）モードの処理
	slog.Info("User synchronization daemon starting")

	// 初回同期実行
	slog.Info("Running initial synchronization")
	if err := runSingleSync(ctx, syncUseCase); err != nil {
		slog.Error("Initial synchronization failed", slog.Any("error", err))
	}

	ticker := time.NewTicker(appConfig.SyncInterval)
	defer ticker.Stop()

	slog.Info("User synchronization daemon started", slog.Duration("interval", appConfig.SyncInterval))

	for {
		select {
		case <-ctx.Done():
			slog.Info("Received signal, shutting down daemon gracefully")
			slog.Info("User synchronization daemon stopped")
			return
		case <-ticker.C:
			slog.Info("Starting periodic synchronization cycle")
			if err := runSingleSync(ctx, syncUseCase); err != nil {
				slog.Error("Periodic synchronization cycle failed", slog.Any("error", err))
			}
		}
	}
}

// runSingleSync は親コンテキストから派生したタイムアウト付きコンテキストで1回分の同期処理を実行します。
func runSingleSync(ctx context.Context, syncUseCase usecase.SyncUserUseCase) error {
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return syncUseCase.Execute(runCtx)
}
