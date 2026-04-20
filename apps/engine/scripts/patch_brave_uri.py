#!/usr/bin/env python3
"""Patch Brave URI input to avoid playbin3 path for SRT streams.

On some GStreamer stacks, playbin3 + dynamic inter* element wiring can fail
with "Unable to add element intervideosink" during startup. For engine ingest
we prefer stability over playbin3 behavior and force classic playbin.
"""

from pathlib import Path


def main() -> None:
    path = Path("brave/inputs/uri.py")
    text = path.read_text(encoding="utf-8")

    old = (
        "        is_rtmp = self.uri.startswith('rtmp')\n"
        "        playbin_element = 'playbin' if is_rtmp else 'playbin3'\n"
    )
    new = (
        "        is_rtmp = self.uri.startswith('rtmp')\n"
        "        is_srt = self.uri.startswith('srt://')\n"
        "        # Prefer classic playbin for SRT/URI ingest stability with\n"
        "        # Brave's runtime mixer inter* rewiring.\n"
        "        playbin_element = 'playbin' if (is_rtmp or is_srt) else 'playbin3'\n"
    )
    if old not in text:
        raise SystemExit(f"patch_brave_uri: expected snippet not found in {path}")
    text = text.replace(old, new, 1)

    path.write_text(text, encoding="utf-8")
    print("[vbs-engine] patched brave/inputs/uri.py (force playbin for srt://)")


if __name__ == "__main__":
    main()
