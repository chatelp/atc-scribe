"""Validation of the runway attributed to an arriving aircraft (docs-fr/31, V7).

For every approach seen towards Orly or De Gaulle in a co-atc daily database:
- the truth: the runway whose centreline the aircraft follows at its last point
  seen on final, with how far out and how high that point is;
- what the old rule (nearest threshold) and the new rule (nearest centreline,
  D68) said when the aircraft first became established on final.

Read-only on the database. Usage: python3 tools/runway-check/validate.py data/co-atc-2026-09-24.db ...
"""
import csv
import math
import sqlite3
import statistics
import sys
from collections import defaultdict

FT_PER_NM = 6076.12
REPO = __import__("os").path.dirname(__import__("os").path.dirname(__import__("os").path.dirname(__import__("os").path.abspath(__file__))))

ends = []   # one per runway end, with the direction flown when landing on it
for r in csv.DictReader(open(REPO + '/assets/runways.csv')):
    if r['airport_ident'] not in ('LFPO', 'LFPG') or r['closed'] == '1':
        continue
    for p, q in (('le', 'he'), ('he', 'le')):
        lat, lon = float(r[p + '_latitude_deg']), float(r[p + '_longitude_deg'])
        olat, olon = float(r[q + '_latitude_deg']), float(r[q + '_longitude_deg'])
        x = (olon - lon) * 60 * math.cos(math.radians(lat))
        y = (olat - lat) * 60
        ends.append(dict(ap=r['airport_ident'], ident=r[p + '_ident'], lat=lat, lon=lon,
                         elev=float(r[p + '_elevation_ft'] or 0),
                         bearing=math.degrees(math.atan2(x, y)) % 360))


def angle_diff(a, b):
    return abs((a - b + 540) % 360 - 180)


def geometry(e, lat, lon):
    """Distance before the threshold along the approach (NM, > 0 on final),
    offset from the centreline (NM), straight distance to the threshold (NM)."""
    x = (lon - e['lon']) * 60 * math.cos(math.radians(e['lat']))
    y = (lat - e['lat']) * 60
    b = math.radians(e['bearing'])
    along = x * math.sin(b) + y * math.cos(b)
    cross = x * math.cos(b) - y * math.sin(b)
    return -along, cross, math.hypot(x, y)


def candidates(lat, lon, track, maxd=10.0):
    """Runway ends the aircraft is on final for, by the detection's own terms:
    within 10 NM of the threshold, 0.5 NM of the centreline, track within 30."""
    out = []
    for e in ends:
        d, c, dist = geometry(e, lat, lon)
        if 0 < d and dist <= maxd and abs(c) <= 0.5 and angle_diff(track, e['bearing']) <= 30:
            out.append((e, d, c, dist))
    return out


def run(db):
    con = sqlite3.connect(f'file:{db}?mode=ro', uri=True)
    rows = con.execute(
        "select hex, flight, timestamp, lat, lon, alt_baro, track, nav_qnh from adsb_targets "
        "where lat between 48.55 and 49.25 and lon between 2.0 and 2.95 and alt_baro > 0 and alt_baro < 6000 "
        "and track is not null order by hex, timestamp").fetchall()
    qnh_by_hour = defaultdict(list)
    for hx, fl, ts, lat, lon, alt, trk, qnh in rows:
        if qnh and alt < 4500 and 940 < qnh < 1060:
            qnh_by_hour[ts[:13]].append(qnh)
    qnh_of = {h: statistics.median(v) for h, v in qnh_by_hour.items()}

    episodes = []
    cur = None
    last_hex, last_t = None, None
    for hx, fl, ts, lat, lon, alt, trk, qnh in rows:
        t = ts[:19]
        # As the detection now does (D68): parallels up to 1 NM past the
        # approach distance compete, one runway within it makes the approach.
        cands = candidates(lat, lon, trk, 11.0)
        new_ep = hx != last_hex
        last_hex = hx
        if not cands or min(c[3] for c in cands) > 10:
            continue
        ap = cands[0][0]['ap']
        if new_ep or cur is None or cur['hex'] != hx or cur['ap'] != ap or gap(cur['last_t'], t) > 120:
            cur = dict(hex=hx, flight=(fl or '').strip(), ap=ap, pts=[], last_t=t)
            episodes.append(cur)
        cur['pts'].append((t, lat, lon, alt, trk, cands, qnh_of.get(ts[:13], 1013.25)))
        cur['last_t'] = t
    return episodes


def gap(t1, t2):
    from datetime import datetime
    f = '%Y-%m-%dT%H:%M:%S'
    return (datetime.strptime(t2, f) - datetime.strptime(t1, f)).total_seconds()


def old_rule(cands):
    return min([c for c in cands if c[3] <= 10], key=lambda c: c[3])[0]['ident']


