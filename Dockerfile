# Stage 1: Build binary
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy dependency definitions
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build CGO-free static executable
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o pulsematrix main.go

# Stage 2: Minimal Runtime image
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for TLS / HTTP calls to Gemini & ExchangeRate APIs
RUN apk add --no-cache ca-certificates tzdata wget \
    && mkdir -p /app/data

COPY --from=builder /app/pulsematrix .
COPY --from=builder /app/static ./static
COPY --from=builder /app/templates ./templates

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget -qO- "http://127.0.0.1:${PORT:-8080}/health" || exit 1

CMD ["./pulsematrix"]
