package main

import (
	"fmt"
	"sort"

	"github.com/qujing226/kvtide/internal/block"
	"github.com/qujing226/kvtide/internal/executor"
	"go.uber.org/zap"
)

func newLogger() *zap.SugaredLogger {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	return logger.Sugar()
}

func newBlockConfigs(
	executors map[string]executor.Executor,
) ([]block.Config, error) {
	if len(executors) == 0 {
		return nil, fmt.Errorf("no executor runtime available")
	}

	executorIDs := make([]string, 0, len(executors))
	for executorID := range executors {
		executorIDs = append(executorIDs, executorID)
	}
	sort.Strings(executorIDs)

	configs := make([]block.Config, 0, len(executorIDs))
	for _, executorID := range executorIDs {
		exec := executors[executorID]
		runtime := exec.GetRuntimeStates()
		if runtime == nil {
			return nil, fmt.Errorf("executor %s returned no runtime", executorID)
		}
		if runtime.ExecutorId != executorID {
			return nil, fmt.Errorf(
				"executor runtime ID %s does not match configured ID %s",
				runtime.ExecutorId,
				executorID,
			)
		}

		cfg := block.Config{
			ExecutorID:   runtime.ExecutorId,
			RuntimeEpoch: runtime.RuntimeEpoch,
			BlockSize:    runtime.BlockSize,
			NumBlocks:    runtime.NumKvBlocks,
		}
		if err := cfg.Validate(); err != nil {
			return nil, fmt.Errorf("invalid runtime for executor %s: %w", executorID, err)
		}
		configs = append(configs, cfg)
	}

	return configs, nil
}
