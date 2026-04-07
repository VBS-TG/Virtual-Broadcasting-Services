FROM golang:1.22-bullseye AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV GOOS=linux GOARCH=amd64 CGO_ENABLED=0

RUN go build -trimpath -ldflags="-s -w" -o /out/console-server ./apps/console/cmd/console-server

FROM debian:bullseye-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/console-server /usr/local/bin/console-server

ENV VBS_CONSOLE_HTTP_BIND=:4000

EXPOSE 4000

CMD ["console-server"]
