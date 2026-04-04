#!/usr/bin/env python3
"""
Sanic 19.3.1 在 Python 3.10 需將 collections.MutableSequence 改為 collections.abc。
不可先 import sanic（會先載入未修補模組而崩潰）。
"""
import pathlib
import site
import sys


def main() -> None:
    patched = False
    for sp in site.getsitepackages():
        p = pathlib.Path(sp) / "sanic" / "blueprint_group.py"
        if not p.is_file():
            continue
        t = p.read_text(encoding="utf-8")
        t2 = t.replace(
            "from collections import MutableSequence",
            "from collections.abc import MutableSequence",
        )
        if t == t2:
            print(f"patch_sanic_py310: expected line missing in {p}", file=sys.stderr)
            sys.exit(1)
        p.write_text(t2, encoding="utf-8")
        patched = True
        break
    if not patched:
        print("patch_sanic_py310: blueprint_group.py not found under site-packages", file=sys.stderr)
        sys.exit(1)
    print("patch_sanic_py310: ok")


if __name__ == "__main__":
    main()
