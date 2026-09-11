package metrics

import "github.com/prometheus/client_golang/prometheus"

type Block interface {
	ObserveBlockStats(executorID string, active, free, cached uint64)
	IncAllocationFailure(executorID string)
	IncEvictedBlock(executorID string)
}

type block struct {
	kvBlocks                *prometheus.GaugeVec
	allocationFailuresTotal *prometheus.CounterVec
	evictedBlocksTotal      *prometheus.CounterVec
}

func newBlock() *block {
	return &block{
		kvBlocks: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "llm_kv_blocks",
			Help: "Current KV block counts; cached blocks may also be free or active",
		}, []string{"executor", "state"}),
		allocationFailuresTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "llm_kv_allocation_failures_total",
			Help: "Number of allocation failures",
		}, []string{"executor"}),
		evictedBlocksTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "llm_prefix_cache_evictions_total",
			Help: "Number of evicted blocks",
		}, []string{"executor"}),
	}
}

func (b *block) ObserveBlockStats(executorID string, active, free, cached uint64) {
	b.kvBlocks.WithLabelValues(executorID, "active").Set(float64(active))
	b.kvBlocks.WithLabelValues(executorID, "free").Set(float64(free))
	b.kvBlocks.WithLabelValues(executorID, "cached").Set(float64(cached))
}
func (b *block) IncAllocationFailure(executorID string) {
	b.allocationFailuresTotal.WithLabelValues(executorID).Inc()
}

func (b *block) IncEvictedBlock(executorID string) {
	b.evictedBlocksTotal.WithLabelValues(executorID).Inc()
}
