package executor

import (
	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/model"
)

func BatchToExecute(epoch uint32, batch *model.Batch) *v1.ExecuteBatchRequest {
	req := &v1.ExecuteBatchRequest{
		BatchId:      batch.BatchID,
		RuntimeEpoch: epoch,
	}
	for _, work := range batch.Items {
		if work.Phase == v1.WorkPhaseDecode {
			req.Items = append(req.Items, &v1.ExecuteItem{
				WorkId:          work.WorkId,
				RequestId:       work.RequestId,
				Phase:           v1.WorkPhaseDecode,
				TokenIds:        work.TokenIDs,
				ComputedTokens:  work.PrefillOffset + work.GeneratedTokens,
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
				TokenIds:        work.TokenIDs,
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
