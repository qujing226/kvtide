package scheduler

import (
	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/model"
)

const DefaultDecodeBudgetTokensPlanned uint32 = 1

func WorkBudgetCost(work *model.WorkItem) uint32 {
	switch work.Phase {
	case v1.WorkPhasePrefill:
		if work.NumNewTokens > 0 {
			return work.NumNewTokens
		}
		return work.PromptTokens
	case v1.WorkPhaseDecode:
		return DefaultDecodeBudgetTokensPlanned
	default:
		return 0
	}
}

func splitPrefillChunk(item *model.WorkItem, tokens uint32) (*model.WorkItem, *model.WorkItem) {
	cost := WorkBudgetCost(item)
	if tokens > cost {
		tokens = cost
	}
	chunk := *item
	chunk.NumNewTokens = tokens

	processed := item.PrefillOffset + tokens
	remainCost := cost - tokens
	if remainCost == 0 {
		return &chunk, nil
	}

	rest := *item
	rest.PrefillOffset = processed
	rest.NumNewTokens = remainCost
	return &chunk, &rest
}
