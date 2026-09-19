package block

import (
	"testing"

	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
	"github.com/qujing226/kvtide/internal/metrics"
	"github.com/qujing226/kvtide/internal/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRegistryBuildsIndependentManagersInStableOrder(t *testing.T) {
	blockRegistry, err := NewRegistry(
		zap.NewNop().Sugar(),
		metrics.NewMetrics(),
		map[string]*model.ExecutorStats{
			"executor-b": {ExecutorId: "executor-b", BlockSize: 16, NumKvBlocks: 8},
			"executor-a": {ExecutorId: "executor-a", BlockSize: 16, NumKvBlocks: 4},
		},
	)
	require.NoError(t, err)

	r := blockRegistry.(*registry)
	executorA := r.managers["executor-a"]
	executorB := r.managers["executor-b"]
	require.NotSame(t, executorA, executorB)
	require.Len(t, executorA.blocks, 4)
	require.Len(t, executorB.blocks, 8)

	ids := blockRegistry.ExecutorIDs()
	require.Equal(t, []string{"executor-a", "executor-b"}, ids)
	ids[0] = "changed-by-caller"
	require.Equal(
		t,
		[]string{"executor-a", "executor-b"},
		blockRegistry.ExecutorIDs(),
	)
}

func TestRegistryRejectsInvalidExecutorRuntimes(t *testing.T) {
	tests := []struct {
		name     string
		runtimes map[string]*model.ExecutorStats
	}{
		{name: "empty registry"},
		{
			name: "missing runtime",
			runtimes: map[string]*model.ExecutorStats{
				"executor-a": nil,
			},
		},
		{
			name: "runtime identity mismatch",
			runtimes: map[string]*model.ExecutorStats{
				"executor-a": {ExecutorId: "executor-b", BlockSize: 16, NumKvBlocks: 4},
			},
		},
		{
			name: "invalid manager config",
			runtimes: map[string]*model.ExecutorStats{
				"executor-a": {ExecutorId: "executor-a", BlockSize: 0, NumKvBlocks: 4},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewRegistry(
				zap.NewNop().Sugar(),
				metrics.NewMetrics(),
				tt.runtimes,
			)
			require.Error(t, err)
		})
	}
}

func TestRegistryRejectsUnknownExecutor(t *testing.T) {
	registry, err := NewRegistry(
		zap.NewNop().Sugar(),
		metrics.NewMetrics(),
		map[string]*model.ExecutorStats{
			"executor-a": {ExecutorId: "executor-a", BlockSize: 16, NumKvBlocks: 4},
		},
	)
	require.NoError(t, err)

	err = registry.Commit("missing", "work-1")
	require.Error(t, err)
}

func TestRegistryRoutesBlockLifecycleByExecutorID(t *testing.T) {
	registry, err := NewRegistry(
		zap.NewNop().Sugar(),
		metrics.NewMetrics(),
		map[string]*model.ExecutorStats{
			"executor-a": {ExecutorId: "executor-a", BlockSize: 2, NumKvBlocks: 4},
			"executor-b": {ExecutorId: "executor-b", BlockSize: 2, NumKvBlocks: 4},
		},
	)
	require.NoError(t, err)

	req := &model.Request{
		RequestId:  "request-1",
		ExecutorID: "executor-b",
		CacheSalt:  "shared-prefix",
		TokenIDs:   []uint32{1, 2, 3},
	}
	match, err := registry.MatchPrefix(req)
	require.NoError(t, err)
	require.False(t, match.Hit)

	work := &model.WorkItem{
		WorkId:       "work-1",
		RequestId:    req.RequestId,
		ExecutorID:   req.ExecutorID,
		Phase:        v1.WorkPhasePrefill,
		Cache:        match,
		TokenIDs:     req.TokenIDs,
		NumNewTokens: 3,
	}
	allocated, err := registry.AllocateBlocks(work)
	require.NoError(t, err)
	require.True(t, allocated)
	require.NoError(t, registry.Commit("executor-b", work.WorkId))
	require.NoError(t, registry.FreeRequest("executor-b", req.RequestId))

	followup := &model.Request{
		RequestId:  "request-2",
		ExecutorID: "executor-b",
		CacheSalt:  "shared-prefix",
		TokenIDs:   []uint32{1, 2, 3},
	}
	match, err = registry.MatchPrefix(followup)
	require.NoError(t, err)
	require.True(t, match.Hit)
}

func TestProbePrefixesReportsEachExecutorInStableOrder(t *testing.T) {
	blockRegistry := newTransferRegistry(t, 4)
	hashes := cacheTransferSource(t, blockRegistry)

	candidates := blockRegistry.ProbePrefixes(&model.Request{
		RequestId: "probe-request",
		CacheSalt: "shared-prefix",
		TokenIDs:  []uint32{1, 2, 3, 4, 5},
	})

	require.Equal(t, []model.PrefixCandidate{
		{ExecutorID: "destination"},
		{
			ExecutorID:    "source",
			CachedTokens:  4,
			MatchedHashes: hashes[:2],
		},
	}, candidates)
}

func TestProbePrefixesDoesNotAcquireOrBindCachedBlocks(t *testing.T) {
	blockRegistry := newTransferRegistry(t, 4)
	r := blockRegistry.(*registry)
	cacheTransferSource(t, blockRegistry)
	req := &model.Request{
		RequestId: "probe-request",
		CacheSalt: "shared-prefix",
		TokenIDs:  []uint32{1, 2, 3, 4, 5},
	}

	blockRegistry.ProbePrefixes(req)

	require.Nil(t, req.Cache)
	require.Empty(t, req.ExecutorID)
	require.NotContains(t, r.managers["source"].requestBlocks, req.RequestId)
	require.NotContains(t, r.managers["destination"].requestBlocks, req.RequestId)
	require.Equal(t, uint32(4), r.managers["source"].freeCount)
	require.Equal(t, uint32(4), r.managers["destination"].freeCount)
	require.Equal(t, uint32(0), r.managers["source"].blocks[0].RefCount)
	require.Equal(t, uint32(0), r.managers["source"].blocks[1].RefCount)
}

