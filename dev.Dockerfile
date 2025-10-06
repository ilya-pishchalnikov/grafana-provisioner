FROM golang:1.24-alpine

# Install debug tools for Go
RUN go install github.com/go-delve/delve/cmd/dlv@v1.24.0

# Install additional development tools
RUN apk add --no-cache \
    git \
    bash \
    curl \
    gcc \
    musl-dev

WORKDIR /workspace

# Copy depedencies for Cache
#COPY go.mod go.sum ./
#RUN go mod download
