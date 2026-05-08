package model

import (
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
)

type Request struct {
	RequestId string
	Model     string
	Prompt    string
	MaxTokens uint64
	Timeout   time.Duration
	Deadline  time.Time

	CacheKey string

	PromptTokens    uint64
	ComputedTokens  uint64
	GeneratedTokens uint64
	Phase           RequestPhase
	FinishReason    v1.FinishReason

	OutputText   string
	CreatedAt    time.Time
	FirstTokenAt time.Time
	FinishedAt   time.Time

	Usage Usage

	Labels map[string]string
}

// GenerateOutput stage2
type GenerateOutput struct {
	RequestId    string
	Index        uint64
	DeltaText    string
	FinishReason v1.FinishReason
	Done         bool
	Usage        Usage
	Timing       Timing
	BatchID      string
	BatchSize    uint32
	ExecutorId   string
	Err          error
}

type WorkItem struct {
	WorkId    string
	RequestId string
	Phase     v1.WorkPhase

	Model     string
	Prompt    string
	MaxTokens uint64
	Deadline  time.Time

	PromptTokens    uint64
	PrefillOffset   uint64 // 已经 prefill 到第几个 token
	GeneratedTokens uint64 // decode 已经生成多少 token
	NumNewTokens    uint64 // 本轮计划 prefill 或 decode 多少 token

	CacheHit bool

	EnqueuedAt time.Time
	ReadyAt    time.Time
}

type Event struct {
	WorkId     string
	RequestId  string
	BatchId    string
	ExecutorId string

	Type v1.EventType

	ChunkIndex uint64
	DeltaText  string
	Done       bool

	Usage        Usage
	Timing       Timing
	FinishReason v1.FinishReason

	At  time.Time
	Err error
}

type RuntimeStats struct {
	PrefillQueueLength uint64
	DecodeQueueLength  uint64
	ActiveRequests     uint64
	InflightBatches    uint64
	BusyExecutors      uint64
	IdleExecutors      uint64
}
