# resty/v2 instrumentation

Compile-time OpenTelemetry instrumentation for `github.com/go-resty/resty/v2`.

## Overview

Hooks `(*Client).execute` to start an HTTP client span for every request sent through Resty, injecting W3C `traceparent` and `tracestate` headers and recording response status / errors per OpenTelemetry semantic conventions.