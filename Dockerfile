# Build stage
FROM docker.1ms.run/golang:1.26-alpine AS builder

WORKDIR /app

# Install dependencies (add gcc and musl-dev for CGO/SQLite)
RUN apk add --no-cache git ca-certificates gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server

# Final stage
FROM docker.1ms.run/library/alpine:latest

RUN apk --no-cache add ca-certificates openssl wget

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/server .

# Copy dashboard static files
COPY --from=builder /app/dashboard ./dashboard

# Expose port
EXPOSE 8080

# Run the server
CMD ["./server", "serve"]
