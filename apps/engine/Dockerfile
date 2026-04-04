# VBS-Engine：BBC Brave + GStreamer + NVIDIA runtime（linux/amd64）
# 參考：https://github.com/bbc/brave

FROM nvidia/cuda:12.2.0-runtime-ubuntu22.04

ENV DEBIAN_FRONTEND=noninteractive
ENV LC_ALL=C.UTF-8
ENV LANG=C.UTF-8
ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=all

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl git \
    build-essential \
    python3 python3-pip python3-venv python3-dev python3-gi python3-gst-1.0 \
    gir1.2-gst-plugins-bad-1.0 libcairo2-dev libgirepository1.0-dev pkg-config \
    gobject-introspection \
    libjpeg-dev zlib1g-dev libopenjp2-7-dev libtiff5-dev libfreetype6-dev \
    libgstreamer1.0-0 libgstreamer-plugins-base1.0-0 libgstreamer-plugins-bad1.0-0 \
    liborc-0.4-0 \
    gstreamer1.0-plugins-base gstreamer1.0-plugins-good gstreamer1.0-plugins-bad \
    gstreamer1.0-plugins-ugly gstreamer1.0-libav gstreamer1.0-tools gstreamer1.0-nice \
    ffmpeg iproute2 \
    && rm -rf /var/lib/apt/lists/* \
    && gst-inspect-1.0 intervideosink >/dev/null \
    && gst-inspect-1.0 interaudiosink >/dev/null \
    && gst-inspect-1.0 queue >/dev/null

RUN pip3 install --no-cache-dir pyyaml

WORKDIR /opt/brave
COPY apps/engine/scripts/patch_brave_pipfile.py /opt/vbs-engine/scripts/patch_brave_pipfile.py
# Brave Pipfile 含 vext / pip 版 pygobject 等；改以 pip 安裝並保留 system-site-packages 給 apt 的 python3-gi。
# python3-venv：否則 `python3 -m venv` 在最小映像會失敗（需與 python3 一併安裝）。
RUN git clone --depth 1 https://github.com/bbc/brave.git . \
    && python3 /opt/vbs-engine/scripts/patch_brave_pipfile.py

# 勿安裝 gbulb：其依賴會拉 PyPI pygobject 並從原始碼編譯，與 apt 的 python3-gi 衝突。
# sanic 鎖 19.3.1（與 Brave 相容）；uvloop 為 brave/api 直接 import（需含 py3.10 manylinux wheel，勿用 0.14）。
RUN python3 -m venv --system-site-packages .venv \
    && .venv/bin/pip install --no-cache-dir --upgrade pip setuptools wheel \
    && .venv/bin/pip install --no-cache-dir \
         'pyyaml>=5.1,<7' \
         'sanic==19.3.1' \
         'uvloop>=0.21,<0.23' \
         'pillow>=9.5,<12' \
         'psutil>=5.6,<6' \
    && sed -i 's/from collections import MutableSequence/from collections.abc import MutableSequence/' \
         .venv/lib/python3.10/site-packages/sanic/blueprint_group.py \
    && .venv/bin/python -c "import sanic; import sanic.app"

COPY apps/engine/scripts/generate_brave_config.py /opt/vbs-engine/scripts/generate_brave_config.py
COPY apps/engine/scripts/entrypoint.sh /opt/vbs-engine/scripts/entrypoint.sh
# Windows 編輯器可能寫入 UTF-8 BOM，會讓 shebang 變成 ﻿#!/... 而無法執行
RUN python3 - <<'PY'
from pathlib import Path
for p in (
    Path("/opt/vbs-engine/scripts/entrypoint.sh"),
    Path("/opt/vbs-engine/scripts/generate_brave_config.py"),
):
    b = p.read_bytes()
    if b.startswith(b"\xef\xbb\xbf"):
        p.write_bytes(b[3:])
PY
RUN chmod +x /opt/vbs-engine/scripts/entrypoint.sh

WORKDIR /opt/brave

EXPOSE 5000

ENTRYPOINT ["/bin/bash", "/opt/vbs-engine/scripts/entrypoint.sh"]
