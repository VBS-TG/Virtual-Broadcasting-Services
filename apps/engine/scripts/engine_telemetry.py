#!/usr/bin/env python3
"""Engine telemetry sender with Console JWT bootstrap/refresh."""
from __future__ import annotations

import asyncio
import json
import os
import ssl
import time
import urllib.request
from dataclasses import dataclass
from typing import Optional, Tuple
from urllib.parse import urljoin, urlparse

import psutil
import websockets


@dataclass
class AuthState:
    token: str = ""
    exp_unix: int = 0


def _env(name: str, default: str = "") -> str:
    return os.environ.get(name, default).strip()


def _jwt_exp_unverified(raw: str) -> int:
    try:
        parts = raw.split(".")
        if len(parts) != 3:
            return 0
        payload = parts[1] + "=" * ((4 - len(parts[1]) % 4) % 4)
        data = json.loads(__import__("base64").urlsafe_b64decode(payload.encode("utf-8")).decode("utf-8"))
        return int(data.get("exp", 0))
    except Exception:
        return 0


def _ws_url(base_url: str, path: str) -> str:
    u = urlparse(base_url)
    if u.scheme == "https":
        scheme = "wss"
    elif u.scheme == "http":
        scheme = "ws"
    else:
        raise ValueError("VBS_CONSOLE_BASE_URL must start with http or https")
    if not path.startswith("/"):
        path = "/" + path
    return f"{scheme}://{u.netloc}{path}"


def _http_post_json(url: str, body: dict, bearer: str, timeout_sec: int = 8) -> dict:
    req = urllib.request.Request(url, method="POST")
    req.add_header("Content-Type", "application/json")
    if bearer:
        req.add_header("Authorization", f"Bearer {bearer}")
    payload = json.dumps(body).encode("utf-8")
    with urllib.request.urlopen(req, payload, timeout=timeout_sec) as resp:
        raw = resp.read()
    return json.loads(raw.decode("utf-8"))


def _issue_token_with_bootstrap(base_url: str, bootstrap: str, node_id: str) -> Tuple[str, int]:
    endpoint = urljoin(base_url.rstrip("/") + "/", "api/v1/auth/token")
    out = _http_post_json(endpoint, {"node_id": node_id, "role": "engine"}, bootstrap)
    token = out.get("access_token", "")
    exp = int(out.get("expires_at_unix", 0))
    if not exp and token:
        exp = _jwt_exp_unverified(token)
    if not token:
        raise RuntimeError("empty access_token from console")
    return token, exp


def _register_device(base_url: str, node_id: str, device_secret: str) -> Tuple[str, int]:
    endpoint = urljoin(base_url.rstrip("/") + "/", "api/v1/auth/register")
    out = _http_post_json(endpoint, {"node_id": node_id, "role": "engine", "device_secret": device_secret}, "")
    token = out.get("access_token", "")
    exp = int(out.get("expires_at_unix", 0))
    if not exp and token:
        exp = _jwt_exp_unverified(token)
    if not token:
        raise RuntimeError("empty access_token from register")
    return token, exp


def _refresh_token(base_url: str, token: str) -> Tuple[str, int]:
    endpoint = urljoin(base_url.rstrip("/") + "/", "api/v1/auth/refresh")
    out = _http_post_json(endpoint, {}, token)
    new_token = out.get("access_token", "")
    exp = int(out.get("expires_at_unix", 0))
    if not exp and new_token:
        exp = _jwt_exp_unverified(new_token)
    if not new_token:
        raise RuntimeError("empty access_token from refresh")
    return new_token, exp


async def main() -> None:
    base_url = _env("VBS_CONSOLE_BASE_URL")
    if not base_url:
        print("[engine][telemetry] disabled: VBS_CONSOLE_BASE_URL not set")
        return
    if _env("VBS_ENGINE_TELEMETRY_ENABLED", "1") in {"0", "false", "False"}:
        print("[engine][telemetry] disabled by VBS_ENGINE_TELEMETRY_ENABLED")
        return

    node_id = _env("VBS_NODE_ID", "vbs-engine")
    ws_path = _env("VBS_ENGINE_TELEMETRY_WS_PATH", "/vbs/telemetry/ws")
    ws_url = _ws_url(base_url, ws_path)
    interval = max(0.2, float(_env("VBS_METRICS_INTERVAL_SEC", "1")))
    telemetry_max = int(_env("VBS_ENGINE_TELEMETRY_MAX_BYTES", "255"))
    insecure_tls = _env("VBS_ENGINE_TELEMETRY_TLS_INSECURE_SKIP_VERIFY", "0") in {"1", "true", "True"}

    auth = AuthState(token=_env("VBS_ENGINE_JWT"))
    if auth.token:
        auth.exp_unix = _jwt_exp_unverified(auth.token)
    bootstrap = _env("VBS_ENGINE_BOOTSTRAP_TOKEN")
    device_id = _env("VBS_ENGINE_DEVICE_ID", node_id)
    device_secret = _env("VBS_ENGINE_DEVICE_SECRET")

    ssl_ctx: Optional[ssl.SSLContext] = None
    if ws_url.startswith("wss://"):
        ssl_ctx = ssl.create_default_context()
        if insecure_tls:
            ssl_ctx.check_hostname = False
            ssl_ctx.verify_mode = ssl.CERT_NONE

    print(f"[engine][telemetry] start ws={ws_url} node_id={node_id}")
    while True:
        try:
            now = int(time.time())
            if (not auth.token) or (auth.exp_unix and auth.exp_unix - now <= 300):
                if auth.token:
                    try:
                        auth.token, auth.exp_unix = _refresh_token(base_url, auth.token)
                    except Exception:
                        auth.token = ""
                if not auth.token and device_secret:
                    auth.token, auth.exp_unix = _register_device(base_url, device_id, device_secret)
                if not auth.token and bootstrap:
                    auth.token, auth.exp_unix = _issue_token_with_bootstrap(base_url, bootstrap, node_id)
                if not auth.token:
                    raise RuntimeError(
                        "token missing/expiring and no auth source set (need VBS_ENGINE_DEVICE_SECRET or VBS_ENGINE_BOOTSTRAP_TOKEN)"
                    )

            metrics = {
                "cpu_pct": round(psutil.cpu_percent(interval=None), 2),
                "mem_bytes": int(psutil.virtual_memory().used),
            }
            payload = {
                "node_id": node_id,
                "node_type": "engine",
                "ts_ms": int(time.time() * 1000),
                "metrics": metrics,
                "auth_mode": "bearer",
            }
            raw = json.dumps(payload, separators=(",", ":")).encode("utf-8")
            if len(raw) > telemetry_max:
                payload["metrics"] = {"cpu_pct": metrics["cpu_pct"]}
                raw = json.dumps(payload, separators=(",", ":")).encode("utf-8")
            if len(raw) > telemetry_max:
                print(f"[engine][telemetry] skip oversize payload bytes={len(raw)}")
                await asyncio.sleep(interval)
                continue

            async with websockets.connect(
                ws_url,
                ssl=ssl_ctx,
                additional_headers={"Authorization": f"Bearer {auth.token}"},
                open_timeout=8,
                close_timeout=5,
                max_size=2 * 1024,
            ) as ws:
                await ws.send(raw.decode("utf-8"))
            await asyncio.sleep(interval)
        except Exception as exc:  # pylint: disable=broad-except
            print(f"[engine][telemetry] send failed: {exc}")
            await asyncio.sleep(min(5.0, interval + 1.0))


if __name__ == "__main__":
    asyncio.run(main())

