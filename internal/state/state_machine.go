package state

import (
	"sync"
	"sync/atomic"
	"time"

	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
	"github.com/qujing226/kvtide/internal/block"
	"github.com/qujing226/kvtide/internal/errors"
	"github.com/qujing226/kvtide/internal/metrics"
	"github.com/qujing226/kvtide/internal/model"
	"github.com/qujing226/kvtide/internal/utils"
	"go.uber.org/zap"
)

type RequestStateManager interface {
	Create(req *model.Request) (*model.WorkItem, error)
	Get(requestId string) (*model.Request, bool)
	Subscribe(requestId string) (<-chan *model.Event, error)

	CanSchedule(work *model.WorkItem) bool
	OnEvent(e *model.Event) ([]*model.WorkItem, error)
	Fail(requestId string, err error)

	Cancel(requestId string)
	Finish(requestId string)
}

type requestStateManager struct {
	l *zap.SugaredLogger

	requests    map[string]*model.Request
	subscribeCh map[string]chan *model.Event
	mu          sync.RWMutex

	blockManager block.Manager

	activeRequests atomic.Int64
	metrics        metrics.Metrics
}

func NewRequestLifecycleStateManager(l *zap.SugaredLogger, blockManager block.Manager,
	metrics metrics.Metrics) RequestStateManager {
	r := &requestStateManager{
		l:            l,
		requests:     make(map[string]*model.Request),
		subscribeCh:  make(map[string]chan *model.Event),
		blockManager: blockManager,
		metrics:      metrics,
	}
	return r
}

func (r *requestStateManager) Create(req *model.Request) (*model.WorkItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.requests[req.RequestId]; exists {
		return nil, errors.New(errors.CodeInvalidArgument, "request already exists")
	}
	r.requests[req.RequestId] = req
	req.Phase = model.RequestPhasePrefillReady

	r.subscribeCh[req.RequestId] = make(chan *model.Event, 5)

	// prefix cache
	prefixMatch := r.blockManager.MatchPrefix(req)

	req.ComputedTokens = prefixMatch.CachedTokens

	// metrics: add active requests
	r.increaseActiveRequestAndCacheHit(prefixMatch.Hit, prefixMatch.CachedTokens)

	now := time.Now()
	workItem := &model.WorkItem{
		WorkId:        utils.MustGenerateUUIDv7(),
		RequestId:     req.RequestId,
		Phase:         v1.WorkPhasePrefill,
		Deadline:      req.Deadline,
		MaxTokens:     req.MaxTokens,
		ModelID:       req.ModelID,
		Cache:         prefixMatch,
		TokenIDs:      req.TokenIDs[prefixMatch.CachedTokens:],
		TokenCntTotal: uint32(len(req.TokenIDs)),
		PrefillOffset: prefixMatch.CachedTokens,
		NumNewTokens:  req.PromptTokens - prefixMatch.CachedTokens,
		ReadyAt:       now,
	}
	// all prompt tokens had prefilled
	if prefixMatch.CachedTokens >= req.PromptTokens {
		workItem.Phase = v1.WorkPhaseDecode
		workItem.NumNewTokens = 1
	}
	return workItem, nil
}

func (r *requestStateManager) Get(requestId string) (*model.Request, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if req, exists := r.requests[requestId]; exists {
		return req, true
	}
	return nil, false
}
func (r *requestStateManager) Subscribe(requestId string) (<-chan *model.Event, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ch, exists := r.subscribeCh[requestId]
	if !exists {
		return nil, errors.New(errors.CodeInternal, "subscribe channel doesn't exist")
	}
	return ch, nil
}

