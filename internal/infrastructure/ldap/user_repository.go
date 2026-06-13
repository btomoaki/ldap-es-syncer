package ldap

import (
	"context"
	"fmt"

	"ldap-es-syncer/internal/domain/model"
	"ldap-es-syncer/internal/infrastructure/config"
	"github.com/go-ldap/ldap/v3"
)

// LdapUserRepository は SourceRepository インターフェースのLDAPによる具象実装です。
type LdapUserRepository struct {
	cfg *config.SourceConfig
}

// NewLdapUserRepository は LdapUserRepository のコンストラクタです。
// 設定全体ではなく、必要な SourceConfig のみを限定注入（Config Injection）します。
func NewLdapUserRepository(cfg *config.SourceConfig) *LdapUserRepository {
	return &LdapUserRepository{
		cfg: cfg,
	}
}

// FetchUsers はLDAPからユーザー一覧を検索してドメインモデル User に変換して返します。
func (r *LdapUserRepository) FetchUsers(ctx context.Context) ([]*model.User, error) {
	// LDAPサーバーに接続
	l, err := ldap.DialURL(r.cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("ldap dial failed: %w", err)
	}
	defer l.Close()

	// 管理者DNでBind認証
	err = l.Bind(r.cfg.BindDN, r.cfg.Password)
	if err != nil {
		return nil, fmt.Errorf("ldap bind failed: %w", err)
	}

	// ユーザー検索リクエストの構築
	// uid, cn, mail, userPassword 属性を取得
	searchRequest := ldap.NewSearchRequest(
		r.cfg.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=person)",
		[]string{"dn", "cn", "mail", "uid", "userPassword"},
		nil,
	)

	// 検索の実行
	sr, err := l.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("ldap search failed: %w", err)
	}

	var users []*model.User
	for _, entry := range sr.Entries {
		uid := entry.GetAttributeValue("uid")
		if uid == "" {
			// uid属性が無い場合は cn をIDフォールバックとして使用
			uid = entry.GetAttributeValue("cn")
		}
		cn := entry.GetAttributeValue("cn")
		mail := entry.GetAttributeValue("mail")
		password := entry.GetAttributeValue("userPassword")

		// ドメインモデルのコンストラクタを呼び出す（内部でパスワード検証・状態設定）
		users = append(users, model.NewUser(uid, cn, mail, password))
	}

	return users, nil
}
