package main

import (
	"fmt"

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

func newBlockConfig(
	executors map[string]executor.Executor,
) (block.Config, error) {
	if len(executors) != 1 {
		return block.Config{}, fmt.Errorf(
			"exactly one executor is currently supported",
		)
	}

	// todo: multi-executor
	for _, exec := range executors {
		runtime := exec.GetRuntimeStates()

		return block.Config{
			ExecutorID:   runtime.ExecutorId,
			RuntimeEpoch: runtime.RuntimeEpoch,
			BlockSize:    runtime.BlockSize,
			NumBlocks:    runtime.NumKvBlocks,
		}, nil
	}

	return block.Config{}, fmt.Errorf("no executor runtime available")
}
