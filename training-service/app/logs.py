"""Log tailing for the SSE endpoints.

Both the prepare and the training runner append their stdout/stderr straight to
a file, so streaming is a plain offset read instead of a pipe relay: a slow or
disconnected client can never block the child process, and reconnecting is just
a new request with the last `offset` it saw.
"""

from __future__ import annotations

import time
from collections.abc import Iterator
from pathlib import Path

from app.settings import settings

# How long a single read waits for new bytes before the caller loops again.
_POLL_INTERVAL_SECONDS = 0.5
# Upper bound on the bytes returned in one SSE event, so one giant log line can
# never blow up a single frame.
_MAX_FRAME_BYTES = 16 * 1024


class LogStreamError(RuntimeError):
    """Raised when the requested log file is not readable."""


def clamp_offset(offset: int, size: int) -> int:
    """Return a start offset inside the retained window of the log file.

    A client that reconnects after a long absence gets the tail of the log
    rather than an unbounded replay, which bounds per-connection memory.
    """
    offset = max(offset, 0)
    window_start = max(0, size - settings.log_tail_bytes)
    if offset < window_start:
        return window_start
    if offset > size:
        return size
    return offset


def read_new_lines(path: Path, offset: int) -> tuple[list[dict[str, int | str]], int]:
    """Read whole lines appended after `offset`. Returns (frames, new_offset)."""
    try:
        size = path.stat().st_size
    except OSError:
        return [], offset

    start = clamp_offset(offset, size)
    if start >= size:
        return [], size

    frames: list[dict[str, int | str]] = []
    try:
        with open(path, "rb") as handle:
            handle.seek(start)
            while True:
                line = handle.readline()
                if not line:
                    break
                end = handle.tell()
                text = line.decode("utf-8", errors="replace").rstrip("\r\n")
                if text:
                    frames.append({"offset": end, "line": text[:_MAX_FRAME_BYTES]})
                start = end
    except OSError:
        return frames, offset
    return frames, start


def tail(path: Path) -> Iterator[dict[str, int | str]]:
    """Yield log frames forever, following appends until the caller stops."""
    offset = 0
    try:
        size = path.stat().st_size
    except OSError:
        size = 0
    offset = clamp_offset(offset, size)
    while True:
        frames, offset = read_new_lines(path, offset)
        yield from frames
        # An idle poll keeps the connection open without busy-waiting; the HTTP
        # layer cancels the generator when the client goes away.
        time.sleep(_POLL_INTERVAL_SECONDS)
