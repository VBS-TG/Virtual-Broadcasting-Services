#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${VBS_ENGINE_SRT_INPUT_1_URI:-}" || -z "${VBS_ENGINE_SRT_INPUT_2_URI:-}" ]]; then
  echo "錯誤: 需設定 VBS_ENGINE_SRT_INPUT_1_URI 與 VBS_ENGINE_SRT_INPUT_2_URI" >&2
  exit 1
fi

if [[ -z "${VBS_SRT_PASSPHRASE:-}" ]]; then
  echo "錯誤: 需設定 VBS_SRT_PASSPHRASE（AES-256）" >&2
  exit 1
fi

if [[ "${VBS_ENGINE_TELEMETRY_ENABLED:-1}" != "0" ]]; then
  if [[ -z "${VBS_CF_ACCESS_JWT:-}" || -z "${VBS_CF_ACCESS_AUD:-}" ]]; then
    echo "錯誤: 啟用 telemetry 時需設定 VBS_CF_ACCESS_JWT 與 VBS_CF_ACCESS_AUD" >&2
    exit 1
  fi
fi

echo "[vbs-engine] 啟動 Eyevinn TypeScript engine core..."
exec node /opt/vbs-engine/dist/index.js