func (r *requestStateManager) CanSchedule(work *model.WorkItem) bool {
	r.mu.Lock()
	req, exists := r.requests[work.RequestId]
	if !exists {
		r.mu.Unlock()
		return false
	}
	// arrival deadline
	if !req.Deadline.IsZero() && time.Now().After(req.Deadline) {
		req.Phase = model.RequestPhaseTimeout
		subCh, exists := r.subscribeCh[work.RequestId]
		r.deleteRequest(work.RequestId)
		r.mu.Unlock()
		if exists {
			subCh <- &model.Event{
				WorkId:       utils.MustGenerateUUIDv7(),
				RequestId:    work.RequestId,
				Type:         v1.EventTypeRequestFailed,
				Done:         true,
				FinishReason: v1.FinishReasonError,
				At:           time.Now(),
				Err:          errors.New(errors.CodeRequestTimeout, "request timeout"),
			}
			close(subCh)
		}
		return false
	}

	switch req.Phase {
	case model.RequestPhaseFinished, model.RequestPhaseCanceled, model.RequestPhaseTimeout, model.RequestPhaseFailed:
		r.mu.Unlock()
		return false
	default:
		r.mu.Unlock()
		return true
	}
}

func (r *requestStateManager) OnEvent(e *model.Event) ([]*model.WorkItem, error) {
	r.mu.Lock()
	req, exists := r.requests[e.RequestId]
	if !exists {
		r.mu.Unlock()
		// The request may have been canceled or finished while a work item was
		// still queued or in-flight. Treat late executor results as stale.
		return nil, nil
	}

	var onWorkItems []*model.WorkItem
	now := time.Now()
	switch e.Type {
	case v1.EventTypePrefillChunk:
		req.ComputedTokens += e.Usage.InputTokens
		req.Usage.InputTokens = req.ComputedTokens
		req.Usage.TotalTokens = req.ComputedTokens + req.GeneratedTokens
		e.Usage = req.Usage
		req.Phase = model.RequestPhasePrefillRunning
		prefillOffset := req.ComputedTokens
		numNewTokens := req.PromptTokens - prefillOffset
		prefillItem := &model.WorkItem{
			WorkId:        utils.MustGenerateUUIDv7(),
			RequestId:     e.RequestId,
			Phase:         v1.WorkPhasePrefill,
			Deadline:      req.Deadline,
			MaxTokens:     req.MaxTokens,
			ModelID:       req.ModelID,
			Cache:         req.Cache,
			TokenIDs:      req.TokenIDs[prefillOffset : prefillOffset+numNewTokens],
			TokenCntTotal: req.PromptTokens,
			PrefillOffset: prefillOffset,
			NumNewTokens:  numNewTokens,
			ReadyAt:       now,
		}
		onWorkItems = append(onWorkItems, prefillItem)
	case v1.EventTypePrefillFinished:
		req.ComputedTokens += e.Usage.InputTokens
		if req.ComputedTokens > req.PromptTokens {
			req.ComputedTokens = req.PromptTokens
		}
		req.GeneratedTokens += e.Usage.OutputTokens

		req.TokenIDs = append(req.TokenIDs, e.TokenId)
		req.Usage.InputTokens = req.ComputedTokens
		req.Usage.TotalTokens = req.ComputedTokens + req.GeneratedTokens
		req.Usage.OutputTokens = req.GeneratedTokens

		e.Usage = req.Usage
		req.Phase = model.RequestPhaseDecodeReady

		decodeItem := &model.WorkItem{
			WorkId:          utils.MustGenerateUUIDv7(),
			RequestId:       e.RequestId,
			Phase:           v1.WorkPhaseDecode,
			Deadline:        req.Deadline,
			MaxTokens:       req.MaxTokens,
			ModelID:         req.ModelID,
			Cache:           req.Cache,
			TokenIDs:        req.TokenIDs,
			TokenCntTotal:   uint32(len(req.TokenIDs)),
			GeneratedTokens: req.GeneratedTokens,
			NumNewTokens:    1,
			ReadyAt:         now,
		}
		onWorkItems = append(onWorkItems, decodeItem)
	case v1.EventTypeDecodeChunk:
		req.GeneratedTokens += e.Usage.OutputTokens
		req.Usage.InputTokens = req.PromptTokens
		req.Usage.OutputTokens = req.GeneratedTokens
		req.Usage.TotalTokens = req.PromptTokens + req.GeneratedTokens
		e.Usage = req.Usage
		req.TokenIDs = append(req.TokenIDs, e.TokenId)
		if e.Done || req.GeneratedTokens >= req.MaxTokens {
			req.Phase = model.RequestPhaseFinished
			req.FinishedAt = now
			if e.FinishReason == v1.FinishReasonUnspecified {
				e.FinishReason = v1.FinishReasonLength
			}
			e.Done = true
			req.FinishReason = e.FinishReason
			// todo：决定清理请求/通知订阅者
		} else {
			req.Phase = model.RequestPhaseDecodeRunning
			decodeItem := &model.WorkItem{
				WorkId:          utils.MustGenerateUUIDv7(),
				RequestId:       e.RequestId,
				Phase:           v1.WorkPhaseDecode,
				ModelID:         req.ModelID,
				Cache:           req.Cache,
				TokenIDs:        req.TokenIDs,
				TokenCntTotal:   uint32(len(req.TokenIDs)),
				MaxTokens:       req.MaxTokens,
				Deadline:        req.Deadline,
				GeneratedTokens: req.GeneratedTokens,
				NumNewTokens:    1,
				ReadyAt:         now,
			}
			onWorkItems = append(onWorkItems, decodeItem)
		}

	case v1.EventTypeRequestFinished:
		req.Phase = model.RequestPhaseFinished
	case v1.EventTypeRequestFailed:
		req.Phase = model.RequestPhaseFailed
	case v1.EventTypeRequestCanceled:
		req.Phase = model.RequestPhaseCanceled
	}

	r.mu.Unlock()

	// publish event
	r.mu.RLock()
	subCh, exists := r.subscribeCh[e.RequestId]
	r.mu.RUnlock()
	if exists {
		// todo: subCh <- e 仍然可能阻塞。后面如果流式消费者慢，状态机会被拖住。现在先不动。
		subCh <- e
	} else {
		r.l.Errorf("request %s not subscribed", e.RequestId)
	}

	return onWorkItems, nil
}

