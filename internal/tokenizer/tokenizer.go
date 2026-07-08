package tokenizer

import (
	"crypto/sha3"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"

	gotokenizer "github.com/qujing226/gotokenizer"
	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/errors"
)

type Tokenizer interface {
	Encode(text string) ([]uint32, error)
	Decode(tokens []uint32) (string, error)
}

type mockTokenizer struct {
	mu      sync.RWMutex
	reverse map[uint32]string
}

func NewTokenizer(cfg *conf.Conf) (Tokenizer, error) {
	for _, tokenzier := range cfg.Tokenizer {
		switch tokenzier.Kind {
		case "", "mock":
			return newMockTokenizer(), nil
		case "qwen":
			return gotokenizer.NewQwenTokenizer(gotokenizer.QwenTokenizerConfig{
				VocabPath:           tokenzier.VocabPath,
				MergesPath:          tokenzier.MergesPath,
				TokenizerConfigPath: tokenzier.TokenizerConfigPath,
			})
		default:
			return nil, errors.New(errors.CodeInvalidArgument, "tokenizer: unsupported kind "+tokenzier.Kind)
		}
	}
	return newMockTokenizer(), nil
}

func newMockTokenizer() Tokenizer {
	return &mockTokenizer{
		reverse: make(map[uint32]string),
	}
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
