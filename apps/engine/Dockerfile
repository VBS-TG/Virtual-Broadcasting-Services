# VBS-Engine：移除 Brave，改為 Eyevinn 風格 TypeScript/Node 控制與管線編排。

FROM node:22-bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive
ENV LC_ALL=C.UTF-8
ENV LANG=C.UTF-8
ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=all

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       ca-certificates \
       curl \
       ffmpeg \
       iproute2 \
       srt-tools \
       tini \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /opt/vbs-engine

COPY apps/engine/package.json ./
RUN npm install

COPY apps/engine/tsconfig.json ./
COPY apps/engine/src ./src
RUN npm run build

COPY apps/engine/scripts/entrypoint.sh /opt/vbs-engine/entrypoint.sh
RUN chmod +x /opt/vbs-engine/entrypoint.sh

EXPOSE 5000
EXPOSE 5010

ENTRYPOINT ["/usr/bin/tini", "--", "bash", "/opt/vbs-engine/entrypoint.sh"]
