package model

import (
	"time"
)

// User は同期対象のユーザーを表すドメインエンティティです。
// 特定のインフラ技術（LDAP属性名やESフィールド名）からは完全に抽象化されています。
type User struct {
	ID           string
	Username     string
	Email        string
	IsActive     bool
	UpdatedAt    time.Time
	Roles        []string
	PasswordHash string
}

// NewUser はUser構造体を生成するコンストラクタ（ファクトリ関数）です。
// パスワードが提供されていない場合は、セキュリティ上非アクティブ状態でユーザーを初期化します。
func NewUser(id, username, email, password string) *User {
	isActive := true
	if password == "" {
		isActive = false
	}

	return &User{
		ID:        id,
		Username:  username,
		Email:     email,
		IsActive:  isActive,
		UpdatedAt: time.Now(),
		Roles:     []string{},
	}
}

// Deactivate はユーザーを非アクティブ状態に設定し、更新日時を現在時刻に更新します。
func (u *User) Deactivate() {
	u.IsActive = false
	u.UpdatedAt = time.Now()
}
