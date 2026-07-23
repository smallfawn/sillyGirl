# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM node:24-bookworm-slim AS frontend
WORKDIR /src

COPY frontend/package*.json ./frontend/
RUN --mount=type=cache,target=/root/.npm \
    cd frontend && if [ -f package-lock.json ]; then npm ci; else npm install; fi

COPY frontend ./frontend
RUN mkdir -p core/admin && cd frontend && npm run build

FROM --platform=$BUILDPLATFORM golang:1.22-bookworm AS builder
ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
COPY --from=frontend /src/core/admin ./core/admin

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build \
    -trimpath \
    -ldflags="-s -w -X github.com/smallfawn/sillyGirl/core.compiled_at=${VERSION}" \
    -o /out/sillyGirl .

FROM --platform=$TARGETPLATFORM node:24-bookworm-slim
WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && corepack enable \
    && corepack prepare pnpm@11.16.0 --activate \
    && mkdir -p /data/plugins /data/conf \
    && ln -s /data/plugins /app/plugins \
    && ln -s /data/conf /app/conf

COPY --from=builder /out/sillyGirl /app/sillyGirl
COPY --from=builder /src/proto3/sillygirl.js /app/proto3/sillygirl.js
COPY --from=builder /src/proto3/sillygirl.d.ts /app/proto3/sillygirl.d.ts
COPY --from=builder /src/proto3/srpc.js /app/proto3/srpc.js

ENV TZ=Asia/Shanghai \
    SILLYGIRL_DATA_PATH=/data
EXPOSE 8080 50051
VOLUME ["/data"]

ENTRYPOINT ["sh", "-c", "mkdir -p /data/plugins /data/conf && rm -rf /data/language/node && rmdir /data/language 2>/dev/null || true; exec /app/sillyGirl \"$@\"", "--"]
