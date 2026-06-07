package metrics

import "github.com/prometheus/client_golang/prometheus"

type Block interface {
	ObserveBlockStats(active, free, cached uint64)
	IncAllocationFailure()
	IncEvictedBlock()
}

type block struct {
	kvBlocks                *prometheus.GaugeVec
	allocationFailuresTotal prometheus.Counter
	evictedBlocksTotal      prometheus.Counter
}

func newBlock() *block {
	return &block{
		kvBlocks: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "llm_kv_blocks",
			Help: "Current KV block counts; cached blocks may also be free or active",
		}, []string{"state"}),
		allocationFailuresTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "llm_kv_allocation_failures_total",
			Help: "Number of allocation failures",
		}),
		evictedBlocksTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "llm_prefix_cache_evictions_total",
			Help: "Number of evicted blocks",
		}),
	}
}

func (b *block) ObserveBlockStats(active, free, cached uint64) {
	b.kvBlocks.WithLabelValues("active").Set(float64(active))
	b.kvBlocks.WithLabelValues("free").Set(float64(free))
	b.kvBlocks.WithLabelValues("cached").Set(float64(cached))
}
func (b *block) IncAllocationFailure() {
	b.allocationFailuresTotal.Inc()
}

func (b *block) IncEvictedBlock() {
	b.evictedBlocksTotal.Inc()
}
