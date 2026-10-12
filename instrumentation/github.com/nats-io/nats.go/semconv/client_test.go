// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package semconv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
)

func spanAttrMap(attrs []attribute.KeyValue) map[attribute.Key]attribute.Value {
	m := make(map[attribute.Key]attribute.Value, len(attrs))
	for _, a := range attrs {
		m[a.Key] = a.Value
	}
	return m
}

func TestSpanName(t *testing.T) {
	tests := []struct {
		name string
		req  Request
		want string
	}{
		{
			name: "send with subject",
			req:  Request{Destination: "orders.created", Operation: NatsOperationSend},
			want: "orders.created send",
		},
		{
			name: "receive with subject",
			req:  Request{Destination: "orders.created", Operation: NatsOperationReceive},
			want: "orders.created receive",
		},
		{
			name: "process with subject",
			req:  Request{Destination: "orders.created", Operation: NatsOperationProcess},
			want: "orders.created process",
		},
		{
			name: "empty subject falls back to operation only",
			req:  Request{Operation: NatsOperationSend},
			want: "send",
		},
		{
			name: "whitespace subject is trimmed",
			req:  Request{Destination: "  ", Operation: NatsOperationSend},
			want: "send",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, SpanName(tt.req))
		})
	}
}

func TestTraceAttrs(t *testing.T) {
	attrs := TraceAttrs(Request{
		Destination:     "orders.created",
		Operation:       NatsOperationSend,
		Endpoint:        "nats.example.com:4222",
		MessageBodySize: 5,
	})
	m := spanAttrMap(attrs)

	assert.Equal(t, "nats", m["messaging.system"].AsString())
	assert.Equal(t, "send", m["messaging.operation.name"].AsString())
	assert.Equal(t, "send", m["messaging.operation.type"].AsString())
	assert.Equal(t, "orders.created", m["messaging.destination.name"].AsString())
	assert.Equal(t, "nats.example.com", m["server.address"].AsString())
	_, hasPort := m["server.port"]
	assert.False(t, hasPort, "default NATS port 4222 must be omitted")
	assert.Equal(t, int64(5), m["messaging.message.body.size"].AsInt64())
}

func TestTraceAttrs_NonDefaultPort(t *testing.T) {
	attrs := TraceAttrs(Request{
		Destination: "orders.created",
		Operation:   NatsOperationProcess,
		Endpoint:    "nats.example.com:6222",
	})
	m := spanAttrMap(attrs)

	assert.Equal(t, "nats.example.com", m["server.address"].AsString())
	assert.Equal(t, int64(6222), m["server.port"].AsInt64())
	assert.Equal(t, "process", m["messaging.operation.type"].AsString())
}

func TestTraceAttrs_EmptyEndpoint(t *testing.T) {
	attrs := TraceAttrs(Request{
		Destination: "orders.created",
		Operation:   NatsOperationReceive,
	})
	m := spanAttrMap(attrs)

	_, hasAddr := m["server.address"]
	assert.False(t, hasAddr, "no server.address when the endpoint is unknown")
	_, hasPort := m["server.port"]
	assert.False(t, hasPort)
}

func TestTraceAttrs_ZeroBodySizeOmitted(t *testing.T) {
	attrs := TraceAttrs(Request{
		Destination: "orders.created",
		Operation:   NatsOperationSend,
	})
	m := spanAttrMap(attrs)

	_, hasSize := m["messaging.message.body.size"]
	assert.False(t, hasSize, "body size 0 (or unset) must not add a size attribute")
}

func TestSplitHostPort(t *testing.T) {
	tests := []struct {
		addr     string
		wantHost string
		wantPort int
		wantOK   bool
	}{
		{addr: "nats:4222", wantHost: "nats", wantPort: 0, wantOK: true},
		{addr: "nats:6222", wantHost: "nats", wantPort: 6222, wantOK: true},
		{addr: "127.0.0.1:5555", wantHost: "127.0.0.1", wantPort: 5555, wantOK: true},
		{addr: "nats", wantOK: false},
		{addr: "nats:4x22", wantOK: false},
		{addr: "nats:", wantHost: "nats", wantPort: 0, wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			host, port, ok := splitHostPort(tt.addr)
			require.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantHost, host)
				assert.Equal(t, tt.wantPort, port)
			}
		})
	}
}
