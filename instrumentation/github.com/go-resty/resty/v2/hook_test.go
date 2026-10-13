// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resty

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/otelc/pkg/hook/hooktest"
)

func setupTestTracer(t *testing.T) (*tracetest.SpanRecorder, *sdktrace.TracerProvider) {
	t.Helper()
	initOnce = sync.Once{}
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return sr, tp
}

func TestBeforeAndAfterExecute_Success(t *testing.T) {
	sr, _ := setupTestTracer(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify propagated traceparent on the wire
		assert.NotEmpty(t, r.Header.Get("traceparent"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	client := resty.New()
	req := client.R()
	req.URL = ts.URL + "/test-path"
	req.Method = http.MethodGet

	mockCtx := hooktest.NewMockHookContext()
	BeforeExecute(mockCtx, client, req)

	// Context should have been injected into request headers
	assert.NotEmpty(t, req.Header.Get("traceparent"))

	// Simulate execute completion
	rawResp := &http.Response{
		StatusCode: http.StatusOK,
	}
	resp := &resty.Response{
		Request:     req,
		RawResponse: rawResp,
	}

	AfterExecute(mockCtx, resp, nil)

	spans := sr.Ended()
	require.Len(t, spans, 1)

	span := spans[0]
	assert.Equal(t, "GET", span.Name())
	assert.Equal(t, trace.SpanKindClient, span.SpanKind())
	assert.Equal(t, codes.Unset, span.Status().Code)

	attrMap := make(map[string]interface{})
	for _, kv := range span.Attributes() {
		attrMap[string(kv.Key)] = kv.Value.AsInterface()
	}

	assert.Equal(t, "GET", attrMap[string(semconv.HTTPRequestMethodKey)])
	assert.Equal(t, int64(200), attrMap[string(semconv.HTTPResponseStatusCodeKey)])
}

func TestBeforeAndAfterExecute_ErrorStatus(t *testing.T) {
	sr, _ := setupTestTracer(t)

	client := resty.New()
	req := client.R()
	req.URL = "http://example.com/not-found"
	req.Method = http.MethodGet

	mockCtx := hooktest.NewMockHookContext()
	BeforeExecute(mockCtx, client, req)

	rawResp := &http.Response{
		StatusCode: http.StatusNotFound,
	}
	resp := &resty.Response{
		Request:     req,
		RawResponse: rawResp,
	}

	AfterExecute(mockCtx, resp, nil)

	spans := sr.Ended()
	require.Len(t, spans, 1)

	span := spans[0]
	assert.Equal(t, codes.Error, span.Status().Code)

	attrMap := make(map[string]interface{})
	for _, kv := range span.Attributes() {
		attrMap[string(kv.Key)] = kv.Value.AsInterface()
	}
	assert.Equal(t, int64(404), attrMap[string(semconv.HTTPResponseStatusCodeKey)])
	assert.Equal(t, "404", attrMap[string(semconv.ErrorTypeKey)])
}

func TestBeforeAndAfterExecute_TransportError(t *testing.T) {
	sr, _ := setupTestTracer(t)

	client := resty.New()
	req := client.R()
	req.URL = "http://example.com/error"
	req.Method = http.MethodPost

	mockCtx := hooktest.NewMockHookContext()
	BeforeExecute(mockCtx, client, req)

	transportErr := errors.New("connection refused")
	AfterExecute(mockCtx, nil, transportErr)

	spans := sr.Ended()
	require.Len(t, spans, 1)

	span := spans[0]
	assert.Equal(t, codes.Error, span.Status().Code)
	assert.Len(t, span.Events(), 1)
	assert.Equal(t, "exception", span.Events()[0].Name)
}

func TestBeforeExecute_Disabled(t *testing.T) {
	t.Setenv("OTEL_GO_DISABLED_INSTRUMENTATIONS", "resty")
	sr, _ := setupTestTracer(t)

	client := resty.New()
	req := client.R()
	req.URL = "http://example.com/disabled"
	req.Method = http.MethodGet

	mockCtx := hooktest.NewMockHookContext()
	BeforeExecute(mockCtx, client, req)

	assert.Nil(t, mockCtx.GetData())
	assert.Empty(t, req.Header.Get("traceparent"))

	AfterExecute(mockCtx, nil, nil)
	assert.Empty(t, sr.Ended())
}
