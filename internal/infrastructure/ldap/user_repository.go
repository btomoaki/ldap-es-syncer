package ldap

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"

	"ldap-es-syncer/internal/domain/model"
	"ldap-es-syncer/internal/domain/repository"
	"ldap-es-syncer/internal/infrastructure/config"
	"github.com/go-ldap/ldap/v3"
)

// LdapUserRepository は SourceRepository インターフェースのLDAPによる具象実装です。
type LdapUserRepository struct {
	cfg *config.SourceConfig
}

// NewLdapUserRepository は LdapUserRepository のコンストラクタです。
// 設定全体ではなく、必要な SourceConfig のみを限定注入（Config Injection）します。
func NewLdapUserRepository(cfg *config.SourceConfig) repository.SourceRepository {
	return &LdapUserRepository{
		cfg: cfg,
	}
}

// FetchUsers はLDAPからユーザー一覧を検索してドメインモデル User に変換して返します。
func (r *LdapUserRepository) FetchUsers(ctx context.Context) ([]*model.User, error) {
	// LDAPサーバーに接続
	l, err := ldap.DialURL(
		r.cfg.URL,
		ldap.DialWithTLSConfig(&tls.Config{InsecureSkipVerify: r.cfg.SkipVerify}),
	)
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
	// 設定された属性および finalFilter を使用
	searchRequest := ldap.NewSearchRequest(
		r.cfg.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		r.cfg.FinalFilter,
		[]string{"dn", r.cfg.MapUsername, r.cfg.MapEmail, r.cfg.MapUID, "userPassword", "memberOf"},
		nil,
	)

	// 検索の実行
	sr, err := l.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("ldap search failed: %w", err)
	}

	var users []*model.User
	for _, entry := range sr.Entries {
		uid := entry.GetAttributeValue(r.cfg.MapUID)
		if uid == "" {
			// UID属性が無い場合は Username 属性をIDフォールバックとして使用
			uid = entry.GetAttributeValue(r.cfg.MapUsername)
		}
		cn := entry.GetAttributeValue(r.cfg.MapUsername)
		mail := entry.GetAttributeValue(r.cfg.MapEmail)
		password := entry.GetAttributeValue("userPassword")

		// memberOf属性から所属グループ（ロール候補）を抽出
		var rawGroups []string
		for _, groupDN := range entry.GetAttributeValues("memberOf") {
			groupName := parseGroupName(groupDN)
			if groupName != "" {
				rawGroups = append(rawGroups, groupName)
			}
		}

		// ドメインモデルのコンストラクタを呼び出す
		user := model.NewUser(uid, cn, mail, password)
		user.Roles = rawGroups
		user.PasswordHash = parseLdapPasswordHash(password)
		// LDAP生存者は明示的に有効とみなす
		user.IsActive = true
		users = append(users, user)
	}

	return users, nil
}

// parseGroupName はグループDNからCN値（グループ名）を抽出します。
func parseGroupName(groupDN string) string {
	parsed, err := ldap.ParseDN(groupDN)
	if err != nil {
		return ""
	}
	for _, rdn := range parsed.RDNs {
		for _, attr := range rdn.Attributes {
			if strings.EqualFold(attr.Type, "cn") {
				return attr.Value
			}
		}
	}
	return ""
}

// parseLdapPasswordHash は `{CRYPT}$2a$...` などのハッシュから `{CRYPT}` のようなプレフィックスを除去します。
func parseLdapPasswordHash(raw string) string {
	if strings.HasPrefix(raw, "{") {
		idx := strings.Index(raw, "}")
		if idx != -1 {
			return raw[idx+1:]
		}
	}
	return raw
}
