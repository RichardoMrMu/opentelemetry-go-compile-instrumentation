// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otelc/test/testutil"
)

func TestOpenAIClient(t *testing.T) {
	t.Parallel()
	testutil.Build(t, "", "openaiclient", "go", "build", "-a")

	testCases := []struct {
		name  string
		model string
	}{{name: "chat_completion", model: "gpt-4"}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := testutil.NewTestFixture(t)
			server := startMockOpenAIServer(t)
			f.Run("openaiclient", fmt.Sprintf("-addr=%s/v1", server.URL), "-api-key=test-key", fmt.Sprintf("-model=%s", tc.model))
			span := f.RequireSingleSpan()
			testutil.RequireGenAIClientSemconv(t, span, "openai", "chat", tc.model, "local", "chatcmpl-test-123", tc.model, []string{"stop"}, 10, 20, 30)
		})
	}
}

// TestOpenAIClientV2 exercises the openai-go/v2 instrumentation package end
// to end via otelc go build, rather than only unit-testing it in isolation.
func TestOpenAIClientV2(t *testing.T) {
	t.Parallel()
	testutil.Build(t, "", "openaiclientv2", "go", "build", "-a")
	f := testutil.NewTestFixture(t)
	server := startMockOpenAIServer(t)
	f.Run("openaiclientv2", fmt.Sprintf("-addr=%s/v1", server.URL), "-api-key=test-key", "-model=gpt-4")
	span := f.RequireSingleSpan()
	testutil.RequireGenAIClientSemconv(t, span, "openai", "chat", "gpt-4", "local", "chatcmpl-test-123", "gpt-4", []string{"stop"}, 10, 20, 30)
}

// TestOpenAIClientV3 exercises the openai-go/v3 instrumentation package end
// to end via otelc go build. This also makes the v3 module part of the
// instrumented-binary govulncheck coverage derived from test/apps.
func TestOpenAIClientV3(t *testing.T) {
	t.Parallel()
	testutil.Build(t, "", "openaiclientv3", "go", "build", "-a")
	f := testutil.NewTestFixture(t)
	server := startMockOpenAIServer(t)
	f.Run("openaiclientv3", fmt.Sprintf("-addr=%s/v1", server.URL), "-api-key=test-key", "-model=gpt-4")
	span := f.RequireSingleSpan()
	testutil.RequireGenAIClientSemconv(t, span, "openai", "chat", "gpt-4", "local", "chatcmpl-test-123", "gpt-4", []string{"stop"}, 10, 20, 30)
}

func startMockOpenAIServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var reqBody struct{ Model string `json:"model"` }
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{"id":"chatcmpl-test-123","object":"chat.completion","model":reqBody.Model,"choices":[]map[string]any{{"index":0,"message":map[string]any{"role":"assistant","content":"Hello!"},"finish_reason":"stop"}},"usage":map[string]any{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}
		if err := json.NewEncoder(w).Encode(resp); err != nil { t.Errorf("failed to encode response: %v", err) }
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}
