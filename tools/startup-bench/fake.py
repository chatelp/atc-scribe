#!/usr/bin/env python3
"""Stand-ins for what the server probes at startup: a tar1090 with one aircraft,
and a transcription sidecar that answers /health. Everything else is a 404,
which is what the frequencies and the weather service get.

    fake.py <port>
"""
import json
import sys
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

STATION = (43.6777, -79.6248)  # configs/config.toml.example


def aircraft():
    return {
        "now": time.time(),
        "messages": 1000,
        "aircraft": [{
            "hex": "c0ffee", "flight": "BENCH01 ", "alt_baro": 12000, "alt_geom": 12100,
            "gs": 300.0, "track": 90.0, "baro_rate": 0, "squawk": "1200",
            "lat": STATION[0] + 0.1, "lon": STATION[1] + 0.1,
            "seen": 0.1, "seen_pos": 0.1, "rssi": -20.0, "messages": 100,
        }],
    }


ROUTES = {
    "/data/aircraft.json": aircraft,
    "/data/receiver.json": lambda: {"version": "bench", "refresh": 1000, "history": 0,
                                    "lat": STATION[0], "lon": STATION[1]},
    "/data/stats.json": lambda: {"now": time.time()},
    "/health": lambda: {"status": "ok", "model": "bench", "degraded": False},
}


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        make = ROUTES.get(self.path.split("?")[0])
        if make is None:
            self.send_error(404)
            return
        body = json.dumps(make()).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


ThreadingHTTPServer(("127.0.0.1", int(sys.argv[1])), Handler).serve_forever()
