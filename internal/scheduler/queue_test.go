package scheduler

import (
	"testing"
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/block"
	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/metrics"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/qujing226/mini-llm-serve/internal/state"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func testQueueConf(length uint32) (*conf.Conf, state.RequestStateManager) {
	m := metrics.NewMetrics()
	return &conf.Conf{
		Server: conf.ServerConf{
			ScheduleConf: conf.ScheduleConf{
				QueueConf: conf.QueueConf{
					QueueLength: length,
				},
			},
		},
	}, state.NewRequestLifecycleStateManager(zap.S(), block.NewManager(zap.S(), m), m)
}

func testQueueWork(t *testing.T, manager state.RequestStateManager, id string, phase v1.WorkPhase) *model.WorkItem {
	t.Helper()

	work, err := manager.Create(&model.Request{
		RequestId:    "req-" + id,
		ModelID:      model.MockModel,
		Prompt:       "hello",
		MaxTokens:    8,
		TokenIDs:     []uint32{1, 2},
		PromptTokens: 2,
		Deadline:     time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	work.WorkId = id
	work.Phase = phase
	return work
}

func TestPrefillQueuePeekDoesNotRemove(t *testing.T) {
	cfg, manager := testQueueConf(3)
	q := NewPrefillQueue(cfg, manager)

	require.NoError(t, q.Enqueue(testQueueWork(t, manager, "p1", v1.WorkPhasePrefill)))
	require.NoError(t, q.Enqueue(testQueueWork(t, manager, "p2", v1.WorkPhasePrefill)))

	item, ok := q.Peek()
	require.True(t, ok)
	require.Equal(t, "p1", item.WorkId)
	require.Equal(t, uint32(2), q.Length())

	item, ok = q.Peek()
	require.True(t, ok)
	require.Equal(t, "p1", item.WorkId)
	require.Equal(t, uint32(2), q.Length())
}

func TestPrefillQueuePopRemovesFIFO(t *testing.T) {
	cfg, manager := testQueueConf(3)
	q := NewPrefillQueue(cfg, manager)

	require.NoError(t, q.Enqueue(testQueueWork(t, manager, "p1", v1.WorkPhasePrefill)))
	require.NoError(t, q.Enqueue(testQueueWork(t, manager, "p2", v1.WorkPhasePrefill)))

	item, ok := q.Pop()
	require.True(t, ok)
	require.Equal(t, "p1", item.WorkId)
	require.Equal(t, uint32(1), q.Length())
	require.Equal(t, uint32(2), q.AvailableSpace())

	item, ok = q.Pop()
	require.True(t, ok)
	require.Equal(t, "p2", item.WorkId)
	require.Equal(t, uint32(0), q.Length())
	require.Equal(t, uint32(3), q.AvailableSpace())
}

func TestPrefillQueueEmptyPeekAndPop(t *testing.T) {
	cfg, manager := testQueueConf(2)
	q := NewPrefillQueue(cfg, manager)

	item, ok := q.Peek()
	require.False(t, ok)
	require.Nil(t, item)

	item, ok = q.Pop()
	require.False(t, ok)
	require.Nil(t, item)
}

func TestPrefillQueueFull(t *testing.T) {
	cfg, manager := testQueueConf(1)
	q := NewPrefillQueue(cfg, manager)

	require.NoError(t, q.Enqueue(testQueueWork(t, manager, "p1", v1.WorkPhasePrefill)))
	require.Error(t, q.Enqueue(testQueueWork(t, manager, "p2", v1.WorkPhasePrefill)))
	require.Equal(t, uint32(1), q.Length())
	require.Equal(t, uint32(0), q.AvailableSpace())
}

func TestPrefillQueueRequeueCanExceedAdmissionCapacity(t *testing.T) {
	cfg, manager := testQueueConf(1)
	q := NewPrefillQueue(cfg, manager)

	q.Requeue(testQueueWork(t, manager, "p1", v1.WorkPhasePrefill))
	q.Requeue(testQueueWork(t, manager, "p2", v1.WorkPhasePrefill))

	require.Equal(t, uint32(2), q.Length())
	require.Equal(t, uint32(0), q.AvailableSpace())
	require.Error(t, q.Enqueue(testQueueWork(t, manager, "p3", v1.WorkPhasePrefill)))
}

func TestDecodeQueueDequeueRespectsMaxSeqs(t *testing.T) {
	cfg, manager := testQueueConf(3)
	q := NewDecodeQueue(cfg, manager)

	require.NoError(t, q.Enqueue(testQueueWork(t, manager, "d1", v1.WorkPhaseDecode)))
	require.NoError(t, q.Enqueue(testQueueWork(t, manager, "d2", v1.WorkPhaseDecode)))
	require.NoError(t, q.Enqueue(testQueueWork(t, manager, "d3", v1.WorkPhaseDecode)))

	items, n := q.Dequeue(2)
	require.Equal(t, uint32(2), n)
	require.Len(t, items, 2)
	require.Equal(t, "d1", items[0].WorkId)
	require.Equal(t, "d2", items[1].WorkId)
	require.Equal(t, uint32(1), q.Length())

	items, n = q.Dequeue(2)
	require.Equal(t, uint32(1), n)
	require.Len(t, items, 1)
	require.Equal(t, "d3", items[0].WorkId)
	require.Equal(t, uint32(0), q.Length())
}

func TestDecodeQueueDequeueZeroOrEmpty(t *testing.T) {
	cfg, manager := testQueueConf(2)
	q := NewDecodeQueue(cfg, manager)

	items, n := q.Dequeue(0)
	require.Equal(t, uint32(0), n)
	require.Nil(t, items)

	items, n = q.Dequeue(1)
	require.Equal(t, uint32(0), n)
	require.Empty(t, items)
}

func TestDecodeQueueFull(t *testing.T) {
	cfg, manager := testQueueConf(1)
	q := NewDecodeQueue(cfg, manager)

	require.NoError(t, q.Enqueue(testQueueWork(t, manager, "d1", v1.WorkPhaseDecode)))
	require.Error(t, q.Enqueue(testQueueWork(t, manager, "d2", v1.WorkPhaseDecode)))
	require.Equal(t, uint32(1), q.Length())
	require.Equal(t, uint32(0), q.AvailableSpace())
}

func TestDecodeQueueRequeueCanExceedAdmissionCapacity(t *testing.T) {
	cfg, manager := testQueueConf(1)
	q := NewDecodeQueue(cfg, manager)

	q.Requeue(testQueueWork(t, manager, "d1", v1.WorkPhaseDecode))
	q.Requeue(testQueueWork(t, manager, "d2", v1.WorkPhaseDecode))

	require.Equal(t, uint32(2), q.Length())
	require.Equal(t, uint32(0), q.AvailableSpace())
	require.Error(t, q.Enqueue(testQueueWork(t, manager, "d3", v1.WorkPhaseDecode)))
}
