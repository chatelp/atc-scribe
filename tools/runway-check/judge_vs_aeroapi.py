"""The runway judge's live verdicts against FlightAware (docs-fr/05, D69).

Reads the verdicts judge_log.py recorded from a running co-atc, fetches the
arrivals and departures FlightAware reports at the same airports over the same
hours, and compares each verdict with actual_runway_on or actual_runway_off.
This checks the server's own code on live traffic, departures included, where
aeroapi_compare.py checked the rule offline on arrivals.

Usage: python3 tools/runway-check/judge_vs_aeroapi.py runs/par-test/verdicts.jsonl [--budget PAGES]
"""
import json
import os
import sys
from collections import defaultdict
from datetime import datetime, timedelta, timezone

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from aeroapi_compare import AeroAPI, CHUNK  # noqa: E402


def parse(ts):
    return datetime.strptime(ts[:19], '%Y-%m-%dT%H:%M:%S').replace(tzinfo=timezone.utc)


def callsigns(f):
    return {(f.get(k) or '').strip() for k in ('atc_ident', 'ident_icao', 'ident')} - {''}


def main():
    path = sys.argv[1]
    budget = int(sys.argv[sys.argv.index('--budget') + 1]) if '--budget' in sys.argv else 400
    events = [json.loads(l) for l in open(path) if l.strip()]
    verdicts = {}   # (hex, kind) -> last verdict
    app_without = [e for e in events if e['event'] == 'app_without_verdict']
    for e in events:
        if e['event'] != 'verdict':
            continue
        for kind in ('arrival', 'departure'):
            v = e['runway'].get(kind)
            if v:
                verdicts[(e['hex'], kind)] = dict(flight=e['flight'], airport=v['airport'], runway=v['runway'], since=parse(v['since']))

    api = AeroAPI(budget)
    windows = defaultdict(set)
    for (hex_, kind), v in verdicts.items():
        t = v['since']
        windows[(v['airport'], kind)].add(t.replace(minute=0, second=0, microsecond=0) - timedelta(hours=t.hour % 3))
    fa = defaultdict(list)
    for (ap, kind), starts in windows.items():
        for s in sorted(starts):
            for chunk in (s - CHUNK, s, s + CHUNK):   # a landing can fall in the next window
                got = api.flights(ap, 'arrivals' if kind == 'arrival' else 'departures', chunk, chunk + CHUNK)
                if got:
                    fa[(ap, kind)] += got

    print(f'AeroAPI pages fetched: {api.pages} (about ${api.pages * 0.005:.2f})')
    print(f'aircraft seen in approach without a verdict: {len(app_without)}')
    for kind, field, tfield, lo, hi in (('arrival', 'actual_runway_on', 'actual_on', 0, 25), ('departure', 'actual_runway_off', 'actual_off', -15, 0)):
        for ap in ('LFPG', 'LFPO'):
            vs = [(h, v) for (h, k), v in verdicts.items() if k == kind and v['airport'] == ap and v['flight']]
            ok, total, missing, nor, bad = 0, 0, 0, 0, []
            seen = set()
            for h, v in vs:
                m = [f for f in fa[(ap, kind)] if v['flight'] in callsigns(f) and f.get(tfield)
                     and timedelta(minutes=lo) <= parse(f[tfield]) - v['since'] <= timedelta(minutes=hi)]
                if not m:
                    missing += 1
                    continue
                f = m[0]
                if f.get('fa_flight_id') in seen:
                    continue
                seen.add(f.get('fa_flight_id'))
                if not f.get(field):
                    nor += 1
                    continue
                total += 1
                if f[field] == v['runway']:
                    ok += 1
                else:
                    bad.append((v['flight'], v['runway'], f[field]))
            if vs:
                print(f"\n== {ap} {kind}s: {len(vs)} verdicts; {missing} not found at FlightAware, {nor} without a runway there; "
                      f"{ok} of {total} agree" + (f' ({100 * ok / total:.1f}%)' if total else ''))
                for b in bad[:10]:
                    print(f'     {b[0]}: judge {b[1]}, FlightAware {b[2]}')


if __name__ == '__main__':
    main()
