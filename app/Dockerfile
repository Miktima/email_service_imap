FROM docker.io/library/golang:latest AS builder
WORKDIR /app
COPY go.mod go.sum main.go ./
RUN \
  go build -v -x -o server

ENTRYPOINT [ "/app/server" ]
    