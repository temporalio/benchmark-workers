FROM golang:1.23 AS builder

WORKDIR /usr/src/worker

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN CGO_ENABLED=0 go build -v -o /usr/local/bin/worker ./cmd/worker
RUN CGO_ENABLED=0 go build -v -o /usr/local/bin/runner ./cmd/runner

FROM scratch

COPY --from=builder /usr/local/bin/worker /usr/local/bin/worker
COPY --from=builder /usr/local/bin/runner /usr/local/bin/runner

CMD ["/usr/local/bin/worker"]
