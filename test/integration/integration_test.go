//go:build integration
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ldap-es-syncer/internal/application/usecase"
	"ldap-es-syncer/internal/domain/model"
	"ldap-es-syncer/internal/infrastructure/config"
	"ldap-es-syncer/internal/infrastructure/elasticsearch"
	"ldap-es-syncer/internal/infrastructure/ldap"
	"ldap-es-syncer/internal/infrastructure/prometheus"
	es8 "github.com/elastic/go-elasticsearch/v8"
)

// loadEnv はプロジェクトルートにある .env を探索してロードします。
func loadEnv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for i := 0; i < 5; i++ {
		envPath := filepath.Join(dir, ".env")
		if _, err := os.Stat(envPath); err == nil {
			data, err := os.ReadFile(envPath)
			if err == nil {
				lines := strings.Split(string(data), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" || strings.HasPrefix(line, "#") {
						continue
					}
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						key := strings.TrimSpace(parts[0])
						val := strings.TrimSpace(parts[1])
						val = strings.Trim(val, `"'`)
						if os.Getenv(key) == "" {
							os.Setenv(key, val)
						}
					}
				}
			}
			break
		}
		dir = filepath.Dir(dir)
	}
}

func TestIntegration_SyncPipeline(t *testing.T) {
	loadEnv()

	// 1. Config のロード
	cfg, err := config.NewConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 2. テスト前に Elasticsearch インデックスをクリーンアップ（削除）して状態を決定論的にする
	esCfg := es8.Config{
		Addresses: cfg.Target.Addresses,
		Username:  cfg.Target.Username,
		Password:  cfg.Target.Password,
	}
	rawEsClient, err := es8.NewClient(esCfg)
	if err != nil {
		t.Fatalf("Failed to create raw elasticsearch client: %v", err)
	}
	// インデックス削除（存在する場合のみ）
	_, err = rawEsClient.Indices.Delete([]string{cfg.Target.IndexName}, rawEsClient.Indices.Delete.WithContext(ctx))
	if err != nil {
		t.Logf("Note: indices delete request returned error/warning (normal if index didn't exist): %v", err)
	}

	// 3. 具象リポジトリの初期化
	sourceRepo := ldap.NewLdapUserRepository(cfg.GetSourceConfig())
	targetRepo, err := elasticsearch.NewEsUserRepository(cfg.GetTargetConfig())
	if err != nil {
		t.Fatalf("Failed to initialize target repository: %v", err)
	}
	metricsRepo := prometheus.NewNoopMetricsRepository()

	// 4. 初回同期の実行
	syncUseCase := usecase.NewSyncUserUseCase(
		sourceRepo,
		targetRepo,
		metricsRepo,
		cfg.Source.FinalFilter,
		cfg.Target.ExcludedUsers,
		cfg.App.SyncMinUsers,
		cfg.App.DryRun,
		cfg.App.SyncSecurityUsers,
	)

	t.Run("Initial E2E Sync from OpenLDAP to Elasticsearch", func(t *testing.T) {
		err := syncUseCase.Execute(ctx)
		if err != nil {
			t.Fatalf("Sync execution failed: %v", err)
		}

		// LDAP側の期待される生存ユーザー数を取得
		ldapUsers, err := sourceRepo.FetchUsers(ctx)
		if err != nil {
			t.Fatalf("Failed to fetch users from LDAP: %v", err)
		}
		if len(ldapUsers) < 1 {
			t.Fatalf("Expected at least one LDAP user to be synced, got 0")
		}

		// ES側に生存ユーザーが IsActive = true で保存されているか検証
		for _, lu := range ldapUsers {
			eu, err := targetRepo.GetUser(ctx, lu.ID)
			if err != nil {
				t.Errorf("Expected user %q to exist in ES, got error: %v", lu.ID, err)
				continue
			}
			if !eu.IsActive {
				t.Errorf("Expected synced user %q to be active, but IsActive is false", lu.ID)
			}
		}

		// グループ外の solo.player がアクティブとして登録されていないことを検証
		eu, err := targetRepo.GetUser(ctx, "solo.player")
		if err == nil && eu.IsActive {
			t.Errorf("Expected 'solo.player' (outside LDAP group filter) not to be active in ES")
		}
	})

	t.Run("User Logical Deactivation and Built-in System User Protection", func(t *testing.T) {
		// (A) LDAPに存在しないダミーユーザーを手動で ES に登録 (IsActive = true)
		dummyUser := model.NewUser("dummy.integration.user", "Dummy Integration User", "dummy@example.com", "")
		dummyUser.IsActive = true
		if err := targetRepo.SaveUser(ctx, dummyUser); err != nil {
			t.Fatalf("Failed to save dummy user: %v", err)
		}

		// (B) 除外（保護）対象のシステムユーザーを手動で ES に登録 (IsActive = true)
		sysUser := model.NewUser("elastic", "elastic", "elastic@example.com", "")
		sysUser.IsActive = true
		if err := targetRepo.SaveUser(ctx, sysUser); err != nil {
			t.Fatalf("Failed to save system user: %v", err)
		}

		// 同期パイプラインを実行してリコンシリエーションを起動
		err := syncUseCase.Execute(ctx)
		if err != nil {
			t.Fatalf("Sync execution failed: %v", err)
		}

		// 検証: ダミーユーザーは論理削除 (IsActive = false) されていること
		euDummy, err := targetRepo.GetUser(ctx, "dummy.integration.user")
		if err != nil {
			t.Fatalf("Failed to get dummy user from ES: %v", err)
		}
		if euDummy.IsActive {
			t.Errorf("Expected dummy user to be logically deactivated (IsActive = false), but got active")
		}

		// 検証: システムユーザー (elastic) は保護され、アクティブ (IsActive = true) を維持していること
		euSys, err := targetRepo.GetUser(ctx, "elastic")
		if err != nil {
			t.Fatalf("Failed to get system user 'elastic' from ES: %v", err)
		}
		if !euSys.IsActive {
			t.Errorf("Expected system user 'elastic' to remain active (IsActive = true) after sync, but got deactivated")
		}
	})

	t.Run("Verify Elasticsearch Native Built-in Reserved Metadata", func(t *testing.T) {
		// Elasticsearchの組み込みユーザーセキュリティAPIエンドポイントを呼び出し
		// ローカル開発用に xpack.security.enabled=false の場合は 405 Method Not Allowed もしくは 404/400 が返却されるため、その場合はスキップします。
		url := fmt.Sprintf("%s/_security/user/elastic", cfg.Target.Addresses[0])
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		if cfg.Target.Username != "" && cfg.Target.Password != "" {
			req.SetBasicAuth(cfg.Target.Username, cfg.Target.Password)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Warning: Failed to reach security API (likely normal if cluster is local/custom): %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
			t.Logf("Elasticsearch Security is disabled (Status Code %d). Skipping native built-in metadata assertion.", resp.StatusCode)
			return
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Logf("Unusual response from Security API: status=%d, body=%s. Skipping native built-in metadata assertion.", resp.StatusCode, string(body))
			return
		}

		// 200 OK の場合、レスポンスの JSON をデコードして metadata._reserved: true を検証
		var secResp map[string]struct {
			Metadata map[string]interface{} `json:"metadata"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&secResp); err != nil {
			t.Fatalf("Failed to decode security API response: %v", err)
		}

		elasticUser, exists := secResp["elastic"]
		if !exists {
			t.Fatalf("User 'elastic' not returned in security API response")
		}

		reserved, ok := elasticUser.Metadata["_reserved"].(bool)
		if !ok || !reserved {
			t.Errorf("Expected metadata._reserved to be true for built-in user 'elastic', got metadata: %v", elasticUser.Metadata)
		} else {
			t.Logf("Verified built-in user 'elastic' contains '_reserved: true' metadata flag successfully.")
		}
	})

	t.Run("Security User Sync and Login Authentication Verification", func(t *testing.T) {
		// Native ユーザー同期を強制有効化したユースケースを作成
		secSyncUseCase := usecase.NewSyncUserUseCase(
			sourceRepo,
			targetRepo,
			metricsRepo,
			cfg.Source.FinalFilter,
			cfg.Target.ExcludedUsers,
			cfg.App.SyncMinUsers,
			false, // dryRun = false
			true,  // syncSecurityUsers = true
		)

		// 同期を実行
		err := secSyncUseCase.Execute(ctx)
		if err != nil {
			t.Fatalf("Sync execution for security users failed: %v", err)
		}

		// 同期された CRYPT ハッシュ（$2a$, $2b$）のアカウントで実際にログイン可能か検証
		testCreds := []struct {
			username      string
			password      string
			expectSuccess bool
		}{
			{"user.crypt2a", "usr-crypt-pass1", true},
			{"user.crypt2b", "usr-crypt-pass2", true},
			{"user.ssha", "usr-ssha-pass3", false}, // SSHAはES未サポートのため、ログイン失敗(401)するはず
			{"user.sha", "usr-sha-pass4", false},  // SHAもES未サポートのため、ログイン失敗(401)するはず
		}

		for _, cred := range testCreds {
			url := fmt.Sprintf("%s/_security/_authenticate", cfg.Target.Addresses[0])
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				t.Fatalf("Failed to create authenticate request for %s: %v", cred.username, err)
			}
			req.SetBasicAuth(cred.username, cred.password)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Errorf("Failed to reach authenticate API for user %s: %v", cred.username, err)
				continue
			}
			defer resp.Body.Close()

			// xpack securityが無効な場合は 405 Method Not Allowed 等になるため検証をスキップ
			if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusBadRequest {
				t.Logf("Elasticsearch Security is disabled. Skipping authenticate check for %s.", cred.username)
				continue
			}

			if cred.expectSuccess {
				if resp.StatusCode != http.StatusOK {
					body, _ := io.ReadAll(resp.Body)
					t.Errorf("Expected authentication success for user %s, but failed: status=%d, body=%s", cred.username, resp.StatusCode, string(body))
					continue
				}

				// レスポンスのusernameが一致することを確認
				var authResp struct {
					Username string `json:"username"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
					t.Errorf("Failed to decode authenticate response for user %s: %v", cred.username, err)
					continue
				}

				if authResp.Username != cred.username {
					t.Errorf("Expected authenticated username %q, got %q", cred.username, authResp.Username)
				} else {
					t.Logf("Successfully authenticated security user %q synced with password hash!", cred.username)
				}
			} else {
				// ログイン失敗（401 Unauthorized）を期待する
				if resp.StatusCode != http.StatusUnauthorized {
					t.Errorf("Expected authentication failure (401) for user %s, but got status=%d", cred.username, resp.StatusCode)
				} else {
					t.Logf("Successfully verified user %q cannot authenticate (expected status 401)", cred.username)
				}
			}
		}
	})
}
