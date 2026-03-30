FROM golang:1.22-bullseye AS builder

WORKDIR /src

# 只複製必要檔案以利快取，未來若有 go.mod/go.sum 再補上
COPY . .

ENV GOOS=linux GOARCH=amd64 CGO_ENABLED=0

RUN go build -o /out/route-server ./apps/route/cmd/route-server

FROM debian:bullseye-slim

WORKDIR /app

COPY --from=builder /out/route-server /usr/local/bin/route-server

# 預留 SRT/SRTLA 相關工具安裝步驟，實際部署時可依環境擴充。

ENV VBS_LOG_LEVEL=info

CMD ["route-server"]

