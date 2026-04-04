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
    python3 python3-pip python3-dev python3-gi python3-gst-1.0 \
    gir1.2-gst-plugins-bad-1.0 libcairo2-dev libgirepository1.0-dev pkg-config \
    gobject-introspection \
    gstreamer1.0-plugins-base gstreamer1.0-plugins-good gstreamer1.0-plugins-bad \
    gstreamer1.0-plugins-ugly gstreamer1.0-libav gstreamer1.0-tools gstreamer1.0-nice \
    ffmpeg iproute2 \
    pipenv \
    && rm -rf /var/lib/apt/lists/*

RUN pip3 install --no-cache-dir pyyaml

WORKDIR /opt/brave
RUN git clone --depth 1 https://github.com/bbc/brave.git . \
    && PIPENV_VENV_IN_PROJECT=1 pipenv --python python3 install --skip-lock

COPY apps/engine/scripts/generate_brave_config.py /opt/vbs-engine/scripts/generate_brave_config.py
COPY apps/engine/scripts/entrypoint.sh /opt/vbs-engine/scripts/entrypoint.sh
RUN chmod +x /opt/vbs-engine/scripts/entrypoint.sh

WORKDIR /opt/brave

EXPOSE 5000

ENTRYPOINT ["/bin/bash", "/opt/vbs-engine/scripts/entrypoint.sh"]
