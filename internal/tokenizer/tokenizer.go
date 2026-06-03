package tokenizer

import (
	"crypto/sha3"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
)

type Tokenizer interface {
	Encode(text string) ([]uint32, error)
	Decode(tokens []uint32) (string, error)
}

type mockTokenizer struct {
	mu      sync.RWMutex
	reverse map[uint32]string
}

func NewTokenizer() Tokenizer {
	t := &mockTokenizer{
		reverse: make(map[uint32]string),
	}
	return t
}

func (m *mockTokenizer) Encode(text string) ([]uint32, error) {
	fields := strings.Fields(text)
	tokens := make([]uint32, 0, len(fields))
	for _, field := range fields {
		tokenId := m.stableHash32(field)
		m.mu.Lock()
		if token, ok := m.reverse[tokenId]; ok && token != field {
			m.mu.Unlock()
			return nil, fmt.Errorf("token collision")
		}

		m.reverse[tokenId] = field
		m.mu.Unlock()
		tokens = append(tokens, tokenId)
	}
	if len(tokens) == 0 {
		tokenId := m.stableHash32("")
		m.mu.Lock()
		m.reverse[tokenId] = ""
		m.mu.Unlock()
		return []uint32{tokenId}, nil
	}
	return tokens, nil
}

func (m *mockTokenizer) Decode(tokens []uint32) (string, error) {
	words := make([]string, 0, len(tokens))
	for _, tokenID := range tokens {
		m.mu.RLock()
		word, ok := m.reverse[tokenID]
		m.mu.RUnlock()
		if !ok {
			return "", fmt.Errorf("token not found")
		}
		words = append(words, word)
	}
	return strings.Join(words, " "), nil
}

func (m *mockTokenizer) stableHash32(field string) uint32 {
	sum := sha3.Sum256([]byte(field))
	return binary.BigEndian.Uint32(sum[:4])
}