def new_rule(cands):
    return min([c for c in cands if c[3] <= 10], key=lambda c: (round(abs(c[2]) * 1852 / 10), c[3]))[0]['ident']


def margin_rule(cands):
    return min(cands, key=lambda c: (round(abs(c[2]) * 1852 / 10), c[3]))[0]['ident']


def nearest_axis_at(ep, dmax):
    """The nearest centreline at the first point of the episode within dmax NM
    of a threshold, every runway of the airport within 0.5 NM considered."""
    for t, lat, lon, alt, trk, cands, qnh in ep['pts']:
        if min(c[1] for c in cands) <= dmax:
            return min(cands, key=lambda c: abs(c[2]))[0]['ident']
    return None


def main():
    eps = []
    for db in sys.argv[1:]:
        eps += run(db)
    by_ap = defaultdict(list)
    for ep in eps:
        if len(ep['pts']) < 3:
            continue
        t, lat, lon, alt, trk, cands, qnh = ep['pts'][-1]
        truth = min(cands, key=lambda c: abs(c[2]))
        e, d, c, dist = truth
        height = alt + (qnh - 1013.25) * 27.3 - e['elev']
        others = [geometry(o, lat, lon) for o in ends
                  if o['ap'] == e['ap'] and o is not e and angle_diff(o['bearing'], e['bearing']) <= 5]
        margin = min((abs(g[1]) for g in others), default=None)
        pts_old = [old_rule(p[5]) == e['ident'] for p in ep['pts']]
        pts_new = [new_rule(p[5]) == e['ident'] for p in ep['pts']]
        pts_margin = [margin_rule(p[5]) == e['ident'] for p in ep['pts']]
        by_ap[e['ap']].append(dict(hex=ep['hex'], flight=ep['flight'], truth=e['ident'], last_d=d,
                                   last_h=height, last_c=abs(c), margin=margin,
                                   pts_old=pts_old, pts_new=pts_new, pts_margin=pts_margin,
                                   at8=nearest_axis_at(ep, 8), at6=nearest_axis_at(ep, 6)))
    def q(v, p):
        return v[min(len(v) - 1, int(p * len(v)))] if v else float('nan')

    labels = dict(old='old (nearest threshold)', new='nearest centreline', margin='nearest centreline, 1 NM stagger margin')
    for ap, lst in sorted(by_ap.items()):
        print(f"\n== {ap}: {len(lst)} approaches seen")
        ds = sorted(x['last_d'] for x in lst)
        hs = sorted(x['last_h'] for x in lst)
        print(f"  A. last point seen on final: {q(ds, .5):.1f} NM from the threshold (quartiles {q(ds, .25):.1f}-{q(ds, .75):.1f}), "
              f"{q(hs, .5):.0f} ft above it (quartiles {q(hs, .25):.0f}-{q(hs, .75):.0f})")
        print('     followed below 2 / 3 / 4 / 6 NM:', ' / '.join(str(sum(x['last_d'] <= lim for x in lst)) for lim in (2, 3, 4, 6)))
        near = [x for x in lst if x['last_d'] <= 4]
        by_rw = defaultdict(int)
        for x in near:
            by_rw[x['truth']] += 1
        print(f"     usable truth (last point within 4 NM): {len(near)}, by runway {dict(sorted(by_rw.items()))}")
        if not near:
            continue
        cs = sorted(x['last_c'] * 1852 for x in near)
        ms = sorted(x['margin'] * 1852 for x in near if x['margin'] is not None)
        print(f"     offset from the centreline at the last point: median {q(cs, .5):.0f} m, 95th percentile {q(cs, .95):.0f} m"
              + (f"; the neighbouring parallel is at least {ms[0]:.0f} m away (median {q(ms, .5):.0f} m)" if ms else ''))
        n_pts = sum(len(x['pts_old']) for x in near)
        for rule in ('old', 'new', 'margin'):
            ok = sum(sum(x['pts_' + rule]) for x in near)
            maj = sum(sum(x['pts_' + rule]) * 2 > len(x['pts_' + rule]) for x in near)
            print(f"  B. {labels[rule]}, point by point: {ok} of {n_pts} points right ({100 * ok / n_pts:.1f}%), "
                  f"majority right for {maj} approaches of {len(near)}")
        for key, lab in (('at8', '8 NM'), ('at6', '6 NM')):
            got = [x for x in near if x[key]]
            ok = sum(x[key] == x['truth'] for x in got)
            print(f"  C. nearest centreline at {lab} = runway at the last point: {ok} of {len(got)}")
            for x in [x for x in got if x[key] != x['truth']][:5]:
                print(f"       {x['flight'] or x['hex']}: {x[key]} at {lab}, {x['truth']} at the last point ({x['last_d']:.1f} NM)")

if __name__ == '__main__':
    main()
