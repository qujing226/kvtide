package tokenizer

import (
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
	if len(cfg.Tokenizer) == 0 {
		t.tokenizers[model.MockModel] = newMockTokenizer()
		return t, nil
	}
	for _, tokenzierCfg := range cfg.Tokenizer {
		modelID, err := tokenizerModelID(tokenzierCfg)
		if err != nil {
			return nil, err
		}
		switch tokenzierCfg.Kind {
		case "", "mock":
			t.tokenizers[modelID] = newMockTokenizer()
		case "qwen":
			tok, err := newQwen3Tokenizer(tokenzierCfg)
			if err != nil {
				return nil, err
			}
			t.tokenizers[modelID] = tok
		default:
			return nil, errors.New(errors.CodeInvalidArgument, "tokenizer: unsupported kind "+tokenzierCfg.Kind)
		}
	}
	return t, nil
}

func tokenizerModelID(cfg conf.TokenizerConf) (model.LLMModelID, error) {
	if cfg.Model != "" {
		return model.ParseModelID(cfg.Model)
	}
	switch cfg.Kind {
	case "", "mock":
		return model.MockModel, nil
	case "qwen":
		return model.Qwen3Model, nil
	default:
		return "", errors.New(errors.CodeInvalidArgument, "tokenizer: model is required for kind "+cfg.Kind)
	}
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