func TestPrepareTransferPinsSourceAndReservesDestinationBlocks(t *testing.T) {
	blockRegistry := newTransferRegistry(t, 4)
	r := blockRegistry.(*registry)
	hashes := cacheTransferSource(t, blockRegistry)[:2]

	plan, err := blockRegistry.PrepareTransfer(
		"transfer-1",
		"source",
		"destination",
		hashes,
	)

	require.NoError(t, err)
	require.Equal(t, "transfer-1", plan.TransferID)
	require.Equal(t, "source", plan.SourceExecutorID)
	require.Equal(t, "destination", plan.DestinationExecutorID)
	require.Equal(t, hashes, plan.BlockHashes)
	require.Equal(t, []uint32{0, 1}, plan.SourceBlockIDs)
	require.Equal(t, []uint32{0, 1}, plan.DestinationBlockIDs)
	require.Equal(t, uint32(2), r.managers["source"].freeCount)
	require.Equal(t, uint32(2), r.managers["destination"].freeCount)
}

func TestCommitTransferPublishesDestinationPrefixAndReleasesReservations(t *testing.T) {
	blockRegistry := newTransferRegistry(t, 4)
	r := blockRegistry.(*registry)
	hashes := cacheTransferSource(t, blockRegistry)[:2]
	plan, err := blockRegistry.PrepareTransfer(
		"transfer-1",
		"source",
		"destination",
		hashes,
	)
	require.NoError(t, err)

	require.NoError(t, blockRegistry.CommitTransfer(plan.TransferID))
	require.Equal(t, uint32(4), r.managers["source"].freeCount)
	require.Equal(t, uint32(4), r.managers["destination"].freeCount)

	match, err := blockRegistry.MatchPrefix(&model.Request{
		RequestId:  "destination-request",
		ExecutorID: "destination",
		CacheSalt:  "shared-prefix",
		TokenIDs:   []uint32{1, 2, 3, 4, 5},
	})
	require.NoError(t, err)
	require.True(t, match.Hit)
	require.Equal(t, uint32(4), match.CachedTokens)
	require.Equal(t, plan.DestinationBlockIDs, match.BlockIDs)
}

func TestRollbackTransferRestoresSourceAndDestinationBlocks(t *testing.T) {
	blockRegistry := newTransferRegistry(t, 4)
	r := blockRegistry.(*registry)
	hashes := cacheTransferSource(t, blockRegistry)[:2]
	plan, err := blockRegistry.PrepareTransfer(
		"transfer-1",
		"source",
		"destination",
		hashes,
	)
	require.NoError(t, err)

	require.NoError(t, blockRegistry.RollbackTransfer(plan.TransferID))
	require.Equal(t, uint32(4), r.managers["source"].freeCount)
	require.Equal(t, uint32(4), r.managers["destination"].freeCount)
	require.Empty(t, r.managers["destination"].cachedBlocks)
}

func TestPrepareTransferReleasesSourcePinsWhenDestinationIsFull(t *testing.T) {
	blockRegistry := newTransferRegistry(t, 1)
	r := blockRegistry.(*registry)
	hashes := cacheTransferSource(t, blockRegistry)[:2]

	_, err := blockRegistry.PrepareTransfer(
		"transfer-1",
		"source",
		"destination",
		hashes,
	)

	require.Error(t, err)
	require.Equal(t, uint32(4), r.managers["source"].freeCount)
	require.Equal(t, uint32(1), r.managers["destination"].freeCount)
}

func newTransferRegistry(t *testing.T, destinationBlocks uint32) Registry {
	t.Helper()
	blockRegistry, err := NewRegistry(
		zap.NewNop().Sugar(),
		metrics.NewMetrics(),
		map[string]*model.ExecutorStats{
			"source":      {ExecutorId: "source", RuntimeEpoch: 7, BlockSize: 2, NumKvBlocks: 4},
			"destination": {ExecutorId: "destination", RuntimeEpoch: 11, BlockSize: 2, NumKvBlocks: destinationBlocks},
		},
	)
	require.NoError(t, err)
	return blockRegistry
}

func cacheTransferSource(t *testing.T, blockRegistry Registry) []string {
	t.Helper()
	req := &model.Request{
		RequestId:  "source-request",
		ExecutorID: "source",
		CacheSalt:  "shared-prefix",
		TokenIDs:   []uint32{1, 2, 3, 4, 5, 6},
	}
	match, err := blockRegistry.MatchPrefix(req)
	require.NoError(t, err)
	work := &model.WorkItem{
		WorkId:       "source-work",
		RequestId:    req.RequestId,
		ExecutorID:   req.ExecutorID,
		Phase:        v1.WorkPhasePrefill,
		Cache:        match,
		TokenIDs:     req.TokenIDs,
		NumNewTokens: uint32(len(req.TokenIDs)),
	}
	allocated, err := blockRegistry.AllocateBlocks(work)
	require.NoError(t, err)
	require.True(t, allocated)
	require.NoError(t, blockRegistry.Commit(req.ExecutorID, work.WorkId))
	require.NoError(t, blockRegistry.FreeRequest(req.ExecutorID, req.RequestId))
	return match.HashesTotal
}
