#!/usr/bin/env python3
"""
Brave 預設非 RTMP 用 playbin3；在 Ubuntu 22.04 + GStreamer 1.20 上，
playbin3 挂上自訂 video-sink bin 後，再對該 bin gst_bin_add(intervideosink/queue) 會失敗
（日誌: Unable to add element intervideosink / queue）。

SRT 輸入改走 playbin（與 RTMP 相同策略），可與 Brave 既有 video-sink 流程相容。
"""
import re
import sys
from pathlib import Path


def main() -> None:
    p = Path("brave/inputs/uri.py")
    if not p.is_file():
        print("patch_brave_uri_playbin_for_srt: uri.py not found", file=sys.stderr)
        sys.exit(1)
    t = p.read_text(encoding="utf-8")
    if "is_srt = self.uri.startswith('srt')" in t:
        print("patch_brave_uri_playbin_for_srt: already applied")
        return
    pattern = (
        r"(is_rtmp = self\.uri\.startswith\('rtmp'\)\s*\n)"
        r"(\s*)(playbin_element = 'playbin' if is_rtmp else 'playbin3')"
    )
    repl = (
        r"\1\2is_srt = self.uri.startswith('srt')\n"
        r"\2playbin_element = 'playbin' if (is_rtmp or is_srt) else 'playbin3'"
    )
    t2, n = re.subn(pattern, repl, t, count=1)
    if n != 1:
        print("patch_brave_uri_playbin_for_srt: pattern not found; Brave upstream may have changed", file=sys.stderr)
        sys.exit(1)
    p.write_text(t2, encoding="utf-8")
    print("patch_brave_uri_playbin_for_srt: srt:// uses playbin")


if __name__ == "__main__":
    main()
