// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resty

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	httpsemconv "go.opentelemetry.io/otelc/instrumentation/net/http/semconv"
	"go.opentelemetry.io/otelc/pkg/hook"
	"go.opentelemetry.io/otelc/pkg/runtime"
)

const (
	instrumentationName = "go.opentelemetry.io/otelc/instrumentation/github.com/go-resty/resty/v2"
	instrumentationKey  = "RESTY"
)

var (
	logger     = runtime.Logger()
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
	httpClient = httpsemconv.HTTPClient{}
	initOnce   sync.Once
)

func initInstrumentation() {
	initOnce.Do(func() {
		tracer = otel.GetTracerProvider().Tracer(
			instrumentationName,
			trace.WithInstrumentationVersion(runtime.ModuleVersion()),
		)
		propagator = otel.GetTextMapPropagator()
		logger.Info("Resty client instrumentation initialized")
	})
}

func debugEnabled() bool {
	return logger.Enabled(context.Background(), slog.LevelDebug)
}

type restyClientEnabler struct{}

func (restyClientEnabler) Enable() bool {
	return runtime.Instrumented(instrumentationKey)
}

var clientEnabler = restyClientEnabler{}

type hookData struct {
	span  trace.Span
	start time.Time
}

// BeforeExecute is invoked before (*Client).execute(req *Request).
func BeforeExecute(ictx hook.HookContext, c *resty.Client, req *resty.Request) {
	if !clientEnabler.Enable() {
		return
	}

	if req == nil {
		return
	}

	initInstrumentation()

	// Extract parent context from Request
	ctx := req.Context()

	// Build raw HTTP request representation for attribute extraction
	var rawReq *http.Request
	if req.RawRequest != nil {
		rawReq = req.RawRequest
	} else {
		// If RawRequest is not built yet (execute builds it in executeBefore),
		// synthesize standard http.Request view
		urlStr := req.URL
		method := req.Method
		if method == "" {
			method = http.MethodGet
		}
		var err error
		rawReq, err = http.NewRequestWithContext(ctx, method, urlStr, nil)
		if err != nil {
			if debugEnabled() {
				logger.Debug("failed to parse resty request URL for tracing", "error", err)
			}
			return
		}
		if req.Header != nil {
			rawReq.Header = req.Header
		}
	}

	attrs := httpClient.RequestTraceAttrs(rawReq)
	spanName := httpsemconv.HTTPClientSpanName(rawReq.Method)

	ctx, span := tracer.Start(ctx,
		spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	)

	// Suppress standard net/http client spans to prevent duplicate spans for Resty calls
	ctx = runtime.SuppressHTTPClientInstrumentation(ctx)

	// Inject trace context into Request headers
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))

	// Update Request context so downstream hooks/transports inherit it
	req.SetContext(ctx)

	ictx.SetData(&hookData{
		span:  span,
		start: time.Now(),
	})
}

// AfterExecute is invoked after (*Client).execute(req *Request) returns (*Response, error).
func AfterExecute(ictx hook.HookContext, resp *resty.Response, err error) {
	data, ok := ictx.GetData().(*hookData)
	if !ok || data == nil || data.span == nil {
		return
	}
	defer data.span.End()

	if resp != nil && resp.RawResponse != nil {
		attrs := httpClient.ResponseTraceAttrs(resp.RawResponse)
		data.span.SetAttributes(attrs...)
		statusCode := resp.StatusCode()
		code, desc := httpClient.Status(statusCode)
		data.span.SetStatus(code, desc)
	}

	if err != nil {
		data.span.RecordError(err)
		errAttr := httpClient.ErrorType(err)
		if errAttr.Key != "" {
			data.span.SetAttributes(errAttr)
		}
		data.span.SetStatus(httpClient.Status(500))
	}
}
