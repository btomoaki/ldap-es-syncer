package config

import (
	"os"
	"reflect"
	"testing"
)

func TestNewConfig_MissingRequired(t *testing.T) {
	// 環境変数をクリアして必須項目の欠損を再現
	os.Clearenv()

	_, err := NewConfig()
	if err == nil {
		t.Fatal("expected error due to missing environment variables, but got nil")
	}
}

func TestNewConfig_Success(t *testing.T) {
	os.Clearenv()

	// 必要な環境変数を設定
	os.Setenv("LDAP_URL", "ldap://localhost:389")
	os.Setenv("LDAP_BIND_DN", "cn=admin,dc=example,dc=org")
	os.Setenv("LDAP_PASSWORD", "admin")
	os.Setenv("LDAP_BASE_DN", "dc=example,dc=org")
	os.Setenv("ES_ADDRESSES", "http://localhost:9200")

	// オプション・デフォルト上書き設定
	os.Setenv("APP_ENV", "production")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("ES_INDEX_NAME", "custom-users")
	os.Setenv("ES_USERNAME", "elastic")
	os.Setenv("ES_PASSWORD", "changeme")

	cfg, err := NewConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// AppConfig 検証
	if cfg.App.Env != "production" {
		t.Errorf("expected App.Env to be 'production', got %q", cfg.App.Env)
	}
	if cfg.App.LogLevel != "debug" {
		t.Errorf("expected App.LogLevel to be 'debug', got %q", cfg.App.LogLevel)
	}

	// SourceConfig 検証
	if cfg.Source.URL != "ldap://localhost:389" {
		t.Errorf("expected Source.URL to be 'ldap://localhost:389', got %q", cfg.Source.URL)
	}
	if cfg.Source.BindDN != "cn=admin,dc=example,dc=org" {
		t.Errorf("expected Source.BindDN to be 'cn=admin,dc=example,dc=org', got %q", cfg.Source.BindDN)
	}
	if cfg.Source.Password != "admin" {
		t.Errorf("expected Source.Password to be 'admin', got %q", cfg.Source.Password)
	}
	if cfg.Source.BaseDN != "dc=example,dc=org" {
		t.Errorf("expected Source.BaseDN to be 'dc=example,dc=org', got %q", cfg.Source.BaseDN)
	}

	// TargetConfig 検証
	expectedAddresses := []string{"http://localhost:9200"}
	if !reflect.DeepEqual(cfg.Target.Addresses, expectedAddresses) {
		t.Errorf("expected Target.Addresses to be %v, got %v", expectedAddresses, cfg.Target.Addresses)
	}
	if cfg.Target.Username != "elastic" {
		t.Errorf("expected Target.Username to be 'elastic', got %q", cfg.Target.Username)
	}
	if cfg.Target.Password != "changeme" {
		t.Errorf("expected Target.Password to be 'changeme', got %q", cfg.Target.Password)
	}
	if cfg.Target.IndexName != "custom-users" {
		t.Errorf("expected Target.IndexName to be 'custom-users', got %q", cfg.Target.IndexName)
	}
}

func TestConfig_Getters(t *testing.T) {
	os.Clearenv()
	os.Setenv("LDAP_URL", "ldap://localhost")
	os.Setenv("LDAP_BIND_DN", "cn=admin")
	os.Setenv("LDAP_PASSWORD", "pass")
	os.Setenv("LDAP_BASE_DN", "dc=example")
	os.Setenv("ES_ADDRESSES", "http://localhost")

	cfg, err := NewConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GetAppConfig() != cfg.App {
		t.Error("GetAppConfig did not return correct pointer")
	}
	if cfg.GetSourceConfig() != cfg.Source {
		t.Error("GetSourceConfig did not return correct pointer")
	}
	if cfg.GetTargetConfig() != cfg.Target {
		t.Error("GetTargetConfig did not return correct pointer")
	}
}
