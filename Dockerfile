# Stage 1: Build the Go binary
FROM golang:1.20-alpine AS builder

# Install git for go modules (if needed)
RUN apk add --no-cache git

# Set working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies (use cache)
RUN go mod download

# Copy all backend source code
COPY . .

# Build the Go binary for Linux OS and AMD64 architecture
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o api ./cmd/api/main.go

# Stage 2: Create a minimal image to run the binary
FROM alpine:latest

# Install ca-certificates for HTTPS support
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/api .

# Expose port 8080
EXPOSE 8080

# Command to run the binary
ENTRYPOINT ["./api"]
