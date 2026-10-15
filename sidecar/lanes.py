"""One worker thread for the model, with two lanes.

Every model call goes through one thread: MLX is not shared between threads,
and one decode at a time is what the GPU does well anyway. With a plain FIFO,
a transmission on one frequency waited for the whole of another's work, its
second (French) reading included: measured by whisper-lab on 14/10, the
compute was 3.1 s a transmission (median) but the wait reached 26.5 s at the
90th percentile whenever both frequencies spoke at once.

So the queue has two lanes. FIRST holds what a listener waits for, the first
reading of each transmission; SECOND holds what can come later, the language
detector and the second reading. A FIRST job always runs before any SECOND
job still waiting; within a lane, jobs run in the order they came. A job
already running is never interrupted.
"""

from __future__ import annotations

import itertools
import queue
import threading
from concurrent.futures import Future

FIRST = 0
SECOND = 1


class PriorityWorker:
    """A single thread running submitted callables, FIRST lane before SECOND."""

    def __init__(self, name: str = "mlx", stack_size: int | None = None):
        self._jobs: queue.PriorityQueue = queue.PriorityQueue()
        self._order = itertools.count()
        previous = threading.stack_size(stack_size) if stack_size else None
        try:
            self._thread = threading.Thread(target=self._run, name=name, daemon=True)
            self._thread.start()
        finally:
            if stack_size:
                threading.stack_size(previous)

    def submit(self, fn, lane: int = FIRST) -> Future:
        """Queue fn() in a lane; the Future holds its result or its exception."""
        future: Future = Future()
        self._jobs.put((lane, next(self._order), fn, future))
        return future

    def waiting(self) -> dict:
        """How many jobs wait in each lane (an estimate, for /health)."""
        counts = {FIRST: 0, SECOND: 0}
        with self._jobs.mutex:
            for lane, _, _, _ in self._jobs.queue:
                counts[lane] = counts.get(lane, 0) + 1
        return {"first": counts[FIRST], "second": counts[SECOND]}

    def _run(self) -> None:
        while True:
            _, _, fn, future = self._jobs.get()
            if not future.set_running_or_notify_cancel():
                continue
            try:
                future.set_result(fn())
            except BaseException as e:  # noqa: BLE001 -- handed to the caller
                future.set_exception(e)
