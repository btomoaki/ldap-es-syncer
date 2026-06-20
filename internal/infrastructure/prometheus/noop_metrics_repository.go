package prometheus

import (
	"time"

	"ldap-es-syncer/internal/domain/repository"
)

// NoopMetricsRepository は何もしない MetricsRepository の実装です。
// テスト時や、メトリクス収集が無効化されている場合に使用されます。
type NoopMetricsRepository struct{}

// NewNoopMetricsRepository は NoopMetricsRepository のインスタンスを生成します。
func NewNoopMetricsRepository() repository.MetricsRepository {
	return &NoopMetricsRepository{}
}

// RecordSyncDuration は何も行いません。
func (r *NoopMetricsRepository) RecordSyncDuration(duration time.Duration) {}

// RecordProcessedUsers は何も行いません。
func (r *NoopMetricsRepository) RecordProcessedUsers(count int) {}

// RecordActiveUsers は何も行いません。
func (r *NoopMetricsRepository) RecordActiveUsers(count int) {}

// RecordSyncStatus は何も行いません。
func (r *NoopMetricsRepository) RecordSyncStatus(success bool) {}
