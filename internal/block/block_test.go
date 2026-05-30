package block

import (
	"testing"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestAllocateSlotsForPrefillUsesTotalKVLength(t *testing.T) {
	m := NewManager(zap.NewNop().Sugar())

	allocation, ok := m.AllocateSlots(&model.WorkItem{
		WorkId:        "work-1",
		RequestId:     "req-1",
		Phase:         v1.WorkPhasePrefill,
		PrefillOffset: 0,
		NumNewTokens:  40,
	})

	require.True(t, ok)
	require.Equal(t, uint32(3), allocation.RequiredBlocks)
	require.Equal(t, []uint32{0, 1, 2}, allocation.AllocatedBlocks)
	require.Equal(t, []uint32{0, 1, 2}, allocation.BlockTable)

	stats := m.Stats()
	require.Equal(t, uint64(TmpTotalBlocks-3), stats.FreeBlocks)
	require.Equal(t, uint64(3), stats.UsedBlocks)
}

func TestAllocateSlotsReusesPartiallyFilledPrefillBlock(t *testing.T) {
	m := NewManager(zap.NewNop().Sugar())

	first, ok := m.AllocateSlots(&model.WorkItem{
		WorkId:        "work-1",
		RequestId:     "req-1",
		Phase:         v1.WorkPhasePrefill,
		PrefillOffset: 0,
		NumNewTokens:  8,
	})
	require.True(t, ok)
	require.Equal(t, uint32(1), first.RequiredBlocks)
	require.Equal(t, []uint32{0}, first.BlockTable)

	second, ok := m.AllocateSlots(&model.WorkItem{
		WorkId:        "work-2",
		RequestId:     "req-1",
		Phase:         v1.WorkPhasePrefill,
		PrefillOffset: 8,
		NumNewTokens:  8,
	})
	require.True(t, ok)
	require.Equal(t, uint32(0), second.RequiredBlocks)
	require.Empty(t, second.AllocatedBlocks)
	require.Equal(t, []uint32{0}, second.BlockTable)
}

func TestAllocateSlotsOnlyAllocatesWhenDecodeCrossesBlockBoundary(t *testing.T) {
	m := NewManager(zap.NewNop().Sugar())

	first, ok := m.AllocateSlots(&model.WorkItem{
		WorkId:        "prefill",
		RequestId:     "req-1",
		Phase:         v1.WorkPhasePrefill,
		PrefillOffset: 0,
		NumNewTokens:  16,
	})
	require.True(t, ok)
	require.Equal(t, uint32(1), first.RequiredBlocks)

	withinBlock, ok := m.AllocateSlots(&model.WorkItem{
		WorkId:          "decode-1",
		RequestId:       "req-1",
		Phase:           v1.WorkPhaseDecode,
		PromptTokens:    16,
		GeneratedTokens: 0,
		NumNewTokens:    1,
	})
	require.True(t, ok)
	require.Equal(t, uint32(1), withinBlock.RequiredBlocks)
	require.Equal(t, []uint32{0, 1}, withinBlock.BlockTable)

	stillWithinBlock, ok := m.AllocateSlots(&model.WorkItem{
		WorkId:          "decode-2",
		RequestId:       "req-1",
		Phase:           v1.WorkPhaseDecode,
		PromptTokens:    16,
		GeneratedTokens: 1,
		NumNewTokens:    1,
	})
	require.True(t, ok)
	require.Equal(t, uint32(0), stillWithinBlock.RequiredBlocks)
	require.Empty(t, stillWithinBlock.AllocatedBlocks)
	require.Equal(t, []uint32{0, 1}, stillWithinBlock.BlockTable)
}

func TestFreeRequestReturnsBlocksToFreeQueue(t *testing.T) {
	m := NewManager(zap.NewNop().Sugar())

	allocation, ok := m.AllocateSlots(&model.WorkItem{
		WorkId:        "work-1",
		RequestId:     "req-1",
		Phase:         v1.WorkPhasePrefill,
		PrefillOffset: 0,
		NumNewTokens:  40,
	})
	require.True(t, ok)
	require.Len(t, allocation.AllocatedBlocks, 3)

	m.FreeRequest("req-1")
	m.FreeRequest("req-1")

	stats := m.Stats()
	require.Equal(t, uint64(TmpTotalBlocks), stats.FreeBlocks)
	require.Equal(t, uint64(0), stats.UsedBlocks)
}
