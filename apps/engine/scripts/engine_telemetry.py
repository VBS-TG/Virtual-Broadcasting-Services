#!/usr/bin/env python3
"""
VBS-Engine 1Hz 遙測（可選）：符合 packages/shared/schemas/telemetry.v1.schema.json。
當設定 VBS_CONSOLE_BASE_URL 時預設啟用；可用 VBS_ENGINE_TELEMETRY_ENABLED=0 關閉。

認證 Phase 0：優先 Authorization Bearer（VBS_ENGINE_JWT），否則 X-VBS-Key（VBS_API_KEY）。
"""
from __future__ import annotations

import asyncio
import json
import os
import socket
import subprocess
import sys
import time
from typing import Any
from urllib.parse import urlparse

import psutil

try:
    import websockets
except ImportError:
    websockets = None  # pragma: no cover


def _env_bool(name: str, default: bool) -> bool:
    v = os.environ.get(name)
    if v is None:
        return default
    return v.strip().lower() in ("1", "true", "yes", "on")


def _interval_sec() -> float:
    raw = os.environ.get("VBS_METRICS_INTERVAL", "1000ms").strip().lower()
    try:
        if raw.endswith("ms"):
            return max(0.5, float(raw[:-2]) / 1000.0)
        return max(0.5, float(raw))
    except ValueError:
        return 1.0


def _nvidia_metrics() -> tuple[float, int]:
    try:
        out = subprocess.check_output(
            [
                "nvidia-smi",
                "--query-gpu=utilization.gpu,memory.used",
                "--format=csv,noheader,nounits",
            ],
            stderr=subprocess.DEVNULL,
            text=True,
            timeout=5,
        )
        line = out.strip().splitlines()[0]
        parts = [p.strip() for p in line.split(",")]
        if len(parts) >= 2:
            gpu_pct = float(parts[0])
            vram_mib = int(float(parts[1]))
            return gpu_pct, vram_mib * 1024 * 1024
    except (subprocess.CalledProcessError, FileNotFoundError, ValueError, IndexError):
        pass
    return 0.0, 0


def _stream_ok() -> bool:
    port = int(os.environ.get("PORT", os.environ.get("VBS_ENGINE_API_PORT", "5000")))
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.settimeout(0.2)
    try:
        s.connect(("127.0.0.1", port))
        s.close()
        return True
    except OSError:
        return False


def _build_payload() -> dict[str, Any]:
    node_id = os.environ.get("VBS_NODE_ID", "vbs-engine")
    ts_ms = int(time.time() * 1000)
    cpu_pct = float(psutil.cpu_percent(interval=None))
    mem_bytes = int(psutil.virtual_memory().used)
    gpu_pct, vram_bytes = _nvidia_metrics()
    jwt = os.environ.get("VBS_ENGINE_JWT", "").strip()
    auth_mode = "bearer" if jwt else "legacy_key"

    return {
        "node_id": node_id,
        "node_type": "engine",
        "ts_ms": ts_ms,
        "metrics": {
            "cpu_pct": round(cpu_pct, 1),
            "mem_bytes": mem_bytes,
            "ingest_mbps": 0.0,
            "gpu_pct": round(gpu_pct, 1),
            "vram_bytes": vram_bytes,
            "stream_ok": _stream_ok(),
        },
        "auth_mode": auth_mode,
    }


def _ws_url() -> str:
    base = os.environ["VBS_CONSOLE_BASE_URL"].strip().rstrip("/")
    path = os.environ.get("VBS_ENGINE_TELEMETRY_WS_PATH", "/vbs/telemetry/ws").strip()
    if not path.startswith("/"):
        path = "/" + path
    u = urlparse(base)
    scheme = "wss" if u.scheme == "https" else "ws"
    host = u.netloc or u.path
    return f"{scheme}://{host}{path}"


def _header_list() -> list[tuple[str, str]]:
    out: list[tuple[str, str]] = []
    jwt = os.environ.get("VBS_ENGINE_JWT", "").strip()
    if jwt:
        out.append(("Authorization", f"Bearer {jwt}"))
    else:
        key = os.environ.get("VBS_API_KEY", "").strip()
        if key:
            out.append(("X-VBS-Key", key))
    return out


async def _run_loop() -> None:
    if websockets is None:
        print("[vbs-engine][telemetry] 缺少 websockets，已停用", file=sys.stderr)
        return

    url = _ws_url()
    hdrs = _header_list()
    interval = _interval_sec()

    while True:
        try:
            async with websockets.connect(url, extra_headers=hdrs, ping_interval=20) as ws:
                while True:
                    payload = _build_payload()
                    raw = json.dumps(payload, separators=(",", ":"), ensure_ascii=False)
                    if len(raw.encode("utf-8")) > 255:
                        print(
                            f"[vbs-engine][telemetry] 警告: payload {len(raw)} bytes > 255",
                            file=sys.stderr,
                        )
                    await ws.send(raw)
                    await asyncio.sleep(interval)
        except Exception as e:
            print(f"[vbs-engine][telemetry] 連線失敗: {e}，5s 後重試", file=sys.stderr)
            await asyncio.sleep(5.0)


def main() -> None:
    if not os.environ.get("VBS_CONSOLE_BASE_URL", "").strip():
        sys.exit(0)
    if not _env_bool("VBS_ENGINE_TELEMETRY_ENABLED", True):
        sys.exit(0)
    if websockets is None:
        sys.exit(0)

    asyncio.run(_run_loop())


if __name__ == "__main__":
    main()
