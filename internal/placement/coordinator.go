package placement

import (
	"context"
	"errors"
	"fmt"

	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
	"github.com/qujing226/kvtide/internal/block"
	"github.com/qujing226/kvtide/internal/model"
	"github.com/qujing226/kvtide/internal/utils"
)

type executorControl interface {
	GetRuntimeStates() map[string]*model.ExecutorStats
	TriggerKVPush(context.Context, *v1.TriggerKVPushRequest) (*v1.TriggerKVPushResponse, error)
}

// Coordinator executes one Engine-controlled prefix KV replication transaction.
// Choosing the prefix, source, destination, and timing remains a policy concern.
type Coordinator struct {
	registry  block.Registry
	executors executorControl
}

func NewCoordinator(registry block.Registry, executors executorControl) *Coordinator {
	return &Coordinator{registry: registry, executors: executors}
}

func (c *Coordinator) Replicate(
	ctx context.Context,
	sourceExecutorID, destinationExecutorID string,
	blockHashes []string,
) (*model.TransferPlan, error) {
	runtimes := c.executors.GetRuntimeStates()
	source, err := runtimeFor(runtimes, sourceExecutorID)
	if err != nil {
		return nil, err
	}
	destination, err := runtimeFor(runtimes, destinationExecutorID)
	if err != nil {
		return nil, err
	}
	if source.KVCompatibilityID == "" || source.KVCompatibilityID != destination.KVCompatibilityID {
		return nil, fmt.Errorf(
			"executors %s and %s are not KV compatible",
			sourceExecutorID,
			destinationExecutorID,
		)
	}
	if destination.TransferEndpoint == "" {
		return nil, fmt.Errorf("destination executor %s has no transfer endpoint", destinationExecutorID)
	}

	plan, err := c.registry.PrepareTransfer(
		utils.MustGenerateUUIDv7(),
		sourceExecutorID,
		destinationExecutorID,
		blockHashes,
	)
	if err != nil {
		return nil, err
	}
	request := &v1.TriggerKVPushRequest{
		TransferId:                  plan.TransferID,
		SourceExecutorId:            plan.SourceExecutorID,
		SourceRuntimeEpoch:          source.RuntimeEpoch,
		SourceBlockIds:              append([]uint32(nil), plan.SourceBlockIDs...),
		DestinationExecutorId:       plan.DestinationExecutorID,
		DestinationRuntimeEpoch:     destination.RuntimeEpoch,
		DestinationTransferEndpoint: destination.TransferEndpoint,
		KvCompatibilityId:           source.KVCompatibilityID,
		BlockHashes:                 append([]string(nil), plan.BlockHashes...),
		DestinationBlockIds:         append([]uint32(nil), plan.DestinationBlockIDs...),
	}
	response, err := c.executors.TriggerKVPush(ctx, request)
	if err != nil {
		return nil, c.rollback(plan.TransferID, fmt.Errorf("trigger KV push: %w", err))
	}
	if err := validateResponse(response, plan, source.RuntimeEpoch, destination.RuntimeEpoch); err != nil {
		return nil, c.rollback(plan.TransferID, err)
	}
	if err := c.registry.CommitTransfer(plan.TransferID); err != nil {
		return nil, c.rollback(plan.TransferID, fmt.Errorf("commit transfer: %w", err))
	}
	return plan, nil
}

func (c *Coordinator) rollback(transferID string, cause error) error {
	if err := c.registry.RollbackTransfer(transferID); err != nil {
		return errors.Join(cause, fmt.Errorf("rollback transfer: %w", err))
	}
	return cause
}

func runtimeFor(runtimes map[string]*model.ExecutorStats, executorID string) (*model.ExecutorStats, error) {
	runtime, exists := runtimes[executorID]
	if !exists || runtime == nil {
		return nil, fmt.Errorf("runtime for executor %s not found", executorID)
	}
	if runtime.ExecutorId != executorID {
		return nil, fmt.Errorf(
			"runtime executor ID %s does not match requested executor %s",
			runtime.ExecutorId,
			executorID,
		)
	}
	return runtime, nil
}

func validateResponse(
	response *v1.TriggerKVPushResponse,
	plan *model.TransferPlan,
	sourceRuntimeEpoch, destinationRuntimeEpoch uint32,
) error {
	if response == nil {
		return fmt.Errorf("executor returned no transfer response")
	}
	if response.TransferId != plan.TransferID ||
		response.SourceExecutorId != plan.SourceExecutorID ||
		response.SourceRuntimeEpoch != sourceRuntimeEpoch ||
		response.DestinationExecutorId != plan.DestinationExecutorID ||
		response.DestinationRuntimeEpoch != destinationRuntimeEpoch {
		return fmt.Errorf("executor returned mismatched confirmation for transfer %s", plan.TransferID)
	}
	return nil
}
