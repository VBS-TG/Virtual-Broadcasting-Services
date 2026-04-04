# VBS-Engine：BBC Brave + GStreamer + NVIDIA runtime（linux/amd64）
# 參考：https://github.com/bbc/brave
#
# Brave 以「系統 python3」執行（與 python3-gi / gst-inspect 同一套 GStreamer），
# 勿用 venv 跑 Brave：否則少數環境下 Gst 外掛掃描與 gst-inspect 不一致，intervideosink 會變 None。

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

# 系統 Python 依賴（與 python3-gi 同行程載入 Gst）
RUN pip3 install --no-cache-dir --upgrade pip setuptools wheel \
    && pip3 install --no-cache-dir \
         'pyyaml>=5.1,<7' \
         'sanic==19.3.1' \
         'uvloop>=0.21,<0.23' \
         'pillow>=9.5,<12' \
         'psutil>=5.6,<6' \
    && python3 - <<'PY'
# 不可先 import sanic（會先載入未修補的 blueprint_group，在 Py3.10 直接崩潰）
import pathlib
import site

patched = False
for sp in site.getsitepackages():
    p = pathlib.Path(sp) / "sanic" / "blueprint_group.py"
    if not p.is_file():
        continue
    t = p.read_text(encoding="utf-8")
    t2 = t.replace(
        "from collections import MutableSequence",
        "from collections.abc import MutableSequence",
    )
    if t == t2:
        raise SystemExit(f"sanic patch: expected line not in {p}")
    p.write_text(t2, encoding="utf-8")
    patched = True
    break
if not patched:
    raise SystemExit("sanic patch: blueprint_group.py not found under site-packages")
PY
    && python3 -c "import sanic; import sanic.app"

WORKDIR /opt/brave
COPY apps/engine/scripts/patch_brave_pipfile.py /opt/vbs-engine/scripts/patch_brave_pipfile.py
COPY apps/engine/scripts/patch_brave_uri_playbin_for_srt.py /opt/vbs-engine/scripts/patch_brave_uri_playbin_for_srt.py
RUN git clone --depth 1 https://github.com/bbc/brave.git . \
    && python3 /opt/vbs-engine/scripts/patch_brave_pipfile.py \
    && python3 /opt/vbs-engine/scripts/patch_brave_uri_playbin_for_srt.py

# 與執行期相同：系統 python3 + Gst 能建立 intervideosink / queue
RUN for d in /usr/lib/x86_64-linux-gnu/gstreamer-1.0 /usr/lib/aarch64-linux-gnu/gstreamer-1.0; do \
      if [ -d "$d" ]; then export GST_PLUGIN_PATH="$d"; break; fi; \
    done \
    && python3 -c "from gi.repository import Gst; Gst.init(None); \
e=Gst.ElementFactory.make('intervideosink','_v'); assert e, 'intervideosink'; \
e=Gst.ElementFactory.make('queue','_q'); assert e, 'queue'" \
    && python3 -c "import sanic; import sanic.app"

COPY apps/engine/scripts/generate_brave_config.py /opt/vbs-engine/scripts/generate_brave_config.py
COPY apps/engine/scripts/entrypoint.sh /opt/vbs-engine/scripts/entrypoint.sh
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
