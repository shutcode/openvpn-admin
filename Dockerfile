# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# modernc.org/sqlite is pure Go — CGO not required.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o server ./cmd/server

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates openssl wget

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=builder /app/dashboard ./dashboard

EXPOSE 8080

CMD ["./server", "serve"]
