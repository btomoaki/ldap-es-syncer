package repository

import (
	"context"
	"ldap-es-syncer/internal/domain/model"
)

// SourceRepository は、同期元のデータストアからユーザー情報をフェッチする役割を持つ Port です。
type SourceRepository interface {
	FetchUsers(ctx context.Context) ([]*model.User, error)
}

// TargetRepository は、同期先のデータストアにユーザー情報を保存（作成または更新）する役割を持つ Port です。
type TargetRepository interface {
	SaveUser(ctx context.Context, user *model.User) error
	GetAllUserIDs(ctx context.Context) ([]string, error)
	GetUser(ctx context.Context, id string) (*model.User, error)
}
