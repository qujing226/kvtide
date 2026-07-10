package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/qujing226/mini-llm-serve/cmd/client"
	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/errors"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"go.uber.org/zap"
)

type Executor interface {
	Execute(ctx context.Context, batch *model.Batch) ([]*model.Event, error)
}

func NewExecutors(logger *zap.SugaredLogger, cfg *conf.Conf) (map[string]Executor, error) {
	executors := make(map[string]Executor)

	for _, ec := range cfg.Executors {
		if ec.ID == "" {
			return nil, fmt.Errorf("executor.id can not be empty")
		}
		if ec.ModelID == "" {
			return nil, fmt.Errorf("executor.modelId can not be empty")
		}
		if len(ec.Address) == 0 {
			return nil, fmt.Errorf("executor.address can not be empty")
		}

		exec, err := newConnectExecutor(logger, ec)
		if err != nil {
			return nil, err
		}
		if _, exists := executors[ec.ID]; exists {
			return nil, fmt.Errorf("executor with id %s already exists", ec.ID)
		}
		executors[ec.ID] = exec
	}
	if len(executors) == 0 {
		return nil, fmt.Errorf("no executors configured")
	}

	return executors, nil
}

type executor struct {
	l         *zap.SugaredLogger
	id        string
	modelID   model.LLMModelID
	endpoints []string
	client    *client.ExecutorClient
}

func newConnectExecutor(l *zap.SugaredLogger, cfg conf.ExecutorConf) (Executor, error) {
	modelID, err := model.ParseModelID(cfg.ModelID)
	if err != nil {
		return nil, err
	}
	e := &executor{
		l:         l,
		id:        cfg.ID,
		modelID:   modelID,
		endpoints: cfg.Address,
		client:    client.NewExecutorClient(cfg.Address, cfg.TimeoutMs),
	}
	return e, nil
}

func (m *executor) Execute(ctx context.Context, batch *model.Batch) ([]*model.Event, error) {
	resp, err := m.client.ExecuteBatch(ctx, BatchToExecute(batch))
	if err != nil {
		return nil, err
	}

	works := make(map[string]*model.WorkItem, len(batch.Items))
	for _, item := range batch.Items {
		works[item.WorkId] = item
	}

	var results []*model.Event
	for _, workRes := range resp.GetResults() {
		workItem, exists := works[workRes.GetWorkId()]
		if !exists {
			m.l.Errorf("executorManager id %s not found", workRes.GetWorkId())
			continue
		}

		var err error
		if workRes.ErrorMessage != "" {
			err = errors.New(errors.CodeInternal, workRes.ErrorMessage)
		}
		results = append(results, &model.Event{
			WorkId:     workRes.WorkId,
			RequestId:  workRes.RequestId,
			BatchId:    batch.BatchID,
			ExecutorId: m.id,
			Type:       nextPhase(workItem, err),
			TokenId:    workRes.TokenId,
			Done:       workRes.Done,
			Usage: model.Usage{
				InputTokens:  workRes.ComputedTokens,
				OutputTokens: workRes.GeneratedTokens,
				TotalTokens:  workRes.ComputedTokens + workRes.GeneratedTokens,
			},
			Timing: model.Timing{
				Queue:     0,
				BatchWait: 0,
				Execution: time.Duration(workRes.ExecutionMs) * time.Millisecond,
				Total:     0,
			},
			FinishReason: workRes.FinishReason,
			Err:          err,
		})

	}

	return results, nil
}

func nextPhase(item *model.WorkItem, err error) v1.EventType {
	if err != nil {
		return v1.EventTypeRequestFailed
	}
	if item.Phase == v1.WorkPhasePrefill {
		if item.PrefillOffset+item.NumNewTokens >= item.TokenCntTotal {
			return v1.EventTypePrefillFinished
		}
		return v1.EventTypePrefillChunk
	}
	if item.Phase == v1.WorkPhaseDecode {
		return v1.EventTypeDecodeChunk
	}

	return v1.EventTypeRequestFinished
}
