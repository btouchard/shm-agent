FROM golang:1.25 AS build-stage

WORKDIR /app

COPY . /app/

RUN go build -ldflags="-s -w" -o shm-agent ./cmd/shm-agent

FROM golang:1.25 AS production-stage

COPY --from=build-stage /app/shm-agent /usr/local/go/bin/shm-agent

WORKDIR /app
