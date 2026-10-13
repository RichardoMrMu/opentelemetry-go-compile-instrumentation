// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"

	"github.com/go-resty/resty/v2"
	_ "go.opentelemetry.io/otel/sdk/trace"
)

var (
	addr = flag.String("addr", "http://127.0.0.1:8080", "The server address")
	path = flag.String("path", "/hello", "The request path")
)

func main() {
	flag.Parse()

	client := resty.New()
	targetURL := fmt.Sprintf("%s%s", *addr, *path)
	resp, err := client.R().Get(targetURL)
	if err != nil {
		log.Fatalf("failed to make request: %v", err)
	}

	slog.Info("response", "status", resp.StatusCode(), "body", resp.String())
}
