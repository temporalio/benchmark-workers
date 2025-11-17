# Build stage
FROM golang:1.23 AS builder

WORKDIR /usr/src/worker

# Copy dependency manifests
COPY go.mod go.sum ./

# Install dependencies
RUN --mount=type=cache,target=/go/pkg/mod go mod download && go mod verify

# Copy source code
COPY . .

# Build binaries
RUN CGO_ENABLED=0 go build -v -o /usr/local/bin/worker ./cmd/worker && \
    CGO_ENABLED=0 go build -v -o /usr/local/bin/runner ./cmd/runner

# Runtime stage
FROM scratch

# Copy binaries from builder
COPY --from=builder /usr/local/bin/worker /usr/local/bin/worker
COPY --from=builder /usr/local/bin/runner /usr/local/bin/runner

CMD ["/usr/local/bin/worker"]
