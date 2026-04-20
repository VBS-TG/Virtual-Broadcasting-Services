#!/usr/bin/env python3
"""Patch Brave Input.add_element() for stricter GStreamer stacks.

Some environments fail to add runtime inter* elements into UriInput's inner bin
once playbin branches are already moving state. This patch adds a two-step
fallback:
1) Temporarily drop the input pipeline to READY and retry bin.add().
2) If still failing, try adding the element to top-level input pipeline.
"""

from pathlib import Path


def main() -> None:
    path = Path("brave/inputs/input.py")
    text = path.read_text(encoding="utf-8")

    old = (
        "    def add_element(self, factory_name, who_its_for, audio_or_video, name=None):\n"
        "        '''\n"
        "        Add an element on the pipeline belonging to this mixer.\n"
        "        Note: this method's interface matches mixer.add_element()\n"
        "        '''\n"
        "        if name is None:\n"
        "            name = factory_name\n"
        "        name = who_its_for.uid + '_' + name + '_' + str(random.randint(1, 1000000))\n"
        "        input_bin = getattr(self, 'final_' + audio_or_video + '_tee').parent\n"
        "        e = Gst.ElementFactory.make(factory_name, name)\n"
        "        if not input_bin.add(e):\n"
        "            self.logger.error('Unable to add element %s' % factory_name)\n"
        "            return None\n"
        "        return e\n"
    )
    new = (
        "    def add_element(self, factory_name, who_its_for, audio_or_video, name=None):\n"
        "        '''\n"
        "        Add an element on the pipeline belonging to this mixer.\n"
        "        Note: this method's interface matches mixer.add_element()\n"
        "        '''\n"
        "        if name is None:\n"
        "            name = factory_name\n"
        "        name = who_its_for.uid + '_' + name + '_' + str(random.randint(1, 1000000))\n"
        "        input_bin = getattr(self, 'final_' + audio_or_video + '_tee').parent\n"
        "        e = Gst.ElementFactory.make(factory_name, name)\n"
        "        if e is None:\n"
        "            self.logger.error('Unable to create element %s' % factory_name)\n"
        "            return None\n"
        "\n"
        "        if input_bin.add(e):\n"
        "            return e\n"
        "\n"
        "        self.logger.warning('Unable to add element %s to input bin; retrying in READY state'\n"
        "                            % factory_name)\n"
        "        prev_state = self.pipeline.get_state(0).state\n"
        "        self.pipeline.set_state(Gst.State.READY)\n"
        "        if input_bin.add(e):\n"
        "            self.pipeline.set_state(prev_state)\n"
        "            return e\n"
        "        self.pipeline.set_state(prev_state)\n"
        "\n"
        "        if self.pipeline.add(e):\n"
        "            self.logger.warning('Added %s to top-level pipeline as fallback' % factory_name)\n"
        "            return e\n"
        "\n"
        "        self.logger.error('Unable to add element %s' % factory_name)\n"
        "        return None\n"
    )
    if old not in text:
        raise SystemExit(f"patch_brave_input_add: expected snippet not found in {path}")

    text = text.replace(old, new, 1)
    path.write_text(text, encoding="utf-8")
    print("[vbs-engine] patched brave/inputs/input.py (add_element fallback)")


if __name__ == "__main__":
    main()
