package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config はアプリケーション設定全体の親構造体です。
type Config struct {
	App    *AppConfig
	Source *SourceConfig
	Target *TargetConfig
}

// AppConfig はアプリケーションの基本動作（動作環境、ログレベル）の設定です。
type AppConfig struct {
	Env                   string        // e.g., "development", "production"
	LogLevel              string        // e.g., "info", "debug", "error"
	DaemonMode            bool          // e.g., true, false
	SyncInterval          time.Duration // e.g., 1 * time.Hour
	SyncMinUsers          int           // セーフティガード：同期を安全に実行するための最小ユーザー数
	MetricsEnabled        bool          // Prometheusメトリクス収集を有効にするか
	MetricsPort           string        // Prometheusメトリクスサーバーの公開ポート (e.g., "8080")
	MetricsPushgatewayURL string        // Prometheus Pushgatewayのプッシュ先URL (e.g., "http://localhost:9091")
	DryRun                bool          // 実際の書き込みをスキップするDry-Runモード
	SyncSecurityUsers     bool          // Kibana/ES Nativeユーザーアカウントの同期を有効にするか
}

// SourceConfig は同期元LDAPサーバーの接続設定です。
type SourceConfig struct {
	URL         string // e.g., "ldap://localhost:389"
	BindDN      string // e.g., "cn=admin,dc=example,dc=org"
	Password    string // 環境変数から動的に注入
	BaseDN      string // e.g., "dc=example,dc=org"
	ActiveUser  string // e.g., "(userPassword=*)"
	Filter      string // e.g., "(&(objectClass=inetOrgPerson)({LDAP_ACTIVE_USER}))"
	FinalFilter string // LDAP_FILTER with {LDAP_ACTIVE_USER} replaced
	MapUID      string // e.g., "uid"
	MapUsername string // e.g., "cn"
	MapEmail    string // e.g., "mail"
	SkipVerify  bool   // TLS証明書の検証をスキップするか
}

// TargetConfig は同期先Elasticsearchの接続設定です。
type TargetConfig struct {
	Addresses     []string // e.g., []string{"http://localhost:9200"}
	Username      string   // ローカルでセキュリティ無効化時は空でも可
	Password      string   // ローカルでセキュリティ無効化時は空でも可
	IndexName     string   // e.g., "users"
	ExcludedUsers []string // e.g., ["elastic", "kibana_system"]
	MaxResults    int      // Elasticsearchからの最大取得件数（上限）
}

// NewConfig はOS of the systemの環境変数から設定をロードし、Config構造体を返します。
// 必須設定が存在しない場合はエラーを返します。
func NewConfig() (*Config, error) {
	appConfig, err := loadAppConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load app config: %w", err)
	}

	sourceConfig, err := loadSourceConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load source config: %w", err)
	}

	targetConfig, err := loadTargetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load target config: %w", err)
	}

	return &Config{
		App:    appConfig,
		Source: sourceConfig,
		Target: targetConfig,
	}, nil
}

// GetAppConfig はDIコンテナへ AppConfig を個別に注入するためのプロバイダーです。
func (c *Config) GetAppConfig() *AppConfig {
	return c.App
}

// GetSourceConfig はDIコンテナへ SourceConfig を個別に注入するためのプロバイダーです。
func (c *Config) GetSourceConfig() *SourceConfig {
	return c.Source
}

// GetTargetConfig はDIコンテナへ TargetConfig を個別に注入するためのプロバイダーです。
func (c *Config) GetTargetConfig() *TargetConfig {
	return c.Target
}

// -- 各設定セグメントのロードヘルパー関数 --

