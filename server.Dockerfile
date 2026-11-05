# Builder stage
FROM golang:1.27.3-alpine AS builder

ENV CGO_ENABLED=0

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN mkdir -p /app/bin && \
    go build -mod=readonly -o /app/bin/server-linux-amd64 ./cmd/server && \
    go build -mod=readonly -o /app/bin/configure-linux-amd64 ./cmd/configure && \
    go build -mod=readonly -o /app/bin/db-release ./cmd/db-release && \
    go -C tools/migrate mod download && \
    go -C tools/migrate mod verify && \
    go -C tools/migrate build -mod=readonly -o /app/migrate .

# Final stage
FROM ubuntu:24.04 AS runner

# Install CA certificates for TLS verification (required for AWS Cognito JWKS)
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir /app && \
    groupadd --gid 1001 appgroup && \
    useradd --uid 1001 --gid 1001 --shell /bin/bash --home /app appuser && \
    chown -R appuser:appgroup /app


COPY --from=builder --chown=appuser:appgroup /app/bin/server-linux-amd64 /app/server
COPY --from=builder --chown=appuser:appgroup /app/bin/configure-linux-amd64 /app/configure
COPY --from=builder --chown=appuser:appgroup /app/migrate /app/migrate
COPY --from=builder --chown=appuser:appgroup /app/bin/db-release /app/db-release
COPY --from=builder --chown=appuser:appgroup /app/release /app/release
COPY --from=builder --chown=appuser:appgroup /app/internal/database/migrations /app/migrations
COPY --from=builder --chown=appuser:appgroup /app/api /app/api
COPY --from=builder --chown=appuser:appgroup /app/scripts/start_server.sh /app/start_server.sh
COPY --from=builder --chown=appuser:appgroup /app/scripts/run_migrations.sh /app/run_migrations.sh

RUN chmod +x /app/start_server.sh /app/run_migrations.sh

WORKDIR /app

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

CMD ["/app/start_server.sh"]
