#!/usr/bin/env python3
"""A long unattended run, read every few minutes: one CSV line per reading.

    python3 tools/night-log/night-log.py --out runs/mesures/nuit-2026-10-14.csv \
        --coatc http://127.0.0.1:8020 --sidecar http://127.0.0.1:8188 --until 10:00

It only reads: the sidecar's /health (its queue by lane, the second readings
not fetched, its counters), co-atc's /api/v1/status (the ADS-B link, the
receiver) and /api/v1/vitals (aircraft, transmissions, the share matched,
the time per transmission), and the memory of both processes (macOS
`footprint`, which counts what the GPU holds, where RSS does not). A reading
that fails is written with what it got: an empty column is a fact too.
Measured latencies per transmission come from co-atc's log afterwards.

It also watches the disk, the run being unattended: the free space and the
size of what grows (the log, the audio archive, the databases). Below
--min-free-gb free, or past --max-log-mb of log, it stops the instance (the
processes named by --stop) and writes why: a full disk would take the whole
Mac down with it, a measure is not worth that.
"""

from __future__ import annotations

import argparse
import csv
import datetime as dt
import json
import os
import re
import shutil
import signal
import subprocess
import time
import urllib.request

COLUMNS = [
    "at", "sidecar_ok", "queue_first", "queue_second", "second_pending", "transcribed",
    "rejected_no_speech", "second_ok", "second_failed", "detector_failed",
    "adsb_ok", "adsb_aircraft", "adsb_msg_s", "adsb_failing_since", "adsb_failing_why",
    "receiver_state", "receiver_stale",
    "transmissions_today", "matched_today", "matched_share", "seconds_per_transmission",
    "coatc_footprint_mb", "sidecar_footprint_mb",
    "disk_free_gb", "log_mb", "archive_mb", "data_mb", "stopped",
]


def get(url: str, timeout: float = 10.0):
    try:
        with urllib.request.urlopen(url, timeout=timeout) as r:
            return json.load(r)
    except Exception:  # noqa: BLE001 -- an empty column says it
        return None


def pid_of(pattern: str) -> str:
    out = subprocess.run(["pgrep", "-f", pattern], capture_output=True, text=True).stdout.split()
    return out[0] if out else ""


def footprint_mb(pid: str):
    if not pid:
        return ""
    out = subprocess.run(["footprint", "-p", pid], capture_output=True, text=True).stdout
    m = re.search(r"phys_footprint:\s*([\d.]+)\s*(KB|MB|GB)", out)
    if not m:
        return ""
    n = float(m.group(1))
    return round(n / 1024 if m.group(2) == "KB" else n * 1024 if m.group(2) == "GB" else n)


def size_mb(path: str):
    if not path or not os.path.exists(path):
        return ""
    if os.path.isfile(path):
        return round(os.path.getsize(path) / 1048576, 1)
    total = 0
    for root, _, files in os.walk(path):
        for name in files:
            try:
                total += os.path.getsize(os.path.join(root, name))
            except OSError:
                pass
    return round(total / 1048576, 1)


def stop_instance(patterns) -> list:
    stopped = []
    for pattern in patterns:
        pid = pid_of(pattern)
        if pid:
            os.kill(int(pid), signal.SIGTERM)
            stopped.append(f"{pattern} ({pid})")
    return stopped


def reading(args) -> dict:
    h = get(args.sidecar + "/health") or {}
    st = get(args.coatc + "/api/v1/status") or {}
    v = get(args.coatc + "/api/v1/vitals") or {}
    adsb = st.get("adsb") or {}
    rx = st.get("receiver") or {}
    q = h.get("queue") or {}
    return {
        "at": dt.datetime.now().astimezone().isoformat(timespec="seconds"),
        "sidecar_ok": h.get("status", ""),
        "queue_first": q.get("first", ""),
        "queue_second": q.get("second", ""),
        "second_pending": h.get("second_pending", ""),
        "transcribed": h.get("transcribed", ""),
        "rejected_no_speech": h.get("rejected_no_speech", ""),
        "second_ok": h.get("second_opinion_ok", ""),
        "second_failed": h.get("second_opinion_failed", ""),
        "detector_failed": h.get("detector_failed", ""),
        "adsb_ok": adsb.get("ok", ""),
        "adsb_aircraft": adsb.get("aircraft", ""),
        "adsb_msg_s": adsb.get("messages_per_second", ""),
        "adsb_failing_since": adsb.get("failing_since", ""),
        "adsb_failing_why": adsb.get("failing_why", ""),
        "receiver_state": rx.get("state", ""),
        "receiver_stale": rx.get("stale", ""),
        "transmissions_today": v.get("transmissions_today", ""),
        "matched_today": v.get("matched_today", ""),
        "matched_share": v.get("matched_share", ""),
        "seconds_per_transmission": v.get("seconds_per_transmission", ""),
        "coatc_footprint_mb": footprint_mb(pid_of(args.coatc_process)),
        "sidecar_footprint_mb": footprint_mb(pid_of(args.sidecar_process)),
        "disk_free_gb": round(shutil.disk_usage(os.path.dirname(os.path.abspath(args.out))).free / 1e9, 1),
        "log_mb": size_mb(args.log),
        "archive_mb": size_mb(args.archive),
        "data_mb": size_mb(args.data),
        "stopped": "",
    }


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    p.add_argument("--out", required=True)
    p.add_argument("--coatc", default="http://127.0.0.1:8020")
    p.add_argument("--sidecar", default="http://127.0.0.1:8188")
    p.add_argument("--coatc-process", default="co-atc -config")
    p.add_argument("--sidecar-process", default="whisper_server.py --port 8188")
    p.add_argument("--every", type=float, default=300, help="seconds between readings")
    p.add_argument("--until", default="", help="local HH:MM to stop at (the next one to come)")
    p.add_argument("--log", default="", help="the instance's log file, watched")
    p.add_argument("--archive", default="", help="the audio archive's folder, watched")
    p.add_argument("--data", default="", help="the databases' folder, watched")
    p.add_argument("--min-free-gb", type=float, default=20)
    p.add_argument("--max-log-mb", type=float, default=500)
    p.add_argument("--stop", action="append", default=[],
                   help="a process pattern (pgrep -f) to stop when the disk is at risk; repeatable")
    args = p.parse_args()

    stop = None
    if args.until:
        hh, mm = (int(x) for x in args.until.split(":"))
        now = dt.datetime.now()
        stop = now.replace(hour=hh, minute=mm, second=0, microsecond=0)
        if stop <= now:
            stop += dt.timedelta(days=1)

    os.makedirs(os.path.dirname(os.path.abspath(args.out)), exist_ok=True)
    new = not os.path.exists(args.out)
    with open(args.out, "a", newline="") as f:
        w = csv.DictWriter(f, fieldnames=COLUMNS)
        if new:
            w.writeheader()
        while stop is None or dt.datetime.now() < stop:
            r = reading(args)
            why = ""
            if r["disk_free_gb"] < args.min_free_gb:
                why = f"disk free {r['disk_free_gb']} GB < {args.min_free_gb}"
            elif r["log_mb"] != "" and r["log_mb"] > args.max_log_mb:
                why = f"log {r['log_mb']} MB > {args.max_log_mb}"
            if why:
                r["stopped"] = why + ": " + ", ".join(stop_instance(args.stop))
            w.writerow(r)
            f.flush()
            if why:
                return
            time.sleep(args.every)


if __name__ == "__main__":
    main()
