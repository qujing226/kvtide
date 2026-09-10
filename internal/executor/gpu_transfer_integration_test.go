package executor

import (
	"context"
	"net"
	"testing"
	"time"

	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
	"github.com/qujing226/kvtide/internal/block"
	"github.com/qujing226/kvtide/internal/conf"
	"github.com/qujing226/kvtide/internal/metrics"
	"github.com/qujing226/kvtide/internal/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	gpuSourceEndpoint      = "http://127.0.0.1:19991"
	gpuDestinationEndpoint = "http://127.0.0.1:19992"
)

func TestGPUKVTransferThroughEngine(t *testing.T) {
	requireGPUExecutor(t, "127.0.0.1:19991")
	requireGPUExecutor(t, "127.0.0.1:19992")

	logger := zap.NewNop().Sugar()
	source := newGPUExecutor(t, logger, "source", gpuSourceEndpoint)
	destination := newGPUExecutor(t, logger, "destination", gpuDestinationEndpoint)
	sourceRuntime := source.GetRuntimeStates()
	destinationRuntime := destination.GetRuntimeStates()
	requireCompatibleGPURuntimes(t, sourceRuntime, destinationRuntime)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	sourceBlocks := []uint32{2, 0}
	destinationPrefixBlocks := []uint32{1, 3}
	migratedBlocks := []uint32{1, 3, 4}
	referenceBlocks := []uint32{5, 6, 7}
	resetGPUBlocks(t, ctx, source, sourceBlocks)
	resetGPUBlocks(t, ctx, destination, []uint32{1, 3, 4, 5, 6, 7})
	defer cleanupGPUBlocks(t, source, sourceBlocks)
	defer cleanupGPUBlocks(t, destination, []uint32{1, 3, 4, 5, 6, 7})

	blockSize := sourceRuntime.BlockSize
	prefix := make([]uint32, 2*blockSize)
	for i := range prefix {
		prefix[i] = uint32(i%11 + 1)
	}
	suffix := []uint32{6, 10}
	hashes := engineBlockHashes(t, logger, sourceRuntime, prefix)

	sourcePrefill := executePrefill(
		t,
		ctx,
		source,
		"source-prefill",
		prefix,
		0,
		sourceBlocks,
		sourceBlocks,
	)

	manager := &executorManager{
		executors: map[string]Executor{
			"source":      source,
			"destination": destination,
		},
	}
	pushStarted := time.Now()
	pushResponse, err := manager.TriggerKVPush(ctx, &v1.TriggerKVPushRequest{
		TransferId:                  "single-gpu-transfer",
		SourceExecutorId:            sourceRuntime.ExecutorId,
		SourceRuntimeEpoch:          sourceRuntime.RuntimeEpoch,
		SourceBlockIds:              sourceBlocks,
		DestinationExecutorId:       destinationRuntime.ExecutorId,
		DestinationRuntimeEpoch:     destinationRuntime.RuntimeEpoch,
		DestinationTransferEndpoint: destinationRuntime.TransferEndpoint,
		KvCompatibilityId:           sourceRuntime.KVCompatibilityID,
		BlockHashes:                 hashes,
		DestinationBlockIds:         destinationPrefixBlocks,
	})
	pushElapsed := time.Since(pushStarted)
	require.NoError(t, err)
	require.Equal(t, "single-gpu-transfer", pushResponse.GetTransferId())
	require.Equal(t, sourceRuntime.ExecutorId, pushResponse.GetSourceExecutorId())
	require.Equal(t, sourceRuntime.RuntimeEpoch, pushResponse.GetSourceRuntimeEpoch())
	require.Equal(t, destinationRuntime.ExecutorId, pushResponse.GetDestinationExecutorId())
	require.Equal(t, destinationRuntime.RuntimeEpoch, pushResponse.GetDestinationRuntimeEpoch())

	migratedFirst := executePrefill(
		t,
		ctx,
		destination,
		"migrated-suffix",
		suffix,
		uint32(len(prefix)),
		migratedBlocks,
		[]uint32{4},
	)
	referenceTokens := append(append([]uint32(nil), prefix...), suffix...)
	referenceFirst := executePrefill(
		t,
		ctx,
		destination,
		"reference-prefill",
		referenceTokens,
		0,
		referenceBlocks,
		referenceBlocks,
	)
	require.Equal(t, referenceFirst.TokenId, migratedFirst.TokenId)

	migratedToken := migratedFirst.TokenId
	referenceToken := referenceFirst.TokenId
	contextLength := uint32(len(referenceTokens))
	for step := 0; step < 4; step++ {
		migrated := executeDecode(
			t,
			ctx,
			destination,
			"migrated-decode",
			migratedToken,
			contextLength,
			uint32(step+1),
			migratedBlocks,
		)
		reference := executeDecode(
			t,
			ctx,
			destination,
			"reference-decode",
			referenceToken,
			contextLength,
			uint32(step+1),
			referenceBlocks,
		)
		require.Equalf(
			t,
			reference.TokenId,
			migrated.TokenId,
			"generated token differs at decode step %d",
			step+1,
		)
		migratedToken = migrated.TokenId
		referenceToken = reference.TokenId
		contextLength++
	}

	t.Logf("source prefill execution: %s", sourcePrefill.Timing.Execution)
	t.Logf("KV push wall time: %s", pushElapsed)
	t.Logf("migrated suffix execution: %s", migratedFirst.Timing.Execution)
	t.Logf("reference prefill execution: %s", referenceFirst.Timing.Execution)
}

