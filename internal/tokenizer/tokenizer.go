package tokenizer

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/errors"
	"github.com/qujing226/mini-llm-serve/internal/model"
)

type Tokenizer interface {
	Encode(model model.LLMModelID, text string) ([]uint32, error)
	Decode(model model.LLMModelID, tokens []uint32) (string, error)
}

type tokenize interface {
	Encode(text string) ([]uint32, error)
	Decode(tokens []uint32) (string, error)
}

type tokenizer struct {
	tokenizers map[model.LLMModelID]tokenize
}

func NewTokenizer(cfg *conf.Conf) (Tokenizer, error) {
	t := &tokenizer{
		tokenizers: make(map[model.LLMModelID]tokenize),
	}
	if len(cfg.Models) == 0 {
		t.tokenizers[model.MockModel] = newMockTokenizer()
		return t, nil
	}
	for _, modelConf := range cfg.Models {
		modelID, err := model.ParseModelID(modelConf.ModelID)
		if err != nil {
			return nil, err
		}
		modelType, err := readModelType(modelConf.ModelPath)
		if err != nil {
			return nil, err
		}
		switch modelType {
		case "mock":
			t.tokenizers[modelID] = newMockTokenizer()
		case "qwen3":
			tok, err := newQwen3Tokenizer(modelConf)
			if err != nil {
				return nil, err
			}
			t.tokenizers[modelID] = tok
		default:
			return nil, errors.New(errors.CodeInvalidArgument, "tokenizer: unsupported model_type "+modelType)
		}
	}
	return t, nil
}

func readModelType(modelPath string) (string, error) {
	if modelPath == "" {
		return "mock", nil
	}
	b, err := os.ReadFile(filepath.Join(modelPath, "config.json"))
	if err != nil {
		return "", err
	}
	var cfg struct {
		ModelType string `json:"model_type"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return "", err
	}
	if cfg.ModelType == "" {
		return "", errors.New(errors.CodeInvalidArgument, "model config missing model_type: "+modelPath)
	}
	return cfg.ModelType, nil
}

func (t *tokenizer) Encode(modelID model.LLMModelID, text string) ([]uint32, error) {
	tok, ok := t.tokenizers[modelID]
	if !ok {
		return nil, errors.New(errors.CodeInvalidArgument, "tokenizer: model not registered "+string(modelID))
	}
	tokens, err := tok.Encode(text)
	if err != nil {
		return nil, err
	}
	return tokens, nil
}

func (t *tokenizer) Decode(modelID model.LLMModelID, tokens []uint32) (string, error) {
	tok, ok := t.tokenizers[modelID]
	if !ok {
		return "", errors.New(errors.CodeInvalidArgument, "tokenizer: model not registered "+string(modelID))
	}
	str, err := tok.Decode(tokens)
	if err != nil {
		return "replace", err
	}
	return str, nil
}
