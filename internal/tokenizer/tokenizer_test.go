package tokenizer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/model"
)

func TestNewTokenizerFallsBackToMock(t *testing.T) {
	tok, err := NewTokenizer(&conf.Conf{})
	if err != nil {
		t.Fatalf("new tokenizer: %v", err)
	}

	ids, err := tok.Encode(model.MockModel, "hello world")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("len(ids) = %d, want 2", len(ids))
	}
}

func TestNewTokenizerUsesQwenConfig(t *testing.T) {
	dir := t.TempDir()
	vocabPath := filepath.Join(dir, "vocab.json")
	mergesPath := filepath.Join(dir, "merges.txt")
	configPath := filepath.Join(dir, "tokenizer_config.json")

	writeTestFile(t, vocabPath, `{
		"h": 0,
		"i": 1,
		"hi": 2
	}`)
	writeTestFile(t, mergesPath, "#version: 0.2\nh i\n")
	writeTestFile(t, configPath, `{"added_tokens_decoder": {}}`)
	writeTestFile(t, filepath.Join(dir, "config.json"), `{"model_type": "qwen3"}`)

	tok, err := NewTokenizer(&conf.Conf{
		Models: []conf.ModelConf{
			{
				ModelID:   string(model.Qwen3Model),
				ModelPath: dir,
			},
		},
	})
	if err != nil {
		t.Fatalf("new tokenizer: %v", err)
	}

	ids, err := tok.Encode(model.Qwen3Model, "hi")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(ids) != 1 || ids[0] != 2 {
		t.Fatalf("ids = %v, want [2]", ids)
	}
}

func TestMockDecodeUnknownTokenReturnsPlaceholder(t *testing.T) {
	tok, err := NewTokenizer(&conf.Conf{})
	if err != nil {
		t.Fatalf("new tokenizer: %v", err)
	}

	text, err := tok.Decode(model.MockModel, []uint32{12345})
	if err != nil {
		t.Fatalf("decode unknown mock token: %v", err)
	}
	if text == "" {
		t.Fatal("decode unknown mock token returned empty text")
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
