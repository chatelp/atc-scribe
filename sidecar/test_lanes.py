"""python -m pytest sidecar/test_lanes.py (or python sidecar/test_lanes.py)"""

import threading
import time

from lanes import FIRST, SECOND, PriorityWorker


def test_a_first_reading_runs_before_second_readings_already_waiting():
    w = PriorityWorker(name="test")
    gate = threading.Event()
    order = []
    # The thread is busy: everything below waits in the queue.
    busy = w.submit(lambda: gate.wait(5))
    seconds = [w.submit(lambda i=i: order.append(f"second-{i}"), SECOND) for i in range(3)]
    first = w.submit(lambda: order.append("first"), FIRST)
    gate.set()
    busy.result(5)
    first.result(5)
    for f in seconds:
        f.result(5)
    assert order == ["first", "second-0", "second-1", "second-2"]


def test_within_a_lane_the_order_of_arrival_holds():
    w = PriorityWorker(name="test")
    gate = threading.Event()
    order = []
    w.submit(lambda: gate.wait(5))
    futures = [w.submit(lambda i=i: order.append(i), FIRST) for i in range(5)]
    gate.set()
    for f in futures:
        f.result(5)
    assert order == [0, 1, 2, 3, 4]


def test_an_exception_reaches_the_caller_and_the_worker_goes_on():
    w = PriorityWorker(name="test")

    def boom():
        raise ValueError("decode failed")

    f = w.submit(boom)
    try:
        f.result(5)
        assert False, "the exception was lost"
    except ValueError:
        pass
    assert w.submit(lambda: 42).result(5) == 42


def test_waiting_counts_each_lane():
    w = PriorityWorker(name="test")
    gate = threading.Event()
    w.submit(lambda: gate.wait(5))
    time.sleep(0.05)  # the busy job has left the queue
    w.submit(lambda: None, SECOND)
    w.submit(lambda: None, SECOND)
    w.submit(lambda: None, FIRST)
    assert w.waiting() == {"first": 1, "second": 2}
    gate.set()


if __name__ == "__main__":
    for name, fn in list(globals().items()):
        if name.startswith("test_"):
            fn()
            print("ok", name)
