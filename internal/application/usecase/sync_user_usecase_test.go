package usecase

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

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
	savedUsers    []*model.User
	existingUsers map[string]*model.User
	existingRoles []string
	err           error
	getAllErr     error
	getErr        error
	roleErr       error
}

func (m *mockTargetRepository) SaveUser(ctx context.Context, user *model.User) error {
	if m.err != nil {
		return m.err
	}
	// Upsert: replace if exists, otherwise append
	found := false
	for i, u := range m.savedUsers {
		if u.ID == user.ID {
			m.savedUsers[i] = user
			found = true
			break
		}
	}
	if !found {
		m.savedUsers = append(m.savedUsers, user)
	}
	if m.existingUsers == nil {
		m.existingUsers = make(map[string]*model.User)
	}
	m.existingUsers[user.ID] = user
	return nil
}

func (m *mockTargetRepository) GetAllUserIDs(ctx context.Context) ([]string, error) {
	if m.getAllErr != nil {
		return nil, m.getAllErr
	}
	var ids []string
	for id := range m.existingUsers {
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *mockTargetRepository) GetUser(ctx context.Context, id string) (*model.User, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	user, exists := m.existingUsers[id]
	if !exists {
		return nil, errors.New("user not found")
	}
	// Copy to prevent tests mutating shared pointer references
	copyUser := *user
	return &copyUser, nil
}

func (m *mockTargetRepository) RoleExists(ctx context.Context, role string) (bool, error) {
	if m.roleErr != nil {
		return false, m.roleErr
	}
	for _, r := range m.existingRoles {
		if r == role {
			return true, nil
		}
	}
	return false, nil
}

// mockMetricsRepository は MetricsRepository のテストモックです。
type mockMetricsRepository struct {
	durationRecorded  bool
	processedRecorded bool
	activeRecorded    bool
	statusRecorded    bool
}

func (m *mockMetricsRepository) RecordSyncDuration(duration time.Duration) {
	m.durationRecorded = true
}
func (m *mockMetricsRepository) RecordProcessedUsers(count int) {
	m.processedRecorded = true
}
func (m *mockMetricsRepository) RecordActiveUsers(count int) {
	m.activeRecorded = true
}
func (m *mockMetricsRepository) RecordSyncStatus(success bool) {
	m.statusRecorded = true
}

func TestSyncUserUseCase_Execute_Success(t *testing.T) {
	testUsers := []*model.User{
		model.NewUser("101", "alice", "alice@example.com", "pass123"),
		model.NewUser("102", "bob", "bob@example.com", "pass456"),
	}

	source := &mockSourceRepository{users: testUsers}
	target := &mockTargetRepository{
		existingUsers: make(map[string]*model.User),
	}

	// slog の出力を一時的にキャプチャして検証
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	finalFilter := "(&(objectClass=inetOrgPerson)(userPassword=*))"
	excludedUsers := []string{"elastic", "kibana_system"}

	u := NewSyncUserUseCase(source, target, &mockMetricsRepository{}, finalFilter, excludedUsers, 1, false)

	err := u.Execute(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// 1. 同期先への保存件数の確認
	if len(target.savedUsers) != 2 {
		t.Errorf("expected 2 users to be saved, got %d", len(target.savedUsers))
	}

	// 2. ログ集約出力の確認（JSON構造の中に統計値とFinalFilterが含まれること）
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, `"total_active_users":2`) {
		t.Errorf("expected log output to contain structured stats total_active_users=2, but got %q", logOutput)
	}
	if !strings.Contains(logOutput, `"final_filter":"(&(objectClass=inetOrgPerson)(userPassword=*))"`) {
		t.Errorf("expected log output to contain final_filter, but got %q", logOutput)
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

	u := NewSyncUserUseCase(source, target, &mockMetricsRepository{}, "", nil, 1, false)

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

	u := NewSyncUserUseCase(source, target, &mockMetricsRepository{}, "", nil, 1, false)

	err := u.Execute(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected wrapped error to contain target error, got %v", err)
	}
}

func TestSyncUserUseCase_Execute_Reconciliation(t *testing.T) {
	// 1. LDAP生存者: Alice (101), Bob (102)
	testUsers := []*model.User{
		model.NewUser("101", "alice", "alice@example.com", "pass123"),
		model.NewUser("102", "bob", "bob@example.com", "pass456"),
	}
	source := &mockSourceRepository{users: testUsers}

	// 2. ESの既存データ: Alice (101, active), Bob (102, active), Charlie (103, active), elastic (system, active)
	charlie := model.NewUser("103", "charlie", "charlie@example.com", "pass789")
	charlie.IsActive = true
	elastic := model.NewUser("elastic", "elastic", "elastic@example.com", "pass999")
	elastic.IsActive = true

	existingMap := map[string]*model.User{
		"101":     model.NewUser("101", "alice", "alice@example.com", "pass123"),
		"102":     model.NewUser("102", "bob", "bob@example.com", "pass456"),
		"103":     charlie,
		"elastic": elastic,
	}

	target := &mockTargetRepository{
		existingUsers: existingMap,
	}

	finalFilter := "(&(objectClass=inetOrgPerson)(userPassword=*))"
	excludedUsers := []string{"elastic", "kibana_system"}

	u := NewSyncUserUseCase(source, target, &mockMetricsRepository{}, finalFilter, excludedUsers, 1, false)

	err := u.Execute(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Alice (101) and Bob (102) must remain Active
	userAlice, err := target.GetUser(context.Background(), "101")
	if err != nil || !userAlice.IsActive {
		t.Errorf("expected Alice (101) to be Active, got error=%v active=%t", err, userAlice.IsActive)
	}

	userBob, err := target.GetUser(context.Background(), "102")
	if err != nil || !userBob.IsActive {
		t.Errorf("expected Bob (102) to be Active, got error=%v active=%t", err, userBob.IsActive)
	}

	// Charlie (103) is not in LDAP survivors and not excluded, so it must be deactivated
	userCharlie, err := target.GetUser(context.Background(), "103")
	if err != nil || userCharlie.IsActive {
		t.Errorf("expected Charlie (103) to be logically deactivated, got error=%v active=%t", err, userCharlie.IsActive)
	}

	// elastic is not in LDAP survivors but is in the exclusion list, so it must remain Active
	userElastic, err := target.GetUser(context.Background(), "elastic")
	if err != nil || !userElastic.IsActive {
		t.Errorf("expected system user 'elastic' to remain Active, got error=%v active=%t", err, userElastic.IsActive)
	}
}

func TestSyncUserUseCase_Execute_SafetyGuard(t *testing.T) {
	// 1. LDAP生存者: 2人
	testUsers := []*model.User{
		model.NewUser("101", "alice", "alice@example.com", "pass123"),
		model.NewUser("102", "bob", "bob@example.com", "pass456"),
	}
	source := &mockSourceRepository{users: testUsers}
	target := &mockTargetRepository{
		existingUsers: make(map[string]*model.User),
	}

	// 閾値を3に設定（生存者2人 < 閾値3 なのでアボートするはず）
	u := NewSyncUserUseCase(source, target, &mockMetricsRepository{}, "", nil, 3, false)

	err := u.Execute(context.Background())
	if err == nil {
		t.Fatal("expected error due to safety guard, but got nil")
	}

	if !strings.Contains(err.Error(), "safety guard triggered") {
		t.Errorf("expected error message to contain 'safety guard triggered', got %q", err.Error())
	}

	// 同期（保存）処理が実行されていないことを確認
	if len(target.savedUsers) != 0 {
		t.Errorf("expected 0 users to be saved, got %d", len(target.savedUsers))
	}
}

func TestSyncUserUseCase_Execute_DryRun(t *testing.T) {
	// 1. LDAP生存者: Alice (101)
	testUsers := []*model.User{
		model.NewUser("101", "alice", "alice@example.com", "pass123"),
	}
	source := &mockSourceRepository{users: testUsers}

	// 2. ESの既存データ: Alice (101, active), Bob (102, active)
	bob := model.NewUser("102", "bob", "bob@example.com", "pass456")
	bob.IsActive = true
	existingMap := map[string]*model.User{
		"101": model.NewUser("101", "alice", "alice@example.com", "pass123"),
		"102": bob,
	}
	target := &mockTargetRepository{
		existingUsers: existingMap,
	}
	metrics := &mockMetricsRepository{}

	// slog の出力を一時的にキャプチャして検証
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	// dryRun = true でユースケース作成
	u := NewSyncUserUseCase(source, target, metrics, "", nil, 1, true)

	err := u.Execute(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// 1. Dry-Runのため、書き込みが実行されていないことを確認
	// target.savedUsers は空であるべき
	if len(target.savedUsers) != 0 {
		t.Errorf("expected 0 users to be saved during dry-run, got %d", len(target.savedUsers))
	}

	// Bob も deactivation されていない（保存されていない）
	userBob, _ := target.GetUser(context.Background(), "102")
	if !userBob.IsActive {
		t.Error("expected Bob to remain active (deactivation skipped)")
	}

	// 2. ログに Dry-Run 固有の出力が含まれているか確認
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "[Dry-Run] Would upsert user") {
		t.Error("expected dry-run upsert log but not found")
	}
	if !strings.Contains(logOutput, "[Dry-Run] Would deactivate user") {
		t.Error("expected dry-run deactivation log but not found")
	}

	// 3. メトリクスへの記録が行われていないことを検証
	if metrics.durationRecorded || metrics.processedRecorded || metrics.activeRecorded || metrics.statusRecorded {
		t.Error("expected no metrics to be recorded during dry-run")
	}
}

func TestSyncUserUseCase_Execute_RoleMapping(t *testing.T) {
	// 1. LDAP生存者: Alice (Roles: admin, unknown_role)
	alice := model.NewUser("101", "alice", "alice@example.com", "pass123")
	alice.Roles = []string{"admin", "unknown_role"}
	testUsers := []*model.User{alice}
	source := &mockSourceRepository{users: testUsers}

	// 2. ESの既存ロールとして "admin" のみを登録
	target := &mockTargetRepository{
		existingUsers: make(map[string]*model.User),
		existingRoles: []string{"admin"},
	}

	// slog の出力を一時的にキャプチャして検証
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	u := NewSyncUserUseCase(source, target, &mockMetricsRepository{}, "", nil, 1, false)

	err := u.Execute(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// 1. 保存された Alice に "admin" のみが割り当てられていることを確認
	if len(target.savedUsers) != 1 {
		t.Fatalf("expected 1 saved user, got %d", len(target.savedUsers))
	}
	savedAlice := target.savedUsers[0]
	if len(savedAlice.Roles) != 1 || savedAlice.Roles[0] != "admin" {
		t.Errorf("expected Roles to be ['admin'], got %v", savedAlice.Roles)
	}

	// 2. 存在しない "unknown_role" についての警告ログ（Warn）が出力されていることを確認
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "Role does not exist in Elasticsearch, skipping assignment") {
		t.Error("expected warning log about missing role, but not found")
	}
	if !strings.Contains(logOutput, `"level":"WARN"`) {
		t.Error("expected WARN log level for role validation skip")
	}
}
