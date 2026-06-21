package di

import (
	"fmt"
	"log/slog"

	"ldap-es-syncer/internal/application/usecase"
	"ldap-es-syncer/internal/domain/repository"
	"ldap-es-syncer/internal/infrastructure/config"
	"ldap-es-syncer/internal/infrastructure/elasticsearch"
	"ldap-es-syncer/internal/infrastructure/ldap"
	"ldap-es-syncer/internal/infrastructure/logging"
	"ldap-es-syncer/internal/infrastructure/prometheus"
)

// Container はアプリケーションの初期化された依存関係を保持する構造体です。
type Container struct {
	cfg         *config.Config
	sourceRepo  repository.SourceRepository
	targetRepo  repository.TargetRepository
	metricsRepo repository.MetricsRepository
	syncUC      usecase.SyncUserUseCase
}

// NewContainer は設定をロードし、依存関係を配線・初期化します。
func NewContainer() (*Container, error) {
	// 1. 設定のロード
	cfg, err := config.NewConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// 2. 構造化ロガー（slog）の初期化と設定
	logHandler := logging.NewSplitHandler()
	slog.SetDefault(slog.New(logHandler))

	// 2.5 メトリクスリポジトリの初期化
	var metricsRepo repository.MetricsRepository
	if cfg.GetAppConfig().MetricsEnabled {
		metricsRepo = prometheus.NewPrometheusMetricsRepository()
		slog.Info("Prometheus metrics enabled")
	} else {
		metricsRepo = prometheus.NewNoopMetricsRepository()
		slog.Info("Prometheus metrics disabled (using Noop metrics)")
	}

	// 3. インフラアダプターの初期化 (Config Injectionの徹底)
	sourceRepo := ldap.NewLdapUserRepository(cfg.GetSourceConfig())

	targetRepo, err := elasticsearch.NewEsUserRepository(cfg.GetTargetConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize elasticsearch repository: %w", err)
	}

	// 4. ユースケースの構築
	syncUC := usecase.NewSyncUserUseCase(
		sourceRepo,
		targetRepo,
		metricsRepo,
		cfg.GetSourceConfig().FinalFilter,
		cfg.GetTargetConfig().ExcludedUsers,
		cfg.GetAppConfig().SyncMinUsers,
		cfg.GetAppConfig().DryRun,
	)

	return &Container{
		cfg:         cfg,
		sourceRepo:  sourceRepo,
		targetRepo:  targetRepo,
		metricsRepo: metricsRepo,
		syncUC:      syncUC,
	}, nil
}

// GetSyncUserUseCase は解決された SyncUserUseCase インスタンスを返します。
func (c *Container) GetSyncUserUseCase() usecase.SyncUserUseCase {
	return c.syncUC
}

// GetAppConfig はロードされた AppConfig インスタンスを返します。
func (c *Container) GetAppConfig() *config.AppConfig {
	return c.cfg.GetAppConfig()
}

// GetMetricsRepository は解決された MetricsRepository インスタンスを返します。
func (c *Container) GetMetricsRepository() repository.MetricsRepository {
	return c.metricsRepo
}
