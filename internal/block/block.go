package block

import "github.com/qujing226/mini-llm-serve/internal/model"

type Manager interface {
	MatchPrefix(req *model.Request) *model.PrefixMatch
	AllocateSlots(work *model.WorkItem) (*model.BlockAllocation, bool)
	Commit(allocation *model.BlockAllocation)
	Rollback(allocation *model.BlockAllocation)
	FreeRequest(requestID string)
	Stats() model.BlockStats
}
