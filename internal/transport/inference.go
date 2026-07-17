package transport

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/cockroachdb/errors"
	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
	"github.com/qujing226/kvtide/gen/go/kvtide/v1/kvtidev1connect"
	"github.com/qujing226/kvtide/internal/conf"
	appErrors "github.com/qujing226/kvtide/internal/errors"
	"github.com/qujing226/kvtide/internal/handler"
	"github.com/qujing226/kvtide/internal/metrics"
	"github.com/qujing226/kvtide/internal/model"
	"github.com/qujing226/kvtide/internal/utils"
	"go.uber.org/fx"
	"go.uber.org/zap"
	brotli "go.withmatt.com/connect-brotli"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

const (
	ServerShutdownTimeout = 30 * time.Second
	CompressionMinBytes   = 1024 // 1KB
)

type inferenceService struct {
	l                *zap.SugaredLogger
	InferenceHandler handler.InferenceHandler
	metrics          metrics.Metrics
}

type InferenceHTTPServer struct {
	Server *http.Server
}

func NewLLMServingServer(l *zap.SugaredLogger, serverConf *conf.Conf, e handler.InferenceHandler, metrics metrics.Metrics) *InferenceHTTPServer {
	mux := http.NewServeMux()
	svc := &inferenceService{
		l:                l,
		InferenceHandler: e,
		metrics:          metrics,
	}

	path, handler := kvtidev1connect.NewInferenceServiceHandler(
		svc,
		connect.WithInterceptors(),
		connect.WithCompressMinBytes(CompressionMinBytes),
		brotli.WithCompression(),
	)
	mux.Handle(path, handler)

	h2cHandler := h2c.NewHandler(mux, &http2.Server{})
	handlerWithCORS := withCORS(serverConf.Server.AllowedOrigins, h2cHandler)

	port, err := utils.ExtractPortNumber(serverConf.Server.Address)
	if err != nil {
		panic(err)
	}
	addr := fmt.Sprintf("0.0.0.0:%d", port)

	srv := &http.Server{
		Addr:                         addr,
		Handler:                      handlerWithCORS,
		DisableGeneralOptionsHandler: false,

		ReadTimeout:       3 * time.Second,
		ReadHeaderTimeout: 3 * time.Second,
		// Inference requests can legitimately run longer than a fixed HTTP
		// write deadline, especially unary calls that do not send headers until
		// generation is complete. Request-level deadlines own cancellation.
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}

	return &InferenceHTTPServer{
		Server: srv,
	}
}

func StartServer(lc fx.Lifecycle, i *InferenceHTTPServer, a *AdminServer, logger *zap.SugaredLogger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			iListener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", i.Server.Addr)
			if err != nil {
				logger.Errorw("[inference] failed to listen", "addr", i.Server.Addr, "err", err)
				return err
			}
			logger.Infof("[inference] listening on %s", i.Server.Addr)

			aListener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", a.Server.Addr)
			if err != nil {
				logger.Errorw("[admin] failed to listen", "addr", a.Server.Addr, "err", err)
				return err
			}
			logger.Infof("[admin] listening on %s", a.Server.Addr)

			go func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Infow("recovered from panic", "err", r)
					}
				}()
				if serErr := i.Server.Serve(iListener); serErr != nil &&
					!errors.Is(serErr, http.ErrServerClosed) {
					logger.Errorw("[inference] failed to serve", "addr", i.Server.Addr, "err", serErr)
				}
			}()

			go func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Infow("recovered from panic", "err", r)
					}
				}()
				if serErr := a.Server.Serve(aListener); serErr != nil &&
					!errors.Is(serErr, http.ErrServerClosed) {
					logger.Errorw("[admin] failed to serve", "addr", a.Server.Addr, "err", serErr)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {

			logger.Info("shutting down [inference] Server...")
			if err := i.Server.Shutdown(ctx); err != nil {
				logger.Errorw("failed to shutdown [inference] Server", "err", err)
				return err
			}
			logger.Info("shutting down [admin] Server...")
			if err := a.Server.Shutdown(ctx); err != nil {
				logger.Errorw("failed to shutdown [admin] Server", "err", err)
				return err
			}
			logger.Info("Server shutdown gracefully")
			return nil
		},
	})
}

