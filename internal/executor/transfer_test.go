package executor

import (
	"testing"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/stretchr/testify/require"
)

func TestBatchToExecuteBuildsPhaseSpecificInputs(t *testing.T) {
	allocation := &model.BlockAllocation{BlockSize: 16, BlockTable: []uint32{0, 1}}
	batch := &model.Batch{
		BatchID: "batch-1",
		Items: []*model.WorkItem{
			{
				WorkId:          "decode",
				RequestId:       "request-decode",
				Phase:           v1.WorkPhaseDecode,
				TokenIDs:        []uint32{10, 11, 12, 13},
				TokenCntTotal:   4,
				GeneratedTokens: 1,
				NumNewTokens:    1,
				BlockAllocation: allocation,
			},
			{
				WorkId:          "prefill-middle",
				RequestId:       "request-prefill-middle",
				Phase:           v1.WorkPhasePrefill,
				TokenIDs:        []uint32{17, 18, 19, 20},
				TokenCntTotal:   24,
				PrefillOffset:   16,
				NumNewTokens:    4,
				BlockAllocation: allocation,
			},
			{
				WorkId:          "prefill-final",
				RequestId:       "request-prefill-final",
				Phase:           v1.WorkPhasePrefill,
				TokenIDs:        []uint32{21, 22, 23, 24},
				TokenCntTotal:   24,
				PrefillOffset:   20,
				NumNewTokens:    4,
				BlockAllocation: allocation,
			},
		},
	}

	request := BatchToExecute(7, batch)
	require.Equal(t, uint32(7), request.RuntimeEpoch)
	require.Len(t, request.Items, 3)

	decode := request.Items[0]
	require.Equal(t, []uint32{13}, decode.TokenIds)
	require.Equal(t, uint32(3), decode.ComputedTokens)
	require.True(t, decode.Sample)

	middle := request.Items[1]
	require.Equal(t, uint32(16), middle.ComputedTokens)
	require.False(t, middle.Sample)

	final := request.Items[2]
	require.Equal(t, uint32(20), final.ComputedTokens)
	require.True(t, final.Sample)
}
