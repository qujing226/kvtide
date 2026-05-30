package state

import (
	"sync"
	"sync/atomic"
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/cache"
	"github.com/qujing226/mini-llm-serve/internal/errors"
	"github.com/qujing226/mini-llm-serve/internal/metrics"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/qujing226/mini-llm-serve/internal/utils"
	"go.uber.org/zap"
)

type RequestLifecycleStateManager interface {
	Create(req *model.Request) (*model.WorkItem, error)
	Get(requestId string) (*model.Request, bool)
	Subscribe(requestId string) (<-chan *model.Event, error)

	CanSchedule(work *model.WorkItem) bool
	OnEvent(e *model.Event) ([]*model.WorkItem, error)
	Fail(requestId string, err error)

	Cancel(requestId string)
	Finish(requestId string)
}

type requestLifecycleStateManager struct {
	l *zap.SugaredLogger

	requests    map[string]*model.Request
	subscribeCh map[string]chan *model.Event
	mu          sync.RWMutex

	prefixCache cache.PrefixCache

	activeRequests atomic.Int64
	metrics        metrics.Metrics
}

func NewRequestLifecycleStateManager(l *zap.SugaredLogger, prefixCache cache.PrefixCache, metrics metrics.Metrics) RequestLifecycleStateManager {
	r := &requestLifecycleStateManager{
		l:           l,
		requests:    make(map[string]*model.Request),
		subscribeCh: make(map[string]chan *model.Event),
		prefixCache: prefixCache,
		metrics:     metrics,
	}
	return r
}

func (r *requestLifecycleStateManager) Create(req *model.Request) (*model.WorkItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.requests[req.RequestId]; exists {
		return nil, errors.New(errors.CodeInvalidArgument, "request already exists")
	}
	r.requests[req.RequestId] = req
	req.Phase = model.RequestPhasePrefillReady

	r.subscribeCh[req.RequestId] = make(chan *model.Event, 5)

	// prefix cache
	cachedTokens, hit := r.prefixCache.Lookup(req.CacheKey, req.PromptTokens)
	req.CachedTokens = cachedTokens
	req.CacheHit = hit
	req.ComputedTokens = cachedTokens

	// metrics: add active requests
	r.increaseActiveRequestAndCacheHit(hit, cachedTokens)

	now := time.Now()
	var workItem *model.WorkItem
	if req.CachedTokens >= req.PromptTokens {
		workItem = &model.WorkItem{
			WorkId:        utils.MustGenerateUUIDv7(),
			RequestId:     req.RequestId,
			Phase:         v1.WorkPhaseDecode,
			Model:         req.Model,
			Prompt:        req.Prompt,
			MaxTokens:     req.MaxTokens,
			Deadline:      req.Deadline,
			PromptTokens:  req.PromptTokens,
			PrefillOffset: req.CachedTokens,
			NumNewTokens:  1,
			CacheHit:      req.CacheHit,
			ReadyAt:       now,
		}
	} else {
		workItem = &model.WorkItem{
			WorkId:        utils.MustGenerateUUIDv7(),
			RequestId:     req.RequestId,
			Phase:         v1.WorkPhasePrefill,
			Model:         req.Model,
			Prompt:        req.Prompt,
			MaxTokens:     req.MaxTokens,
			Deadline:      req.Deadline,
			PromptTokens:  req.PromptTokens,
			PrefillOffset: req.CachedTokens,
			NumNewTokens:  req.PromptTokens - req.CachedTokens,
			CacheHit:      req.CacheHit,
			ReadyAt:       now,
		}
	}

	return workItem, nil
}

func (r *requestLifecycleStateManager) Get(requestId string) (*model.Request, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if req, exists := r.requests[requestId]; exists {
		return req, true
	}
	return nil, false
}
func (r *requestLifecycleStateManager) Subscribe(requestId string) (<-chan *model.Event, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ch, exists := r.subscribeCh[requestId]
	if !exists {
		return nil, errors.New(errors.CodeInternal, "subscribe channel doesn't exist")
	}
	return ch, nil
}

