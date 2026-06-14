package usecase

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"ldap-es-syncer/internal/domain/model"
)

// mockSourceRepository は SourceRepository のテストモックです。
type mockSourceRepository struct {
	users []*model.User
	err   error
}

func (m *mockSourceRepository) FetchUsers(ctx context.Context) ([]*model.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.users, nil
}

// mockTargetRepository は TargetRepository のテストモックです。
type mockTargetRepository struct {
	savedUsers []*model.User
	err        error
}

func (m *mockTargetRepository) SaveUser(ctx context.Context, user *model.User) error {
	if m.err != nil {
		return m.err
	}
	m.savedUsers = append(m.savedUsers, user)
	return nil
}

func TestSyncUserUseCase_Execute_Success(t *testing.T) {
	testUsers := []*model.User{
		model.NewUser("101", "alice", "alice@example.com", "pass123"),
		model.NewUser("102", "bob", "bob@example.com", "pass456"),
	}

	source := &mockSourceRepository{users: testUsers}
	target := &mockTargetRepository{}

	// slog の出力を一時的にキャプチャして検証
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	u := NewSyncUserUseCase(source, target)

	err := u.Execute(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// 1. 同期先への保存件数の確認
	if len(target.savedUsers) != 2 {
		t.Errorf("expected 2 users to be saved, got %d", len(target.savedUsers))
	}

	// 2. ログ集約出力の確認（JSON構造の中に統計値が含まれること）
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, `"processed_count":2`) || !strings.Contains(logOutput, `"total_users":2`) {
		t.Errorf("expected log output to contain structured stats processed_count=2 and total_users=2, but got %q", logOutput)
	}

	// 3. ループ内での個別成功ログが出力されていないことの検証
	if strings.Contains(logOutput, "Successfully synchronized user") {
		t.Error("unexpected individual user log found in output")
	}
}

func TestSyncUserUseCase_Execute_SourceError(t *testing.T) {
	expectedErr := errors.New("source connection error")
	source := &mockSourceRepository{err: expectedErr}
	target := &mockTargetRepository{}

	u := NewSyncUserUseCase(source, target)

	err := u.Execute(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected wrapped error to contain source error, got %v", err)
	}
}

func TestSyncUserUseCase_Execute_TargetError(t *testing.T) {
	testUsers := []*model.User{
		model.NewUser("101", "alice", "alice@example.com", "pass123"),
		model.NewUser("102", "bob", "bob@example.com", "pass456"),
	}
	expectedErr := errors.New("target save error")
	source := &mockSourceRepository{users: testUsers}
	target := &mockTargetRepository{err: expectedErr}

	u := NewSyncUserUseCase(source, target)

	err := u.Execute(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected wrapped error to contain target error, got %v", err)
	}
}
