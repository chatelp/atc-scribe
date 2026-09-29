// Replays a co-atc daily database through the PAR view's own geometry
// (www/par/par-geometry.js) and measures its settings on real traffic:
// how close established arrivals ride the glide path once corrected for the
// QNH, how much of each arrival and departure the cones keep and which limit
// drops the rest, what share of what is shown belongs to the runway, and how
// often an aircraft jumps between frames. docs-fr/31-vue-par.md.
//
// Read-only on the database. The runways are read from a running co-atc, the
// same data the view uses:
//   node tools/runway-check/par-replay.js data/co-atc-2026-09-29.db http://localhost:8012
'use strict';

const { DatabaseSync } = require('node:sqlite');
const path = require('node:path');
const G = require(path.join(__dirname, '../../www/par/par-geometry.js'));

const [dbPath, base = 'http://localhost:8012'] = process.argv.slice(2);
const AIRPORTS = ['LFPO', 'LFPG'];
const O = Object.assign({}, G.DEFAULTS, JSON.parse(process.env.PAR_OPTS || '{}'));

const median = (v) => pct(v, 0.5);
function pct(v, p) {
    if (!v.length) return NaN;
    const s = [...v].sort((a, b) => a - b);
    return s[Math.min(s.length - 1, Math.floor(p * s.length))];
}
const round = (x, n = 0) => (Number.isFinite(x) ? x.toFixed(n) : '-');

