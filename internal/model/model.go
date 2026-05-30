package model

import (
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
)

type Request struct {
	RequestId string
	Model     string
	Prompt    string
	MaxTokens uint32
	Timeout   time.Duration
	Deadline  time.Time

	CacheKey     string
	CachedTokens uint32
	CacheHit     bool
	TokenIDs     []uint32

	PromptTokens    uint32
	ComputedTokens  uint32
	GeneratedTokens uint32
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
	MaxTokens uint32
	Deadline  time.Time

	TokenIDs []uint32

	PromptTokens    uint32
	PrefillOffset   uint32 // 已经 prefill 到第几个 token
	GeneratedTokens uint32 // decode 已经生成多少 token
	NumNewTokens    uint32 // 本轮计划 prefill 或 decode 多少 token

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
