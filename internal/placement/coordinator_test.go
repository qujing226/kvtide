package placement

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
	"github.com/qujing226/kvtide/internal/block"
	"github.com/qujing226/kvtide/internal/metrics"
	"github.com/qujing226/kvtide/internal/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type executorControlStub struct {
	runtimes map[string]*model.ExecutorStats
	push     func(*v1.TriggerKVPushRequest) (*v1.TriggerKVPushResponse, error)
}

func (s *executorControlStub) GetRuntimeStates() map[string]*model.ExecutorStats {
	return s.runtimes
}

func (s *executorControlStub) TriggerKVPush(
	_ context.Context,
	request *v1.TriggerKVPushRequest,
) (*v1.TriggerKVPushResponse, error) {
	return s.push(request)
}

func TestReplicateCommitsTransferredPrefixAfterExecutorConfirms(t *testing.T) {
	registry, hashes := transferRegistryWithCachedSource(t, 2)
	var received *v1.TriggerKVPushRequest
	control := &executorControlStub{
		runtimes: compatibleTransferRuntimes(),
		push: func(request *v1.TriggerKVPushRequest) (*v1.TriggerKVPushResponse, error) {
			received = request
			return &v1.TriggerKVPushResponse{
				TransferId:              request.TransferId,
				SourceExecutorId:        "source",
				SourceRuntimeEpoch:      7,
				DestinationExecutorId:   "destination",
				DestinationRuntimeEpoch: 11,
			}, nil
		},
	}
	coordinator := NewCoordinator(registry, control)

	plan, err := coordinator.Replicate(
		context.Background(),
		"source",
		"destination",
		hashes[:2],
	)

	require.NoError(t, err)
	require.NotEmpty(t, plan.TransferID)
	require.Equal(t, plan.TransferID, received.TransferId)
	require.Equal(t, "source", received.SourceExecutorId)
	require.Equal(t, uint32(7), received.SourceRuntimeEpoch)
	require.Equal(t, []uint32{0, 1}, received.SourceBlockIds)
	require.Equal(t, "destination", received.DestinationExecutorId)
	require.Equal(t, uint32(11), received.DestinationRuntimeEpoch)
	require.Equal(t, "http://destination:19991", received.DestinationTransferEndpoint)
	require.Equal(t, "qwen3-bf16-layout-v1", received.KvCompatibilityId)
	require.Equal(t, hashes[:2], received.BlockHashes)
	require.Equal(t, []uint32{0, 1}, received.DestinationBlockIds)

	match, err := registry.MatchPrefix(&model.Request{
		RequestId:  "destination-request",
		ExecutorID: "destination",
		CacheSalt:  "shared-prefix",
		TokenIDs:   []uint32{1, 2, 3, 4, 5},
	})
	require.NoError(t, err)
	require.True(t, match.Hit)
	require.Equal(t, uint32(4), match.CachedTokens)
}

func TestReplicateRollsBackReservationsWhenExecutorPushFails(t *testing.T) {
	registry, hashes := transferRegistryWithCachedSource(t, 2)
	attempts := 0
	control := &executorControlStub{
		runtimes: compatibleTransferRuntimes(),
		push: func(request *v1.TriggerKVPushRequest) (*v1.TriggerKVPushResponse, error) {
			attempts++
			if attempts == 1 {
				return nil, errors.New("network failed")
			}
			return &v1.TriggerKVPushResponse{
				TransferId:              request.TransferId,
				SourceExecutorId:        request.SourceExecutorId,
				SourceRuntimeEpoch:      request.SourceRuntimeEpoch,
				DestinationExecutorId:   request.DestinationExecutorId,
				DestinationRuntimeEpoch: request.DestinationRuntimeEpoch,
			}, nil
		},
	}
	coordinator := NewCoordinator(registry, control)

	_, err := coordinator.Replicate(
		context.Background(),
		"source",
		"destination",
		hashes[:2],
	)
	require.ErrorContains(t, err, "network failed")

	_, err = coordinator.Replicate(
		context.Background(),
		"source",
		"destination",
		hashes[:2],
	)
	require.NoError(t, err)
	require.Equal(t, 2, attempts)
}

func TestReplicateRejectsMismatchedConfirmationAndRollsBack(t *testing.T) {
	registry, hashes := transferRegistryWithCachedSource(t, 2)
	attempts := 0
	control := &executorControlStub{
		runtimes: compatibleTransferRuntimes(),
		push: func(request *v1.TriggerKVPushRequest) (*v1.TriggerKVPushResponse, error) {
			attempts++
			response := &v1.TriggerKVPushResponse{
				TransferId:              request.TransferId,
				SourceExecutorId:        request.SourceExecutorId,
				SourceRuntimeEpoch:      request.SourceRuntimeEpoch,
				DestinationExecutorId:   request.DestinationExecutorId,
				DestinationRuntimeEpoch: request.DestinationRuntimeEpoch,
			}
			if attempts == 1 {
				response.DestinationRuntimeEpoch++
			}
			return response, nil
		},
	}
	coordinator := NewCoordinator(registry, control)

	_, err := coordinator.Replicate(
		context.Background(),
		"source",
		"destination",
		hashes[:2],
	)
	require.ErrorContains(t, err, "mismatched confirmation")

	_, err = coordinator.Replicate(
		context.Background(),
		"source",
		"destination",
		hashes[:2],
	)
	require.NoError(t, err)
}

func compatibleTransferRuntimes() map[string]*model.ExecutorStats {
	return map[string]*model.ExecutorStats{
		"source": {
			ExecutorId:        "source",
			RuntimeEpoch:      7,
			KVCompatibilityID: "qwen3-bf16-layout-v1",
		},
		"destination": {
			ExecutorId:        "destination",
			RuntimeEpoch:      11,
			KVCompatibilityID: "qwen3-bf16-layout-v1",
			TransferEndpoint:  "http://destination:19991",
		},
	}
}

func transferRegistryWithCachedSource(t *testing.T, destinationBlocks uint32) (block.Registry, []string) {
	t.Helper()
	registry, err := block.NewRegistry(
		zap.NewNop().Sugar(),
		metrics.NewMetrics(),
		map[string]*model.ExecutorStats{
			"source":      {ExecutorId: "source", RuntimeEpoch: 7, BlockSize: 2, NumKvBlocks: 4},
			"destination": {ExecutorId: "destination", RuntimeEpoch: 11, BlockSize: 2, NumKvBlocks: destinationBlocks},
		},
	)
	require.NoError(t, err)

	req := &model.Request{
		RequestId:  "source-request",
		ExecutorID: "source",
		CacheSalt:  "shared-prefix",
		TokenIDs:   []uint32{1, 2, 3, 4, 5, 6},
	}
	match, err := registry.MatchPrefix(req)
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
	allocated, err := registry.AllocateBlocks(work)
	require.NoError(t, err)
	require.True(t, allocated)
	require.NoError(t, registry.Commit(req.ExecutorID, work.WorkId))
	require.NoError(t, registry.FreeRequest(req.ExecutorID, req.RequestId))
	return registry, match.HashesTotal
}
