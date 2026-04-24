#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${VBS_SRT_PASSPHRASE:-}" ]]; then
  echo "錯誤: 需設定 VBS_SRT_PASSPHRASE（AES-256）" >&2
  exit 1
fi

if [[ "${VBS_ENGINE_TELEMETRY_ENABLED:-1}" != "0" ]]; then
  if [[ -z "${VBS_CF_ACCESS_AUD:-}" ]]; then
    echo "錯誤: 啟用 telemetry 時需設定 VBS_CF_ACCESS_AUD" >&2
    exit 1
  fi
  if [[ -z "${VBS_CF_ACCESS_JWT:-}" && ( -z "${VBS_CF_ACCESS_CLIENT_ID:-}" || -z "${VBS_CF_ACCESS_CLIENT_SECRET:-}" ) ]]; then
    echo "錯誤: 啟用 telemetry 時需設定 VBS_CF_ACCESS_JWT，或同時設定 VBS_CF_ACCESS_CLIENT_ID 與 VBS_CF_ACCESS_CLIENT_SECRET" >&2
    exit 1
  fi
fi

if [[ "${VBS_ENGINE_CONTROL_API_ENABLED:-1}" != "0" ]]; then
  if [[ -z "${VBS_ADMIN_EMAILS:-}" || -z "${VBS_CONSOLE_JWT_PUBLIC_KEYS:-}" ]]; then
    echo "錯誤: 啟用控制面驗證時需設定 VBS_ADMIN_EMAILS 與 VBS_CONSOLE_JWT_PUBLIC_KEYS" >&2
    exit 1
  fi
fi

echo "[vbs-engine] 啟動 Eyevinn TypeScript engine core..."
exec node /opt/vbs-engine/dist/index.js

