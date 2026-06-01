package handler

import (
	"context"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/errors"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/qujing226/mini-llm-serve/internal/scheduler"
	"github.com/qujing226/mini-llm-serve/internal/state"
	"go.uber.org/zap"
)

type InferenceHandler interface {
	GenerateStream(ctx context.Context, req *model.Request) (<-chan *model.GenerateOutput, error)
}

type inferenceHandler struct {
	l *zap.SugaredLogger

	scheduler      scheduler.Scheduler
	requestManager state.RequestStateManager
}

func NewInferenceHandle(l *zap.SugaredLogger, s scheduler.Scheduler, r state.RequestStateManager) InferenceHandler {
	e := &inferenceHandler{
		l:              l,
		scheduler:      s,
		requestManager: r,
	}
	return e
}

func (e *inferenceHandler) GenerateStream(ctx context.Context, req *model.Request) (<-chan *model.GenerateOutput, error) {
	prefillItem, err := e.requestManager.Create(req)
	if err != nil {
		return nil, errors.New(errors.CodeInternal, err.Error())
	}

	ch, err := e.requestManager.Subscribe(req.RequestId)
	if err != nil {
		e.requestManager.Fail(prefillItem.RequestId, err)
		return nil, errors.New(errors.CodeInternal, err.Error())
	}

	err = e.scheduler.Enqueue(prefillItem)
	if err != nil {
		//e.requestManager.Cancel(prefillItem.RequestId)
		return nil, errors.New(errors.CodeInternal, err.Error())
	}

	chOut := make(chan *model.GenerateOutput, 5)

	go func() {
		for {
			select {
			case event, ok := <-ch:
				if !ok {
					return
				}
				if event.Type == v1.EventTypePrefillChunk || event.Type == v1.EventTypePrefillFinished {
					continue
				}
				output := &model.GenerateOutput{
					RequestId:    event.RequestId,
					Index:        event.ChunkIndex,
					DeltaText:    event.DeltaText,
					FinishReason: event.FinishReason,
					Done:         event.Done,
					Usage:        event.Usage,
					Timing:       event.Timing,
					BatchID:      event.BatchId,
					BatchSize:    0,
					ExecutorId:   event.ExecutorId,
					Err:          event.Err,
				}
				chOut <- output
				if event.Done {
					e.requestManager.Finish(prefillItem.RequestId)
					close(chOut)
					return
				}
			case <-ctx.Done():
				e.requestManager.Cancel(prefillItem.RequestId)
				close(chOut)
				return
			}
		}
	}()

	return chOut, nil
}
