#!/usr/bin/env python3
"""
Sanic 19.3.1 在 Python 3.10+ 的相容修補。不可先 import sanic（會先載入未修補模組）。

- collections.MutableSequence → collections.abc（blueprint_group.py）
- asyncio 3.10 起移除多數 API 的 loop= 參數（server.py、worker.py）
"""
import pathlib
import site
import sys

PATCHES: list[tuple[str, str, str]] = [
    (
        "blueprint_group.py",
        "from collections import MutableSequence",
        "from collections.abc import MutableSequence",
    ),
    (
        "server.py",
        "        self._not_paused = asyncio.Event(loop=loop)",
        "        self._not_paused = asyncio.Event()",
    ),
    (
        "server.py",
        "        _shutdown = asyncio.gather(*coros, loop=loop)",
        "        _shutdown = asyncio.gather(*coros)",
    ),
    (
        "worker.py",
        "        self._runner = asyncio.ensure_future(self._run(), loop=self.loop)",
        "        self._runner = asyncio.ensure_future(self._run())",
    ),
    (
        "worker.py",
        "            _shutdown = asyncio.gather(*coros, loop=self.loop)",
        "            _shutdown = asyncio.gather(*coros)",
    ),
    (
        "worker.py",
        "                    await asyncio.sleep(1.0, loop=self.loop)",
        "                    await asyncio.sleep(1.0)",
    ),
]


def main() -> None:
    for sp in site.getsitepackages():
        base = pathlib.Path(sp) / "sanic"
        if not base.is_dir():
            continue
        for rel, old, new in PATCHES:
            p = base / rel
            if not p.is_file():
                print(f"patch_sanic_py310: missing {p}", file=sys.stderr)
                sys.exit(1)
            t = p.read_text(encoding="utf-8")
            if old not in t:
                if new in t or (rel == "blueprint_group.py" and "from collections.abc import MutableSequence" in t):
                    continue
                print(f"patch_sanic_py310: pattern not found in {p}", file=sys.stderr)
                sys.exit(1)
            p.write_text(t.replace(old, new, 1), encoding="utf-8")
        print("patch_sanic_py310: ok")
        return
    print("patch_sanic_py310: sanic not found under site-packages", file=sys.stderr)
    sys.exit(1)


if __name__ == "__main__":
    main()
