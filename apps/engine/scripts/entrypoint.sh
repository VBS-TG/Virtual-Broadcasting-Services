#!/usr/bin/bash
set -euo pipefail

# 少數容器環境下 Gst 未掃到 plugins-bad 的 inter*；明確指向 multiarch 外掛目錄
if [[ -z "${GST_PLUGIN_PATH:-}" ]]; then
  for _gst in /usr/lib/x86_64-linux-gnu/gstreamer-1.0 /usr/lib/aarch64-linux-gnu/gstreamer-1.0; do
    if [[ -d "${_gst}" ]]; then
      export GST_PLUGIN_PATH="${_gst}"
      break
    fi
  done
fi

if [[ -z "${VBS_ENGINE_SRT_INPUT_1_URI:-}" || -z "${VBS_ENGINE_SRT_INPUT_2_URI:-}" ]]; then
  echo "錯誤: 請設定 VBS_ENGINE_SRT_INPUT_1_URI 與 VBS_ENGINE_SRT_INPUT_2_URI" >&2
  exit 1
fi

if [[ -z "${VBS_ENGINE_PGM_SRT_URI:-}" ]]; then
  echo "錯誤: 請設定 VBS_ENGINE_PGM_SRT_URI" >&2
  exit 1
fi

export PORT="${PORT:-${VBS_ENGINE_API_PORT:-5000}}"
export STUN_SERVER="${VBS_ENGINE_STUN_SERVER:-stun.l.google.com:19302}"
if [[ -n "${VBS_ENGINE_TURN_SERVER:-}" ]]; then
  export TURN_SERVER="${VBS_ENGINE_TURN_SERVER}"
fi

TCP_PORT="${VBS_ENGINE_PGM_TCP_PORT:-30090}"

# 可選：行動網路 / 隧道 MTU（.cursorrules：1400）
if [[ -n "${VBS_ENGINE_MTU_IFACE:-}" && -n "${VBS_ENGINE_MTU:-}" ]]; then
  if command -v ip >/dev/null 2>&1; then
    ip link set dev "${VBS_ENGINE_MTU_IFACE}" mtu "${VBS_ENGINE_MTU}" || true
  fi
fi

# NVIDIA 硬體：預設啟動前必須偵測到 GPU（符合 .cursorrules fail-fast）
if [[ "${VBS_ENGINE_REQUIRE_NVIDIA:-1}" != "0" ]]; then
  if ! command -v nvidia-smi >/dev/null 2>&1; then
    echo "錯誤: 未找到 nvidia-smi；請確認已安裝 NVIDIA 驅動並使用 --gpus all / runtime: nvidia" >&2
    exit 1
  fi
  if ! nvidia-smi -L >/dev/null 2>&1; then
    echo "錯誤: nvidia-smi 無法列出 GPU；請確認容器可存取 GPU" >&2
    exit 1
  fi
fi

# 可選：強制要求 Gst 存在 nvh265dec 元素（除錯或嚴格模式）
if [[ "${VBS_ENGINE_REQUIRE_GST_NVH265DEC:-0}" == "1" ]]; then
  if ! gst-inspect-1.0 nvh265dec >/dev/null 2>&1; then
    echo "錯誤: gst-inspect 找不到 nvh265dec（請確認映像含 NVIDIA 解碼外掛）" >&2
    exit 1
  fi
fi

python3 /opt/vbs-engine/scripts/generate_brave_config.py

cd /opt/brave

cleanup() {
  [[ -n "${BRAVE_PID:-}" ]] && kill "$BRAVE_PID" 2>/dev/null || true
  [[ -n "${FF_PID:-}" ]] && kill "$FF_PID" 2>/dev/null || true
  [[ -n "${TEL_PID:-}" ]] && kill "$TEL_PID" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# 可選：1Hz 遙測（需 VBS_CONSOLE_BASE_URL；見 engine_telemetry.py）
TEL_PID=""
if [[ -n "${VBS_CONSOLE_BASE_URL:-}" && "${VBS_ENGINE_TELEMETRY_ENABLED:-1}" != "0" ]]; then
  echo "[vbs-engine] 啟動遙測子進程…"
  python3 /opt/vbs-engine/scripts/engine_telemetry.py &
  TEL_PID=$!
fi

restart_max="${VBS_ENGINE_RESTART_BACKOFF_MAX_SEC:-30}"
restart_delay="${VBS_ENGINE_RESTART_INITIAL_SEC:-1}"

run_pipeline_once() {
  echo "[vbs-engine] 啟動 Brave…"
  python3 brave.py -c "${VBS_ENGINE_BRAVE_CONFIG_PATH:-/tmp/brave.yaml}" &
  BRAVE_PID=$!

  echo "[vbs-engine] 等待 Brave TCP PGM (${TCP_PORT})…"
  for _ in $(seq 1 120); do
    if ss -tln 2>/dev/null | grep -q ":${TCP_PORT}"; then
      break
    fi
    sleep 1
  done

  if ! ss -tln 2>/dev/null | grep -q ":${TCP_PORT}"; then
    echo "錯誤: 等待 TCP ${TCP_PORT} 逾時" >&2
    return 1
  fi

  echo "[vbs-engine] 啟動 ffmpeg → SRT PGM…"
  ffmpeg -hide_banner -loglevel info \
    -fflags +genpts \
    -i "tcp://127.0.0.1:${TCP_PORT}?timeout=0" \
    -c copy \
    -f mpegts \
    "${VBS_ENGINE_PGM_SRT_URI}" &
  FF_PID=$!

  while kill -0 "$BRAVE_PID" 2>/dev/null && kill -0 "$FF_PID" 2>/dev/null; do
    sleep 1
  done
  kill "$BRAVE_PID" 2>/dev/null || true
  kill "$FF_PID" 2>/dev/null || true
  wait "$BRAVE_PID" 2>/dev/null || true
  wait "$FF_PID" 2>/dev/null || true
  return 0
}

while true; do
  run_pipeline_once || true
  echo "[vbs-engine] 管線結束，${restart_delay}s 後重啟（指數退避上限 ${restart_max}s）…"
  sleep "$restart_delay"
  restart_delay=$(( restart_delay * 2 ))
  if [[ "$restart_delay" -gt "$restart_max" ]]; then
    restart_delay="$restart_max"
  fi
done
