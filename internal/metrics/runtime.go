package metrics

type RuntimeStats interface {
	SetPrefillQueueLength(n int)
	SetDecodeQueueLength(n int)
	SetActiveRequests(n int)
	SetInflightBatches(n int)
}

func (m *metrics) SetPrefillQueueLength(n int) {
	m.prefillQueueLength.Set(float64(n))
	m.mu.Lock()
	m.runtimeStats.PrefillQueueLength = uint64(n)
	m.mu.Unlock()
}

func (m *metrics) SetDecodeQueueLength(n int) {
	m.decodeQueueLength.Set(float64(n))
	m.mu.Lock()
	m.runtimeStats.DecodeQueueLength = uint64(n)
	m.mu.Unlock()
}

func (m *metrics) SetActiveRequests(n int) {
	m.activeRequests.Set(float64(n))
	m.mu.Lock()
	m.runtimeStats.ActiveRequests = uint64(n)
	m.mu.Unlock()
}

func (m *metrics) SetInflightBatches(n int) {
	m.inflightBatches.Set(float64(n))
	m.mu.Lock()
	m.runtimeStats.InflightBatches = uint64(n)
	m.mu.Unlock()
}
