FROM golang:1.18 AS builder

WORKDIR /usr/src/worker

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN CGO_ENABLED=0 go build -v -o /usr/local/bin/worker ./cmd/worker

FROM scratch

COPY --from=builder /usr/local/bin/worker /usr/local/bin/worker

CMD ["/usr/local/bin/worker"]
