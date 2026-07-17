package scheduler

import (
	"sync"

	"github.com/qujing226/kvtide/internal/conf"
	"github.com/qujing226/kvtide/internal/errors"
	"github.com/qujing226/kvtide/internal/model"
	"github.com/qujing226/kvtide/internal/state"
)

type DecodeQueue interface {
	Enqueue(work *model.WorkItem) error
	Dequeue(maxSeqs uint32) ([]*model.WorkItem, uint32)
	Requeue(work *model.WorkItem)
	Length() uint32
	AvailableSpace() uint32
}
type decodeQueue struct {
	requestManager state.RequestStateManager
	mu             sync.Mutex
	works          []*workItemEntry
	size           uint32
}

type workItemEntry struct {
	work    *model.WorkItem
	deficit uint32
	quant   uint32
}

func NewDecodeQueue(cfg *conf.Conf, requestManager state.RequestStateManager) DecodeQueue {
	length := cfg.Server.ScheduleConf.QueueConf.QueueLength
	if length == 0 {
		length = 100
	}
	q := &decodeQueue{
		size:           length,
		requestManager: requestManager,
		works:          make([]*workItemEntry, 0, length),
	}
	return q
}

func (q *decodeQueue) Enqueue(t *model.WorkItem) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if uint32(len(q.works)) >= q.size {
		return errors.New(errors.CodeQueueFull, "decodeQueue is full")
	}
	q.works = append(q.works, &workItemEntry{
		work:    t,
		deficit: 0,
		quant:   0,
	})
	return nil
}

func (q *decodeQueue) Dequeue(maxSeqs uint32) ([]*model.WorkItem, uint32) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if maxSeqs == 0 {
		return nil, 0
	}
	workList := make([]*model.WorkItem, 0, maxSeqs)
	for i := uint32(0); i < maxSeqs; i++ {
		if len(q.works) == 0 {
			break
		}
		w := q.works[0]
		q.works = q.works[1:]
		if q.requestManager.CanSchedule(w.work) {
			workList = append(workList, w.work)
		}
	}
	return workList, uint32(len(workList))
}

func (q *decodeQueue) Requeue(work *model.WorkItem) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.works = append(q.works, &workItemEntry{
		work:    work,
		deficit: 0,
		quant:   0,
	})
}

func (q *decodeQueue) Length() uint32 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return uint32(len(q.works))
}

func (q *decodeQueue) AvailableSpace() uint32 {
	q.mu.Lock()
	defer q.mu.Unlock()
	// Note: if Requeue expanded slice length, return 0 for protection.
	if uint32(len(q.works)) >= q.size {
		return 0
	}
	return q.size - uint32(len(q.works))
}
