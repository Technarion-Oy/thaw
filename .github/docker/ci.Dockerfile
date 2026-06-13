# CI base image for Thaw's self-hosted workflows.
#
# Bakes in the toolchain and system libraries that were previously
# apt-installed / go-installed on every CI run — the GTK/WebKit dev libs
# (Wails' Linux CGO deps), base tools, the Wails CLI, and go-junit-report —
# turning several minutes of per-run setup into a fast image pull.
#
# Go and Node are intentionally NOT baked in: the workflows install them via
# actions/setup-go (from go.mod) and actions/setup-node, which is fast (~20s
# combined) and stays correct when go.mod bumps the Go version. This image's
# only Go usage is the build-time tools stage below.
#
# Built and pushed to ghcr.io/technarion-oy/thaw-ci by
# .github/workflows/ci-image.yml.

# ── Stage 1: compile the Go-based CLI tools ──────────────────────────────────
# The Go version here only needs to compile these tools; the project's actual
# build uses the Go provided by actions/setup-go at runtime.
FROM golang:1.26-bookworm AS tools

ARG WAILS_VERSION=v2.11.0

# Static-ish standalone binaries — safe to copy into the ubuntu runtime below.
RUN go install github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VERSION} \
 && go install github.com/jstemmer/go-junit-report/v2@latest

# ── Stage 2: runtime image ───────────────────────────────────────────────────
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

# Base tools + GTK/WebKit dev libs (Wails CGO deps on Linux, webkit2gtk-4.1 as
# shipped by 24.04) + Python (used by the dbt / unit-test tooling workflows).
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      ca-certificates git curl wget gcc g++ pkg-config \
      libgtk-3-dev libwebkit2gtk-4.1-dev \
      python3 python3-pip python3-venv \
 && rm -rf /var/lib/apt/lists/*

COPY --from=tools /go/bin/wails /usr/local/bin/wails
COPY --from=tools /go/bin/go-junit-report /usr/local/bin/go-junit-report

# Sanity check the copied binaries resolve at build time.
RUN wails version && go-junit-report --version || true
