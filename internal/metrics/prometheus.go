package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	backupDuration    prometheus.Histogram
	backupSize        prometheus.Gauge
	backupTotal       prometheus.Counter
	backupFailures    prometheus.Counter
	lastBackupTime    prometheus.Gauge
	lastBackupSuccess prometheus.Gauge
	storageUsed       prometheus.Gauge
}

func New(namespace string) *Metrics {
	if namespace == "" {
		namespace = "datasaver"
	}

	m := &Metrics{
		backupDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "backup_duration_seconds",
			Help:      "Duration of backup operations in seconds",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 12),
		}),
		backupSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "backup_size_bytes",
			Help:      "Size of the last backup in bytes",
		}),
		backupTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "backups_total",
			Help:      "Total number of backups attempted",
		}),
		backupFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "backup_failures_total",
			Help:      "Total number of failed backups",
		}),
		lastBackupTime: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "last_backup_timestamp",
			Help:      "Timestamp of the last backup attempt",
		}),
		lastBackupSuccess: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "last_backup_success",
			Help:      "Whether the last backup was successful (1) or not (0)",
		}),
		storageUsed: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "storage_used_bytes",
			Help:      "Total storage used by all backups in bytes",
		}),
	}

	prometheus.MustRegister(
		m.backupDuration,
		m.backupSize,
		m.backupTotal,
		m.backupFailures,
		m.lastBackupTime,
		m.lastBackupSuccess,
		m.storageUsed,
	)

	return m
}

func (m *Metrics) RecordBackupSuccess(duration time.Duration, sizeBytes int64) {
	m.backupTotal.Inc()
	m.backupDuration.Observe(duration.Seconds())
	m.backupSize.Set(float64(sizeBytes))
	m.lastBackupTime.SetToCurrentTime()
	m.lastBackupSuccess.Set(1)
}

func (m *Metrics) RecordBackupFailure() {
	m.backupTotal.Inc()
	m.backupFailures.Inc()
	m.lastBackupTime.SetToCurrentTime()
	m.lastBackupSuccess.Set(0)
}

func (m *Metrics) SetStorageUsed(bytes int64) {
	m.storageUsed.Set(float64(bytes))
}

func Handler() http.Handler {
	return promhttp.Handler()
}
