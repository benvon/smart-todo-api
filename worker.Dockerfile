# Builder stage
FROM golang:1.25.5-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download && go mod verify && go mod tidy && go mod vendor
COPY . .
RUN mkdir -p /app/bin && \
    go build -o /app/bin/worker-linux-amd64 ./cmd/worker

# Final stage
FROM ubuntu:24.04 AS runner

# Install CA certificates for TLS verification (required for AWS Cognito JWKS and OpenAI API)
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir /app && \
    groupadd --gid 1001 appgroup && \
    useradd --uid 1001 --gid 1001 --shell /bin/bash --home /app appuser && \
    chown -R appuser:appgroup /app

COPY --from=builder --chown=appuser:appgroup /app/bin/worker-linux-amd64 /app/worker

RUN chmod +x /app/worker

WORKDIR /app

USER appuser

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ps aux | grep -v grep | grep -q worker || exit 1

CMD ["/app/worker"]
