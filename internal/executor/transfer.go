package executor

import (
	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
	"github.com/qujing226/kvtide/internal/model"
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
				TokenIds:        work.TokenIDs[len(work.TokenIDs)-int(work.NumNewTokens):],
				ComputedTokens:  work.TokenCntTotal - work.NumNewTokens,
				GeneratedTokens: work.GeneratedTokens,
				NumNewTokens:    work.NumNewTokens,
				KvBlocks: &v1.KVBlockMetadata{
					BlockSize:       work.BlockAllocation.BlockSize,
					BlockTable:      work.BlockAllocation.BlockTable,
					AllocatedBlocks: work.BlockAllocation.AllocatedBlocks,
				},
				Sample: true,
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
				Sample: work.PrefillOffset+work.NumNewTokens >= work.TokenCntTotal,
			})
		}
	}
	return req
}
