#!/usr/bin/env python3
"""Engine telemetry: Cloudflare Access register/refresh + WSS to Console."""
from __future__ import annotations

import json
import os
import ssl
import time
import urllib.request
from dataclasses import dataclass
from typing import Tuple
from urllib.parse import urljoin, urlparse

import psutil
import websocket


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


def _http_post_json(url: str, body: dict, bearer: str, timeout_sec: int = 8, extra_headers: dict | None = None) -> dict:
    req = urllib.request.Request(url, method="POST")
    req.add_header("Content-Type", "application/json")
    if bearer:
        req.add_header("Authorization", f"Bearer {bearer}")
    if extra_headers:
        for k, v in extra_headers.items():
            if v:
                req.add_header(k, v)
    payload = json.dumps(body).encode("utf-8")
    with urllib.request.urlopen(req, payload, timeout=timeout_sec) as resp:
        raw = resp.read()
    return json.loads(raw.decode("utf-8"))


def _register_with_cf_access(base_url: str, node_id: str, client_id: str, client_secret: str) -> Tuple[str, int]:
    endpoint = urljoin(base_url.rstrip("/") + "/", "api/v1/auth/register")
    out = _http_post_json(
        endpoint,
        {
            "node_id": node_id,
            "role": "engine",
            "access_client_id": client_id,
            "access_client_secret": client_secret,
        },
        "",
        extra_headers={
            "CF-Access-Client-Id": client_id,
            "CF-Access-Client-Secret": client_secret,
            "X-VBS-Access-Client-Id": client_id,
            "X-VBS-Access-Client-Secret": client_secret,
            "X-VBS-Node-ID": node_id,
        },
    )
    token = out.get("access_token", "")
    exp = int(out.get("expires_at_unix", 0))
    if not exp and token:
        exp = _jwt_exp_unverified(token)
    if not token:
        raise RuntimeError("empty access_token from cf access register")
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


def main() -> None:
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

    cf_client_id = _env("VBS_CF_ACCESS_CLIENT_ID")
    cf_client_secret = _env("VBS_CF_ACCESS_CLIENT_SECRET")

    auth = AuthState()

    sslopt = {}
    if ws_url.startswith("wss://") and insecure_tls:
        sslopt = {"cert_reqs": ssl.CERT_NONE, "check_hostname": False}

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
                if not auth.token:
                    if not (cf_client_id and cf_client_secret):
                        raise RuntimeError("VBS_CF_ACCESS_CLIENT_ID and VBS_CF_ACCESS_CLIENT_SECRET are required")
                    auth.token, auth.exp_unix = _register_with_cf_access(base_url, node_id, cf_client_id, cf_client_secret)

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
                time.sleep(interval)
                continue

            headers = [f"Authorization: Bearer {auth.token}"]
            ws = websocket.create_connection(ws_url, timeout=8, header=headers, sslopt=sslopt)
            try:
                ws.send(raw.decode("utf-8"))
            finally:
                ws.close()
            time.sleep(interval)
        except Exception as exc:  # pylint: disable=broad-except
            print(f"[engine][telemetry] send failed: {exc}")
            time.sleep(min(5.0, interval + 1.0))


if __name__ == "__main__":
    main()
