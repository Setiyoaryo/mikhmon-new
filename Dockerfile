# syntax=docker/dockerfile:1
#
# mikhmon-api — Go microservice (Mikhmon hybrid backend helper).
# Built here only; the PHP frontend is served by Dockerfile.php.

# ---------- Stage 1: build ----------
FROM golang:1.22-alpine AS builder

WORKDIR /src

# Copy module files first for layer caching.
# go.sum may not exist yet, so the wildcard keeps the copy from failing.
COPY go.mod go.sum* ./
RUN go mod download || true

# Copy the rest of the sources (cmd/, internal/, ...) and build a static binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/mikhmon-api ./cmd/mikhmon-api

# ---------- Stage 2: runtime ----------
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S mikhmon \
    && adduser -S -G mikhmon -h /home/mikhmon mikhmon

COPY --from=builder /out/mikhmon-api /usr/local/bin/mikhmon-api
RUN chmod 0755 /usr/local/bin/mikhmon-api

USER mikhmon

EXPOSE 8088

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8088/healthz || exit 1

ENTRYPOINT ["/usr/local/bin/mikhmon-api"]
