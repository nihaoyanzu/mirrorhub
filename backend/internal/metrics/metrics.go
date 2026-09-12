package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	RequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mirrorhub_requests_total",
		Help: "Proxy requests",
	}, []string{"platform", "strategy", "cache"})

	CacheUsageRatio = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mirrorhub_cache_usage_ratio",
		Help: "Cache size / max size",
	})
	DiskFreeBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mirrorhub_disk_free_bytes",
		Help: "Free bytes on cache filesystem",
	})
	DownloadResumesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mirrorhub_download_resumes_total",
		Help: "Upstream stream Range resumes",
	})
	ErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mirrorhub_errors_total",
		Help: "Errors by kind",
	}, []string{"kind"})
	InflightGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mirrorhub_inflight",
		Help: "In-flight downloads",
	})
	SchedulerP0Active = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mirrorhub_scheduler_p0_active",
		Help: "Active interactive/resume tasks",
	})
	PrefetchPaused = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mirrorhub_prefetch_paused",
		Help: "1 if prefetch paused",
	})
	QueueWaitSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "mirrorhub_queue_wait_seconds",
		Help:    "Task wait time before start",
		Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5, 15, 60},
	})
	Tasks = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mirrorhub_tasks",
		Help: "Tasks by status and priority",
	}, []string{"status", "priority"})
)

func init() {
	prometheus.MustRegister(
		RequestsTotal,
		CacheUsageRatio,
		DiskFreeBytes,
		DownloadResumesTotal,
		ErrorsTotal,
		InflightGauge,
		SchedulerP0Active,
		PrefetchPaused,
		QueueWaitSeconds,
		Tasks,
	)
}
