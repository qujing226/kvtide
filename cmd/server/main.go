package main

import (
	"github.com/qujing226/mini-llm-serve/internal/block"
	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/executor"
	"github.com/qujing226/mini-llm-serve/internal/handler"
	"github.com/qujing226/mini-llm-serve/internal/metrics"
	"github.com/qujing226/mini-llm-serve/internal/scheduler"
	"github.com/qujing226/mini-llm-serve/internal/state"
	"github.com/qujing226/mini-llm-serve/internal/tokenizer"
	connect "github.com/qujing226/mini-llm-serve/internal/transport"
	"github.com/spf13/pflag"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
)

func main() {
	parseFlags := fx.Annotate(
		func() (confPath string) {
			pflag.StringVarP(&confPath, "conf", "c", "server.toml", "Path to the configuration file (e.g., --config=./server.toml/server.yml/server.yaml/server.json)")
			pflag.Parse()
			return confPath
		},
		fx.ResultTags(`name:"confPath"`),
	)

	app := fx.New(
		fx.Provide(
			parseFlags,
			fx.Annotate(conf.NewConfFromPath, fx.ParamTags(`name:"confPath"`)),
			newLogger),
		fx.WithLogger(func(log *zap.SugaredLogger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log.Desugar()}
		}),
		fx.Options(),

		// initialize scheduler
		fx.Provide(
			scheduler.NewDecodeQueue,
			fx.Annotate(
				scheduler.NewPrefillQueue,
				fx.ResultTags(`name:"prefillQueueSmall"`),
			),
			fx.Annotate(
				scheduler.NewPrefillQueue,
				fx.ResultTags(`name:"prefillQueueLarge"`),
			),
			fx.Annotate(
				scheduler.NewScheduler,
				fx.ParamTags(``, ``, `name:"prefillQueueSmall"`, `name:"prefillQueueLarge"`, ``, ``, ``, ``, ``),
			),
		),
		fx.Provide(
			tokenizer.NewTokenizer,
			block.NewManager,
			metrics.NewMetrics,
			executor.NewExecutors,
			executor.NewExecutorManager,
			state.NewRequestLifecycleStateManager,
			handler.NewInferenceHandle,
			connect.NewLLMServingServer,
			connect.NewAdminService,
		),
		fx.Invoke(
			connect.StartServer,
			StartBatchLoop,
		),
	)
	app.Run()
}