async function main() {
    const frames = {};
    const centre = {};
    for (const ap of AIRPORTS) {
        const d = await (await fetch(`${base}/api/v1/airports/${ap}`)).json();
        frames[ap] = G.buildFrames(ap, d.runways);
        centre[ap] = { lat: d.latitude, lon: d.longitude };
    }
    const db = new DatabaseSync(dbPath, { readOnly: true });
    const rows = db.prepare(
        `select hex, flight, timestamp as t, lat, lon, alt_baro as alt, track, nav_qnh as qnh
         from adsb_targets where lat between 48.3 and 49.4 and lon between 1.8 and 3.1
         and alt_baro is not null and alt_baro > 0 and alt_baro <= 16000 and track is not null
         order by hex, timestamp`).all();
    const phases = new Map();   // hex -> [{t, phase, airport}]
    for (const r of db.prepare('select hex, phase, timestamp as t, airport from phase_changes order by hex, timestamp').all()) {
        if (!phases.has(r.hex)) phases.set(r.hex, []);
        phases.get(r.hex).push(r);
    }
    const phaseAt = (hex, t) => {
        let cur = null;
        for (const p of phases.get(hex) || []) { if (p.t <= t) cur = p; else break; }
        return cur;
    };
    const role = (hex, ap) => {
        const ps = (phases.get(hex) || []).filter((p) => p.airport === ap).map((p) => p.phase);
        if (ps.some((p) => ['APP', 'T/D'].includes(p))) return 'arrival';
        if (ps.some((p) => ['T/O', 'CLB', 'DEP'].includes(p))) return 'departure';
        if (ps.includes('ARR')) return 'inbound';
        return 'other';
    };

    // QNH as the view estimates it: median setting of the low aircraft near the
    // airport, here per five minutes.
    const qnhBucket = {};
    for (const ap of AIRPORTS) qnhBucket[ap] = new Map();
    for (const r of rows) {
        if (r.qnh == null || r.alt > O.transitionAltitudeFt - 500 || r.qnh < 940 || r.qnh > 1060) continue;
        for (const ap of AIRPORTS) {
            const dx = (r.lon - centre[ap].lon) * 60 * Math.cos(centre[ap].lat * Math.PI / 180);
            const dy = (r.lat - centre[ap].lat) * 60;
            if (Math.hypot(dx, dy) > 40) continue;
            const k = r.t.slice(0, 15) + (Number(r.t[15]) < 5 ? '0' : '5');
            if (!qnhBucket[ap].has(k)) qnhBucket[ap].set(k, []);
            qnhBucket[ap].get(k).push(r.qnh);
        }
    }
    const qnhOf = (ap, t) => {
        const v = qnhBucket[ap].get(t.slice(0, 15) + (Number(t[15]) < 5 ? '0' : '5'));
        return v && v.length ? median(v) : G.STD_HPA;
    };

    // Which single limit keeps a point out of every frame.
    const LIMITS = [
        ['range', { rangeNM: 60 }], ['half-width', { halfWidthDeg: 89 }],
        ['elevation', { elevApproachDeg: 89, elevDepartDeg: 89 }], ['ceiling', { ceilingFt: 1e9 }],
        ['crossing', { maxCrossingDeg: 90 }],
    ];
    const cause = (fr, input) => {
        for (const [name, o] of LIMITS) if (G.classify(fr, input, Object.assign({}, O, o))) return name;
        return 'several';
    };

    for (const ap of AIRPORTS) {
        const fr = frames[ap];
        const endOf = (p) => (p.side === 'rwy' ? null : fr.find((f) => f.id === p.frame)[p.side]);
        const shown = { arrival: 0, departure: 0, inbound: 0, other: 0 };
        const others = new Map();
        const glideFt = [], glideDeg = [], glideRawFt = [], azArr = [];
        const glideByDist = { '3-5': [], '5-7': [], '7-10': [] };
        const azByDist = { '2-4': [], '4-6': [], '6-8': [] };
        let dep7 = 0;
        const arrivals = [], departures = [];
        let switches = 0, displayedAircraft = 0;
        const byHex = new Map();
        for (const r of rows) {
            if (!byHex.has(r.hex)) byHex.set(r.hex, []);
            byHex.get(r.hex).push(r);
        }
        for (const [hex, pts] of byHex) {
            const rl = role(hex, ap);
            let prev = null, lastFrame = null, everShown = false;
            const seq = [];
            for (const r of pts) {
                const ph = phaseAt(hex, r.t);
                if (ph && ph.airport && ph.airport !== ap) { prev = null; continue; }
                const qnh = qnhOf(ap, r.t);
                const input = { lat: r.lat, lon: r.lon, altFt: G.qnhAltitude(r.alt, qnh), track: r.track };
                const p = G.classify(fr, input, O, prev);
                seq.push({ r, p, input, qnh });
                if (!p) { prev = null; continue; }
                prev = p.frame;
                everShown = true;
                shown[rl]++;
                if (rl === 'other') {
                    const k = hex + ' ' + (r.flight || '').trim();
                    others.set(k, (others.get(k) || 0) + 1);
                }
                if (lastFrame && lastFrame !== p.frame) switches++;
                lastFrame = p.frame;
            }
            if (everShown) displayedAircraft++;

            if (rl === 'arrival') {
                // Its runway: the end it approaches at its last point within 6 NM.
                const last = [...seq].reverse().find((x) => x.p && x.p.side !== 'rwy' && !x.p.movingAway && x.p.dist <= 6);
                if (!last) continue;
                const f = fr.find((q) => q.id === last.p.frame);
                const endKey = last.p.side;
                const e = f[endKey];
                let first = null, inFrame = 0, total = 0;
                const causes = {};
                for (const x of seq) {
                    const pr = G.project(f, x.r.lat, x.r.lon);
                    const d = endKey === 'le' ? -pr.s : pr.s - f.lengthNM;
                    if (d <= 0 || d > O.rangeNM) continue;
                    total++;
                    const ok = x.p && x.p.frame === f.id && x.p.side === endKey;
                    if (ok) {
                        inFrame++;
                        if (first === null) first = d;
                        const dThr = endKey === 'le' ? e.thresholdS - pr.s : pr.s - e.thresholdS;
                        if (Math.abs(x.p.azimuthDeg) <= 1 && dThr >= 3 && dThr <= 10) {
                            const gp = G.glidePathFt(f, endKey, pr.s);
                            glideFt.push(x.input.altFt - gp);
                            glideRawFt.push(x.r.alt - gp);
                            const angle = Math.atan2(x.input.altFt - e.elevFt - e.thresholdCrossingFt, dThr * G.FT_PER_NM) * 180 / Math.PI;
                            glideDeg.push(angle - e.slopeDeg);
                            glideByDist[dThr < 5 ? '3-5' : dThr < 7 ? '5-7' : '7-10'].push(x.input.altFt - gp);
                        }
                        if (dThr >= 2 && dThr <= 8) {
                            azArr.push(Math.abs(x.p.azimuthDeg));
                            azByDist[dThr < 4 ? '2-4' : dThr < 6 ? '4-6' : '6-8'].push(Math.abs(x.p.azimuthDeg));
                        }
                    } else {
                        const c = x.p ? 'other frame' : cause(fr, x.input);
                        causes[c] = (causes[c] || 0) + 1;
                    }
                }
                arrivals.push({ hex, first, inFrame, total, causes, end: e.ident });
            }
            if (rl === 'departure') {
                // Its runway: the frame it climbs out of within 3 NM of the end.
                const firstOut = seq.find((x) => x.p && x.p.side !== 'rwy' && x.p.movingAway && x.p.dist <= 3);
                if (!firstOut) continue;
                const f = fr.find((q) => q.id === firstOut.p.frame);
                const side = firstOut.p.side;
                let inFrame = 0, total = 0, lastIn = 0;
                const causes = {};
                for (const x of seq) {
                    const pr = G.project(f, x.r.lat, x.r.lon);
                    const d = side === 'le' ? -pr.s : pr.s - f.lengthNM;
                    if (d <= 0 || d > 10) continue;
                    total++;
                    const p7 = G.classify(fr, x.input, Object.assign({}, O, { elevDepartDeg: O.elevApproachDeg }), null);
                    if (p7 && p7.frame === f.id && p7.side === side) dep7++;
                    if (x.p && x.p.frame === f.id && x.p.side === side) { inFrame++; lastIn = Math.max(lastIn, d); } else {
                        const c = x.p ? 'other frame' : cause(fr, x.input);
                        causes[c] = (causes[c] || 0) + 1;
                    }
                }
                departures.push({ hex, inFrame, total, causes, lastIn, end: f[side === 'le' ? 'he' : 'le'].ident });
            }
        }

        console.log(`\n== ${ap}`);
        console.log(`  Glide path, established arrivals (within 1 degree of the axis, 3 to 10 NM out): ${glideFt.length} points`);
        console.log(`    height above the 3-degree path, QNH-corrected: median ${round(median(glideFt))} ft, quartiles ${round(pct(glideFt, .25))} to ${round(pct(glideFt, .75))}, 5-95% ${round(pct(glideFt, .05))} to ${round(pct(glideFt, .95))}`);
        console.log(`    the same on pressure altitude, uncorrected: median ${round(median(glideRawFt))} ft`);
        console.log(`    by distance from the threshold: ${Object.entries(glideByDist).map(([k, v]) => `${k} NM ${round(median(v))} ft (${v.length})`).join(', ')}`);
        console.log(`    angular deviation: median ${round(median(glideDeg), 2)} deg; within +/-${O.elevToleranceDeg} deg: ${round(100 * glideDeg.filter((v) => Math.abs(v) <= O.elevToleranceDeg).length / glideDeg.length, 1)}%`);
        console.log(`  Azimuth by distance, share within +/-${O.azToleranceDeg} deg: ${Object.entries(azByDist).map(([k, v]) => `${k} NM ${round(100 * v.filter((w) => w <= O.azToleranceDeg).length / v.length, 1)}% (${v.length})`).join(', ')}`);
        console.log(`  Azimuth of arrivals 2 to 8 NM out: median ${round(median(azArr), 2)} deg, 95% ${round(pct(azArr, .95), 2)} deg; within +/-${O.azToleranceDeg} deg: ${round(100 * azArr.filter((v) => v <= O.azToleranceDeg).length / azArr.length, 1)}%`);
        const cov = (lst) => {
            const inF = lst.reduce((a, x) => a + x.inFrame, 0), tot = lst.reduce((a, x) => a + x.total, 0);
            const causes = {};
            for (const x of lst) for (const [k, v] of Object.entries(x.causes)) causes[k] = (causes[k] || 0) + v;
            return { inF, tot, causes };
        };
        const a = cov(arrivals);
        const firsts = arrivals.map((x) => x.first).filter((v) => v != null);
        console.log(`  Arrivals with a runway: ${arrivals.length}; their points up to ${O.rangeNM} NM on the approach side: ${a.inF} of ${a.tot} shown (${round(100 * a.inF / a.tot, 1)}%)`);
        console.log(`    first shown at ${round(median(firsts), 1)} NM from the runway end in the median (quartiles ${round(pct(firsts, .25), 1)}-${round(pct(firsts, .75), 1)})`);
        console.log(`    points left out, by the limit that drops them: ${JSON.stringify(a.causes)}`);
        const dpt = cov(departures);
        const lasts = departures.map((x) => x.lastIn);
        console.log(`  Departures with a runway: ${departures.length}; their points up to 10 NM out: ${dpt.inF} of ${dpt.tot} shown (${round(100 * dpt.inF / dpt.tot, 1)}%), followed to ${round(median(lasts), 1)} NM in the median`);
        console.log(`    points left out: ${JSON.stringify(dpt.causes)}`);
        console.log(`    with the PAR's 7 degrees for departures too: ${dep7} of ${dpt.tot} shown (${round(100 * dep7 / dpt.tot, 1)}%)`);
        const tot = Object.values(shown).reduce((x, y) => x + y, 0);
        console.log(`  What is shown (${tot} points, ${displayedAircraft} aircraft): arrivals ${round(100 * shown.arrival / tot, 1)}%, departures ${round(100 * shown.departure / tot, 1)}%, inbound not yet on approach ${round(100 * shown.inbound / tot, 1)}%, other ${round(100 * shown.other / tot, 1)}%`);
        console.log(`    most shown among "other": ${[...others.entries()].sort((x, y) => y[1] - x[1]).slice(0, 6).map(([k, v]) => `${k} (${v})`).join(', ')}`);
        console.log(`  Frame changes while shown: ${switches} over ${displayedAircraft} aircraft`);
    }
    // QNH: the view's estimate against the METARs of Orly, per half hour.
    console.log('\n== QNH estimate (Orly, median per half hour)');
    const half = new Map();
    for (const [k, v] of qnhBucket.LFPO) {
        const h = k.slice(11, 13) + ':' + (Number(k[14]) < 3 ? '00' : '30');
        if (!half.has(h)) half.set(h, []);
        half.get(h).push(...v);
    }
    for (const [h, v] of [...half.entries()].sort()) console.log(`  ${h}Z  ${round(median(v), 1)} hPa (${v.length} reports)`);
}

main().catch((e) => { console.error(e); process.exit(1); });
