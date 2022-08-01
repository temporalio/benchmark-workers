FROM golang:1.18

WORKDIR /usr/src/worker

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /usr/local/bin/worker ./cmd/worker

CMD ["worker"]