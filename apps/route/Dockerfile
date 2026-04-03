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
# 從原始碼建置 srtla / SRT 工具，避免執行環境缺少二進位。
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates git build-essential pkg-config cmake tclsh autoconf automake libtool libssl-dev \
  && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /tmp/vbs && cd /tmp/vbs && \
  git clone --depth 1 https://github.com/BELABOX/srtla.git srtla && cd srtla && make && \
  mkdir -p /out && true && \
  cp ./srtla_rec /out/ || true

RUN cd /tmp/vbs && \
  git clone --depth 1 https://github.com/BELABOX/srt.git srt && cd srt && \
  cmake -S . -B build -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=/usr/local && \
  cmake --build build -j"$(nproc)" --target srt-live-transmit && \
  cmake --build build -j"$(nproc)" --target install && \
  test -f /usr/local/bin/srt-live-transmit || (echo "srt-live-transmit not found after cmake install" && exit 1)

FROM debian:bullseye-slim

WORKDIR /app

COPY --from=builder /out/route-server /usr/local/bin/route-server
COPY --from=builder /out/srtla_rec /usr/local/bin/srtla_rec
COPY --from=builder /usr/local/bin/srt-live-transmit /usr/local/bin/srt-live-transmit

# 部分 SRT 相關動態庫需要從 builder 補上。
COPY --from=builder /usr/local/lib /usr/local/lib

# srt/srtla 二進位通常依賴 OpenSSL runtime；確保 bullseye-slim 內有對應 library。
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libssl1.1 iproute2 || true && rm -rf /var/lib/apt/lists/*

ENV VBS_LOG_LEVEL=info

# 以 PATH 直接呼叫 srtla_rec、srt-live-transmit
ENV PATH="/usr/local/bin:${PATH}"

CMD ["route-server"]

