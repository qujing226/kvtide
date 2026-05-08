package executor

import (
	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/model"
)

func BatchToExecute(batch *model.Batch) *v1.ExecuteBatchRequest {
	req := &v1.ExecuteBatchRequest{
		BatchId: batch.BatchID,
	}
	for _, r := range batch.Items {
		hasPrompt := r.Phase == v1.WorkPhasePrefill && r.PrefillOffset == 0 && r.Prompt != ""
		prompt := ""
		if hasPrompt {
			prompt = r.Prompt
		}

		if r.Phase == v1.WorkPhaseDecode {
			req.Items = append(req.Items, &v1.ExecuteItem{
				WorkId:          r.WorkId,
				RequestId:       r.RequestId,
				Phase:           v1.WorkPhaseDecode,
				Prompt:          "",
				HasPrompt:       false,
				PromptTokens:    uint32(r.PromptTokens),
				ComputedTokens:  uint32(r.PromptTokens + r.GeneratedTokens),
				GeneratedTokens: uint32(r.GeneratedTokens),
				NumNewTokens:    uint32(r.NumNewTokens),
			})
		} else {
			req.Items = append(req.Items, &v1.ExecuteItem{
				WorkId:          r.WorkId,
				RequestId:       r.RequestId,
				Phase:           v1.WorkPhasePrefill,
				Prompt:          prompt,
				HasPrompt:       hasPrompt,
				PromptTokens:    uint32(r.PromptTokens),
				ComputedTokens:  uint32(r.PrefillOffset),
				GeneratedTokens: 0,
				NumNewTokens:    uint32(r.NumNewTokens),
			})
		}
	}
	return req
}
