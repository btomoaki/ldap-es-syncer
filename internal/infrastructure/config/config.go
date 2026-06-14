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
	Env          string        // e.g., "development", "production"
	LogLevel     string        // e.g., "info", "debug", "error"
	DaemonMode   bool          // e.g., true, false
	SyncInterval time.Duration // e.g., 1 * time.Hour
}

// SourceConfig は同期元LDAPサーバーの接続設定です。
type SourceConfig struct {
	URL      string // e.g., "ldap://localhost:389"
	BindDN   string // e.g., "cn=admin,dc=example,dc=org"
	Password string // 環境変数から動的に注入
	BaseDN   string // e.g., "dc=example,dc=org"
}

// TargetConfig は同期先Elasticsearchの接続設定です。
type TargetConfig struct {
	Addresses []string // e.g., []string{"http://localhost:9200"}
	Username  string   // ローカルでセキュリティ無効化時は空でも可
	Password  string   // ローカルでセキュリティ無効化時は空でも可
	IndexName string   // e.g., "users"
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
	daemonModeStr := getEnv("DAEMON_MODE", "true")
	daemonMode, err := strconv.ParseBool(daemonModeStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DAEMON_MODE: %w", err)
	}

	intervalStr := getEnv("SYNC_INTERVAL", "1h")
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SYNC_INTERVAL: %w", err)
	}

	return &AppConfig{
		Env:          getEnv("APP_ENV", "development"),
		LogLevel:     getEnv("LOG_LEVEL", "info"),
		DaemonMode:   daemonMode,
		SyncInterval: interval,
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

	return &SourceConfig{
		URL:      url,
		BindDN:   bindDN,
		Password: password,
		BaseDN:   baseDN,
	}, nil
}

func loadTargetConfig() (*TargetConfig, error) {
	addressesStr := os.Getenv("ES_ADDRESSES")
	if addressesStr == "" {
		return nil, fmt.Errorf("required environment variable ES_ADDRESSES is missing")
	}
	addresses := strings.Split(addressesStr, ",")

	indexName := getEnv("ES_INDEX_NAME", "users")

	return &TargetConfig{
		Addresses: addresses,
		Username:  os.Getenv("ES_USERNAME"), // セキュリティ無効化時は空許容
		Password:  os.Getenv("ELASTIC_PASSWORD"), // セキュリティ無効化時は空許容
		IndexName: indexName,
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
