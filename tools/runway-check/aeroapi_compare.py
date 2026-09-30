"""The runway of each arrival, checked against FlightAware AeroAPI (docs-fr/31, V7).

AeroAPI gives the runway each flight actually landed on (`actual_runway_on`,
"when known"). For every approach the station saw towards Orly or De Gaulle,
the flight is found by callsign and landing time, and its runway compared with
the one co-atc gives: the nearest centreline at the last point the station saw,
at 8 NM out, and what the former nearest-threshold rule gave at each point.

Arrivals lists go back 10 days. The Personal tier bills $0.005 per page of 15
flights and allows 10 pages a minute: the script waits accordingly and stops at
a page budget.

Key: ~/.config/atc-scribe/aeroapi.key or $AEROAPI_KEY_FILE -- never in the
repository. Answers are cached in ~/.cache/atc-scribe/aeroapi/, also outside
it: the Personal licence allows personal use only, and only counts are
published.

Usage: python3 tools/runway-check/aeroapi_compare.py [--budget PAGES] db1 [db2 ...]
"""
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from collections import defaultdict
from datetime import datetime, timedelta, timezone

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import validate as V  # noqa: E402

KEY_FILE = os.environ.get('AEROAPI_KEY_FILE', os.path.expanduser('~/.config/atc-scribe/aeroapi.key'))
CACHE = os.path.expanduser('~/.cache/atc-scribe/aeroapi')
BASE = 'https://aeroapi.flightaware.com/aeroapi'
UA = 'atc-scribe/runway-check'
PAGES_PER_MINUTE = 10
CHUNK = timedelta(hours=3)


class AeroAPI:
    def __init__(self, budget):
        self.key = open(KEY_FILE).read().strip()
        self.budget = budget
        self.pages = 0
        self.stamps = []

    def _throttle(self, pages):
        now = time.time()
        self.stamps = [t for t in self.stamps if now - t < 61]
        while sum(p for _, p in [(t, 1) for t in self.stamps]) + pages > PAGES_PER_MINUTE:
            time.sleep(5)
            now = time.time()
            self.stamps = [t for t in self.stamps if now - t < 61]

    def arrivals(self, airport, start, end):
        return self.flights(airport, 'arrivals', start, end)

    def flights(self, airport, kind, start, end):
        """Arrivals or departures at an airport between start and end, cached."""
        os.makedirs(CACHE, exist_ok=True)
        tag = (f"{airport}_{start:%Y%m%dT%H%M}_{end:%Y%m%dT%H%M}.json" if kind == 'arrivals'
               else f"{airport}_{kind}_{start:%Y%m%dT%H%M}_{end:%Y%m%dT%H%M}.json")
        path = os.path.join(CACHE, tag)
        if os.path.exists(path):
            return json.load(open(path))
        flights = []
        url = (f'{BASE}/airports/{airport}/flights/{kind}?'
               + urllib.parse.urlencode({'start': start.strftime('%Y-%m-%dT%H:%M:%SZ'),
                                         'end': end.strftime('%Y-%m-%dT%H:%M:%SZ'), 'max_pages': 1}))
        while url:
            if self.pages >= self.budget:
                return None
            self._throttle(1)
            req = urllib.request.Request(url, headers={'x-apikey': self.key, 'Accept': 'application/json', 'User-Agent': UA})
            try:
                d = json.load(urllib.request.urlopen(req, timeout=90))
            except urllib.error.HTTPError as e:
                if e.code == 429:
                    time.sleep(30)
                    continue
                raise
            self.pages += 1
            self.stamps.append(time.time())
            flights += d.get(kind) or []
            nxt = (d.get('links') or {}).get('next')
            url = (BASE + nxt) if nxt else None
        # A window still open, or just closed, is not kept: flights are still
        # landing and FlightAware still filling in runways.
        if end < datetime.now(timezone.utc) - timedelta(minutes=30):
            json.dump(flights, open(path, 'w'))
        return flights


