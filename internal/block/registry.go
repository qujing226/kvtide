package block

import (
	"fmt"
	"sort"

	"github.com/qujing226/kvtide/internal/metrics"
	"github.com/qujing226/kvtide/internal/model"
	"go.uber.org/zap"
)

type Registry interface {
	ExecutorIDs() []string
	MatchPrefix(req *model.Request) (*model.PrefixMatch, error)
	AllocateBlocks(work *model.WorkItem) (bool, error)
	Commit(executorID, workID string) error
	Rollback(executorID, workID string) error
	FreeRequest(executorID, requestID string) error
}

type registry struct {
	managers    map[string]*manager
	executorIDs []string
}

func NewRegistry(
	logger *zap.SugaredLogger,
	metrics metrics.Metrics,
	configs []Config,
) (Registry, error) {
	if len(configs) == 0 {
		return nil, fmt.Errorf("block registry requires at least one executor")
	}

	r := &registry{
		managers:    make(map[string]*manager, len(configs)),
		executorIDs: make([]string, 0, len(configs)),
	}
	for _, cfg := range configs {
		if cfg.ExecutorID == "" {
			return nil, fmt.Errorf("executor ID must not be empty")
		}
		if _, exists := r.managers[cfg.ExecutorID]; exists {
			return nil, fmt.Errorf("duplicate executor ID: %s", cfg.ExecutorID)
		}

		manager, err := newManager(logger, metrics, cfg)
		if err != nil {
			return nil, fmt.Errorf(
				"create block manager for executor %s: %w",
				cfg.ExecutorID,
				err,
			)
		}
		r.managers[cfg.ExecutorID] = manager
		r.executorIDs = append(r.executorIDs, cfg.ExecutorID)
	}

	sort.Strings(r.executorIDs)
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
