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
	Deadline  time.Time
	MaxTokens uint32
	Model     string

	CacheHit bool

	// TokenIDs in WorkItem is a part of TokenIDs in Request.
	TokenIDs      []uint32
	TokenCntTotal uint32

	// BlockAllocation is the KV block reservation made for this WorkItem.
	// It is committed after successful execution or rolled back if the work is not executed.
	BlockAllocation *BlockAllocation

	// PrefillOffset is the number of prompt tokens that already have KV cache.
	PrefillOffset uint32
	// GeneratedTokens is the number of tokens already generated in decode.
	GeneratedTokens uint32
	// NumNewTokens is the number of tokens this WorkItem plans to process.
	NumNewTokens uint32

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
