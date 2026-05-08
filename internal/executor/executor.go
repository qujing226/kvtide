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
		if !ec.Enabled {
			continue
		}
		if ec.ID == "" {
			return nil, fmt.Errorf("executor.id can not be empty")
		}
		if ec.Kind == "" {
			return nil, fmt.Errorf("executor.kind can not be empty")
		}
		if len(ec.Address) == 0 {
			return nil, fmt.Errorf("executor.address can not be empty")
		}

		var exec Executor
		var err error
		switch ec.Kind {
		case "connect":
			exec, err = newPythonExecutor(logger, ec)
		case "http":
			exec, err = newHTTPExecutor(ec)
		default:
			return nil, fmt.Errorf("unsupported executor.kind: %s", ec.Kind)
		}
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

type mockExecutor struct {
	l         *zap.SugaredLogger
	id        string
	endpoints []string
	client    *client.ExecutorClient
}

func newPythonExecutor(l *zap.SugaredLogger, cfg conf.ExecutorConf) (Executor, error) {
	e := &mockExecutor{
		l:         l,
		id:        cfg.ID,
		endpoints: cfg.Address,
		client:    client.NewExecutorClient(cfg.Address),
	}
	return e, nil
}

func (m *mockExecutor) Execute(ctx context.Context, batch *model.Batch) ([]*model.Event, error) {
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
			DeltaText:  workRes.OutputText,
			Done:       workRes.Done,
			Usage: model.Usage{
				InputTokens:  uint64(workRes.ComputedTokens),
				OutputTokens: uint64(workRes.GeneratedTokens),
				TotalTokens:  uint64(workRes.ComputedTokens + workRes.GeneratedTokens),
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

func newHTTPExecutor(cfg conf.ExecutorConf) (Executor, error) {
	return nil, fmt.Errorf("http executor not implemented")
}

func nextPhase(item *model.WorkItem, err error) v1.EventType {
	if err != nil {
		return v1.EventTypeRequestFailed
	}
	if item.Phase == v1.WorkPhasePrefill {
		if item.PrefillOffset+item.NumNewTokens >= item.PromptTokens {
			return v1.EventTypePrefillFinished
		}
		return v1.EventTypePrefillChunk
	}
	if item.Phase == v1.WorkPhaseDecode {
		return v1.EventTypeDecodeChunk
	}

	return v1.EventTypeRequestFinished
}
