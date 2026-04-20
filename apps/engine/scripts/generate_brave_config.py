#!/usr/bin/env python3
"""依環境變數產生 Brave 設定檔（2 路 SRT uri 入、左右分割 mixer、可選 WebRTC + TCP 出）。"""
import os
import sys

import yaml


def _env_truthy(name: str, default: bool = False) -> bool:
    v = os.environ.get(name)
    if v is None:
        return default
    return v.strip().lower() in ("1", "true", "yes", "on")


def main() -> None:
    enable_audio = _env_truthy("VBS_ENGINE_BRAVE_ENABLE_AUDIO", True)
    use_test = _env_truthy("VBS_ENGINE_USE_TEST_SOURCES", False)
    if use_test:
        pattern = int(os.environ.get("VBS_ENGINE_TEST_VIDEO_PATTERN", "18"))
        freq = int(os.environ.get("VBS_ENGINE_TEST_AUDIO_FREQ", "440"))
        inputs = [
            {"type": "test_video", "pattern": pattern, "state": "PLAYING"},
            (
                {"type": "test_audio", "freq": freq, "state": "PLAYING"}
                if enable_audio
                else {"type": "test_video", "pattern": pattern, "state": "PLAYING"}
            ),
        ]
    else:
        try:
            u1 = os.environ["VBS_ENGINE_SRT_INPUT_1_URI"]
            u2 = os.environ["VBS_ENGINE_SRT_INPUT_2_URI"]
        except KeyError as e:
            print(f"缺少必要環境變數: {e}", file=sys.stderr)
            sys.exit(1)
        inputs = [
            {"type": "uri", "uri": u1, "state": "PLAYING"},
            {"type": "uri", "uri": u2, "state": "PLAYING"},
        ]

    w = int(os.environ.get("VBS_ENGINE_MIXER_WIDTH", "854"))
    h = int(os.environ.get("VBS_ENGINE_MIXER_HEIGHT", "480"))
    tw = max(1, w // 2)
    tcp_port = int(os.environ.get("VBS_ENGINE_PGM_TCP_PORT", "30090"))

    api_port = int(os.environ.get("PORT", os.environ.get("VBS_ENGINE_API_PORT", "5000")))
    enable_webrtc = _env_truthy("VBS_ENGINE_BRAVE_ENABLE_WEBRTC", False)

    outputs = [
        {
            "type": "tcp",
            "state": "PLAYING",
            "host": "127.0.0.1",
            "port": tcp_port,
            "source": "mixer1",
            "container": "mpeg",
            "width": w,
            "height": h,
        },
    ]
    if enable_webrtc:
        outputs.append(
            {
                "type": "webrtc",
                "state": "PLAYING",
                "source": "mixer1",
                "width": w,
                "height": h,
            }
        )

    cfg = {
        "enable_video": True,
        "enable_audio": enable_audio,
        "api_host": os.environ.get("VBS_ENGINE_API_HOST", "0.0.0.0"),
        "api_port": api_port,
        "default_mixer_width": w,
        "default_mixer_height": h,
        "stun_server": os.environ.get("VBS_ENGINE_STUN_SERVER", "stun.l.google.com:19302"),
        "inputs": inputs,
        "mixers": [
            {
                "width": w,
                "height": h,
                "pattern": 0,
                "sources": [
                    {
                        "uid": "input1",
                        "in_mix": True,
                        "zorder": 1,
                        "width": tw,
                        "height": h,
                        "xpos": 0,
                        "ypos": 0,
                    },
                    {
                        "uid": "input2",
                        "in_mix": True,
                        "zorder": 1,
                        "width": tw,
                        "height": h,
                        "xpos": tw,
                        "ypos": 0,
                    },
                ],
            }
        ],
        "outputs": outputs,
    }

    out = os.environ.get("VBS_ENGINE_BRAVE_CONFIG_PATH", "/tmp/brave.yaml")
    with open(out, "w", encoding="utf-8") as f:
        yaml.safe_dump(cfg, f, allow_unicode=True, default_flow_style=False, sort_keys=False)
    print(f"[vbs-engine] 已寫入 Brave 設定: {out} (mixer {w}x{h}, tcp {tcp_port})")


if __name__ == "__main__":
    main()
