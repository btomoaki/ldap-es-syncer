package repository

import "time"

// MetricsRepository はメトリクスの記録を行うための抽象インターフェース（ポート）です。
type MetricsRepository interface {
	RecordSyncDuration(duration time.Duration)
	RecordProcessedUsers(count int)
	RecordActiveUsers(count int)
	RecordSyncStatus(success bool)
}