func loadAppConfig() (*AppConfig, error) {
	daemonModeStr := getEnv("SYNC_DAEMON_MODE", "true")
	daemonMode, err := strconv.ParseBool(daemonModeStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SYNC_DAEMON_MODE: %w", err)
	}

	intervalStr := getEnv("SYNC_INTERVAL", "1h")
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SYNC_INTERVAL: %w", err)
	}

	minUsersStr := getEnv("SYNC_MIN_USERS", "1")
	minUsers, err := strconv.Atoi(minUsersStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SYNC_MIN_USERS: %w", err)
	}

	metricsEnabledStr := getEnv("METRICS_ENABLED", "false")
	metricsEnabled, err := strconv.ParseBool(metricsEnabledStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse METRICS_ENABLED: %w", err)
	}

	dryRunStr := getEnv("SYNC_DRY_RUN", "false")
	dryRun, err := strconv.ParseBool(dryRunStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SYNC_DRY_RUN: %w", err)
	}

	syncSecUsersStr := getEnv("SYNC_SECURITY_USERS", "false")
	syncSecUsers, err := strconv.ParseBool(syncSecUsersStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SYNC_SECURITY_USERS: %w", err)
	}

	return &AppConfig{
		Env:                   getEnv("APP_ENV", "development"),
		LogLevel:              getEnv("APP_LOG_LEVEL", "info"),
		DaemonMode:            daemonMode,
		SyncInterval:          interval,
		SyncMinUsers:          minUsers,
		MetricsEnabled:        metricsEnabled,
		MetricsPort:           getEnv("METRICS_PORT", "8080"),
		MetricsPushgatewayURL: getEnv("METRICS_PUSHGATEWAY_URL", ""),
		DryRun:                dryRun,
		SyncSecurityUsers:     syncSecUsers,
	}, nil
}

func loadSourceConfig() (*SourceConfig, error) {
	url := os.Getenv("LDAP_URL")
	if url == "" {
		return nil, fmt.Errorf("required environment variable LDAP_URL is missing")
	}

	bindDN := os.Getenv("LDAP_BIND_DN")
	if bindDN == "" {
		return nil, fmt.Errorf("required environment variable LDAP_BIND_DN is missing")
	}

	password := os.Getenv("LDAP_PASSWORD")
	if password == "" {
		return nil, fmt.Errorf("required environment variable LDAP_PASSWORD is missing")
	}

	baseDN := os.Getenv("LDAP_BASE_DN")
	if baseDN == "" {
		return nil, fmt.Errorf("required environment variable LDAP_BASE_DN is missing")
	}

	activeUser := getEnv("LDAP_ACTIVE_USER", "(userPassword=*)")
	filter := getEnv("LDAP_FILTER", "(&(objectClass=inetOrgPerson)({LDAP_ACTIVE_USER}))")
	finalFilter := strings.ReplaceAll(filter, "{LDAP_ACTIVE_USER}", activeUser)

	skipVerifyStr := getEnv("LDAP_SKIP_VERIFY", "false")
	skipVerify, err := strconv.ParseBool(skipVerifyStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LDAP_SKIP_VERIFY: %w", err)
	}

	return &SourceConfig{
		URL:         url,
		BindDN:      bindDN,
		Password:    password,
		BaseDN:      baseDN,
		ActiveUser:  activeUser,
		Filter:      filter,
		FinalFilter: finalFilter,
		MapUID:      getEnv("LDAP_MAP_UID", "uid"),
		MapUsername: getEnv("LDAP_MAP_USERNAME", "cn"),
		MapEmail:    getEnv("LDAP_MAP_EMAIL", "mail"),
		SkipVerify:  skipVerify,
	}, nil
}

func loadTargetConfig() (*TargetConfig, error) {
	addressesStr := os.Getenv("ES_ADDRESSES")
	if addressesStr == "" {
		return nil, fmt.Errorf("required environment variable ES_ADDRESSES is missing")
	}
	addresses := strings.Split(addressesStr, ",")

	indexName := getEnv("ES_INDEX_NAME", "users")

	excludedUsersStr := getEnv("ES_EXCLUDED_USERS", "elastic,kibana_system")
	var excludedUsers []string
	if excludedUsersStr != "" {
		for _, u := range strings.Split(excludedUsersStr, ",") {
			trimmed := strings.TrimSpace(u)
			if trimmed != "" {
				excludedUsers = append(excludedUsers, trimmed)
			}
		}
	}

	maxResultsStr := getEnv("ES_MAX_RESULTS", "1000")
	maxResults, err := strconv.Atoi(maxResultsStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ES_MAX_RESULTS: %w", err)
	}

	return &TargetConfig{
		Addresses:     addresses,
		Username:      os.Getenv("ES_USERNAME"), // セキュリティ無効化時は空許容
		Password:      os.Getenv("ELASTIC_PASSWORD"), // セキュリティ無効化時は空許容
		IndexName:     indexName,
		ExcludedUsers: excludedUsers,
		MaxResults:    maxResults,
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
