"""The runway of each arrival, checked against OpenSky Network (docs-fr/31, V7).

For every approach the station saw towards Orly or De Gaulle, OpenSky's track of
the same aircraft is fetched, and the runway is read from its last waypoints
near the airport -- usually much lower than the station follows them, often on
the ground. That runway is compared with the one co-atc gives: the nearest
centreline at the last point the station saw, and at 8 NM out.

OpenSky counts credits by the age of the data: a track under 24 hours old
costs 4, two days old 180. Run it on the day's database.

Credentials: an OpenSky API client, {"clientId", "clientSecret"}, in
~/.config/atc-scribe/opensky.json or $OPENSKY_CREDENTIALS -- never in the
repository. OpenSky's answers are cached in ~/.cache/atc-scribe/opensky/, also
outside it: they are OpenSky's data, and only counts are published.

Usage: python3 tools/runway-check/opensky_compare.py data/co-atc-2026-09-30.db [max_requests]
"""
import json
import math
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import validate as V  # noqa: E402

CREDS = os.environ.get('OPENSKY_CREDENTIALS', os.path.expanduser('~/.config/atc-scribe/opensky.json'))
CACHE = os.path.expanduser('~/.cache/atc-scribe/opensky')
UA = 'atc-scribe/runway-check'
FLOOR = 100          # stop before the day's credits run out entirely


class OpenSky:
    def __init__(self):
        self.token = None
        self.expires = 0
        self.remaining = None
        self.requests = 0

    def _token(self):
        if self.token and time.time() < self.expires - 60:
            return self.token
        c = json.load(open(CREDS))
        data = urllib.parse.urlencode({'grant_type': 'client_credentials', 'client_id': c['clientId'],
                                       'client_secret': c['clientSecret']}).encode()
        req = urllib.request.Request(
            'https://auth.opensky-network.org/auth/realms/opensky-network/protocol/openid-connect/token',
            data=data, headers={'User-Agent': UA})
        t = json.load(urllib.request.urlopen(req, timeout=30))
        self.token, self.expires = t['access_token'], time.time() + t.get('expires_in', 1800)
        return self.token

    def track(self, icao24, t):
        os.makedirs(CACHE, exist_ok=True)
        path = os.path.join(CACHE, f'{icao24}_{t}.json')
        if os.path.exists(path):
            return json.load(open(path))
        if self.remaining is not None and self.remaining < FLOOR:
            return None
        req = urllib.request.Request(f'https://opensky-network.org/api/tracks/all?icao24={icao24}&time={t}',
                                     headers={'Authorization': 'Bearer ' + self._token(), 'User-Agent': UA})
        self.requests += 1
        try:
            r = urllib.request.urlopen(req, timeout=60)
            body = json.load(r)
            self.remaining = int(r.headers.get('X-Rate-Limit-Remaining') or 0)
        except urllib.error.HTTPError as e:
            if e.code == 404:
                # Not computed yet (OpenSky builds tracks with its nightly
                # flights), or never seen: not cached, to try again later.
                return {'path': []}
            if e.code == 429:
                self.remaining = 0
                return None
            else:
                raise
        json.dump(body, open(path, 'w'))
        time.sleep(1)
        return body


def epoch(ts):
    from datetime import datetime, timezone
    return int(datetime.strptime(ts[:19], '%Y-%m-%dT%H:%M:%S').replace(tzinfo=timezone.utc).timestamp())


def opensky_runway(path, ap, after):
    """The runway end OpenSky's last waypoints near the airport lie on: the last
    waypoint after `after` that is on final for, or on, a runway of `ap`."""
    best = None
    for w in path:
        t, lat, lon, alt_m, trk, ground = w
        if t < after - 60 or lat is None:
            continue
        for e in V.ends:
            if e['ap'] != ap:
                continue
            d, c, dist = V.geometry(e, lat, lon)
            # On final within 4 NM, or on the runway itself (up to its length past the threshold).
            if -2.5 < d <= 4 and abs(c) <= 0.25 and (trk is None or V.angle_diff(trk, e['bearing']) <= 30 or ground):
                cand = (t, e['ident'], d, abs(c), ground)
                if best is None or cand[0] > best[0] or (cand[0] == best[0] and cand[3] < best[3]):
                    best = cand
    return best


def main():
    db = sys.argv[1]
    budget = int(sys.argv[2]) if len(sys.argv) > 2 else 10**9
    eps = [ep for ep in V.run(db) if len(ep['pts']) >= 3]
    os_api = OpenSky()
    rows = []
    for ep in eps:
        t_last, lat, lon, alt, trk, cands, qnh = ep['pts'][-1]
        ours = min(cands, key=lambda c: abs(c[2]))
        at8 = V.nearest_axis_at(ep, 8)
        if ours[1] > 6:
            continue        # not followed far enough down to say which runway
        if os_api.requests >= budget and not os.path.exists(os.path.join(CACHE, f"{ep['hex']}_{epoch(t_last)}.json")):
            continue
        tr = os_api.track(ep['hex'], epoch(t_last))
        if tr is None:
            continue
        got = opensky_runway(tr.get('path') or [], ep['ap'], epoch(t_last))
        rows.append(dict(ap=ep['ap'], flight=ep['flight'] or ep['hex'], ours=ours[0]['ident'], ours_d=ours[1],
                         at8=at8, os=got[1] if got else None, os_d=got[2] if got else None,
                         os_ground=got[4] if got else None))
    print(f'OpenSky requests: {os_api.requests}, credits left: {os_api.remaining}')
    for ap in ('LFPG', 'LFPO'):
        lst = [r for r in rows if r['ap'] == ap]
        seen = [r for r in lst if r['os']]
        print(f'\n== {ap}: {len(lst)} arrivals followed within 6 NM, {len(seen)} with a runway from OpenSky')
        if not seen:
            continue
        ds = sorted(r['os_d'] for r in seen)
        print(f"  OpenSky's last point on final: {V_q(ds, .5):.2f} NM from the threshold in the median "
              f"(quartiles {V_q(ds, .25):.2f} to {V_q(ds, .75):.2f}; negative = past it), on the ground for {sum(1 for r in seen if r['os_ground'])}")
        ours_d = sorted(r['ours_d'] for r in seen)
        print(f"  the station's last point: {V_q(ours_d, .5):.2f} NM in the median")
        for key, lab in (('ours', "nearest centreline at the station's last point"), ('at8', 'nearest centreline at 8 NM')):
            got = [r for r in seen if r[key]]
            ok = sum(r[key] == r['os'] for r in got)
            print(f'  {lab}: {ok} of {len(got)} agree with OpenSky')
            for r in [r for r in got if r[key] != r['os']][:8]:
                print(f"     {r['flight']}: co-atc {r[key]}, OpenSky {r['os']} ({r['os_d']:.2f} NM)")


def V_q(v, p):
    return v[min(len(v) - 1, int(p * len(v)))] if v else float('nan')


if __name__ == '__main__':
    main()
