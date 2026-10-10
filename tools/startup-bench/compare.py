#!/usr/bin/env python3
"""Compare the logs of two bench runs (tools/startup-bench/run.sh).

The server logs from many goroutines at once -- the audio processors, the ADS-B
poller, the weather retries -- so two runs of the same binary never print the
same file. Three views are compared instead, each stable across runs of one
commit (measured on main before anything moved):

  1. main: the lines of the unnamed logger, which main() writes itself, in
     order and with their fields; the per-listener lines of the HTTP servers,
     written from one goroutine each, are sorted within their block.
  2. construction order: main()'s lines interleaved with the first line of
     each named logger that speaks first from main()'s own goroutine, which is
     the order the services are built and started in. Loggers born in a
     goroutine of their own (BORN_IN_GOROUTINE) race with it and are left out
     of this view: measured, web-socket comes before or after runway-use from
     one run to the next.
  3. messages: the set of distinct (level, logger, message).

    compare.py <before dir> <after dir>
"""
import difflib
import re
import sys

ANSI = re.compile(r"\x1b\[[0-9;]*m")
CONCURRENT = ("Starting HTTP server", "Attempting to shutdown HTTP server",
              "HTTP server shutdown complete")
BORN_IN_GOROUTINE = {"web-socket", "weather-client", "weather-cache", "central-audio-p",
                     "local-xscribe", "grammar-process", "freq-stream"}


def parse(path):
    lines = []
    with open(path, encoding="utf-8", errors="replace") as f:
        for raw in f:
            raw = ANSI.sub("", raw.rstrip("\n"))
            if not raw or raw.startswith("\t"):  # stack trace continuation
                continue
            parts = raw.split("\t")
            if len(parts) < 2:
                continue
            level = parts[0]
            # Named loggers pad their name to a column; the unnamed one has none.
            if len(parts) >= 3 and re.fullmatch(r"[a-z0-9-]+ *", parts[1]):
                name, msg, fields = parts[1].strip(), parts[2], "\t".join(parts[3:])
            else:
                name, msg, fields = "", parts[1], "\t".join(parts[2:])
            lines.append((level, name, msg, fields))
    return lines


def main_view(lines):
    out, block = [], []
    for level, name, msg, fields in lines:
        if name:
            continue
        line = f"{level} {msg} {fields}".rstrip()
        if msg in CONCURRENT:
            block.append(line)
            continue
        out += sorted(block)
        block = []
        out.append(line)
    return out + sorted(block)


def construction(lines):
    seen, order = set(), []
    for level, name, msg, _ in lines:
        if not name:
            if msg not in CONCURRENT:
                order.append(f"main: {msg}")
        elif name not in seen and name not in BORN_IN_GOROUTINE:
            seen.add(name)
            order.append(f"{name}: {msg}")
    return order


def messages(lines):
    return {(level, name, msg) for level, name, msg, _ in lines}


def main():
    a, b = parse(sys.argv[1] + "/server.log"), parse(sys.argv[2] + "/server.log")
    ok = True

    ma, mb = main_view(a), main_view(b)
    if ma != mb:
        ok = False
        print("DIFF main():")
        for d in difflib.unified_diff(ma, mb, "before", "after", lineterm="", n=1):
            print("  " + d)
    else:
        print(f"same main() lines: {len(ma)}")

    ca, cb = construction(a), construction(b)
    if ca != cb:
        ok = False
        print("DIFF construction order:")
        for d in difflib.unified_diff(ca, cb, "before", "after", lineterm="", n=1):
            print("  " + d)
    else:
        print(f"same construction order: {len(ca)} steps")

    sa, sb = messages(a), messages(b)
    if sa != sb:
        ok = False
        for m in sorted(sa - sb):
            print(f"only before: {m}")
        for m in sorted(sb - sa):
            print(f"only after:  {m}")
    else:
        print(f"same distinct messages: {len(sa)}")

    print("IDENTICAL" if ok else "DIFFERENT")
    sys.exit(0 if ok else 1)


main()
