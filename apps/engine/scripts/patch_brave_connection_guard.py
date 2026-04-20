#!/usr/bin/env python3
"""Guard Brave connection wiring when inter* elements cannot be created.

If Gst cannot attach intervideosink/interaudiosink at runtime, Brave currently
raises NoneType errors during channel setup. This patch makes connection setup
fail-safe: skip the broken source path instead of crashing the whole process.
"""

from pathlib import Path


def patch_connection_py() -> None:
    path = Path("brave/connections/connection.py")
    text = path.read_text(encoding="utf-8")

    old = (
        "    def _create_inter_elements(self, audio_or_video):\n"
        "        '''\n"
        "        Creates intervideosrc and intervideosink (or the equivalent audio ones)\n"
        "        '''\n"
        "        intersrc = self._create_intersrc(audio_or_video)\n"
        "        intersink = self._create_intersink(audio_or_video)\n"
        "        self._block_intersrc(audio_or_video)\n"
        "\n"
        "        # Give the 'inter' elements a channel name. It doesn't matter what, so long as they're unique.\n"
        "        channel_name = create_intersink_channel_name()\n"
        "        intersink.set_property('channel', channel_name)\n"
        "        intersrc.set_property('channel', channel_name)\n"
        "        return intersrc, intersink\n"
    )
    new = (
        "    def _create_inter_elements(self, audio_or_video):\n"
        "        '''\n"
        "        Creates intervideosrc and intervideosink (or the equivalent audio ones)\n"
        "        '''\n"
        "        intersrc = self._create_intersrc(audio_or_video)\n"
        "        intersink = self._create_intersink(audio_or_video)\n"
        "        if not intersrc or not intersink:\n"
        "            self.logger.error('Failed to create inter elements for %s' % audio_or_video)\n"
        "            return None, None\n"
        "        self._block_intersrc(audio_or_video)\n"
        "\n"
        "        # Give the 'inter' elements a channel name. It doesn't matter what, so long as they're unique.\n"
        "        channel_name = create_intersink_channel_name()\n"
        "        intersink.set_property('channel', channel_name)\n"
        "        intersrc.set_property('channel', channel_name)\n"
        "        return intersrc, intersink\n"
    )
    if old not in text:
        raise SystemExit("patch_brave_connection_guard: _create_inter_elements snippet not found")
    path.write_text(text.replace(old, new, 1), encoding="utf-8")


