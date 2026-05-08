package scheduler

import (
	"testing"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/stretchr/testify/require"
)

func TestWorkBudgetCostPrefillUsesScheduledPrefillTokens(t *testing.T) {
	work := &model.WorkItem{
		Phase:        v1.WorkPhasePrefill,
		PromptTokens: 2048,
		NumNewTokens: 512,
	}

	require.Equal(t, uint64(512), WorkBudgetCost(work))
}

func TestWorkBudgetCostPrefillFallsBackToPromptTokens(t *testing.T) {
	work := &model.WorkItem{
		Phase:        v1.WorkPhasePrefill,
		PromptTokens: 2048,
	}

	require.Equal(t, uint64(2048), WorkBudgetCost(work))
}

func TestWorkBudgetCostDecodeIsOneTokenPerStep(t *testing.T) {
	work := &model.WorkItem{
		Phase: v1.WorkPhaseDecode,
	}

	require.Equal(t, uint64(1), WorkBudgetCost(work))
}

func TestSplitPrefillChunkReturnsOnlyChunkWhenComplete(t *testing.T) {
	work := &model.WorkItem{
		Phase:         v1.WorkPhasePrefill,
		PromptTokens:  1000,
		PrefillOffset: 500,
		NumNewTokens:  500,
	}

	chunk, rest := splitPrefillChunk(work, 500)

	require.NotNil(t, chunk)
	require.Nil(t, rest)
	require.Equal(t, uint64(500), chunk.PrefillOffset)
	require.Equal(t, uint64(500), chunk.NumNewTokens)
}

func TestSplitPrefillChunkReturnsRemainingWork(t *testing.T) {
	work := &model.WorkItem{
		Phase:         v1.WorkPhasePrefill,
		PromptTokens:  1000,
		PrefillOffset: 200,
		NumNewTokens:  800,
	}

	chunk, rest := splitPrefillChunk(work, 300)

	require.NotNil(t, chunk)
	require.NotNil(t, rest)
	require.Equal(t, uint64(200), chunk.PrefillOffset)
	require.Equal(t, uint64(300), chunk.NumNewTokens)
	require.Equal(t, uint64(500), rest.PrefillOffset)
	require.Equal(t, uint64(500), rest.NumNewTokens)
}

func TestSplitPrefillChunkCapsTokensAtRemainingCost(t *testing.T) {
	work := &model.WorkItem{
		Phase:         v1.WorkPhasePrefill,
		PromptTokens:  1000,
		PrefillOffset: 700,
		NumNewTokens:  300,
	}

	chunk, rest := splitPrefillChunk(work, 1024)

	require.NotNil(t, chunk)
	require.Nil(t, rest)
	require.Equal(t, uint64(700), chunk.PrefillOffset)
	require.Equal(t, uint64(300), chunk.NumNewTokens)
}
