package scheduler

import (
	"sync"

	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/errors"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/qujing226/mini-llm-serve/internal/state"
)

type PrefillQueue interface {
	Enqueue(t *model.WorkItem) error
	Pop() (*model.WorkItem, bool)
	Peek() (*model.WorkItem, bool)
	Length() uint32
	AvailableSpace() uint32
}

type prefillQueue struct {
	requestManager state.RequestLifecycleStateManager
	mu             sync.Mutex
	works          []*model.WorkItem
	size           uint32
}

func NewPrefillQueue(cfg *conf.Conf, requestManager state.RequestLifecycleStateManager) PrefillQueue {
	length := cfg.Server.ScheduleConf.QueueConf.QueueLength
	if length == 0 {
		length = 100
	}
	q := &prefillQueue{
		size:           length,
		requestManager: requestManager,
		works:          make([]*model.WorkItem, 0, length),
	}
	return q
}

func (q *prefillQueue) Enqueue(t *model.WorkItem) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if uint32(len(q.works)) >= q.size {
		return errors.New(errors.CodeQueueFull, "prefillQueue is full")
	}
	q.works = append(q.works, t)
	return nil
}

func (q *prefillQueue) Pop() (*model.WorkItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.works) > 0 {
		work := q.works[0]
		q.works = q.works[1:]
		if q.requestManager.CanSchedule(work) {
			return work, true
		}
	}
	return nil, false
}

func (q *prefillQueue) Peek() (*model.WorkItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.works) > 0 {
		work := q.works[0]
		if q.requestManager.CanSchedule(work) {
			return work, true
		}
		q.works = q.works[1:]
	}
	return nil, false
}

func (q *prefillQueue) Dequeue(tokens uint32) ([]*model.WorkItem, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if tokens == 0 || len(q.works) == 0 {
		return nil, nil
	}
	workList := make([]*model.WorkItem, 0, 10)
	used := uint32(0)

	for len(q.works) > 0 {
		cost := WorkBudgetCost(q.works[0])
		if used+cost > tokens {
			break
		}
		workList = append(workList, q.works[0])
		q.works = q.works[1:]
		used += cost
	}
	if len(workList) == 0 {
		workList = append(workList, q.works[0])
		q.works = q.works[1:]
	}
	return workList, nil
}

func (q *prefillQueue) Length() uint32 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return uint32(len(q.works))
}

func (q *prefillQueue) AvailableSpace() uint32 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.size - uint32(len(q.works))
}
