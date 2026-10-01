"""Record the runway judge's verdicts from a running co-atc (docs-fr/05, D69).

Polls /api/v1/aircraft and writes one JSON line each time an aircraft's
verdict appears or changes, and one for each aircraft seen in approach (APP)
without a verdict, so that the night's verdicts can be compared with the
runways FlightAware reports (aeroapi_compare.py --judge). Stops at --until.

Usage: python3 tools/runway-check/judge_log.py --out runs/par-test/verdicts.jsonl --until 07:30 [--base http://localhost:8012]
"""
import argparse
import json
import time
import urllib.request
from datetime import datetime, timedelta, timezone


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--out', required=True)
    ap.add_argument('--until', default='07:30', help='local time to stop at, HH:MM')
    ap.add_argument('--base', default='http://localhost:8012')
    ap.add_argument('--every', type=float, default=20.0)
    a = ap.parse_args()
    hh, mm = map(int, a.until.split(':'))
    now = datetime.now()
    stop = now.replace(hour=hh, minute=mm, second=0, microsecond=0)
    if stop <= now:
        stop += timedelta(days=1)

    seen = {}          # hex -> last verdict written
    app_only = set()   # hex seen in APP without a verdict, already written
    failures = 0
    with open(a.out, 'a') as out:
        while datetime.now() < stop:
            t = datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')
            try:
                d = json.load(urllib.request.urlopen(a.base + '/api/v1/aircraft', timeout=15))
                failures = 0
            except Exception as e:  # the server may restart: note it, keep going
                failures += 1
                if failures in (1, 15, 90):
                    out.write(json.dumps({'t': t, 'event': 'api_error', 'error': str(e)[:120], 'failures': failures}) + '\n')
                    out.flush()
                time.sleep(a.every)
                continue
            aircraft = d['aircraft'].values() if isinstance(d['aircraft'], dict) else d['aircraft']
            for x in aircraft:
                hex_ = x.get('hex')
                flight = (x.get('flight') or '').strip()
                rw = x.get('runway')
                ph = ((x.get('phase') or {}).get('current') or [{}])[0]
                if rw and rw != seen.get(hex_):
                    seen[hex_] = rw
                    out.write(json.dumps({'t': t, 'event': 'verdict', 'hex': hex_, 'flight': flight, 'runway': rw,
                                          'phase': ph.get('phase'), 'phase_airport': ph.get('airport')}) + '\n')
                elif not rw and ph.get('phase') == 'APP' and hex_ not in app_only:
                    app_only.add(hex_)
                    t_ = x.get('adsb') or {}
                    out.write(json.dumps({'t': t, 'event': 'app_without_verdict', 'hex': hex_, 'flight': flight,
                                          'phase_airport': ph.get('airport'), 'lat': t_.get('lat'), 'lon': t_.get('lon'),
                                          'alt': t_.get('alt_baro'), 'track': t_.get('track')}) + '\n')
            out.flush()
            time.sleep(a.every)


if __name__ == '__main__':
    main()
