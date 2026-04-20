# VBS-Engine：BBC Brave + GStreamer + NVIDIA runtime（linux/amd64）
# 參考：https://github.com/bbc/brave

FROM nvidia/cuda:12.2.0-runtime-ubuntu20.04

ENV DEBIAN_FRONTEND=noninteractive
ENV LC_ALL=C.UTF-8
ENV LANG=C.UTF-8
ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=all
# Brave 透過 GI 載入 Gst；須讓登錄檔掃到 gst-plugins-bad（intervideosink/interaudiosink）。
ENV GST_PLUGIN_PATH=/usr/lib/x86_64-linux-gnu/gstreamer-1.0

RUN printf "%s\n" \
    "Acquire::Retries \"5\";" \
    "Acquire::http::Timeout \"30\";" \
    "Acquire::https::Timeout \"30\";" \
    > /etc/apt/apt.conf.d/99vbs-retry \
    && sed -i \
      -e 's|http://archive.ubuntu.com/ubuntu|http://free.nchc.org.tw/ubuntu|g' \
      -e 's|http://security.ubuntu.com/ubuntu|http://free.nchc.org.tw/ubuntu|g' \
      /etc/apt/sources.list \
    && apt-get -o Acquire::ForceIPv4=true update \
    && apt-get install -y --no-install-recommends \
    ca-certificates curl git \
    build-essential \
    python3 python3-pip python3-venv python3-dev python3-gi python3-gst-1.0 \
    gir1.2-gst-plugins-bad-1.0 libcairo2-dev libgirepository1.0-dev pkg-config \
    gobject-introspection \
    libjpeg-dev zlib1g-dev libopenjp2-7-dev libtiff5-dev libfreetype6-dev \
    gstreamer1.0-plugins-base gstreamer1.0-plugins-good gstreamer1.0-plugins-bad \
    gstreamer1.0-plugins-ugly gstreamer1.0-libav gstreamer1.0-tools gstreamer1.0-nice \
    ffmpeg iproute2 \
    && gst-inspect-1.0 intervideosink >/dev/null \
    && gst-inspect-1.0 intervideosrc >/dev/null \
    && gst-inspect-1.0 x264enc >/dev/null \
    && gst-inspect-1.0 mpegtsmux >/dev/null \
    && rm -rf /var/lib/apt/lists/*

RUN pip3 install --no-cache-dir pyyaml

WORKDIR /opt/brave
COPY apps/engine/scripts/patch_brave_pipfile.py /opt/vbs-engine/scripts/patch_brave_pipfile.py
COPY apps/engine/scripts/patch_brave_session.py /opt/vbs-engine/scripts/patch_brave_session.py
# Brave Pipfile 含 vext / pip 版 pygobject 等；改以 pip 安裝並保留 system-site-packages 給 apt 的 python3-gi。
# python3-venv：否則 `python3 -m venv` 在最小映像會失敗（需與 python3 一併安裝）。
RUN git clone --depth 1 https://github.com/bbc/brave.git . \
    && python3 /opt/vbs-engine/scripts/patch_brave_pipfile.py \
    && sed -i 's/has_mux = config\.enable_video$/has_mux = config.enable_video()/' brave/outputs/tcp.py \
    && grep -q 'has_mux = config.enable_video()' brave/outputs/tcp.py \
    && python3 /opt/vbs-engine/scripts/patch_brave_session.py \
    && grep -q 'inp._consider_changing_state' brave/session.py

# 勿安裝 gbulb：其依賴會拉 PyPI pygobject 並從原始碼編譯，與 apt 的 python3-gi 衝突。
# sanic 鎖 19.3.1（與 Brave 相容）；uvloop 為 brave/api 直接 import。
# Ubuntu 20.04 / Python 3.8 線上，沿用系統相容的 pip/setuptools 版本，避免舊套件 metadata 相容性問題。
RUN python3 -m venv --system-site-packages .venv \
    && .venv/bin/pip install --no-cache-dir \
         'pyyaml>=5.1,<7' \
         'sanic==19.3.1' \
         'uvloop>=0.17,<0.21' \
         'pillow>=9.5,<12' \
         'psutil>=5.6,<6' \
         'websocket-client>=1.6,<2'

# 與執行期 Brave 相同方式驗證 inter 元件可被建立（避免僅 gst-inspect 通過但 GI 失敗）。
RUN cd /opt/brave && GST_PLUGIN_PATH=/usr/lib/x86_64-linux-gnu/gstreamer-1.0 .venv/bin/python -c "import gi; gi.require_version('Gst','1.0'); from gi.repository import Gst; Gst.init(None); m=[f for f in ('intervideosink','intervideosrc','queue') if Gst.ElementFactory.make(f,'p') is None]; m and __import__('sys').exit('Gst.ElementFactory missing: ' + str(m))"

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
