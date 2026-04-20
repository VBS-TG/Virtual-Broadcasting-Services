#!/usr/bin/env python3
"""Guard Brave connection element lists against None entries.

When runtime add_element fails, some helpers may return None. Upstream code
appends these to internal element arrays, then later calls methods on them and
crashes (e.g. sync_state_with_parent on None). This patch skips None safely.
"""

from pathlib import Path


def main() -> None:
    path = Path("brave/connections/connection.py")
    text = path.read_text(encoding="utf-8")

    old_add_dest = (
        "    def _add_element_to_dest_pipeline(self, factory_name, audio_or_video, name=None):\n"
        "        '''\n"
        "        Add an element on the destination pipeline\n"
        "        '''\n"
        "        e = self.dest.add_element(factory_name, self.source, audio_or_video=audio_or_video, name=name)\n"
        "        self._elements_on_dest_pipeline.append(e)\n"
        "        return e\n"
    )
    new_add_dest = (
        "    def _add_element_to_dest_pipeline(self, factory_name, audio_or_video, name=None):\n"
        "        '''\n"
        "        Add an element on the destination pipeline\n"
        "        '''\n"
        "        e = self.dest.add_element(factory_name, self.source, audio_or_video=audio_or_video, name=name)\n"
        "        if e is not None:\n"
        "            self._elements_on_dest_pipeline.append(e)\n"
        "        return e\n"
    )
    if old_add_dest not in text:
        raise SystemExit("patch_brave_none_guard: _add_element_to_dest_pipeline snippet not found")
    text = text.replace(old_add_dest, new_add_dest, 1)

    old_add_src = (
        "    def _add_element_to_src_pipeline(self, factory_name, audio_or_video, name=None):\n"
        "        '''\n"
        "        Add an element on the source pipeline\n"
        "        '''\n"
        "        e = self.source.add_element(factory_name, self.dest, audio_or_video=audio_or_video, name=name)\n"
        "        self._elements_on_src_pipeline.append(e)\n"
        "        return e\n"
    )
    new_add_src = (
        "    def _add_element_to_src_pipeline(self, factory_name, audio_or_video, name=None):\n"
        "        '''\n"
        "        Add an element on the source pipeline\n"
        "        '''\n"
        "        e = self.source.add_element(factory_name, self.dest, audio_or_video=audio_or_video, name=name)\n"
        "        if e is not None:\n"
        "            self._elements_on_src_pipeline.append(e)\n"
        "        return e\n"
    )
    if old_add_src not in text:
        raise SystemExit("patch_brave_none_guard: _add_element_to_src_pipeline snippet not found")
    text = text.replace(old_add_src, new_add_src, 1)

    old_sync = (
        "    def _sync_element_states(self):\n"
        "        '''\n"
        "        Make sure the elements created on the source and destination have their state set to match their pipeline.\n"
        "        '''\n"
        "        for e in self._elements_on_dest_pipeline:\n"
        "            if not e.sync_state_with_parent():\n"
        "                self.logger.warning('Unable to set %s to state of parent source' % e.name)\n"
        "        for e in self._elements_on_src_pipeline:\n"
        "            if not e.sync_state_with_parent():\n"
        "                self.logger.warning('Unable to set %s to state of parent source' % e.name)\n"
    )
    new_sync = (
        "    def _sync_element_states(self):\n"
        "        '''\n"
        "        Make sure the elements created on the source and destination have their state set to match their pipeline.\n"
        "        '''\n"
        "        for e in self._elements_on_dest_pipeline:\n"
        "            if e is None:\n"
        "                continue\n"
        "            if not e.sync_state_with_parent():\n"
        "                self.logger.warning('Unable to set %s to state of parent source' % e.name)\n"
        "        for e in self._elements_on_src_pipeline:\n"
        "            if e is None:\n"
        "                continue\n"
        "            if not e.sync_state_with_parent():\n"
        "                self.logger.warning('Unable to set %s to state of parent source' % e.name)\n"
    )
    if old_sync not in text:
        raise SystemExit("patch_brave_none_guard: _sync_element_states snippet not found")
    text = text.replace(old_sync, new_sync, 1)

    old_set_dest = (
        "    def _set_dest_element_state(self, state):\n"
        "        '''\n"
        "        Set the state of all elements on the dest pipeline\n"
        "        '''\n"
        "        for e in self._elements_on_dest_pipeline:\n"
        "            if e.set_state(state) != Gst.StateChangeReturn.SUCCESS:\n"
        "                self.dest.logger.warning('Unable to set element %s to %s state' % (e.name, state.value_nick.upper()))\n"
    )
    new_set_dest = (
        "    def _set_dest_element_state(self, state):\n"
        "        '''\n"
        "        Set the state of all elements on the dest pipeline\n"
        "        '''\n"
        "        for e in self._elements_on_dest_pipeline:\n"
        "            if e is None:\n"
        "                continue\n"
        "            if e.set_state(state) != Gst.StateChangeReturn.SUCCESS:\n"
        "                self.dest.logger.warning('Unable to set element %s to %s state' % (e.name, state.value_nick.upper()))\n"
    )
    if old_set_dest not in text:
        raise SystemExit("patch_brave_none_guard: _set_dest_element_state snippet not found")
    text = text.replace(old_set_dest, new_set_dest, 1)

    old_set_src = (
        "    def _set_source_element_state(self, state):\n"
        "        '''\n"
        "        Set the state of all elements on the src pipeline\n"
        "        '''\n"
        "        for e in self._elements_on_src_pipeline:\n"
        "            if e.set_state(state) != Gst.StateChangeReturn.SUCCESS:\n"
        "                self.logger.warning('Unable to set input element %s to %s state' % (e.name, state.value_nick.upper()))\n"
    )
    new_set_src = (
        "    def _set_source_element_state(self, state):\n"
        "        '''\n"
        "        Set the state of all elements on the src pipeline\n"
        "        '''\n"
        "        for e in self._elements_on_src_pipeline:\n"
        "            if e is None:\n"
        "                continue\n"
        "            if e.set_state(state) != Gst.StateChangeReturn.SUCCESS:\n"
        "                self.logger.warning('Unable to set input element %s to %s state' % (e.name, state.value_nick.upper()))\n"
    )
    if old_set_src not in text:
        raise SystemExit("patch_brave_none_guard: _set_source_element_state snippet not found")
    text = text.replace(old_set_src, new_set_src, 1)

    path.write_text(text, encoding="utf-8")
    print("[vbs-engine] patched brave/connections/connection.py (None guards)")


if __name__ == "__main__":
    main()
