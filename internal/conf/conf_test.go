package conf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewConfFromPathLoadsModelsAndExecutors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.toml")
	if err := os.WriteFile(path, []byte(`
[[models]]
modelId = "Qwen/Qwen3-0.6B"
modelPath = "./models/Qwen3-0.6B"

[[executors]]
executorId = "executor-qwen3-0.6B"
modelId = "Qwen/Qwen3-0.6B"
address = ["http://127.0.0.1:19991"]
timeoutMs = 120000
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := NewConfFromPath(path)
	if err != nil {
		t.Fatalf("new conf: %v", err)
	}
	if len(cfg.Models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(cfg.Models))
	}
	if cfg.Models[0].ModelPath != "./models/Qwen3-0.6B" {
		t.Fatalf("modelPath = %q", cfg.Models[0].ModelPath)
	}
	if len(cfg.Executors) != 1 {
		t.Fatalf("len(executors) = %d, want 1", len(cfg.Executors))
	}
}
