package config

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
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
	os.Setenv("DAEMON_MODE", "false")
	os.Setenv("SYNC_INTERVAL", "30s")
	os.Setenv("ES_INDEX_NAME", "custom-users")
	os.Setenv("ES_USERNAME", "elastic")
	os.Setenv("ELASTIC_PASSWORD", "changeme")

	// LDAP動的クエリ・マッピング設定
	os.Setenv("LDAP_ACTIVE_USER", "(userPassword=*)")
	os.Setenv("LDAP_FILTER", "(&(objectClass=inetOrgPerson)({LDAP_ACTIVE_USER}))")
	os.Setenv("LDAP_MAP_UID", "uid")
	os.Setenv("LDAP_MAP_USERNAME", "cn")
	os.Setenv("LDAP_MAP_EMAIL", "mail")
	os.Setenv("ES_EXCLUDED_USERS", "elastic,kibana_system,test_user")
	os.Setenv("LDAP_SKIP_VERIFY", "true")

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
	if cfg.App.DaemonMode != false {
		t.Errorf("expected App.DaemonMode to be false, got %v", cfg.App.DaemonMode)
	}
	if cfg.App.SyncInterval != 30*time.Second {
		t.Errorf("expected App.SyncInterval to be 30s, got %v", cfg.App.SyncInterval)
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
	if cfg.Source.ActiveUser != "(userPassword=*)" {
		t.Errorf("expected Source.ActiveUser to be '(userPassword=*)', got %q", cfg.Source.ActiveUser)
	}
	if cfg.Source.Filter != "(&(objectClass=inetOrgPerson)({LDAP_ACTIVE_USER}))" {
		t.Errorf("expected Source.Filter to be '(&(objectClass=inetOrgPerson)({LDAP_ACTIVE_USER}))', got %q", cfg.Source.Filter)
	}
	expectedFinalFilter := "(&(objectClass=inetOrgPerson)((userPassword=*)))"
	if cfg.Source.FinalFilter != expectedFinalFilter {
		t.Errorf("expected Source.FinalFilter to be %q, got %q", expectedFinalFilter, cfg.Source.FinalFilter)
	}
	if cfg.Source.MapUID != "uid" {
		t.Errorf("expected Source.MapUID to be 'uid', got %q", cfg.Source.MapUID)
	}
	if cfg.Source.MapUsername != "cn" {
		t.Errorf("expected Source.MapUsername to be 'cn', got %q", cfg.Source.MapUsername)
	}
	if cfg.Source.MapEmail != "mail" {
		t.Errorf("expected Source.MapEmail to be 'mail', got %q", cfg.Source.MapEmail)
	}
	if cfg.Source.SkipVerify != true {
		t.Errorf("expected Source.SkipVerify to be true, got %v", cfg.Source.SkipVerify)
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
	expectedExcluded := []string{"elastic", "kibana_system", "test_user"}
	if !reflect.DeepEqual(cfg.Target.ExcludedUsers, expectedExcluded) {
		t.Errorf("expected Target.ExcludedUsers to be %v, got %v", expectedExcluded, cfg.Target.ExcludedUsers)
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

	if cfg.App.DaemonMode != true {
		t.Errorf("expected default App.DaemonMode to be true, got %v", cfg.App.DaemonMode)
	}
	if cfg.App.SyncInterval != 1*time.Hour {
		t.Errorf("expected default App.SyncInterval to be 1h, got %v", cfg.App.SyncInterval)
	}
	if cfg.Source.SkipVerify != false {
		t.Errorf("expected default Source.SkipVerify to be false, got %v", cfg.Source.SkipVerify)
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

func TestNewConfig_InvalidSkipVerify(t *testing.T) {
	os.Clearenv()
	os.Setenv("LDAP_URL", "ldap://localhost")
	os.Setenv("LDAP_BIND_DN", "cn=admin")
	os.Setenv("LDAP_PASSWORD", "pass")
	os.Setenv("LDAP_BASE_DN", "dc=example")
	os.Setenv("ES_ADDRESSES", "http://localhost")
	os.Setenv("LDAP_SKIP_VERIFY", "invalid_bool")

	_, err := NewConfig()
	if err == nil {
		t.Fatal("expected error due to invalid LDAP_SKIP_VERIFY, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse LDAP_SKIP_VERIFY") {
		t.Errorf("expected error message to contain 'failed to parse LDAP_SKIP_VERIFY', got %q", err.Error())
	}
}
