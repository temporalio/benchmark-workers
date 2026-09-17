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

# scratch carries no CA bundle, so Go finds no system roots and every public
# TLS handshake fails with "certificate signed by unknown authority". An API key
# is presented over TLS to a publicly-signed endpoint, so it needs these. The
# mTLS path did not, because TEMPORAL_TLS_CA populates RootCAs explicitly.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy binaries from builder
COPY --from=builder /usr/local/bin/worker /usr/local/bin/worker
COPY --from=builder /usr/local/bin/runner /usr/local/bin/runner

CMD ["/usr/local/bin/worker"]
