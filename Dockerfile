# Build stage
FROM golang:1.25 AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum* ./

COPY vendor vendor

COPY main.go main.go

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-w -s" -o kubelet-volume-stats-exporter .

FROM gcr.io/distroless/base:latest

# Copy the binary from builder
COPY --from=builder /app/kubelet-volume-stats-exporter .


ENTRYPOINT ["/kubelet-volume-stats-exporter"]

