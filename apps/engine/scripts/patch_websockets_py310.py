#!/usr/bin/env python3
"""
Sanic 19.3.1 依賴 websockets>=6,<7；websockets 6.x 在 asyncio 呼叫使用 loop=，
Python 3.10 起 Lock / Future / Queue / wait / wait_for / ensure_future 等已移除該參數。

於 site-packages 修補 websockets/protocol.py、websockets/server.py（不升級 websockets 主版本，避免與 Sanic API 不相容）。
"""
import pathlib
import site
import sys

# (relative path under websockets package, old, new)
PATCHES: list[tuple[str, str, str]] = [
    (
        "protocol.py",
        "stream_reader = asyncio.StreamReader(limit=read_limit // 2, loop=loop)",
        "stream_reader = asyncio.StreamReader(limit=read_limit // 2)",
    ),
    (
        "protocol.py",
        "        super().__init__(stream_reader, self.client_connected, loop)",
        "        super().__init__(stream_reader, self.client_connected)",
    ),
    (
        "protocol.py",
        "        self._drain_lock = asyncio.Lock(loop=loop)",
        "        self._drain_lock = asyncio.Lock()",
    ),
    (
        "protocol.py",
        "        self.connection_lost_waiter = asyncio.Future(loop=loop)",
        "        self.connection_lost_waiter = asyncio.Future()",
    ),
    (
        "protocol.py",
        "        self.messages = asyncio.queues.Queue(max_queue, loop=loop)",
        "        self.messages = asyncio.queues.Queue(max_queue)",
    ),
    (
        "protocol.py",
        """        self.transfer_data_task = asyncio_ensure_future(
            self.transfer_data(), loop=self.loop)""",
        """        self.transfer_data_task = asyncio_ensure_future(
            self.transfer_data())""",
    ),
    (
        "protocol.py",
        """        self.close_connection_task = asyncio_ensure_future(
            self.close_connection(), loop=self.loop)""",
        """        self.close_connection_task = asyncio_ensure_future(
            self.close_connection())""",
    ),
    (
        "protocol.py",
        """        next_message = asyncio_ensure_future(
            self.messages.get(), loop=self.loop)""",
        """        next_message = asyncio_ensure_future(
            self.messages.get())""",
    ),
    (
        "protocol.py",
        """            done, pending = yield from asyncio.wait(
                [next_message, self.transfer_data_task],
                loop=self.loop, return_when=asyncio.FIRST_COMPLETED)""",
        """            done, pending = yield from asyncio.wait(
                [next_message, self.transfer_data_task],
                return_when=asyncio.FIRST_COMPLETED)""",
    ),
    (
        "protocol.py",
        """            yield from asyncio.wait_for(
                self.write_close_frame(serialize_close(code, reason)),
                self.timeout, loop=self.loop)""",
        """            yield from asyncio.wait_for(
                self.write_close_frame(serialize_close(code, reason)),
                self.timeout)""",
    ),
    (
        "protocol.py",
        """            yield from asyncio.wait_for(
                self.transfer_data_task,
                self.timeout, loop=self.loop)""",
        """            yield from asyncio.wait_for(
                self.transfer_data_task,
                self.timeout)""",
    ),
    (
        "protocol.py",
        "        self.pings[data] = asyncio.Future(loop=self.loop)",
        "        self.pings[data] = asyncio.Future()",
    ),
    (
        "protocol.py",
        """                yield from asyncio.wait_for(
                    asyncio.shield(self.connection_lost_waiter),
                    self.timeout, loop=self.loop)""",
        """                yield from asyncio.wait_for(
                    asyncio.shield(self.connection_lost_waiter),
                    self.timeout)""",
    ),
    (
        "protocol.py",
        """            self.close_connection_task = asyncio_ensure_future(
                self.close_connection(), loop=self.loop)""",
        """            self.close_connection_task = asyncio_ensure_future(
                self.close_connection())""",
    ),
    (
        "server.py",
        """        self.handler_task = asyncio_ensure_future(
            self.handler(), loop=self.loop)""",
        """        self.handler_task = asyncio_ensure_future(
            self.handler())""",
    ),
    (
        "server.py",
        """            yield from asyncio.wait(
                [websocket.handler_task for websocket in self.websockets] +
                [websocket.close_connection_task
                    for websocket in self.websockets],
                loop=self.loop)""",
        """            yield from asyncio.wait(
                [websocket.handler_task for websocket in self.websockets] +
                [websocket.close_connection_task
                    for websocket in self.websockets])""",
    ),
]


def main() -> None:
    for sp in site.getsitepackages():
        base = pathlib.Path(sp) / "websockets"
        p_protocol = base / "protocol.py"
        if not p_protocol.is_file():
            continue
        for rel, old, new in PATCHES:
            p = base / rel
            if not p.is_file():
                print(f"patch_websockets_py310: missing {p}", file=sys.stderr)
                sys.exit(1)
            t = p.read_text(encoding="utf-8")
            if old not in t:
                if new in t:
                    continue
                print(f"patch_websockets_py310: pattern not found in {p}", file=sys.stderr)
                sys.exit(1)
            p.write_text(t.replace(old, new, 1), encoding="utf-8")
        print("patch_websockets_py310: ok")
        return
    print("patch_websockets_py310: websockets not found under site-packages", file=sys.stderr)
    sys.exit(1)


if __name__ == "__main__":
    main()
