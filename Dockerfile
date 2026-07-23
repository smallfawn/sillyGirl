# syntax=docker/dockerfile:1

FROM node:20-bookworm-slim AS frontend
WORKDIR /src

COPY frontend/package*.json ./frontend/
RUN cd frontend && if [ -f package-lock.json ]; then npm ci; else npm install; fi

COPY frontend ./frontend
RUN mkdir -p core/admin && cd frontend && npm run build

FROM golang:1.22-bookworm AS builder
ARG VERSION=dev
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend /src/core/admin ./core/admin

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -trimpath \
    -ldflags="-s -w -X github.com/smallfawn/sillyGirl/core.compiled_at=${VERSION}" \
    -o /out/sillyGirl .

FROM debian:bookworm-slim
WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /data/plugins /data/conf /data/language \
    && ln -s /data/plugins /app/plugins \
    && ln -s /data/conf /app/conf \
    && ln -s /data/language /app/language

COPY --from=builder /out/sillyGirl /app/sillyGirl

ENV TZ=Asia/Shanghai \
    SILLYGIRL_DATA_PATH=/data
EXPOSE 8080 50051
VOLUME ["/data"]

ENTRYPOINT ["sh", "-c", "mkdir -p /data/plugins /data/conf /data/language && exec /app/sillyGirl \"$@\"", "--"]
