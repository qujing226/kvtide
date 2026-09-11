package main

import (
	"github.com/qujing226/kvtide/internal/block"
	"github.com/qujing226/kvtide/internal/executor"
	"github.com/qujing226/kvtide/internal/metrics"
	"go.uber.org/zap"
)

func newLogger() *zap.SugaredLogger {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	return logger.Sugar()
}

func newBlockRegistry(
	logger *zap.SugaredLogger,
	m metrics.Metrics,
	executors executor.Manager,
) (block.Registry, error) {
	return block.NewRegistry(logger, m, executors.GetRuntimeStates())
}
