package executor

import (
	"testing"

	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"go.uber.org/zap"
)

func TestNewExecutorsUsesBuiltInConnectProtocol(t *testing.T) {
	executors, err := NewExecutors(zap.NewNop().Sugar(), &conf.Conf{
		Executors: []conf.ExecutorConf{
			{
				ID:        "executor-qwen3-0.6b",
				ModelID:   string(model.Qwen3Model),
				Address:   []string{"http://127.0.0.1:19991"},
				TimeoutMs: 1000,
			},
		},
	})
	if err != nil {
		t.Fatalf("new executors: %v", err)
	}
	if len(executors) != 1 {
		t.Fatalf("len(executors) = %d, want 1", len(executors))
	}
}
