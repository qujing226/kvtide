package model

import (
	"time"
)

type Batch struct {
	BatchID   string
	BatchSize uint32
	CreateAt  time.Time
	Items     []*WorkItem
}

type Usage struct {
	InputTokens  uint32
	OutputTokens uint32
	TotalTokens  uint32
}

type Timing struct {
	Queue     time.Duration
	BatchWait time.Duration
	Execution time.Duration
	Total     time.Duration
}
