package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ldap-es-syncer/internal/application/usecase"
	"ldap-es-syncer/internal/di"
)

func main() {
	// 標準出力(stdout)をライフサイクルイベント用に設定
	log.SetOutput(os.Stdout)

	// DIコンテナの初期化
	container, err := di.NewContainer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to initialize application dependencies: %v\n", err)
		os.Exit(1)
	}

	appConfig := container.GetAppConfig()
	syncUseCase := container.GetSyncUserUseCase()

	// ワンオフ（バッチ）モードの処理
	if !appConfig.DaemonMode {
		log.Println("Lifecycle: User synchronization process started (one-off mode).")
		if err := runSingleSync(syncUseCase); err != nil {
			fmt.Fprintf(os.Stderr, "Error: User synchronization failed: %v\n", err)
			os.Exit(1)
		}
		log.Println("Lifecycle: User synchronization process completed.")
		return
	}

	// デーモン（常駐）モードの処理
	log.Println("Lifecycle: User synchronization daemon starting...")

	// シグナルハンドリングの設定
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 初回同期実行
	log.Println("Lifecycle: Running initial synchronization...")
	if err := runSingleSync(syncUseCase); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Initial synchronization failed: %v\n", err)
	}

	ticker := time.NewTicker(appConfig.SyncInterval)
	defer ticker.Stop()

	log.Printf("Lifecycle: User synchronization daemon started. Interval: %v\n", appConfig.SyncInterval)

	for {
		select {
		case sig := <-sigChan:
			log.Printf("Lifecycle: Received signal %v. Shutting down daemon gracefully...\n", sig)
			log.Println("Lifecycle: User synchronization daemon stopped.")
			return
		case <-ticker.C:
			log.Println("Lifecycle: Starting periodic synchronization cycle...")
			if err := runSingleSync(syncUseCase); err != nil {
				fmt.Fprintf(os.Stderr, "Error: Periodic synchronization cycle failed: %v\n", err)
			}
		}
	}
}

// runSingleSync はタイムアウト付きのコンテキストで1回分の同期処理を実行します。
func runSingleSync(syncUseCase usecase.SyncUserUseCase) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return syncUseCase.Execute(ctx)
}
