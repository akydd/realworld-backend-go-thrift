# Stage 1: Build the Go binary
FROM golang:1.26.1-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Statically compile the binary for compatibility with minimal images
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server ./cmd/server

# Stage 2: Run the application from a minimal base image
FROM scratch
WORKDIR /root/
# Copy the built binary from the builder stage
COPY --from=builder /app/server .
EXPOSE 8090
CMD ["./server"]
