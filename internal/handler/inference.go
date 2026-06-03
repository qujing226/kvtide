package handler

import (
	"context"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/errors"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/qujing226/mini-llm-serve/internal/scheduler"
	"github.com/qujing226/mini-llm-serve/internal/state"
	"github.com/qujing226/mini-llm-serve/internal/tokenizer"
	"go.uber.org/zap"
)

type InferenceHandler interface {
	GenerateStream(ctx context.Context, req *model.Request) (<-chan *model.GenerateOutput, error)
}

type inferenceHandler struct {
	l              *zap.SugaredLogger
	tokenizer      tokenizer.Tokenizer
	scheduler      scheduler.Scheduler
	requestManager state.RequestStateManager
}

func NewInferenceHandle(l *zap.SugaredLogger, tokenizer tokenizer.Tokenizer, s scheduler.Scheduler, r state.RequestStateManager) InferenceHandler {
	e := &inferenceHandler{
		l:              l,
		tokenizer:      tokenizer,
		scheduler:      s,
		requestManager: r,
	}
	return e
}

func (e *inferenceHandler) GenerateStream(ctx context.Context, req *model.Request) (<-chan *model.GenerateOutput, error) {
	var err error
	// 1. Tokenize
	req.TokenIDs, err = e.tokenizer.Encode(req.Prompt)
	if err != nil {
		return nil, err
	}
	req.PromptTokens = uint32(len(req.TokenIDs))

	// 2. Register inference request instance
	prefillItem, err := e.requestManager.Create(req)
	if err != nil {
		return nil, errors.New(errors.CodeInternal, err.Error())
	}

	eventCh, err := e.requestManager.Subscribe(req.RequestId)
	if err != nil {
		e.requestManager.Fail(prefillItem.RequestId, err)
		return nil, errors.New(errors.CodeInternal, err.Error())
	}

	// 3. schedule
	err = e.scheduler.Enqueue(prefillItem)
	if err != nil {
		//e.requestManager.Cancel(prefillItem.RequestId)
		return nil, errors.New(errors.CodeInternal, err.Error())
	}

	chOut := make(chan *model.GenerateOutput, 5)

	go func() {
		for {
			select {
			case event, ok := <-eventCh:
				if !ok {
					return
				}
				if event.Type == v1.EventTypePrefillChunk || event.Type == v1.EventTypePrefillFinished {
					continue
				}
				// 4. deTokenize
				//text, decodeErr := e.tokenizer.Decode(event.DeltaText)
				//if event.Err != nil {
				//	event.Err = decodeErr
				//}

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
