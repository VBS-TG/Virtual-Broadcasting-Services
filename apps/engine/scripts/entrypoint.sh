#!/usr/bin/env bash
set -euo pipefail

if [[ "${VBS_ENGINE_USE_TEST_SOURCES:-0}" != "1" ]]; then
  if [[ -z "${VBS_ENGINE_SRT_INPUT_1_URI:-}" || -z "${VBS_ENGINE_SRT_INPUT_2_URI:-}" ]]; then
    echo "錯誤: 請設定 VBS_ENGINE_SRT_INPUT_1_URI 與 VBS_ENGINE_SRT_INPUT_2_URI（或使用 VBS_ENGINE_USE_TEST_SOURCES=1）" >&2
    exit 1
  fi
fi

if [[ -z "${VBS_ENGINE_PGM_SRT_URI:-}" ]]; then
  echo "錯誤: 請設定 VBS_ENGINE_PGM_SRT_URI" >&2
  exit 1
fi

export PORT="${PORT:-${VBS_ENGINE_API_PORT:-5000}}"
TCP_PORT="${VBS_ENGINE_PGM_TCP_PORT:-30090}"

# GStreamer：確保 Brave 使用的 Gst 登錄檔掃到 plugins-bad（intervideosink）；映像為 amd64。
for _gstplug in /usr/lib/x86_64-linux-gnu/gstreamer-1.0 /usr/lib/aarch64-linux-gnu/gstreamer-1.0; do
  if [[ -d "${_gstplug}" ]]; then
    export GST_PLUGIN_PATH="${_gstplug}${GST_PLUGIN_PATH:+:${GST_PLUGIN_PATH}}"
    break
  fi
done

python3 /opt/vbs-engine/scripts/generate_brave_config.py

cd /opt/brave

if ! gst-inspect-1.0 intervideosink >/dev/null 2>&1; then
  echo "[vbs-engine] 錯誤: gst-inspect 找不到 intervideosink（缺 gstreamer1.0-plugins-bad 或 GST_PLUGIN_PATH）。GST_PLUGIN_PATH=${GST_PLUGIN_PATH:-}" >&2
  exit 1
fi
if ! GST_PLUGIN_PATH="${GST_PLUGIN_PATH:-}" .venv/bin/python -c "
import gi
gi.require_version('Gst','1.0')
from gi.repository import Gst
Gst.init(None)
assert Gst.ElementFactory.make('intervideosink','probe') is not None
"; then
  echo "[vbs-engine] 錯誤: Python/GI 無法建立 intervideosink（請確認映像已 docker build --no-cache）。" >&2
  exit 1
fi

cleanup() {
  [[ -n "${BRAVE_PID:-}" ]] && kill "$BRAVE_PID" 2>/dev/null || true
  [[ -n "${FF_PID:-}" ]] && kill "$FF_PID" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

echo "[vbs-engine] 啟動 Brave…"
/opt/brave/.venv/bin/python brave.py -c "${VBS_ENGINE_BRAVE_CONFIG_PATH:-/tmp/brave.yaml}" &
BRAVE_PID=$!

WAIT_SEC="${VBS_ENGINE_PGM_TCP_WAIT_SEC:-240}"
echo "[vbs-engine] 等待 Brave TCP PGM (${TCP_PORT})，最多 ${WAIT_SEC}s…"
for _ in $(seq 1 "${WAIT_SEC}"); do
  if ss -tln 2>/dev/null | grep -q ":${TCP_PORT}"; then
    echo "[vbs-engine] Brave TCP PGM 已就緒 (${TCP_PORT})"
    break
  fi
  sleep 1
done

if ! ss -tln 2>/dev/null | grep -q ":${TCP_PORT}"; then
  echo "[vbs-engine] 警告: 等待 TCP ${TCP_PORT} 逾時（${WAIT_SEC}s），略過 ffmpeg PGM（僅啟動 telemetry，如有設定）。可調高 VBS_ENGINE_PGM_TCP_WAIT_SEC 或檢查 Brave 日誌／輸入源是否就緒。" >&2
else
  echo "[vbs-engine] 啟動 ffmpeg → SRT PGM…"
  ffmpeg -hide_banner -loglevel info \
    -fflags +genpts \
    -i "tcp://127.0.0.1:${TCP_PORT}?timeout=0" \
    -c copy \
    -f mpegts \
    "${VBS_ENGINE_PGM_SRT_URI}" &
  FF_PID=$!
fi

if [[ -n "${VBS_CONSOLE_BASE_URL:-}" && "${VBS_ENGINE_TELEMETRY_ENABLED:-1}" != "0" ]]; then
  echo "[vbs-engine] 啟動 telemetry → Console…"
  /opt/brave/.venv/bin/python /opt/vbs-engine/scripts/engine_telemetry.py &
  TELEMETRY_PID=$!
fi

wait "$BRAVE_PID" ${FF_PID:+$FF_PID} "${TELEMETRY_PID:-}"

