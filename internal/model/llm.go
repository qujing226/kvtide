package model

import (
	"github.com/qujing226/kvtide/internal/errors"
)

type LLMModelID string

const (
	MockModel  LLMModelID = "mock"
	Qwen3Model LLMModelID = "Qwen/Qwen3-0.6B"
)

func ParseModelID(modelID string) (LLMModelID, error) {
	switch modelID {
	case string(Qwen3Model), "Qwen3-0.6B", "qwen3":
		return Qwen3Model, nil
	case string(MockModel), "":
		return MockModel, nil
	default:
		return "", errors.New(errors.CodeInvalidArgument, "invalid model id: "+modelID)
	}
}

type LLMModelSpec struct {
	ID          LLMModelID
	ModelType   string // mock, qwen3, llama...
	TokenizerID string
	ExecutorID  string
}
