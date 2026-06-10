package conf

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Conf struct {
	Server    ServerConf     `koanf:"server"`
	Executors []ExecutorConf `koanf:"executors"`
}

type ServerConf struct {
	Address        string   `koanf:"address"`
	AdminPort      uint64   `koanf:"adminPort"`
	AllowedOrigins []string `koanf:"allowedOrigins"`
	ScheduleConf   ScheduleConf
}

type ScheduleConf struct {
	QueueConf QueueConf

	MaxBatchSeq               uint32 `koanf:"maxBatchSeqs"`
	MaxBatchTokens            uint32 `koanf:"maxBatchTokens"`
	LongPrefillTokenThreshold uint32 `koanf:"longPrefillTokenThreshold"`
	MaxPartialPrefills        uint32 `koanf:"maxPartialPrefills"`
	MaxLongPartialPrefills    uint32 `koanf:"maxLongPartialPrefills"`
	MaxScheduleDelay          uint64 `koanf:"maxScheduleDelay"`
}

type QueueConf struct {
	QueueLength uint32 `koanf:"queueLength"`
}

func (s ScheduleConf) ScheduleDelay() time.Duration {
	return time.Duration(s.MaxScheduleDelay) * time.Millisecond
}

type ExecutorConf struct {
	ID      string   `koanf:"id"`
	Kind    string   `koanf:"kind"`
	Address []string `koanf:"address"`
	Enabled bool     `koanf:"enabled"`
}

func NewConfFromPath(path string) (*Conf, error) {
	ext := filepath.Ext(path)
	var parser koanf.Parser
	switch ext {
	case ".toml":
		parser = toml.Parser()
	case ".yaml", ".yml":
		parser = yaml.Parser()
	case ".json":
		parser = json.Parser()
	default:
		return nil, fmt.Errorf("conf: unsupported file type: %s", ext)
	}
	k := koanf.New(".")
	if err := k.Load(file.Provider(path), parser); err != nil {
		return nil, err
	}

	var cfg Conf
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
