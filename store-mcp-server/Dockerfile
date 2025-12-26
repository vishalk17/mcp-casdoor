# ---- Build stage ----
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go files
COPY go.mod ./
RUN go mod download

# Copy source
COPY main.go ./

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o mcp-server

# ---- Runtime stage ----
FROM alpine:3.20

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/mcp-server /app/mcp-server

# MCP servers usually use stdio
ENTRYPOINT ["/app/mcp-server"]
