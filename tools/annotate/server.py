#!/usr/bin/env python3
"""Minimal annotation server for building ATC transcription ground truth.

Serves a corpus directory of short audio clips plus a single-page UI, and
persists annotations to a JSON file. Standard library only, no dependencies.

    python3 tools/annotate/server.py --corpus /path/to/clips --out truth.json

The corpus directory is expected to contain a `manifeste.json` describing the
sample (see docs-fr/09-jeu-de-test.md), or, failing that, any nested audio
files, which are then discovered by scanning.

If the corpus also holds a `candidats.json`, the UI can reveal what the models
made of a clip -- on demand, never by default. Revealing is recorded, and the
blind attempt is kept alongside the final text, because seeing a reading changes
what you hear and the size of that effect is worth knowing rather than assuming.
The readings are served unlabelled, in an order drawn once per clip and stored,
so an annotator cannot systematically favour one model over another.

Binds to localhost only. This tool has no authentication and is not meant to
be reachable from anywhere else.
"""
import argparse
import json
import mimetypes
import os
import posixpath
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, unquote, urlparse

AUDIO_EXTENSIONS = (".mp3", ".wav", ".flac", ".ogg", ".m4a")
HERE = os.path.dirname(os.path.abspath(__file__))

lock = threading.Lock()


def load_items(corpus):
    """Return the ordered list of clips to annotate."""
    manifest = os.path.join(corpus, "manifeste.json")
    if os.path.exists(manifest):
        with open(manifest, encoding="utf-8") as fh:
            data = json.load(fh)
        items = data["items"] if isinstance(data, dict) else data
        return [dict(it) for it in items]

    found = []
    for root, _dirs, files in os.walk(corpus):
        for name in sorted(files):
            if name.lower().endswith(AUDIO_EXTENSIONS):
                rel = os.path.relpath(os.path.join(root, name), corpus)
                found.append({"id": rel.replace(os.sep, "/"), "fichier": name})
    return found


def load_candidates(corpus):
    """Model readings per clip, or {} when none were produced."""
    path = os.path.join(corpus, "candidats.json")
    if not os.path.exists(path):
        return {}
    with open(path, encoding="utf-8") as fh:
        return json.load(fh)


def load_annotations(path):
    if os.path.exists(path):
        with open(path, encoding="utf-8") as fh:
            return json.load(fh)
    return {}


def save_annotations(path, data):
    """Write atomically: a half-written truth file would be worse than none."""
    tmp = path + ".tmp"
    with open(tmp, "w", encoding="utf-8") as fh:
        json.dump(data, fh, ensure_ascii=False, indent=1, sort_keys=True)
    os.replace(tmp, path)


def make_handler(corpus, out_path, exclude=frozenset()):
    class Handler(BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"

        def log_message(self, fmt, *args):  # quieter than the default
            if not self.path.startswith("/audio/"):
                super().log_message(fmt, *args)

        def _send(self, code, body, ctype="application/json; charset=utf-8"):
            if isinstance(body, str):
                body = body.encode("utf-8")
            self.send_response(code)
            self.send_header("Content-Type", ctype)
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def do_GET(self):
            path = unquote(urlparse(self.path).path)

            if path in ("/", "/index.html"):
                with open(os.path.join(HERE, "index.html"), "rb") as fh:
                    return self._send(200, fh.read(), "text/html; charset=utf-8")

            if path == "/api/items":
                with lock:
                    payload = {
                        # Hidden frequencies are only hidden from the annotator.
                        # The manifest and the annotations file are untouched,
                        # so scoring still sees every clip already done.
                        "items": [it for it in load_items(corpus)
                                  if str(it.get("freq", "")) not in exclude],
                        "annotations": load_annotations(out_path),
                        # Whether the reveal button has anything to reveal. The
                        # readings themselves are not in this payload.
                        "has_readings": bool(load_candidates(corpus)),
                    }
                return self._send(200, json.dumps(payload, ensure_ascii=False))

            if path == "/api/candidates":
                # One clip at a time, and only when asked. Shipping every
                # reading with the clip list would put them a devtools panel
                # away from an annotator who is trying not to look.
                clip_id = parse_qs(urlparse(self.path).query).get("id", [""])[0]
                entry = load_candidates(corpus).get(unquote(clip_id))
                if not entry:
                    return self._send(404, '{"error":"no readings for this clip"}')
                order = entry.get("ordre") or [
                    k for k in entry if isinstance(entry.get(k), dict)
                ]
                readings = [
                    {"texte": entry[name].get("texte", "")}
                    for name in order if isinstance(entry.get(name), dict)
                ]
                return self._send(200, json.dumps({"readings": readings},
                                                  ensure_ascii=False))

            if path.startswith("/audio/"):
                rel = posixpath.normpath(path[len("/audio/"):])
                if rel.startswith("..") or os.path.isabs(rel):
                    return self._send(403, '{"error":"forbidden"}')
                full = os.path.join(corpus, rel)
                if not os.path.isfile(full):
                    return self._send(404, '{"error":"not found"}')
                ctype = mimetypes.guess_type(full)[0] or "application/octet-stream"
                with open(full, "rb") as fh:
                    return self._send(200, fh.read(), ctype)

            return self._send(404, '{"error":"not found"}')

        def do_POST(self):
            if urlparse(self.path).path != "/api/annotation":
                return self._send(404, '{"error":"not found"}')
            length = int(self.headers.get("Content-Length", 0))
            entry = json.loads(self.rfile.read(length) or b"{}")
            clip_id = entry.get("id")
            if not clip_id:
                return self._send(400, '{"error":"missing id"}')
            with lock:
                data = load_annotations(out_path)
                data[clip_id] = entry
                save_annotations(out_path, data)
                done = sum(1 for v in data.values() if v.get("text") or v.get("no_speech"))
            return self._send(200, json.dumps({"ok": True, "done": done}))

    return Handler


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--corpus", required=True, help="directory holding the clips")
    parser.add_argument("--out", default="ground-truth.json", help="annotations file")
    parser.add_argument("--port", type=int, default=8777)
    parser.add_argument("--exclude-freq", default="",
                        help="comma-separated frequencies to stop presenting, e.g. 129525,128950")
    args = parser.parse_args()
    exclude = frozenset(f.strip() for f in args.exclude_freq.split(",") if f.strip())

    corpus = os.path.abspath(args.corpus)
    out_path = os.path.abspath(args.out)
    items = load_items(corpus)
    done = len([v for v in load_annotations(out_path).values()
                if v.get("text") or v.get("no_speech")])
    print(f"{len(items)} clips in {corpus}")
    if exclude:
        hidden = sum(1 for it in items if str(it.get("freq", "")) in exclude)
        print(f"{hidden} hidden (frequencies {', '.join(sorted(exclude))}), "
              f"{len(items) - hidden} presented")
    cands = load_candidates(corpus)
    print(f"{len(cands)} clips have model readings"
          if cands else "no candidats.json -- the reveal button stays hidden")
    print(f"{done} already annotated in {out_path}")
    print(f"open http://127.0.0.1:{args.port}/  (Ctrl-C to stop)")

    ThreadingHTTPServer(("127.0.0.1", args.port), make_handler(corpus, out_path, exclude)).serve_forever()


if __name__ == "__main__":
    main()
