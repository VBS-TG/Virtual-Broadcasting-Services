#!/usr/bin/env python3
"""Strip problematic Brave Pipfile [packages] entries for Docker/CI builds."""
import re
import sys
from pathlib import Path

SKIP = frozenset({"vext", "gobject", "pygobject", "pytest"})


def main() -> None:
    pipfile = Path("Pipfile")
    if not pipfile.is_file():
        print("patch_brave_pipfile: Pipfile not found", file=sys.stderr)
        sys.exit(1)
    lines = pipfile.read_text(encoding="utf-8").splitlines()
    out: list[str] = []
    in_packages = False
    for line in lines:
        stripped = line.strip()
        if stripped.startswith("["):
            in_packages = stripped == "[packages]"
            out.append(line)
            continue
        if in_packages:
            m = re.match(r"^\s*([A-Za-z0-9_-]+)\s*=", line)
            if m and m.group(1) in SKIP:
                continue
        out.append(line)
    pipfile.write_text("\n".join(out) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