def parse(ts):
    return datetime.strptime(ts[:19], '%Y-%m-%dT%H:%M:%S').replace(tzinfo=timezone.utc)


def main():
    args = sys.argv[1:]
    budget = 10**6
    if args[:1] == ['--budget']:
        budget, args = int(args[1]), args[2:]
    api = AeroAPI(budget)
    eps = []
    for db in args:
        eps += [ep for ep in V.run(db) if len(ep['pts']) >= 3]

    # Fetch the arrivals over the hours the station saw approaches in.
    windows = defaultdict(set)
    for ep in eps:
        t = parse(ep['pts'][-1][0])
        windows[ep['ap']].add(t.replace(minute=0, second=0) - timedelta(hours=t.hour % 3))
    fa = defaultdict(list)
    for ap, starts in windows.items():
        for s in sorted(starts):
            got = api.arrivals(ap, s, s + CHUNK)
            if got is None:
                print(f'page budget reached at {ap} {s:%d/%m %H:%M}')
                break
            fa[ap] += got

    rows = []
    for ep in eps:
        if not ep['flight']:
            continue
        t_last, lat, lon, alt, trk, cands, qnh = ep['pts'][-1]
        ours = min(cands, key=lambda c: abs(c[2]))
        if ours[1] > 6:
            continue
        t = parse(t_last)
        # The station hears the radio callsign (AFR16MC); FlightAware files the
        # flight under its number (AFR1234) and gives the other as atc_ident.
        match = [f for f in fa[ep['ap']]
                 if ep['flight'] in {(f.get(k) or '').strip() for k in ('atc_ident', 'ident_icao', 'ident')}
                 and f.get('actual_on') and timedelta(0) <= parse(f['actual_on']) - t <= timedelta(minutes=15)]
        if not match:
            rows.append(dict(ap=ep['ap'], flight=ep['flight'], fa=None))
            continue
        f = match[0]
        rows.append(dict(ap=ep['ap'], flight=ep['flight'], fa=f.get('actual_runway_on'), ours=ours[0]['ident'],
                         ours_d=ours[1], at8=V.nearest_axis_at(ep, 8),
                         old=[V.old_rule(p[5]) for p in ep['pts']], new=[V.margin_rule(p[5]) for p in ep['pts']]))

    print(f'AeroAPI pages fetched: {api.pages} (about ${api.pages * 0.005:.2f})')
    for ap in ('LFPG', 'LFPO'):
        lst = [r for r in rows if r['ap'] == ap]
        matched = [r for r in lst if r.get('fa')]
        print(f"\n== {ap}: {len(lst)} approaches with a callsign followed within 6 NM; "
              f"{sum(1 for r in lst if r['fa'] is None and 'ours' not in r)} not found at FlightAware, "
              f"{sum(1 for r in lst if 'ours' in r and not r['fa'])} found without a runway, {len(matched)} with one")
        if not matched:
            continue
        by = defaultdict(int)
        for r in matched:
            by[r['fa']] += 1
        print('  runways per FlightAware:', dict(sorted(by.items())))
        for key, lab in (('ours', "nearest centreline at the station's last point"), ('at8', 'nearest centreline at 8 NM')):
            got = [r for r in matched if r.get(key)]
            ok = sum(r[key] == r['fa'] for r in got)
            print(f'  {lab}: {ok} of {len(got)} agree ({100 * ok / len(got):.1f}%)')
            for r in [r for r in got if r[key] != r['fa']][:8]:
                print(f"     {r['flight']}: co-atc {r[key]}, FlightAware {r['fa']} (last seen {r['ours_d']:.1f} NM out)")
        for key, lab in (('old', 'former rule (nearest threshold)'), ('new', 'current rule (nearest centreline, stagger margin)')):
            pts = sum(len(r[key]) for r in matched)
            ok = sum(sum(1 for x in r[key] if x == r['fa']) for r in matched)
            print(f'  {lab}, point by point on final: {ok} of {pts} ({100 * ok / pts:.1f}%)')


if __name__ == '__main__':
    main()
