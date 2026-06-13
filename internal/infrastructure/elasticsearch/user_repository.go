package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"ldap-es-syncer/internal/domain/model"
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
func NewEsUserRepository(cfg *config.TargetConfig) (*EsUserRepository, error) {
	esCfg := elasticsearch.Config{
		Addresses: cfg.Addresses,
		Username:  cfg.Username,
		Password:  cfg.Password,
	}

	client, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create elasticsearch client: %w", err)
	}

	return &EsUserRepository{
		client: client,
		index:  cfg.IndexName,
	}, nil
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
