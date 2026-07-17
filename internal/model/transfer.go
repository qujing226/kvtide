package model

import (
	"time"

	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
)

func ProtoMsgToModel(in *v1.GenerateRequest) (*Request, error) {
	var (
		CacheSalt string
	)
	if in.UserId == "" {
		CacheSalt = "request:" + in.RequestId
	} else {
		CacheSalt = "user:" + in.UserId
	}
	modelID, err := ParseModelID(in.ModelId)
	if err != nil {
		return nil, err
	}
	out := &Request{
		RequestId:       in.RequestId,
		UserId:          in.UserId,
		ModelID:         modelID,
		Prompt:          in.Prompt,
		MaxTokens:       in.MaxTokens,
		Timeout:         time.Duration(int64(in.TimeoutMs)) * time.Millisecond,
		Deadline:        time.Now().Add(time.Duration(in.TimeoutMs) * time.Millisecond),
		CacheSalt:       CacheSalt,
		PromptTokens:    0,
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
		InputTokens:  in.Usage.InputTokens,
		OutputTokens: in.Usage.OutputTokens,
		TotalTokens:  in.Usage.TotalTokens,
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

func RuntimeProtoToModel(res *v1.GetRuntimeResponse) *ExecutorStats {
	return &ExecutorStats{
		ExecutorId:           res.ExecutorId,
		RuntimeEpoch:         res.RuntimeEpoch,
		ModelId:              res.ModelId,
		ModelType:            res.ModelType,
		Dtype:                res.Dtype,
		DeviceType:           res.DeviceType,
		TensorParallelSize:   res.TensorParallelSize,
		BlockSize:            res.BlockSize,
		NumKvBlocks:          res.NumKvBlocks,
		NumHiddenLayers:      res.NumHiddenLayers,
		NumKvHeads:           res.NumKvHeads,
		HeadDim:              res.HeadDim,
		TotalMemoryBytes:     res.TotalMemoryBytes,
		AvailableMemoryBytes: res.AvailableMemoryBytes,
		KVCacheBytes:         res.KvCacheBytes,
	}
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
