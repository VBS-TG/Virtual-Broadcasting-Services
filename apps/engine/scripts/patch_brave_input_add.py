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
        "        base_name = who_its_for.uid + '_' + name\n"
        "        input_bin = getattr(self, 'final_' + audio_or_video + '_tee').parent\n"
        "\n"
        "        def _new_element(tag=''):\n"
        "            candidate = base_name + '_' + tag + '_' + str(random.randint(1, 1000000))\n"
        "            while input_bin.get_by_name(candidate) or self.pipeline.get_by_name(candidate):\n"
        "                candidate = base_name + '_' + tag + '_' + str(random.randint(1, 1000000))\n"
        "            e = Gst.ElementFactory.make(factory_name, candidate)\n"
        "            if e is None:\n"
        "                self.logger.error('Unable to create element %s' % factory_name)\n"
        "            return e\n"
        "\n"
        "        def _cleanup_element(element):\n"
        "            if element is None:\n"
        "                return\n"
        "            try:\n"
        "                parent = element.get_parent()\n"
        "                if parent:\n"
        "                    element.set_state(Gst.State.NULL)\n"
        "                    parent.remove(element)\n"
        "            except Exception:\n"
        "                pass\n"
        "\n"
        "        def _safe_add(target_bin, element):\n"
        "            try:\n"
        "                return bool(target_bin.add(element))\n"
        "            except Exception as add_exc:\n"
        "                self.logger.warning('add(%s) failed on %s: %s' %\n"
        "                                    (factory_name, target_bin.get_name(), add_exc))\n"
        "                _cleanup_element(element)\n"
        "                return False\n"
        "\n"
        "        e = _new_element('first')\n"
        "        if e and _safe_add(input_bin, e):\n"
        "            return e\n"
        "\n"
        "        self.logger.warning('Unable to add element %s to input bin; retrying in READY state'\n"
        "                            % factory_name)\n"
        "        prev_state = self.pipeline.get_state(0).state\n"
        "        self.pipeline.set_state(Gst.State.READY)\n"
        "        e_retry = _new_element('retry1')\n"
        "        if e_retry and _safe_add(input_bin, e_retry):\n"
        "            self.pipeline.set_state(prev_state)\n"
        "            return e_retry\n"
        "        e_retry2 = _new_element('retry2')\n"
        "        if e_retry2 and _safe_add(input_bin, e_retry2):\n"
        "            self.pipeline.set_state(prev_state)\n"
        "            return e_retry2\n"
        "        self.pipeline.set_state(prev_state)\n"
        "\n"
        "        e_pipe = _new_element('pipe')\n"
        "        if e_pipe and _safe_add(self.pipeline, e_pipe):\n"
        "            self.logger.warning('Added %s to top-level pipeline as fallback' % factory_name)\n"
        "            return e_pipe\n"
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
