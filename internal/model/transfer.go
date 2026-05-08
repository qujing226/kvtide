package model

import (
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
)

func ProtoMsgToModel(in *v1.GenerateRequest) (*Request, error) {
	out := &Request{
		RequestId:       in.RequestId,
		Model:           in.Model,
		Prompt:          in.Prompt,
		MaxTokens:       uint64(in.MaxTokens),
		Timeout:         time.Duration(int64(in.TimeoutMs)) * time.Millisecond,
		Deadline:        time.Now().Add(time.Duration(in.TimeoutMs) * time.Millisecond),
		CacheKey:        in.CacheKey,
		PromptTokens:    uint64(max(1, len(in.Prompt)/4)),
		ComputedTokens:  0,
		GeneratedTokens: 0,
		Phase:           0,
		FinishReason:    0,
		OutputText:      "",
		CreatedAt:       time.Time{},
		FirstTokenAt:    time.Time{},
		FinishedAt:      time.Time{},
		Usage:           Usage{},
		Labels:          in.Labels,
	}
	return out, nil
}

func ModelToProtoMsg(in *GenerateOutput) (*v1.GenerateResponse, error) {
	usage := &v1.Usage{
		InputTokens:  uint32(in.Usage.InputTokens),
		OutputTokens: uint32(in.Usage.OutputTokens),
		TotalTokens:  uint32(in.Usage.TotalTokens),
	}

	timing := &v1.Timing{
		QueueMs:     durationToMilliseconds(in.Timing.Queue),
		BatchWaitMs: durationToMilliseconds(in.Timing.BatchWait),
		ExecutionMs: durationToMilliseconds(in.Timing.Execution),
		TotalMs:     durationToMilliseconds(in.Timing.Total),
	}

	batch := &v1.BatchInfo{
		BatchId:   in.BatchID,
		BatchSize: in.BatchSize,
	}

	out := &v1.GenerateResponse{
		RequestId:    in.RequestId,
		OutputText:   in.DeltaText,
		FinishReason: in.FinishReason,
		Usage:        usage,
		Timing:       timing,
		Batch:        batch,
		ExecutorId:   in.ExecutorId,
		ErrorMessage: errorMessage(in.Err),
	}

	return out, nil
}

func ModelToProtoMsgStream(in *GenerateOutput) (*v1.GenerateResponseChunk, error) {
	out := &v1.GenerateResponseChunk{
		RequestId:    in.RequestId,
		Index:        uint32(in.Index),
		DeltaText:    in.DeltaText,
		Done:         in.Done,
		FinishReason: in.FinishReason,
		Usage: &v1.Usage{
			InputTokens:  uint32(in.Usage.InputTokens),
			OutputTokens: uint32(in.Usage.OutputTokens),
			TotalTokens:  uint32(in.Usage.TotalTokens),
		},
		ErrorMessage: errorMessage(in.Err),
	}

	return out, nil
}

func durationToMilliseconds(d time.Duration) uint32 {
	if d <= 0 {
		return 0
	}
	return uint32(d / time.Millisecond)
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
