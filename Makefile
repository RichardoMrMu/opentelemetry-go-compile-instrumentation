# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

# Use bash for all shell commands (required for pipefail and other bash features)
SHELL := /bin/bash

.PHONY: all test test-unit test-integration test-e2e format lint build build-all build/pkg install package manifest verify-manifest clean setup-git \
        build-demo build-demo-grpc build-demo-http format/go format/yaml lint/go lint/yaml \
        lint/action lint/makefile lint/license-header lint/license-header/fix lint/dockerfile actionlint yamlfmt gotestfmt ratchet ratchet/pin \
        ratchet/update ratchet/check golangci-lint embedmd checkmake hadolint help docs check-embed check-api-sync check-golden-files check-test-file-naming \
        test-unit/update-golden test-unit/tool test-unit/pkg test-unit/instrumentation test-unit/demo test-unit/helper \
        test-unit/coverage test-unit/tool/coverage test-unit/pkg/coverage test-unit/instrumentation/coverage \
        check-coverage test-integration/coverage test-e2e/coverage test-latestlibrun test-versionmatrix \
        weaver-install tidy/test-apps \
        fetch-upstream-semconv lint-schema \
        adr-tools adr-new adr-list \
        benchmark/codspeed benchmark/threshold govulncheck govulncheck/instrumentation

# Constant variables
BINARY_NAME := otelc
PLATFORMS := darwin/amd64 linux/amd64 windows/amd64 darwin/arm64 linux/arm64
TOOL_DIR := tool/cmd/otelc
INST_BUNDLE_ARCHIVE = otelc-bundle.tgz
INST_BUNDLE_PKG_TMP = pkg_temp
INST_BUNDLE_INST_TMP = instrumentation_temp
API_SYNC_SOURCE = pkg/hook/context.go
API_SYNC_TARGET = tool/internal/instrument/api.tmpl
TOOLS_DIR = .tools
GO_VERSION = 1.26
INTEGRATION_TEST_RUN ?= .
TOOL_COVERAGE_THRESHOLD ?= 68
PKG_COVERAGE_THRESHOLD ?= 70

# Modules/apps scanned by govulncheck for known Go CVEs (tool version pinned in
# .tools/go.mod, Renovate-managed like every other .tools binary).
# Core modules: root module (covers tool/) plus every pkg/ module.
# Instrumentation is scanned separately via instrumented integration-test binaries
# (see govulncheck/instrumentation): source scans skip //go:build ignore files and
# cannot typecheck modules that need compile-time field injection (e.g. database/sql).
# This coverage is only as complete as the top-level apps under test/apps.
# Demos are intentionally excluded (pinned example deps).
GOVULNCHECK_CORE_MODULES := . $(shell find pkg -type f -name 'go.mod' -exec dirname {} \; | sort)
# Top-level integration apps only (skip nested modules such as
# test/apps/gincustom/instrumentation).
GOVULNCHECK_TEST_APPS := $(shell find test/apps -mindepth 1 -maxdepth 1 -type d | sort)
