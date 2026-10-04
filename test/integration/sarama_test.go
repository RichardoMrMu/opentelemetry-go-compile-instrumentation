// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package test

import (
	"fmt"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"go.opentelemetry.io/otelc/test/testutil"
)

func TestSaramaClient(t *testing.T) {
	t.Parallel()
	testutil.Build(t, "", "saramaclient", "go", "build", "-a")

	broker := startKafkaMock(t)

	f := testutil.NewTestFixture(t)
	frontPort := testutil.FreePort(t)
	f.Start("saramaclient",
		fmt.Sprintf("-broker-addr=%s", broker.Addr()),
		fmt.Sprintf("-front-port=%d", frontPort),
	)
	testutil.WaitForTCP(t, fmt.Sprintf("127.0.0.1:%d", frontPort))

	// Produce: one SyncProducer.SendMessage against the mock broker produces
	// one producer span.
	produceBody := hitEndpoint(t, frontPort, "/produce")
	require.Contains(t, produceBody, "partition=0")

	f.WaitForSpans(2)
	testutil.RequireSpan(
		t,
		f.Traces(),
		testutil.IsServer,
		func(s ptrace.Span) bool { return s.Name() == "GET /produce" },
	)
	producerSpan := testutil.RequireSpan(
		t,
		f.Traces(),
		testutil.IsProducer,
		func(s ptrace.Span) bool { return s.Name() == "orders send" },
	)
	// SendMessage has no context parameter and the app does not put one in
	// ProducerMessage.Metadata, so the producer span is a root span on its own
	// trace, not a child of the HTTP span.
	require.True(t, producerSpan.ParentSpanID().IsEmpty(), "producer span must be a root span")
	testutil.RequireSaramaProducerSemconv(t, producerSpan, "orders", broker.Addr())

	// Consume: one ConsumePartition read produces one consumer span. The mock
	// broker does not echo message headers back, so no trace context is
	// extracted and the consumer span is also a root span; header propagation
	// between producer and consumer is covered by the unit tests.
	consumeBody := hitEndpoint(t, frontPort, "/consume")
	require.Contains(t, consumeBody, "received")

	f.WaitForSpans(4)
	consumerSpan := testutil.RequireSpan(
		t,
		f.Traces(),
		testutil.IsConsumer,
		func(s ptrace.Span) bool { return s.Name() == "orders receive" },
	)
	require.True(t, consumerSpan.ParentSpanID().IsEmpty(), "consumer span must be a root span")
	testutil.RequireSaramaConsumerSemconv(t, consumerSpan, "orders", "0", 0)
}

// startKafkaMock starts an in-process Kafka broker that accepts a metadata
// round-trip, a successful produce, an offset lookup and a single fetch.
func startKafkaMock(t *testing.T) *sarama.MockBroker {
	t.Helper()
	broker := sarama.NewMockBroker(t, 1)
	broker.SetHandlerByMap(map[string]sarama.MockResponse{
		// NewClient opens a broker connection before the first metadata fetch,
		// and the client library probes ApiVersions on that connection; without
		// a handler the client waits for a response until its read timeout.
		// The advertised api keys must cover the APIs this client actually uses
		// (Produce, Fetch, Offset, Metadata) or the version negotiation rejects
		// the metadata round-trip with "The version of API is not supported".
		"ApiVersionsRequest": sarama.NewMockApiVersionsResponse(t).SetApiKeys([]sarama.ApiVersionsResponseKey{
			{ApiKey: 0, MinVersion: 0, MaxVersion: 3}, // Produce
			{ApiKey: 1, MinVersion: 0, MaxVersion: 4}, // Fetch
			{ApiKey: 2, MinVersion: 0, MaxVersion: 1}, // Offset
			{ApiKey: 3, MinVersion: 0, MaxVersion: 1}, // Metadata
		}),
		"MetadataRequest": sarama.NewMockMetadataResponse(t).
			SetBroker(broker.Addr(), broker.BrokerID()).
			SetLeader("orders", 0, broker.BrokerID()),
		"ProduceRequest": sarama.NewMockProduceResponse(t),
		"OffsetRequest": sarama.NewMockOffsetResponse(t).
			SetOffset("orders", 0, sarama.OffsetNewest, 0).
			SetOffset("orders", 0, sarama.OffsetOldest, 0),
		"FetchRequest": sarama.NewMockFetchResponse(t, 1).
			SetMessage("orders", 0, 0, sarama.StringEncoder("hello")),
	})
	t.Cleanup(func() { broker.Close() })
	return broker
}
