package block

import (
	"fmt"
	"sort"
	"sync"

	"github.com/qujing226/kvtide/internal/metrics"
	"github.com/qujing226/kvtide/internal/model"
	"go.uber.org/zap"
)

type Registry interface {
	ExecutorIDs() []string
	ProbePrefixes(req *model.Request) []model.PrefixCandidate
	MatchPrefix(req *model.Request) (*model.PrefixMatch, error)
	AllocateBlocks(work *model.WorkItem) (bool, error)
	Commit(executorID, workID string) error
	Rollback(executorID, workID string) error
	FreeRequest(executorID, requestID string) error
	PrepareTransfer(transferID, sourceExecutorID, destinationExecutorID string, blockHashes []string) (*model.TransferPlan, error)
	CommitTransfer(transferID string) error
	RollbackTransfer(transferID string) error
}

type registry struct {
	managers    map[string]*manager
	executorIDs []string

	transferMu       sync.Mutex
	pendingTransfers map[string]*model.TransferPlan
}

func NewRegistry(
	logger *zap.SugaredLogger,
	metrics metrics.Metrics,
	runtimes map[string]*model.ExecutorStats,
) (Registry, error) {
	if len(runtimes) == 0 {
		return nil, fmt.Errorf("block registry requires at least one executor")
	}

	r := &registry{
		managers:         make(map[string]*manager, len(runtimes)),
		executorIDs:      make([]string, 0, len(runtimes)),
		pendingTransfers: make(map[string]*model.TransferPlan),
	}
	for executorID := range runtimes {
		r.executorIDs = append(r.executorIDs, executorID)
	}
	sort.Strings(r.executorIDs)
	for _, executorID := range r.executorIDs {
		runtime := runtimes[executorID]
		if runtime == nil {
			return nil, fmt.Errorf("executor %s returned no runtime", executorID)
		}
		if runtime.ExecutorId != executorID {
			return nil, fmt.Errorf(
				"executor runtime ID %s does not match registered ID %s",
				runtime.ExecutorId,
				executorID,
			)
		}

		manager, err := newManager(logger, metrics, runtime)
		if err != nil {
			return nil, fmt.Errorf(
				"create block manager for executor %s: %w",
				executorID,
				err,
			)
		}
		r.managers[executorID] = manager
	}

	return r, nil
}

func (r *registry) managerFor(executorID string) (*manager, error) {
	manager, exists := r.managers[executorID]
	if !exists {
		return nil, fmt.Errorf("block manager for executor %s not found", executorID)
	}
	return manager, nil
}

func (r *registry) ExecutorIDs() []string {
	return append([]string(nil), r.executorIDs...)
}

func (r *registry) ProbePrefixes(req *model.Request) []model.PrefixCandidate {
	candidates := make([]model.PrefixCandidate, 0, len(r.executorIDs))
	for _, executorID := range r.executorIDs {
		candidates = append(candidates, r.managers[executorID].probePrefix(req))
	}
	return candidates
}

func (r *registry) MatchPrefix(req *model.Request) (*model.PrefixMatch, error) {
	manager, err := r.managerFor(req.ExecutorID)
	if err != nil {
		return nil, err
	}
	return manager.MatchPrefix(req), nil
}

func (r *registry) AllocateBlocks(work *model.WorkItem) (bool, error) {
	manager, err := r.managerFor(work.ExecutorID)
	if err != nil {
		return false, err
	}
	return manager.AllocateBlocks(work), nil
}

func (r *registry) Commit(executorID, workID string) error {
	manager, err := r.managerFor(executorID)
	if err != nil {
		return err
	}
	manager.Commit(workID)
	return nil
}

func (r *registry) Rollback(executorID, workID string) error {
	manager, err := r.managerFor(executorID)
	if err != nil {
		return err
	}
	manager.Rollback(workID)
	return nil
}

func (r *registry) FreeRequest(executorID, requestID string) error {
	manager, err := r.managerFor(executorID)
	if err != nil {
		return err
	}
	manager.FreeRequest(requestID)
	return nil
}

func (r *registry) PrepareTransfer(
	transferID, sourceExecutorID, destinationExecutorID string,
	blockHashes []string,
) (*model.TransferPlan, error) {
	if transferID == "" {
		return nil, fmt.Errorf("transfer ID must not be empty")
	}
	if sourceExecutorID == destinationExecutorID {
		return nil, fmt.Errorf("source and destination executors must differ")
	}
	if len(blockHashes) == 0 {
		return nil, fmt.Errorf("transfer requires at least one block hash")
	}

	source, err := r.managerFor(sourceExecutorID)
	if err != nil {
		return nil, err
	}
	destination, err := r.managerFor(destinationExecutorID)
	if err != nil {
		return nil, err
	}
	if source.blockSize != destination.blockSize {
		return nil, fmt.Errorf("source and destination block sizes differ")
	}

	r.transferMu.Lock()
	defer r.transferMu.Unlock()
	if _, exists := r.pendingTransfers[transferID]; exists {
		return nil, fmt.Errorf("transfer %s already exists", transferID)
	}

	sourceBlockIDs, err := source.pinCachedBlocks(blockHashes)
	if err != nil {
		return nil, err
	}
	destinationBlockIDs, ok := destination.reserveTransferBlocks(uint32(len(blockHashes)))
	if !ok {
		source.releaseTransferBlocks(sourceBlockIDs)
		return nil, fmt.Errorf("destination executor %s has insufficient free blocks", destinationExecutorID)
	}

	plan := &model.TransferPlan{
		TransferID:            transferID,
		SourceExecutorID:      sourceExecutorID,
		SourceBlockIDs:        append([]uint32(nil), sourceBlockIDs...),
		DestinationExecutorID: destinationExecutorID,
		DestinationBlockIDs:   append([]uint32(nil), destinationBlockIDs...),
		BlockHashes:           append([]string(nil), blockHashes...),
	}
	r.pendingTransfers[transferID] = plan
	return cloneTransferPlan(plan), nil
}

func (r *registry) CommitTransfer(transferID string) error {
	r.transferMu.Lock()
	defer r.transferMu.Unlock()

	plan, exists := r.pendingTransfers[transferID]
	if !exists {
		return fmt.Errorf("transfer %s not found", transferID)
	}
	destination := r.managers[plan.DestinationExecutorID]
	if err := destination.commitImportedBlocks(plan.DestinationBlockIDs, plan.BlockHashes); err != nil {
		return err
	}
	r.managers[plan.SourceExecutorID].releaseTransferBlocks(plan.SourceBlockIDs)
	delete(r.pendingTransfers, transferID)
	return nil
}

func (r *registry) RollbackTransfer(transferID string) error {
	r.transferMu.Lock()
	defer r.transferMu.Unlock()

	plan, exists := r.pendingTransfers[transferID]
	if !exists {
		return fmt.Errorf("transfer %s not found", transferID)
	}
	r.managers[plan.DestinationExecutorID].releaseTransferBlocks(plan.DestinationBlockIDs)
	r.managers[plan.SourceExecutorID].releaseTransferBlocks(plan.SourceBlockIDs)
	delete(r.pendingTransfers, transferID)
	return nil
}

func cloneTransferPlan(plan *model.TransferPlan) *model.TransferPlan {
	clone := *plan
	clone.SourceBlockIDs = append([]uint32(nil), plan.SourceBlockIDs...)
	clone.DestinationBlockIDs = append([]uint32(nil), plan.DestinationBlockIDs...)
	clone.BlockHashes = append([]string(nil), plan.BlockHashes...)
	return &clone
}