func (r *requestLifecycleStateManager) CanSchedule(work *model.WorkItem) bool {
	r.mu.Lock()
	req, exists := r.requests[work.RequestId]
	if !exists {
		r.mu.Unlock()
		return false
	}
	if !req.Deadline.IsZero() && time.Now().After(req.Deadline) {
		req.Phase = model.RequestPhaseTimeout
		delete(r.requests, work.RequestId)
		r.reduceActiveRequest()
		subCh, exists := r.subscribeCh[work.RequestId]
		delete(r.subscribeCh, work.RequestId)
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

func (r *requestLifecycleStateManager) OnEvent(e *model.Event) ([]*model.WorkItem, error) {
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
		prefillItem := &model.WorkItem{
			WorkId:        utils.MustGenerateUUIDv7(),
			RequestId:     e.RequestId,
			Phase:         v1.WorkPhasePrefill,
			Model:         req.Model,
			Prompt:        req.Prompt,
			MaxTokens:     req.MaxTokens,
			Deadline:      req.Deadline,
			PromptTokens:  req.PromptTokens,
			PrefillOffset: req.ComputedTokens,
			NumNewTokens:  req.PromptTokens - req.ComputedTokens,
			CacheHit:      false,
			ReadyAt:       now,
		}
		onWorkItems = append(onWorkItems, prefillItem)
	case v1.EventTypePrefillFinished:
		req.ComputedTokens += e.Usage.InputTokens
		if req.ComputedTokens > req.PromptTokens {
			req.ComputedTokens = req.PromptTokens
		}
		req.Usage.InputTokens = req.ComputedTokens
		req.Usage.TotalTokens = req.ComputedTokens + req.GeneratedTokens
		e.Usage = req.Usage
		req.Phase = model.RequestPhaseDecodeReady
		decodeItem := &model.WorkItem{
			WorkId:          utils.MustGenerateUUIDv7(),
			RequestId:       e.RequestId,
			Phase:           v1.WorkPhaseDecode,
			Model:           req.Model,
			Prompt:          req.Prompt,
			MaxTokens:       req.MaxTokens,
			Deadline:        req.Deadline,
			PromptTokens:    req.PromptTokens,
			GeneratedTokens: req.GeneratedTokens,
			NumNewTokens:    1,
			CacheHit:        false,
			ReadyAt:         now,
		}
		onWorkItems = append(onWorkItems, decodeItem)

		// update prefix cache
		if req.ComputedTokens >= req.PromptTokens {
			r.prefixCache.Put(req.CacheKey, req.PromptTokens)
		}
	case v1.EventTypeDecodeChunk:
		req.GeneratedTokens += e.Usage.OutputTokens
		req.Usage.InputTokens = req.PromptTokens
		req.Usage.OutputTokens = req.GeneratedTokens
		req.Usage.TotalTokens = req.PromptTokens + req.GeneratedTokens
		e.Usage = req.Usage
		req.OutputText += e.DeltaText
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
				Model:           req.Model,
				Prompt:          req.Prompt,
				MaxTokens:       req.MaxTokens,
				Deadline:        req.Deadline,
				PromptTokens:    req.PromptTokens,
				GeneratedTokens: req.GeneratedTokens,
				NumNewTokens:    1,
				CacheHit:        false,
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

func (r *requestLifecycleStateManager) Cancel(requestId string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if req, exists := r.requests[requestId]; exists {
		req.Phase = model.RequestPhaseCanceled
		delete(r.requests, requestId)
		r.reduceActiveRequest()
	}
	if subCh, exists := r.subscribeCh[requestId]; exists {
		delete(r.subscribeCh, requestId)
		close(subCh)
	}

}

func (r *requestLifecycleStateManager) Fail(requestId string, err error) {
	r.mu.Lock()
	if req, exists := r.requests[requestId]; exists {
		req.Phase = model.RequestPhaseFailed
		delete(r.requests, requestId)
		r.reduceActiveRequest()
	}
	subCh, exists := r.subscribeCh[requestId]
	delete(r.subscribeCh, requestId)
	r.mu.Unlock()
	if exists {
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

func (r *requestLifecycleStateManager) Finish(requestId string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if req, exists := r.requests[requestId]; exists {
		req.Phase = model.RequestPhaseFinished
		delete(r.requests, requestId)
		r.reduceActiveRequest()
	}
	if subCh, exists := r.subscribeCh[requestId]; exists {
		delete(r.subscribeCh, requestId)
		close(subCh)
	}
}

func (r *requestLifecycleStateManager) increaseActiveRequestAndCacheHit(hit bool, cachedTokens uint32) {
	r.metrics.SetActiveRequests(int(r.activeRequests.Add(1)))
	r.metrics.IncPrefixCacheRequests(hit)
	if hit {
		r.metrics.AddPrefixCacheTokensSaved(uint64(cachedTokens))
	}
}
func (r *requestLifecycleStateManager) reduceActiveRequest() {
	r.metrics.SetActiveRequests(int(r.activeRequests.Add(-1)))
}
