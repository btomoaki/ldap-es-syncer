package prometheus

import (
	"fmt"
	"time"

	"ldap-es-syncer/internal/domain/repository"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/push"
)

// PrometheusMetricsRepository は MetricsRepository の Prometheus による実装です。
type PrometheusMetricsRepository struct {
	syncDuration   prometheus.Histogram
	processedUsers prometheus.Counter
	activeUsers    prometheus.Gauge
	syncSuccess    prometheus.Gauge
	registry       *prometheus.Registry
}

var (
	// コレクターの重複登録エラーを防ぐため、デフォルトのレジストリで promauto を使う代わりに、
	// 独自のレジストリを用意してシングルトン的に管理します。
	instance *PrometheusMetricsRepository
)

// NewPrometheusMetricsRepository は PrometheusMetricsRepository のインスタンスを生成・取得します。
func NewPrometheusMetricsRepository() repository.MetricsRepository {
	if instance != nil {
		return instance
	}

	reg := prometheus.NewRegistry()

	syncDuration := promauto.With(reg).NewHistogram(prometheus.HistogramOpts{
		Name:    "ldap_es_syncer_sync_duration_seconds",
		Help:    "Duration of user synchronization in seconds.",
		Buckets: prometheus.DefBuckets,
	})

	processedUsers := promauto.With(reg).NewCounter(prometheus.CounterOpts{
		Name: "ldap_es_syncer_processed_users_total",
		Help: "Total number of users processed (upserted or deactivated).",
	})

	activeUsers := promauto.With(reg).NewGauge(prometheus.GaugeOpts{
		Name: "ldap_es_syncer_active_users",
		Help: "Current number of active users fetched from LDAP.",
	})

	syncSuccess := promauto.With(reg).NewGauge(prometheus.GaugeOpts{
		Name: "ldap_es_syncer_sync_success",
		Help: "Sync status of the last run (1 for success, 0 for failure).",
	})

	instance = &PrometheusMetricsRepository{
		syncDuration:   syncDuration,
		processedUsers: processedUsers,
		activeUsers:    activeUsers,
		syncSuccess:    syncSuccess,
		registry:       reg,
	}

	return instance
}

// RecordSyncDuration は同期時間を記録します。
func (r *PrometheusMetricsRepository) RecordSyncDuration(duration time.Duration) {
	r.syncDuration.Observe(duration.Seconds())
}

// RecordProcessedUsers は処理されたユーザー数をカウントします。
func (r *PrometheusMetricsRepository) RecordProcessedUsers(count int) {
	r.processedUsers.Add(float64(count))
}

// RecordActiveUsers はアクティブなユーザー数を記録します。
func (r *PrometheusMetricsRepository) RecordActiveUsers(count int) {
	r.activeUsers.Set(float64(count))
}

// RecordSyncStatus は同期処理の成否を記録します。
func (r *PrometheusMetricsRepository) RecordSyncStatus(success bool) {
	if success {
		r.syncSuccess.Set(1)
	} else {
		r.syncSuccess.Set(0)
	}
}

// GetRegistry は Prometheus の Registry を取得します。
func (r *PrometheusMetricsRepository) GetRegistry() *prometheus.Registry {
	return r.registry
}

// PushToPushgateway は Pushgateway へ現在のメトリクスをプッシュします。
func (r *PrometheusMetricsRepository) PushToPushgateway(url, jobName string) error {
	pusher := push.New(url, jobName).Gatherer(r.registry)
	if err := pusher.Push(); err != nil {
		return fmt.Errorf("failed to push metrics to Pushgateway: %w", err)
	}
	return nil
}
