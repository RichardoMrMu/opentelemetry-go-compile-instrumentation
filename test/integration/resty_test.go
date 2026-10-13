// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"go.opentelemetry.io/otelc/test/testutil"
)

func TestRestyClient(t *testing.T) {
	t.Parallel()
	testutil.Build(t, "", "restyclient", "go", "build", "-a")

	t.Run("Basic", func(t *testing.T) {
		f := testutil.NewTestFixture(t)
		server := StartHTTPServerWithResponse(t, http.StatusOK, `{"status":"ok"}`)

		out := f.Run("restyclient", "-addr="+server.URL, "-path=/hello")
		require.Contains(t, out, "status=200")

		span := f.RequireSingleSpan()
		require.Equal(t, "GET", span.Name())
		require.Equal(t, ptrace.SpanKindClient, span.Kind())

		expectedURL := server.URL + "/hello"
		testutil.RequireHTTPClientSemconv(
			t,
			span,
			"GET",
			expectedURL,
			"127.0.0.1",
			200,
			server.Port(),
			"1.1",
			"http",
		)
	})

	t.Run("Disabled", func(t *testing.T) {
		f := testutil.NewTestFixture(t)
		f.SetEnv("OTEL_GO_DISABLED_INSTRUMENTATIONS", "resty")

		server := StartHTTPServerWithResponse(t, http.StatusOK, `{"status":"ok"}`)

		out := f.Run("restyclient", "-addr="+server.URL, "-path=/hello")
		require.Contains(t, out, "status=200")

		span := f.RequireSingleSpan()
		require.Equal(t, "GET", span.Name())
		require.Equal(t, ptrace.SpanKindClient, span.Kind())
	})
}
