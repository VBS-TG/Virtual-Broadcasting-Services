FROM golang:1.22-bullseye AS builder

WORKDIR /src

# 只複製必要檔案以利快取，未來若有 go.mod/go.sum 再補上
COPY . .

ENV GOOS=linux GOARCH=amd64 CGO_ENABLED=0

RUN go build -o /out/route-server ./apps/route/cmd/route-server

# 安裝 Route pipeline 所需的 SRTLA / SRT 工具：
# - srtla_rec：接收 SRTLA（BELABOX/srtla）
# - srt-live-transmit：把 internal listener 轉成對 Engine 的 SRT listener（同 BELABOX/srtla README）
#
# MVP：以從原始碼 build 的方式確保容器內就有二進位，避免你在 EC2 上額外手動安裝。
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates git build-essential pkg-config autoconf automake libtool libssl-dev \
  && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /tmp/vbs && cd /tmp/vbs && \
  git clone --depth 1 https://github.com/BELABOX/srtla.git srtla && cd srtla && make && \
  mkdir -p /out && true && \
  cp ./srtla_rec /out/ || true

RUN cd /tmp/vbs && \
  git clone --depth 1 https://github.com/BELABOX/srt.git srt && cd srt && \
  ./configure && make && \
  # srt-live-transmit 通常會在 srt repo 內生成，盡量找出其路徑並複製到 /out
  (cp ./srt-live-transmit /out/ 2>/dev/null || true) && \
  (cp ./apps/srt-live-transmit/srt-live-transmit /out/ 2>/dev/null || true) && \
  (cp ./build/srt-live-transmit /out/ 2>/dev/null || true) && \
  test -f /out/srt-live-transmit || (echo "srt-live-transmit not found after build" && exit 1)

FROM debian:bullseye-slim

WORKDIR /app

COPY --from=builder /out/route-server /usr/local/bin/route-server
COPY --from=builder /out/srtla_rec /usr/local/bin/srtla_rec
COPY --from=builder /out/srt-live-transmit /usr/local/bin/srt-live-transmit

# srt/srtla 二進位通常依賴 OpenSSL runtime；確保 bullseye-slim 內有對應 library。
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libssl1.1 || true && rm -rf /var/lib/apt/lists/*

ENV VBS_LOG_LEVEL=info

# MVP：使用 PATH 直接呼叫二進位
ENV PATH="/usr/local/bin:${PATH}"

CMD ["route-server"]