func (i *inferenceService) Generate(ctx context.Context, request *v1.GenerateRequest) (*v1.GenerateResponse, error) {
	var (
		status           = "ok"
		executorId       = "unknown"
		requestStartTime = time.Now()

		firstTokenAt time.Time
		lastTokenAt  time.Time
	)
	defer func() {
		requestEndTime := time.Now()
		i.metrics.ObserveRequestDuration(requestEndTime.Sub(requestStartTime).Seconds())
		i.metrics.IncRequest(status, executorId)
	}()

	req, err := model.ProtoMsgToModel(request)
	if err != nil {
		status = string(appErrors.CodeOf(err))
		return nil, appErrors.ToConnectError(err)
	}
	ch, err := i.InferenceHandler.GenerateStream(ctx, req)
	if err != nil {
		status = string(appErrors.CodeOf(err))
		return nil, appErrors.ToConnectError(err)
	}
	out := &model.GenerateOutput{RequestId: request.RequestId}

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				status = string(appErrors.CodeInternal)
				return nil, appErrors.ToConnectError(appErrors.New(appErrors.CodeInternal, "stream closed before completion"))
			}
			out.DeltaText += msg.DeltaText
			if msg.DeltaText != "" {
				now := time.Now()
				if firstTokenAt.IsZero() {
					firstTokenAt = now
					i.metrics.ObserveTTFT(firstTokenAt.Sub(requestStartTime).Seconds())
				} else if !lastTokenAt.IsZero() {
					i.metrics.ObserveTBT(now.Sub(lastTokenAt).Seconds())
				}
				lastTokenAt = now
			}
			if msg.Done {
				out.Usage = msg.Usage
				out.BatchID = msg.BatchID
				out.BatchSize = msg.BatchSize
				out.ExecutorId = msg.ExecutorId
				out.Done = msg.Done
				out.Timing = msg.Timing
				out.FinishReason = msg.FinishReason
				out.Index = msg.Index
				out.Err = msg.Err
				executorId = msg.ExecutorId
				goto ret
			}

		case <-ctx.Done():
			status = string(appErrors.CodeOf(ctx.Err()))
			return nil, appErrors.ToConnectError(ctx.Err())
		}
	}
ret:
	resp, err := model.ModelToProtoMsg(out)
	if err != nil {
		status = string(appErrors.CodeOf(err))
		return nil, appErrors.ToConnectError(err)
	}
	return resp, nil
}

func (i *inferenceService) GenerateStream(ctx context.Context, request *v1.GenerateRequest, c *connect.ServerStream[v1.GenerateResponseChunk]) error {
	var (
		status           = "ok"
		executorId       = "unknown"
		requestStartTime = time.Now()

		firstTokenAt time.Time
		lastTokenAt  time.Time
	)
	defer func() {
		requestEndTime := time.Now()
		i.metrics.ObserveRequestDuration(requestEndTime.Sub(requestStartTime).Seconds())
		i.metrics.IncRequest(status, executorId)
	}()

	req, err := model.ProtoMsgToModel(request)
	if err != nil {
		status = string(appErrors.CodeOf(err))
		return appErrors.ToConnectError(err)
	}
	ch, err := i.InferenceHandler.GenerateStream(ctx, req)
	if err != nil {
		status = string(appErrors.CodeOf(err))
		return appErrors.ToConnectError(err)
	}
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				status = string(appErrors.CodeInternal)
				return appErrors.ToConnectError(appErrors.New(appErrors.CodeInternal, "stream closed before completion"))
			}
			if msg.ExecutorId != "" {
				executorId = msg.ExecutorId
			}
			if msg.DeltaText != "" {
				now := time.Now()
				if firstTokenAt.IsZero() {
					firstTokenAt = now
					i.metrics.ObserveTTFT(firstTokenAt.Sub(requestStartTime).Seconds())
				} else if !lastTokenAt.IsZero() {
					i.metrics.ObserveTBT(now.Sub(lastTokenAt).Seconds())
				}
				lastTokenAt = now
			}
			resp, transferErr := model.ModelToProtoMsgStream(msg)
			if transferErr != nil {
				status = string(appErrors.CodeOf(transferErr))
				return appErrors.ToConnectError(transferErr)
			}
			sendErr := c.Send(resp)
			if sendErr != nil {
				status = string(appErrors.CodeOf(sendErr))
				return appErrors.ToConnectError(sendErr)
			}

			if msg.Done {
				return nil
			}
		case <-ctx.Done():
			status = string(appErrors.CodeOf(ctx.Err()))
			return appErrors.ToConnectError(ctx.Err())
		}
	}
}
