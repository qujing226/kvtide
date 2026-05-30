package cache

import (
	"sync"
)

type PrefixCache interface {
	Lookup(cacheKey string, promptTokens uint32) (cacheTokens uint32, hit bool)
	Put(cacheKey string, promptTokens uint32)
}

type prefixCache struct {
	prefixes map[string]uint32
	mu       sync.RWMutex
}

func NewPrefixCache() PrefixCache {
	p := &prefixCache{
		prefixes: make(map[string]uint32),
	}
	return p
}

func (p *prefixCache) Lookup(cacheKey string, promptTokens uint32) (cacheTokens uint32, hit bool) {
	if cacheKey == "" {
		return 0, false
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	cachedTokens, exists := p.prefixes[cacheKey]
	if !exists {
		return 0, false
	}
	return min(cachedTokens, promptTokens), true
}

func (p *prefixCache) Put(cacheKey string, promptTokens uint32) {
	if cacheKey == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	tokens, exists := p.prefixes[cacheKey]
	if exists {
		p.prefixes[cacheKey] = max(tokens, promptTokens)
	} else {
		p.prefixes[cacheKey] = promptTokens
	}
}
