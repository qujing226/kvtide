package block

import (
	"testing"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestAllocateSlotsForPrefillUsesTotalKVLength(t *testing.T) {
	m := NewManager(zap.NewNop().Sugar()).(*manager)

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
	require.Equal(t, uint32(0), m.blocks[0].TokenCount)
	require.Equal(t, uint32(0), m.blocks[1].TokenCount)
	require.Equal(t, uint32(0), m.blocks[2].TokenCount)

	m.Commit(allocation)
	require.Equal(t, uint32(16), m.blocks[0].TokenCount)
	require.Equal(t, uint32(16), m.blocks[1].TokenCount)
	require.Equal(t, uint32(8), m.blocks[2].TokenCount)

	stats := m.Stats()
	require.Equal(t, uint64(TmpTotalBlocks-3), stats.FreeBlocks)
	require.Equal(t, uint64(3), stats.UsedBlocks)
}

func TestAllocateSlotsReusesPartiallyFilledPrefillBlock(t *testing.T) {
	m := NewManager(zap.NewNop().Sugar()).(*manager)

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
	require.Equal(t, uint32(0), m.blocks[0].TokenCount)

	m.Commit(first)
	require.Equal(t, uint32(8), m.blocks[0].TokenCount)

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
	require.Equal(t, uint32(8), m.blocks[0].TokenCount)

	m.Commit(second)
	require.Equal(t, uint32(16), m.blocks[0].TokenCount)
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
