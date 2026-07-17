package scheduler

import (
	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
	"github.com/qujing226/kvtide/internal/model"
)

type batchBudget struct {
	// remainTokens is the number of tokens can be processed in one batch.
	remainTokens uint32
	// remainSeqs is the number of workItem can be processed in one batch.
	remainSeqs uint32
	// remainPrefill is the number of prefill workItem can be processed in one batch.
	remainPrefill uint32
	// remainLargePrefill is the number of large prefill chunks can be processed in one batch.
	remainLargePrefill uint32
}

const DefaultDecodeBudgetTokensPlanned uint32 = 1

// WorkBudgetCost calculate the cost of a workItem.
// For prefill, it is the (chunked) prompt length.
// For decode, it usually equals with 1(DefaultDecodeBudgetTokensPlanned).
func WorkBudgetCost(work *model.WorkItem) uint32 {
	switch work.Phase {
	case v1.WorkPhasePrefill:
		if work.NumNewTokens > 0 {
			return work.NumNewTokens
		}
		return uint32(len(work.TokenIDs))
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

	remainCost := cost - tokens
	if remainCost == 0 {
		return &chunk, nil
	}

	rest := *item
	rest.PrefillOffset = item.PrefillOffset + tokens
	rest.NumNewTokens = remainCost

	// Important: rest and chunk need to split
	rest.TokenIDs = rest.TokenIDs[tokens:]
	chunk.TokenIDs = chunk.TokenIDs[:tokens]
	return &chunk, &rest
}

func (s *scheduler) pickDecode(batch *[]*model.WorkItem, budget *batchBudget) {
	workItems, itemLength := s.decodeQueue.Dequeue(min(budget.remainSeqs, budget.remainTokens))
	if itemLength == 0 {
		budget.remainPrefill, budget.remainLargePrefill = budget.remainSeqs, budget.remainSeqs
		return
	}

	for _, work := range workItems {
		if budget.remainSeqs == 0 || budget.remainTokens == 0 {
			s.requeueWork(work)
			continue
		}
		if !s.blockManager.AllocateBlocks(work) {
			s.requeueWork(work)
			continue
		}
		*batch = append(*batch, work)
		budget.remainTokens--
		budget.remainSeqs--
	}
}

func (s *scheduler) pickSmallPrefill(batch *[]*model.WorkItem, budget *batchBudget) {
	maxScan := s.prefillQueueSmall.Length()
	for scanned := uint32(0); scanned < maxScan && budget.remainSeqs > 0 && budget.remainTokens > 0 && budget.remainPrefill > 0; scanned++ {
		small, ok := s.prefillQueueSmall.Peek()
		if !ok {
			return
		}
		cost := WorkBudgetCost(small)
		if cost > budget.remainTokens {
			break
		}
		small, ok = s.prefillQueueSmall.Pop()
		if !ok {
			continue
		}
		if !s.blockManager.AllocateBlocks(small) {
			s.requeueWork(small)
			continue
		}
		*batch = append(*batch, small)
		budget.remainTokens -= cost
		budget.remainSeqs--
		budget.remainPrefill--
	}
}

func (s *scheduler) pickLargePrefill(batch *[]*model.WorkItem, budget *batchBudget) {
	maxScan := s.prefillQueueLarge.Length()
	for scanned := uint32(0); scanned < maxScan && budget.remainSeqs > 0 && budget.remainTokens > 0 && budget.remainPrefill > 0 && budget.remainLargePrefill > 0; scanned++ {
		large, ok := s.prefillQueueLarge.Pop()
		if !ok {
			return
		}

		cost := WorkBudgetCost(large)
		scheduledTokens := min(cost, budget.remainTokens)
		chunk, _ := splitPrefillChunk(large, scheduledTokens)
		if !s.blockManager.AllocateBlocks(chunk) {
			s.requeueWork(large)
			continue
		}

		*batch = append(*batch, chunk)
		budget.remainTokens -= scheduledTokens
		budget.remainSeqs--
		budget.remainPrefill--
		budget.remainLargePrefill--
	}
}
