package main

import (
	"context"

	"github.com/qujing226/kvtide/internal/executor"
	"github.com/qujing226/kvtide/internal/scheduler"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func StartBatchLoop(lc fx.Lifecycle, log *zap.SugaredLogger, s scheduler.Scheduler, e executor.Manager) {
	runCtx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				log.Info("scheduler batch loop started")
				s.Batch(runCtx)
				log.Info("scheduler batch loop stopped")
			}()
			go func() {
				log.Info("executor manager consume loop started")
				e.Consume(runCtx)
				log.Info("executor manager consume loop stopped")
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			cancel()
			return nil
		},
	})
}