func requireGPUExecutor(t *testing.T, address string) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
	if err != nil {
		t.Skipf("GPU executor is not listening on %s", address)
	}
	require.NoError(t, conn.Close())
}

func newGPUExecutor(
	t *testing.T,
	logger *zap.SugaredLogger,
	executorID string,
	endpoint string,
) Executor {
	t.Helper()
	exec, err := newExecutor(logger, conf.ExecutorConf{
		ExecutorID: executorID,
		Address:    []string{endpoint},
		TimeoutMs:  120000,
	})
	require.NoError(t, err)
	return exec
}

func requireCompatibleGPURuntimes(
	t *testing.T,
	source *model.ExecutorStats,
	destination *model.ExecutorStats,
) {
	t.Helper()
	require.Equal(t, "cuda", source.DeviceType)
	require.Equal(t, "cuda", destination.DeviceType)
	require.Equal(t, "bfloat16", source.Dtype)
	require.Equal(t, source.Dtype, destination.Dtype)
	require.Equal(t, source.ModelId, destination.ModelId)
	require.Equal(t, source.ModelRevision, destination.ModelRevision)
	require.Equal(t, source.BlockSize, destination.BlockSize)
	require.Equal(t, source.NumHiddenLayers, destination.NumHiddenLayers)
	require.Equal(t, source.NumKvHeads, destination.NumKvHeads)
	require.Equal(t, source.HeadDim, destination.HeadDim)
	require.Equal(t, source.KVLayoutVersion, destination.KVLayoutVersion)
	require.Equal(t, source.KVCompatibilityID, destination.KVCompatibilityID)
	require.Greater(t, source.BlockSize, uint32(0))
	require.Greater(t, source.NumKvBlocks, uint32(7))
	require.Greater(t, destination.NumKvBlocks, uint32(7))
	require.Equal(t, gpuDestinationEndpoint, destination.TransferEndpoint)
}

func resetGPUBlocks(
	t *testing.T,
	ctx context.Context,
	exec Executor,
	blockIDs []uint32,
) {
	t.Helper()
	remote := exec.(*executor)
	_, err := remote.client.ReleaseBlocks(ctx, remote.runtime.RuntimeEpoch, blockIDs)
	require.NoError(t, err)
}

func cleanupGPUBlocks(t *testing.T, exec Executor, blockIDs []uint32) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	remote := exec.(*executor)
	if _, err := remote.client.ReleaseBlocks(
		ctx,
		remote.runtime.RuntimeEpoch,
		blockIDs,
	); err != nil {
		t.Errorf("release test blocks from %s: %v", remote.id, err)
	}
}

func engineBlockHashes(
	t *testing.T,
	logger *zap.SugaredLogger,
	runtime *model.ExecutorStats,
	tokens []uint32,
) []string {
	t.Helper()
	manager, err := block.NewManager(logger, metrics.NewMetrics(), block.Config{
		ExecutorID:   runtime.ExecutorId,
		RuntimeEpoch: runtime.RuntimeEpoch,
		BlockSize:    runtime.BlockSize,
		NumBlocks:    runtime.NumKvBlocks,
	})
	require.NoError(t, err)
	match := manager.MatchPrefix(&model.Request{
		RequestId: "single-gpu-prefix",
		CacheSalt: "single-gpu-transfer",
		TokenIDs:  tokens,
	})
	require.Len(t, match.HashesTotal, 2)
	return match.HashesTotal
}

func executePrefill(
	t *testing.T,
	ctx context.Context,
	exec Executor,
	name string,
	tokens []uint32,
	computedTokens uint32,
	blockTable []uint32,
	allocatedBlocks []uint32,
) *model.Event {
	t.Helper()
	return executeOne(t, ctx, exec, &model.WorkItem{
		WorkId:        name,
		RequestId:     name,
		Phase:         v1.WorkPhasePrefill,
		TokenIDs:      tokens,
		TokenCntTotal: computedTokens + uint32(len(tokens)),
		PrefillOffset: computedTokens,
		NumNewTokens:  uint32(len(tokens)),
		BlockAllocation: &model.BlockAllocation{
			BlockSize:       exec.GetRuntimeStates().BlockSize,
			BlockTable:      blockTable,
			AllocatedBlocks: allocatedBlocks,
		},
	})
}

func executeDecode(
	t *testing.T,
	ctx context.Context,
	exec Executor,
	name string,
	token uint32,
	computedTokens uint32,
	generatedTokens uint32,
	blockTable []uint32,
) *model.Event {
	t.Helper()
	return executeOne(t, ctx, exec, &model.WorkItem{
		WorkId:          name,
		RequestId:       name,
		Phase:           v1.WorkPhaseDecode,
		TokenIDs:        []uint32{token},
		TokenCntTotal:   computedTokens + 1,
		GeneratedTokens: generatedTokens,
		NumNewTokens:    1,
		BlockAllocation: &model.BlockAllocation{
			BlockSize:  exec.GetRuntimeStates().BlockSize,
			BlockTable: blockTable,
		},
	})
}

func executeOne(
	t *testing.T,
	ctx context.Context,
	exec Executor,
	work *model.WorkItem,
) *model.Event {
	t.Helper()
	events, err := exec.Execute(ctx, &model.Batch{
		BatchID:   work.WorkId,
		BatchSize: 1,
		Items:     []*model.WorkItem{work},
	})
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.NoError(t, events[0].Err)
	return events[0]
}
