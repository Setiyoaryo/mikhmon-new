# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /build
COPY go.mod ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o mikhmon ./cmd/mikhmon

# Runtime stage
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /build/mikhmon .
VOLUME /app/data
ENV MIKHMON_CONFIG=/app/data/config.json
EXPOSE 8080
ENTRYPOINT ["./mikhmon", "-config", "/app/data/config.json", "-port", "8080"]
