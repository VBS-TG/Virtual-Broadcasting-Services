#!/usr/bin/env python3
"""Defer input state changes until after mixer.setup_sources().

Brave's default order calls input.setup() (PLAYING/PAUSED) before
mixer.setup_sources(), which attaches intervideosink into the URI input's
internal bin. On common GStreamer stacks Gst.Bin.add() then fails while the
playbin branch is already prerolling, producing 'Unable to add element
intervideosink' and NoneType at intersink.set_property.

Upstream reference: brave/session.py _setup_initial_inputs_outputs_mixers_and_overlays
"""
from pathlib import Path


def main() -> None:
    path = Path("brave/session.py")
    text = path.read_text(encoding="utf-8")
    old_inputs = (
        "        for input_config in config.inputs():\n"
        "            input = self.inputs.add(**input_config)\n"
        "            input.setup()\n"
    )
    new_inputs = (
        "        for input_config in config.inputs():\n"
        "            input = self.inputs.add(**input_config)\n"
        "            input.create_elements()\n"
        "            input.handle_updated_props()\n"
        "            input.setup_complete = True\n"
    )
    if old_inputs not in text:
        raise SystemExit(f"patch_brave_session: expected input loop not found in {path}")
    text = text.replace(old_inputs, new_inputs, 1)

    anchor = (
        "        for id, mixer in self.mixers.items():\n"
        "            mixer.setup_sources()\n\n"
        "        if config.enable_video():\n"
    )
    replacement = (
        "        for id, mixer in self.mixers.items():\n"
        "            mixer.setup_sources()\n\n"
        "        for id, inp in self.inputs.items():\n"
        "            inp._consider_changing_state()\n\n"
        "        if config.enable_video():\n"
    )
    if anchor not in text:
        raise SystemExit("patch_brave_session: anchor (mixer loop + overlays) not found")
    text = text.replace(anchor, replacement, 1)

    path.write_text(text, encoding="utf-8")
    print("[vbs-engine] patched brave/session.py (defer input playback after mixer.setup_sources)")


if __name__ == "__main__":
    main()
