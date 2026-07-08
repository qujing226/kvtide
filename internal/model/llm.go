package model

import (
	"strings"

	"github.com/qujing226/mini-llm-serve/internal/errors"
)

type LLMModelID string

const (
	MockModel  LLMModelID = "mock"
	Qwen3Model LLMModelID = "qwen3"
)

func ParseModelID(modelID string) (LLMModelID, error) {
	if strings.HasPrefix(modelID, string(Qwen3Model)) {
		return Qwen3Model, nil
	} else if strings.HasPrefix(modelID, string(MockModel)) {
		return MockModel, nil
	} else {
		return "", errors.New(errors.CodeInvalidArgument, "invalid model id: "+modelID)
	}
}

type LLMModelSpec struct {
	ID          LLMModelID
	Family      string // mock, qwen3, llama...
	TokenizerID string
	ExecutorID  string
}
