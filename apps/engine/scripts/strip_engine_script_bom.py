#!/usr/bin/env python3
"""移除已 COPY 進映像之腳本開頭的 UTF-8 BOM，避免 shebang 失效。"""
from pathlib import Path

TARGETS = (
    Path("/opt/vbs-engine/scripts/entrypoint.sh"),
    Path("/opt/vbs-engine/scripts/generate_brave_config.py"),
)

for p in TARGETS:
    b = p.read_bytes()
    if b.startswith(b"\xef\xbb\xbf"):
        p.write_bytes(b[3:])
