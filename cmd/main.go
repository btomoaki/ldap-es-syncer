package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ldap-es-syncer/internal/application/usecase"
	"ldap-es-syncer/internal/di"
	"ldap-es-syncer/internal/infrastructure/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
		err := runSingleSync(ctx, syncUseCase)

		// バッチモード時の Pushgateway へのプッシュ
		if appConfig.MetricsEnabled && appConfig.MetricsPushgatewayURL != "" {
			promRepo, ok := container.GetMetricsRepository().(*prometheus.PrometheusMetricsRepository)
			if ok {
				slog.Info("Pushing metrics to Prometheus Pushgateway", slog.String("url", appConfig.MetricsPushgatewayURL))
				if pErr := promRepo.PushToPushgateway(appConfig.MetricsPushgatewayURL, "ldap_es_syncer"); pErr != nil {
					slog.Error("Failed to push metrics to Pushgateway", slog.Any("error", pErr))
				} else {
					slog.Info("Successfully pushed metrics to Pushgateway")
				}
			}
		}

		if err != nil {
			slog.Error("User synchronization failed", slog.Any("error", err))
			os.Exit(1)
		}
		slog.Info("User synchronization process completed")
		return
	}

	// デーモン（常駐）モードの処理
	slog.Info("User synchronization daemon starting")

	// メトリクスサーバーの起動
	var metricsSrv *http.Server
	if appConfig.MetricsEnabled {
		promRepo, ok := container.GetMetricsRepository().(*prometheus.PrometheusMetricsRepository)
		if ok {
			mux := http.NewServeMux()
			mux.Handle("/metrics", promhttp.HandlerFor(promRepo.GetRegistry(), promhttp.HandlerOpts{}))
			metricsSrv = &http.Server{
				Addr:    ":" + appConfig.MetricsPort,
				Handler: mux,
			}
			go func() {
				slog.Info("Starting Prometheus metrics server", slog.String("port", appConfig.MetricsPort))
				if srvErr := metricsSrv.ListenAndServe(); srvErr != nil && !errors.Is(srvErr, http.ErrServerClosed) {
					slog.Error("Metrics server failed", slog.Any("error", srvErr))
				}
			}()
		}
	}

	// 初回同期実行
	slog.Info("Running initial synchronization")
	if err := runSingleSync(ctx, syncUseCase); err != nil {
		slog.Error("Initial synchronization failed", slog.Any("error", err))
	}

	ticker := time.NewTicker(appConfig.SyncInterval)
	defer ticker.Stop()

	slog.Info("User synchronization daemon started", slog.Duration("interval", appConfig.SyncInterval))

	// デーモン実行ループ
	for {
		select {
		case <-ctx.Done():
			slog.Info("Received signal, shutting down daemon gracefully")
			// メトリクスサーバーのシャットダウン
			if metricsSrv != nil {
				slog.Info("Shutting down Prometheus metrics server")
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
				if srvErr := metricsSrv.Shutdown(shutdownCtx); srvErr != nil {
					slog.Error("Metrics server shutdown failed", slog.Any("error", srvErr))
				}
				shutdownCancel()
			}
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