func (r *requestStateManager) Cancel(requestId string) {
	r.mu.Lock()
	// take subCh first cause deleteRequest will delete subCh.
	subCh, subscribed := r.subscribeCh[requestId]
	if req, exists := r.requests[requestId]; exists {
		req.Phase = model.RequestPhaseCanceled
		r.deleteRequest(requestId)
	} else {
		delete(r.subscribeCh, requestId)
	}
	r.mu.Unlock()

	if subscribed {
		close(subCh)
	}
}

func (r *requestStateManager) Fail(requestId string, err error) {
	r.mu.Lock()
	// take subCh first cause deleteRequest will delete subCh.
	subCh, subscribed := r.subscribeCh[requestId]
	if req, exists := r.requests[requestId]; exists {
		req.Phase = model.RequestPhaseFailed
		r.deleteRequest(requestId)
	} else {
		delete(r.subscribeCh, requestId)
	}
	r.mu.Unlock()
	if subscribed {
		subCh <- &model.Event{
			WorkId:       utils.MustGenerateUUIDv7(),
			RequestId:    requestId,
			Type:         v1.EventTypeRequestFailed,
			Done:         true,
			FinishReason: v1.FinishReasonError,
			At:           time.Now(),
			Err:          err,
		}
		close(subCh)
	}
}

func (r *requestStateManager) Finish(requestId string) {
	r.mu.Lock()
	// take subCh first cause deleteRequest will delete subCh.
	subCh, subscribed := r.subscribeCh[requestId]
	if req, exists := r.requests[requestId]; exists {
		req.Phase = model.RequestPhaseFinished
		r.deleteRequest(requestId)
	} else {
		delete(r.subscribeCh, requestId)
	}
	r.mu.Unlock()

	if subscribed {
		close(subCh)
	}
}

func (r *requestStateManager) increaseActiveRequestAndCacheHit(hit bool, cachedTokens uint32) {
	r.metrics.SetActiveRequests(int(r.activeRequests.Add(1)))
	r.metrics.IncPrefixCacheRequests(hit)
	if hit {
		r.metrics.AddPrefixCacheTokensSaved(uint64(cachedTokens))
	}
}

func (r *requestStateManager) deleteRequest(requestId string) {
	delete(r.requests, requestId)
	delete(r.subscribeCh, requestId)
	// free all blocks allocated from block.Manager for current request.
	r.blockManager.FreeRequest(requestId)
	r.metrics.SetActiveRequests(int(r.activeRequests.Add(-1)))
}
