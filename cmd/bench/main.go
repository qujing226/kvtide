package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

func main() {
	var (
		mode        string
		target      string
		metricsURL  string
		requests    int
		concurrency int
		timeoutMs   int
	)

	pflag.StringVarP(&mode, "mode", "m", "mixed_prompt", "benchmark scenario name")
	pflag.StringVarP(&target, "target", "t", "http://127.0.0.1:8800", "benchmark inference target address")
	pflag.StringVar(&metricsURL, "metrics-url", "", "metrics endpoint address, default is <target>/metrics")
	pflag.IntVarP(&requests, "requests", "r", 0, "request count override")
	pflag.IntVarP(&concurrency, "concurrency", "c", 0, "concurrency override")
	pflag.IntVar(&timeoutMs, "timeout-ms", 0, "request timeout override in ms")
	pflag.Parse()

	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	scenario, err := ScenarioPreset(mode)
	if err != nil {
		logger.Fatal("invalid scenario", zap.Error(err))
	}
	scenario.Target = target
	if metricsURL == "" {
		scenario.MetricsURL = fmt.Sprintf("%s/metrics", target)
	} else {
		scenario.MetricsURL = metricsURL
	}
	if requests > 0 {
		scenario.Requests = requests
	}
	if concurrency > 0 {
		scenario.Concurrency = concurrency
	}
	if timeoutMs > 0 {
		scenario.Timeout = time.Duration(timeoutMs) * time.Millisecond
	}

	result, err := RunScenario(logger, scenario)
	if err != nil {
		logger.Fatal("benchmark failed", zap.Error(err))
	}

	printResult(os.Stdout, result)
	logger.Sync()
	time.Sleep(50 * time.Millisecond)
}
