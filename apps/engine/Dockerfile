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
    gstreamer1.0-plugins-base gstreamer1.0-plugins-good gstreamer1.0-plugins-bad \
    gstreamer1.0-plugins-ugly gstreamer1.0-libav gstreamer1.0-tools gstreamer1.0-nice \
    ffmpeg iproute2 \
    && rm -rf /var/lib/apt/lists/*

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
         'websocket-client>=1.6,<2' \
    && .venv/bin/python - <<'PY' \
import pathlib
root = pathlib.Path('/opt/brave/.venv/lib/python3.10/site-packages/sanic')
for p in root.rglob('*.py'):
    s = p.read_text(encoding='utf-8')
    ns = s.replace('from collections import MutableSequence', 'from collections.abc import MutableSequence')
    ns = ns.replace('from collections import Mapping', 'from collections.abc import Mapping')
    ns = ns.replace('from collections import MutableMapping', 'from collections.abc import MutableMapping')
    if ns != s:
        p.write_text(ns, encoding='utf-8')
print('patched sanic collection imports')
PY

COPY apps/engine/scripts/generate_brave_config.py /opt/vbs-engine/scripts/generate_brave_config.py
COPY apps/engine/scripts/engine_telemetry.py /opt/vbs-engine/scripts/engine_telemetry.py
COPY apps/engine/scripts/sitecustomize.py /opt/brave/sitecustomize.py
COPY apps/engine/scripts/entrypoint.sh /opt/vbs-engine/scripts/entrypoint.sh
RUN sed -i '1s/^\xEF\xBB\xBF//' /opt/vbs-engine/scripts/entrypoint.sh \
    && sed -i '1s/^\xEF\xBB\xBF//' /opt/vbs-engine/scripts/engine_telemetry.py \
    && sed -i '1s/^\xEF\xBB\xBF//' /opt/vbs-engine/scripts/generate_brave_config.py \
    && sed -i '1s/^\xEF\xBB\xBF//' /opt/brave/sitecustomize.py \
    && chmod +x /opt/vbs-engine/scripts/entrypoint.sh

WORKDIR /opt/brave

EXPOSE 5000

ENTRYPOINT ["bash", "/opt/vbs-engine/scripts/entrypoint.sh"]
