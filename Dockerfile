# syntax=docker/dockerfile:1.6
#
# SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
# Copyright (C) 2026 Packrune Contributors
#
# Multi-stage build that produces a static, scratch-based image (~15 MB).
# Pure-Go SQLite (modernc.org/sqlite) lets us skip CGO entirely, so the
# final binary runs on any kernel without a libc.

# --- Stage 1: build the frontend -------------------------------------------
FROM node:20-alpine AS web
WORKDIR /app
COPY web/package.json web/pnpm-lock.yaml* ./web/
COPY web/ ./web/
RUN mkdir -p ./internal/web/dist
WORKDIR /app/web
RUN corepack enable && corepack prepare pnpm@9.12.0 --activate
RUN pnpm install --no-frozen-lockfile && pnpm build

# --- Stage 2: build the Go binary ------------------------------------------
FROM golang:1.24-alpine AS build
WORKDIR /src
RUN apk add --no-cache git make
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Replace the in-tree placeholder dist/ with the freshly-built frontend.
COPY --from=web /app/internal/web/dist ./internal/web/dist
ARG VERSION=docker
ARG COMMIT=unknown
ARG DATE=unknown
RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /out/packrune ./cmd/packrune

# --- Stage 3: minimal runtime ----------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/packrune /app/packrune
EXPOSE 8080
USER nonroot:nonroot
VOLUME ["/data"]
ENV PACKRUNE_DATABASE_DSN=/data/packrune.db \
    PACKRUNE_STORAGE_FS_ROOT=/data/storage \
    PACKRUNE_SERVER_ADDR=:8080
ENTRYPOINT ["/app/packrune", "serve"]