def patch_connection_to_mixer_py() -> None:
    path = Path("brave/connections/connection_to_mixer.py")
    text = path.read_text(encoding="utf-8")

    old_video = (
        "    def _create_video_elements(self):\n"
        "        '''\n"
        "        Create the elements to connect the src and dest pipelines.\n"
        "        Src pipeline looks like: tee -> queue -> intervideosink\n"
        "        Dest pipeline looks like: intervideosrc -> videoscale -> videoconvert -> capsfilter -> queue -> tee\n"
        "        '''\n"
        "        intervideosrc, intervideosink = self._create_inter_elements('video')\n"
        "        self._create_dest_elements_after_intervideosrc(intervideosrc)\n"
    )
    new_video = (
        "    def _create_video_elements(self):\n"
        "        '''\n"
        "        Create the elements to connect the src and dest pipelines.\n"
        "        Src pipeline looks like: tee -> queue -> intervideosink\n"
        "        Dest pipeline looks like: intervideosrc -> videoscale -> videoconvert -> capsfilter -> queue -> tee\n"
        "        '''\n"
        "        intervideosrc, intervideosink = self._create_inter_elements('video')\n"
        "        if not intervideosrc or not intervideosink:\n"
        "            self.logger.error('Skipping video connection: inter video elements unavailable')\n"
        "            return False\n"
        "        self._create_dest_elements_after_intervideosrc(intervideosrc)\n"
        "        return True\n"
    )
    if old_video not in text:
        raise SystemExit("patch_brave_connection_guard: _create_video_elements snippet not found")
    text = text.replace(old_video, new_video, 1)

    old_audio = (
        "    def _create_audio_elements(self):\n"
        "        '''\n"
        "        The audio equivalent of _create_video_elements\n"
        "        '''\n"
        "        interaudiosrc, interaudiosink = self._create_inter_elements('audio')\n"
        "\n"
        "        # A queue ensures that disconnection from the audiomixer does not result in a pipeline failure:\n"
        "        queue = self._add_element_to_dest_pipeline('queue', 'audio', name='audio_queue')\n"
        "\n"
        "        # We use a tee even though we only have one output because then we can use\n"
        "        # allow-not-linked which means this bit of the pipeline does not fail when it's disconnected.\n"
        "        self._tee['audio'] = self._add_element_to_dest_pipeline('tee', 'audio', name='audio_tee_after_queue')\n"
        "        self._tee['audio'].set_property('allow-not-linked', True)\n"
        "\n"
        "        if not interaudiosrc.link(queue):\n"
        "            self.logger.error('Cannot link interaudiosrc to queue')\n"
        "        if not queue.link(self._tee['audio']):\n"
        "            self.logger.error('Cannot link queue to tee')\n"
    )
    new_audio = (
        "    def _create_audio_elements(self):\n"
        "        '''\n"
        "        The audio equivalent of _create_video_elements\n"
        "        '''\n"
        "        interaudiosrc, interaudiosink = self._create_inter_elements('audio')\n"
        "        if not interaudiosrc or not interaudiosink:\n"
        "            self.logger.error('Skipping audio connection: inter audio elements unavailable')\n"
        "            return False\n"
        "\n"
        "        # A queue ensures that disconnection from the audiomixer does not result in a pipeline failure:\n"
        "        queue = self._add_element_to_dest_pipeline('queue', 'audio', name='audio_queue')\n"
        "\n"
        "        # We use a tee even though we only have one output because then we can use\n"
        "        # allow-not-linked which means this bit of the pipeline does not fail when it's disconnected.\n"
        "        self._tee['audio'] = self._add_element_to_dest_pipeline('tee', 'audio', name='audio_tee_after_queue')\n"
        "        self._tee['audio'].set_property('allow-not-linked', True)\n"
        "\n"
        "        if not interaudiosrc.link(queue):\n"
        "            self.logger.error('Cannot link interaudiosrc to queue')\n"
        "        if not queue.link(self._tee['audio']):\n"
        "            self.logger.error('Cannot link queue to tee')\n"
        "        return True\n"
    )
    if old_audio not in text:
        raise SystemExit("patch_brave_connection_guard: _create_audio_elements snippet not found")
    text = text.replace(old_audio, new_audio, 1)

    add_to_mix_start = "    def _add_to_mix(self, audio_or_video):\n"
    add_to_mix_end = "    def _handle_video_mix_props(self):\n"
    old_add_to_mix = (
        "    def _add_to_mix(self, audio_or_video):\n"
        "        if audio_or_video not in self._mix_request_pad:\n"
        "            # We need to conect the tee to the destination. This is the pad of the tee:\n"
        "            tee_pad = self._get_or_create_tee_pad(audio_or_video)\n"
        "            self._mix_request_pad[audio_or_video] = self.dest.get_new_pad_for_source(audio_or_video)\n"
        "\n"
        "            link_response = tee_pad.link(self._mix_request_pad[audio_or_video])\n"
        "            if link_response != Gst.PadLinkReturn.OK:\n"
        "                self.logger.error('Cannot link %s to mix, response was %s' % (audio_or_video, link_response))\n"
        "\n"
        "        if audio_or_video == 'audio':\n"
        "            self._handle_audio_mix_props()\n"
        "        else:\n"
        "            self._handle_video_mix_props()\n"
    )
    new_add_to_mix = (
        "    def _add_to_mix(self, audio_or_video):\n"
        "        if audio_or_video not in self._tee:\n"
        "            self.logger.error('Skipping %s mix: tee is unavailable' % audio_or_video)\n"
        "            return\n"
        "        if audio_or_video not in self._mix_request_pad:\n"
        "            # We need to conect the tee to the destination. This is the pad of the tee:\n"
        "            tee_pad = self._get_or_create_tee_pad(audio_or_video)\n"
        "            if tee_pad is None:\n"
        "                self.logger.error('Skipping %s mix: tee pad unavailable' % audio_or_video)\n"
        "                return\n"
        "            self._mix_request_pad[audio_or_video] = self.dest.get_new_pad_for_source(audio_or_video)\n"
        "            if self._mix_request_pad[audio_or_video] is None:\n"
        "                self.logger.error('Skipping %s mix: destination pad unavailable' % audio_or_video)\n"
        "                return\n"
        "\n"
        "            link_response = tee_pad.link(self._mix_request_pad[audio_or_video])\n"
        "            if link_response != Gst.PadLinkReturn.OK:\n"
        "                self.logger.error('Cannot link %s to mix, response was %s' % (audio_or_video, link_response))\n"
        "                return\n"
        "\n"
        "        if audio_or_video == 'audio':\n"
        "            self._handle_audio_mix_props()\n"
        "        else:\n"
        "            self._handle_video_mix_props()\n"
    )
    s_add = text.find(add_to_mix_start)
    e_add = text.find(add_to_mix_end)
    if s_add == -1 or e_add == -1 or e_add <= s_add:
        raise SystemExit("patch_brave_connection_guard: _add_to_mix boundaries not found")
    # Only patch if method still resembles upstream body.
    current_add = text[s_add:e_add]
    if "Skipping %s mix: tee is unavailable" not in current_add:
        text = text[:s_add] + new_add_to_mix + "\n" + text[e_add:]

    new_ensure = (
        "    def _ensure_elements_are_created(self):\n"
        "        # STEP 1: Connect the source to the destination, unless that's already been done\n"
        "        video_created = True\n"
        "        audio_created = True\n"
        "        if self.has_video() and not hasattr(self, 'video_is_linked'):\n"
        "            video_created = self._create_video_elements()\n"
        "        if self.has_audio() and not hasattr(self, 'audio_is_linked'):\n"
        "            audio_created = self._create_audio_elements()\n"
        "\n"
        "        # STEP 2: Get the new elements in the same state as their pipelines:\n"
        "        self._sync_element_states()\n"
        "\n"
        "        # STEP 3: Connect the input's tee to these new elements\n"
        "        # (It's important we don't do this earlier, as if the elements were not\n"
        "        # ready we could disrupt the existing pipeline.)\n"
        "        if self.has_video() and not hasattr(self, 'video_is_linked') and video_created:\n"
        "            self._connect_tee_to_intersink('video')\n"
        "            self.video_is_linked = True\n"
        "        if self.has_audio() and not hasattr(self, 'audio_is_linked') and audio_created:\n"
        "            self._connect_tee_to_intersink('audio')\n"
        "            self.audio_is_linked = True\n"
        "\n"
        "        # If source and destination have already started, we need to unblock straightaway:\n"
        "        self.unblock_intersrc_if_ready()\n"
    )
    ensure_start = "    def _ensure_elements_are_created(self):\n"
    ensure_end = "    def _create_video_elements(self):\n"
    s = text.find(ensure_start)
    e = text.find(ensure_end)
    if s == -1 or e == -1 or e <= s:
        raise SystemExit("patch_brave_connection_guard: _ensure_elements_are_created boundaries not found")
    text = text[:s] + new_ensure + "\n" + text[e:]

    path.write_text(text, encoding="utf-8")


def main() -> None:
    patch_connection_py()
    patch_connection_to_mixer_py()
    print("[vbs-engine] patched brave/connections/* (inter element guard)")


if __name__ == "__main__":
    main()
