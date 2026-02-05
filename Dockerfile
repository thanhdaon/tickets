# Build stage
FROM golang:1.25.7-alpine3.22 AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /tickets ./cmd/server

# Runtime stage
FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /tickets /app/tickets

EXPOSE 8080

ENTRYPOINT ["/app/tickets"]
