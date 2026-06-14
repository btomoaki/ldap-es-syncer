package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"ldap-es-syncer/internal/domain/model"
	"ldap-es-syncer/internal/domain/repository"
	"ldap-es-syncer/internal/infrastructure/config"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

// EsUserRepository は TargetRepository インターフェースの Elasticsearch による具象実装です。
type EsUserRepository struct {
	client *elasticsearch.Client
	index  string
}

// NewEsUserRepository は EsUserRepository のコンストラクタです。
// 設定全体ではなく、必要な TargetConfig のみを限定注入（Config Injection）します。
func NewEsUserRepository(cfg *config.TargetConfig) (repository.TargetRepository, error) {
	esCfg := elasticsearch.Config{
		Addresses: cfg.Addresses,
		Username:  cfg.Username,
		Password:  cfg.Password,
	}

	client, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create elasticsearch client: %w", err)
	}

	repo := &EsUserRepository{
		client: client,
		index:  cfg.IndexName,
	}

	// アプリ起動時のインデックス自動作成とマッピング定義の初期化
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := repo.initializeIndex(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize elasticsearch index mappings: %w", err)
	}

	return repo, nil
}

// SaveUser はユーザー情報を Elasticsearch に対して Upsert (Index API) します。
func (r *EsUserRepository) SaveUser(ctx context.Context, user *model.User) error {
	// JSON シリアライズ
	data, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user to json: %w", err)
	}

	// Index Requestの作成（ID指定でUpsert動作）
	req := esapi.IndexRequest{
		Index:      r.index,
		DocumentID: user.ID,
		Body:       bytes.NewReader(data),
		Refresh:    "true", // 即座に反映させるためリフレッシュを有効化
	}

	// 実行
	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("elasticsearch index request failed: %w", err)
	}
	defer res.Body.Close()

	// レスポンスステータスの検証
	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("elasticsearch index request failed: status=%s, body=%s", res.Status(), string(body))
	}

	return nil
}

// initializeIndex はインデックスの存在を確認し、存在しない場合は明示的なマッピングを指定して作成します。
func (r *EsUserRepository) initializeIndex(ctx context.Context) error {
	// 1. インデックスの存在確認
	existsReq := esapi.IndicesExistsRequest{
		Index: []string{r.index},
	}
	res, err := existsReq.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("failed to check if index %q exists: %w", r.index, err)
	}
	defer res.Body.Close()

	if res.StatusCode == 200 {
		return nil // 既に存在するため初期化不要
	}

	if res.StatusCode != 404 {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to check index existence: status=%s, body=%s", res.Status(), string(body))
	}

	// 2. マッピングを定義してインデックスを作成
	mapping := `{
		"mappings": {
			"properties": {
				"ID": { "type": "keyword" },
				"Username": { "type": "keyword" },
				"Email": { "type": "keyword" },
				"IsActive": { "type": "boolean" },
				"UpdatedAt": { "type": "date" }
			}
		}
	}`

	createReq := esapi.IndicesCreateRequest{
		Index: r.index,
		Body:  strings.NewReader(mapping),
	}

	createRes, err := createReq.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("failed to create index %q: %w", r.index, err)
	}
	defer createRes.Body.Close()

	if createRes.IsError() {
		body, _ := io.ReadAll(createRes.Body)
		return fmt.Errorf("failed to create index %q: status=%s, body=%s", r.index, createRes.Status(), string(body))
	}

	return nil
}

// GetAllUserIDs は、Elasticsearchに登録されている全ユーザーのIDリストを取得します。
func (r *EsUserRepository) GetAllUserIDs(ctx context.Context) ([]string, error) {
	var buf bytes.Buffer
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
		},
		"_source": []string{"ID"},
		"size":    10000,
	}
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return nil, fmt.Errorf("failed to encode query: %w", err)
	}

	res, err := r.client.Search(
		r.client.Search.WithContext(ctx),
		r.client.Search.WithIndex(r.index),
		r.client.Search.WithBody(&buf),
		r.client.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch search request failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("elasticsearch search failed: status=%s, body=%s", res.Status(), string(body))
	}

	var searchRes struct {
		Hits struct {
			Hits []struct {
				ID string `json:"_id"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&searchRes); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	ids := make([]string, len(searchRes.Hits.Hits))
	for i, hit := range searchRes.Hits.Hits {
		ids[i] = hit.ID
	}

	return ids, nil
}

// GetUser は、指定されたIDのユーザー情報を Elasticsearch から取得します。
func (r *EsUserRepository) GetUser(ctx context.Context, id string) (*model.User, error) {
	res, err := r.client.Get(
		r.index,
		id,
		r.client.Get.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch get request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil, fmt.Errorf("user not found: %s", id)
	}

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("elasticsearch get request failed: status=%s, body=%s", res.Status(), string(body))
	}

	var hit struct {
		Source model.User `json:"_source"`
	}
	if err := json.NewDecoder(res.Body).Decode(&hit); err != nil {
		return nil, fmt.Errorf("failed to decode user: %w", err)
	}

	return &hit.Source, nil
}

