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
		profileName string
		target      string
		metricsURL  string
	)

	pflag.StringVarP(&profileName, "profile", "p", "quick", "benchmark profile: quick or report")
	pflag.StringVarP(&target, "target", "t", "http://127.0.0.1:8800", "benchmark inference target address")
	pflag.StringVar(&metricsURL, "metrics-url", "", "metrics endpoint address, default is <target>/metrics")
	pflag.Parse()

	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	profile, err := ProfilePreset(profileName)
	if err != nil {
		logger.Fatal("invalid profile", zap.Error(err))
	}
	if metricsURL == "" {
		metricsURL = fmt.Sprintf("%s/metrics", target)
	}

	for _, scenario := range ScenariosForProfile(profile) {
		scenario.Target = target
		scenario.MetricsURL = metricsURL

		result, runErr := RunScenario(logger, scenario)
		if runErr != nil {
			logger.Fatal("benchmark failed", zap.String("scenario", scenario.Name), zap.Error(runErr))
		}
		printResult(os.Stdout, result)
		if profile.Name == "quick" {
			if validationErr := ValidateQuickResult(result); validationErr != nil {
				logger.Fatal("benchmark regression", zap.Error(validationErr))
			}
		}
	}
	logger.Sync()
	time.Sleep(50 * time.Millisecond)
}
