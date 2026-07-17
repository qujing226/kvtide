package scheduler

import (
	"sync/atomic"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/qujing226/kvtide/internal/model"
)

type lockFreeQueue struct {
	buf  []*model.WorkItem
	size uint64
	//seq  atomic.Uint64

	writePos   atomic.Uint64
	reclaimPos atomic.Uint64

	round time.Duration
}

func (q *lockFreeQueue) Pop() (*model.WorkItem, error) {
	//TODO implement me
	panic("implement me")
}

func (q *lockFreeQueue) Peek() (*model.WorkItem, error) {
	//TODO implement me
	panic("implement me")
}

//func (q *lockFreeQueue) Dequeue(tokens uint64) ([]*model.WorkItem, error) {
//	//TODO implement me
//	panic("implement me")
//}

//func NewLockFreeQueue(cfg *conf.Conf) Queue {
//	length := cfg.Server.QueueLength
//	if length < 1024 || length > 10240 {
//		length = 4096
//	} else {
//		n := 1
//		for n < int(length) {
//			n <<= 1
//		}
//		length = uint64(n)
//	}
//	q := &lockFreeQueue{
//		size:  length,
//		buf:   make([]*model.WorkItem, length),
//		round: cfg.Server.QueueRoundInterval(),
//	}
//	return q
//}

func (q *lockFreeQueue) Enqueue(task *model.WorkItem) error {
	if q.AvailableSpace() == 0 {
		return errors.New("lockFreeQueue is full")
	}
	w := q.writePos.Load()
	q.buf[w&(q.size-1)] = task
	q.writePos.Add(1)
	return nil
}

func (q *lockFreeQueue) Dequeue(n uint64) ([]*model.WorkItem, error) {
	remainTasks := q.writePos.Load() - q.reclaimPos.Load()
	if remainTasks < n {
		n = remainTasks
	}
	var taskList []*model.WorkItem
	pos := q.reclaimPos.Load() & (q.size - 1)
	if pos+n > q.size-1 {
		taskList = append(taskList, q.buf[pos:]...)
		taskList = append(taskList, q.buf[:n-(q.size-pos)]...)
	} else {
		taskList = q.buf[pos : pos+n]
	}
	q.reclaimPos.Add(n)
	return taskList, nil
}

func (q *lockFreeQueue) Length() uint64 {
	w := q.writePos.Load()
	r := q.reclaimPos.Load()
	return w - r
}

func (q *lockFreeQueue) AvailableSpace() uint64 {
	w := q.writePos.Load()
	r := q.reclaimPos.Load()
	return q.size - (w - r)
}
