# Build stage
FROM golang:1.21-bullseye AS builder
WORKDIR /src
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod tidy && go build -o /out/manager ./cmd/manager && go build -o /out/agent ./cmd/agent

# Runtime
FROM gcr.io/distroless/base-debian12
WORKDIR /
COPY --from=builder /out/manager /usr/local/bin/v2mgr
COPY --from=builder /out/agent /usr/local/bin/v2agent
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/v2mgr"]