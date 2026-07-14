package metrics

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/qujing226/mini-llm-serve/internal/model"
)

type Metrics interface {
	RuntimeStats
	Block

	IncRequest(status string, executorID string)
	ObserveRequestDuration(s float64)
	ObserveTTFT(s float64)
	ObserveTBT(s float64)
	ObserveQueueWait(s float64)
	ObserveExecution(s float64, executorID string)
	ObserveBatch(size int, prefillItems int, decodeItems int)
	IncBatches(executorID string)
	IncQueueRejected()
	IncExecutorErrors(executorID string)

	// Prefix Cache
	IncPrefixCacheRequests(hit bool)
	AddPrefixCacheTokensSaved(tokens uint64)

	// Handler Snapshot RuntimeStats
	Handler() http.Handler
	Snapshot() model.EngineRuntimeStats
}

type metrics struct {
	re *prometheus.Registry

	*block

	requestsTotal          *prometheus.CounterVec
	requestDurationSeconds prometheus.Histogram
	ttftSeconds            prometheus.Histogram
	tbtSeconds             prometheus.Histogram
	queueWaitSeconds       prometheus.Histogram
	executionSeconds       *prometheus.HistogramVec

	batchSize                   *prometheus.HistogramVec
	batchItemsTotal             *prometheus.CounterVec
	batchesTotal                *prometheus.CounterVec
	prefillQueueLength          prometheus.Gauge
	decodeQueueLength           prometheus.Gauge
	activeRequests              prometheus.Gauge
	inflightBatches             prometheus.Gauge
	prefixCacheRequestsTotal    *prometheus.CounterVec
	prefixCacheTokensSavedTotal prometheus.Counter
	queueRejectedTotal          prometheus.Counter
	executorErrorsTotal         *prometheus.CounterVec

	mu           sync.RWMutex
	runtimeStats *model.EngineRuntimeStats
}

func NewMetrics() Metrics {
	m := &metrics{
		block: newBlock(),

		requestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "llm_requests_total",
			Help: "Total number of requests",
		}, []string{"status", "executor"}),
		requestDurationSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "llm_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: []float64{0.05, 0.1, 0.2, 0.5, 1, 1.5, 2, 3, 5, 10},
		}),
		ttftSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "llm_ttft_seconds",
			Help:    "Time from request arrival to first generated token",
			Buckets: []float64{0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 5},
		}),
		tbtSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "llm_tbt_seconds",
			Help:    "Time between generated token chunks",
			Buckets: []float64{0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1},
		}),
		queueWaitSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "llm_queue_wait_seconds",
			Help:    "Request queue wait in seconds",
			Buckets: []float64{0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1},
		}),
		executionSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "llm_execution_seconds",
			Help: "Execution time in seconds",
		}, []string{"executor"}),
		batchSize: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "llm_batch_size",
			Help:    "Number of tasks in a batch",
			Buckets: []float64{1, 2, 4, 8, 16, 32, 64},
		}, []string{"phase"}),
		batchItemsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "llm_batch_items_total",
			Help: "Total number of work items scheduled in batches",
		}, []string{"phase"}),
		batchesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "llm_batches_total",
			Help: "Number of batches which executor had processed",
		}, []string{"executor", "phase"}),
		prefillQueueLength: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "llm_prefill_queue_length",
			Help: "Number of queued prefill works",
		}),
		decodeQueueLength: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "llm_decode_queue_length",
			Help: "Number of queued decode works",
		}),
		activeRequests: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "llm_active_requests",
			Help: "Number of active requests",
		}),
		inflightBatches: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "llm_inflight_batches",
			Help: "Number of inflight batches",
		}),
		prefixCacheRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "llm_prefix_cache_requests_total",
			Help: "Total number of prefix cache lookups",
		}, []string{"status"}),
		prefixCacheTokensSavedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "llm_prefix_cache_tokens_saved_total",
			Help: "Total number of prefill tokens saved by prefix cache",
		}),
		queueRejectedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "llm_queue_rejected_total",
			Help: "Total Number of rejected requests due to full queue",
		}),
		executorErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "llm_executor_errors_total",
			Help: "Total Number of executor errors",
		}, []string{"executor"}),
		runtimeStats: &model.EngineRuntimeStats{
			PrefillQueueLength: 0,
			ActiveRequests:     0,
			InflightBatches:    0,
			BusyExecutors:      0,
			IdleExecutors:      0,
		},
	}
	m.re = prometheus.NewRegistry()
	m.re.MustRegister(
		m.block.kvBlocks,
		m.block.evictedBlocksTotal,
		m.block.allocationFailuresTotal,

		m.requestsTotal,
		m.requestDurationSeconds,
		m.ttftSeconds,
		m.tbtSeconds,
		m.queueWaitSeconds,
		m.executionSeconds,
		m.batchSize,
		m.batchItemsTotal,
		m.batchesTotal,
		m.prefillQueueLength,
		m.decodeQueueLength,
		m.activeRequests,
		m.inflightBatches,
		m.queueRejectedTotal,
		m.executorErrorsTotal,
		m.prefixCacheRequestsTotal,
		m.prefixCacheTokensSavedTotal,
	)
	return m
}

func (m *metrics) IncRequest(status string, executorID string) {
	m.requestsTotal.WithLabelValues(status, executorID).Inc()
}

func (m *metrics) ObserveRequestDuration(s float64) {
	m.requestDurationSeconds.Observe(s)
}

func (m *metrics) ObserveTTFT(s float64) {
	m.ttftSeconds.Observe(s)
}

func (m *metrics) ObserveTBT(s float64) {
	m.tbtSeconds.Observe(s)
}

func (m *metrics) ObserveQueueWait(s float64) {
	m.queueWaitSeconds.Observe(s)
}

func (m *metrics) ObserveExecution(s float64, executorID string) {
	m.executionSeconds.WithLabelValues(executorID).Observe(s)

}

func (m *metrics) ObserveBatch(size int, prefillItems int, decodeItems int) {
	//m.batchSize.WithLabelValues(phase.String()).Observe(float64(size))
	m.batchSize.WithLabelValues("mixed").Observe(float64(size))
	m.batchItemsTotal.WithLabelValues("prefill").Add(float64(prefillItems))
	m.batchItemsTotal.WithLabelValues("decode").Add(float64(decodeItems))
}

func (m *metrics) IncBatches(executorID string) {
	//m.batchesTotal.WithLabelValues(executorID, phase.String()).Inc()
	m.batchesTotal.WithLabelValues(executorID, "mixed").Inc()
}

func (m *metrics) IncQueueRejected() {
	m.queueRejectedTotal.Inc()

}

func (m *metrics) IncExecutorErrors(executorID string) {
	m.executorErrorsTotal.WithLabelValues(executorID).Inc()
}

func (m *metrics) IncPrefixCacheRequests(hit bool) {
	if hit {
		m.prefixCacheRequestsTotal.WithLabelValues("hit").Inc()
	} else {
		m.prefixCacheRequestsTotal.WithLabelValues("miss").Inc()
	}
}

func (m *metrics) AddPrefixCacheTokensSaved(tokens uint64) {
	m.prefixCacheTokensSavedTotal.Add(float64(tokens))
}

func (m *metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.re, promhttp.HandlerOpts{})
}

func (m *metrics) Snapshot() model.EngineRuntimeStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := *m.runtimeStats
	return s
}
