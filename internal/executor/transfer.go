package executor

import (
	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/model"
)

func BatchToExecute(batch *model.Batch) *v1.ExecuteBatchRequest {
	req := &v1.ExecuteBatchRequest{
		BatchId: batch.BatchID,
	}
	for _, work := range batch.Items {
		hasPrompt := work.Phase == v1.WorkPhasePrefill && work.PrefillOffset == 0 && work.Prompt != ""
		prompt := ""
		if hasPrompt {
			prompt = work.Prompt
		}

		if work.Phase == v1.WorkPhaseDecode {
			req.Items = append(req.Items, &v1.ExecuteItem{
				WorkId:          work.WorkId,
				RequestId:       work.RequestId,
				Phase:           v1.WorkPhaseDecode,
				Prompt:          "",
				HasPrompt:       false,
				PromptTokens:    work.PromptTokens,
				ComputedTokens:  work.PromptTokens + work.GeneratedTokens,
				GeneratedTokens: work.GeneratedTokens,
				NumNewTokens:    work.NumNewTokens,
				KvBlocks: &v1.KVBlockMetadata{
					BlockSize:       work.BlockAllocation.BlockSize,
					BlockTable:      work.BlockAllocation.BlockTable,
					AllocatedBlocks: work.BlockAllocation.AllocatedBlocks,
				},
			})
		} else {
			req.Items = append(req.Items, &v1.ExecuteItem{
				WorkId:          work.WorkId,
				RequestId:       work.RequestId,
				Phase:           v1.WorkPhasePrefill,
				Prompt:          prompt,
				HasPrompt:       hasPrompt,
				PromptTokens:    work.PromptTokens,
				ComputedTokens:  work.PrefillOffset,
				GeneratedTokens: 0,
				NumNewTokens:    work.NumNewTokens,
				KvBlocks: &v1.KVBlockMetadata{
					BlockSize:       work.BlockAllocation.BlockSize,
					BlockTable:      work.BlockAllocation.BlockTable,
					AllocatedBlocks: work.BlockAllocation.AllocatedBlocks,
				},
			})
		}
	}
	return req
}
