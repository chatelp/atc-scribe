/**
 * "Then": what an aircraft did after an instruction it was given, told as a
 * fact and never as a verdict (D78, decision 6). The column of heard
 * instructions in the aircraft's story says "level at 4 000 ft, 2 min later",
 * not "complied" or "deviation". Pure functions, no DOM -- loaded as a script
 * in the page (window.StoryThen) and as a module by the tests
 * (node --test www/story/).
 *
 *     StoryThen.describe(instruction, track, options) -> { text, at, kind } or null
 *
 * (Not named `then`: a module or object with a `then` method is a thenable,
 * and `await import(...)` or an async function returning it would call it.)
 * In the page it needs AltitudeScale and ParGeometry loaded before it.
 *
 * instruction: { role, text, text_fr, value, unit, at }, one entry of a
 *   transcription's `plain` (GET /api/v1/transcriptions) with `at` the time it
 *   was said. When value is missing it is read from text (valueOf below).
 * track: points of GET /api/v1/aircraft/{hex}/tracks (`history`), as served
 *   ({ timestamp, altitude, track, mag_heading, speed_gs, lat, lon }) or with
 *   the ADS-B names (alt_baro, gs). Points before `at` are ignored.
 * options: windowMin (6), levelTolFt (200), headingTolDeg (10),
 *   speedTolKt (15), qnh and ta (the transition altitude) to read a level,
 *   transmissions ([{ at, frequency }], the aircraft's later transmissions,
 *   for a frequency change), frames (ParGeometry.buildFrames, to name the
 *   runway an aircraft landed on or took off from).
 *
 * text is English, short, and says what was observed; at is the time of the
 * observation it describes, in ms since the epoch; kind is level, heading,
 * speed, frequency, runway or clearance. null when nothing can be said: no
 * value, no point after the instruction, an instruction that asks nothing
 * (a QNH, a squawk, "passing FL100").
 *
 * Numbers follow the design system (Content > Numbers): a narrow no-break
 * space between thousands ("4 000 ft"), "FL 110", "270°", "220 kt",
 * "119.255", and a comma before the delay (", 2 min later").
 */
(function (root, factory) {
    const isNode = typeof module === 'object' && module.exports;
    const api = isNode
        ? factory(require('../altitude/altitude-scale.js'), require('../par/par-geometry.js'))
        : factory(root.AltitudeScale, root.ParGeometry);
    if (isNode) {
        module.exports = api;
    } else {
        root.StoryThen = api;
    }
})(typeof self !== 'undefined' ? self : this, function (AltitudeScale, ParGeometry) {
    'use strict';

    const DEFAULTS = Object.freeze({
        windowMin: 6,
        levelTolFt: 200,
        headingTolDeg: 10,
        speedTolKt: 15,
        // A target reached is "level at" when the aircraft stays within the
        // tolerance this long after reaching it, or until the track ends.
        holdSec: 30,
        // How far back the end of the track is compared to tell a movement.
        trendSec: 60,
        turnTrendSec: 30,
        climbFtPerMin: 300,
        turnDeg: 5,
        speedChangeKt: 5,
        // The track ends this long before the window does: the delay is said.
        endGapSec: 30,
        // Without frames, the track at touchdown or lift-off is compared to
        // the runway's number times ten.
        runwayTolDeg: 20,
        // With frames: on the runway is within this distance of its axis.
        runwayHalfWidthNM: 0.05,
    });

    const KINDS = {
        flight_level: 'level',
        altitude: 'level',
        heading: 'heading',
        speed: 'speed',
        frequency: 'frequency',
        runway: 'runway',
        clearance: 'clearance',
    };

    // ------------------------------------------------------------ reading

    function timeOf(t) {
        if (t instanceof Date) return t.getTime();
        if (typeof t === 'number') return Number.isFinite(t) ? t : null;
        if (typeof t === 'string' && t) {
            const ms = Date.parse(t);
            return Number.isFinite(ms) ? ms : null;
        }
        return null;
    }

    function num(v) {
        return typeof v === 'number' && Number.isFinite(v) ? v : null;
    }

    // A point with the names the API serves, or the ADS-B ones. altitude is
    // alt_baro (0 on the ground); the heading is the magnetic one the
    // controller gives when the aircraft sends it, else the ground track.
    function pointOf(p) {
        if (!p) return null;
        const t = timeOf(p.timestamp);
        if (t == null) return null;
        const alt = num(p.alt_baro) != null ? p.alt_baro : num(p.altitude);
        return {
            t,
            alt,
            ground: p.on_ground === true || p.alt_baro === 'ground' || (alt != null && alt <= 0),
            hdg: num(p.mag_heading) != null ? p.mag_heading : num(p.track),
            trk: num(p.track),
            gs: num(p.gs) != null ? p.gs : num(p.speed_gs),
            lat: num(p.lat),
            lon: num(p.lon),
        };
    }

    /**
     * The value of an instruction, from value/unit when the server sent them,
     * else read from its English text by role:
     *   flight_level  "FL100", "FL 100", "flight level 100"  -> { n: 100, unit: 'FL' }
     *   altitude      "4000 ft", "4 000 ft", "4,000 ft"      -> { n: 4000, unit: 'ft' }
     *   heading       "heading 270", "heading 024"           -> { n: 270, unit: 'deg' }
     *   speed         "220 kt", "220 knots"                  -> { n: 220, unit: 'kt' }
     *   frequency     118.000 to 136.999, "119.255"          -> { s: '119.255', unit: 'MHz' }
     *   runway, clearance  "runway 27R", "runway 09"         -> { s: '27R', unit: 'runway' }
     * A clearance without a runway has { s: '' }. null when nothing is read.
     */
    function valueOf(instruction) {
        const role = instruction && instruction.role;
        const text = String((instruction && instruction.text) || '');
        const given = instruction && instruction.value != null ? String(instruction.value).trim() : '';
        const unit = instruction && instruction.unit;
        let m;
        switch (role) {
            case 'flight_level':
            case 'altitude': {
                if (given) {
                    const n = Number(given.replace(/^FL\s*/i, '').replace(/[\s, ]/g, ''));
                    if (!Number.isFinite(n)) return null;
                    const u = unit === 'ft' || unit === 'FL' ? unit : (role === 'altitude' ? 'ft' : 'FL');
                    return { n, unit: u };
                }
                if ((m = text.match(/\bFL\s?(\d{2,3})\b/i) || text.match(/\bflight level (\d{2,3})\b/i))) {
                    return { n: Number(m[1]), unit: 'FL' };
                }
                if ((m = text.match(/\b(\d{1,2}(?:[ , ]?\d{3})|\d{3,5})\s?(?:ft|feet)\b/i))) {
                    return { n: Number(m[1].replace(/[ , ]/g, '')), unit: 'ft' };
                }
                return null;
            }
            case 'heading': {
                const s = given || ((m = text.match(/\bheading\s+(\d{1,3})\b/i)) && m[1]);
                const n = Number(s);
                return s && Number.isFinite(n) && n >= 0 && n <= 360 ? { n: n % 360, unit: 'deg' } : null;
            }
            case 'speed': {
                const s = given || ((m = text.match(/\b(\d{2,3})\s?(?:kt|knots)\b/i)) && m[1]);
                const n = Number(s);
                return s && Number.isFinite(n) && n > 0 ? { n, unit: 'kt' } : null;
            }
            case 'frequency': {
                const s = given || ((m = text.match(/\b(1(?:1[89]|2\d|3[0-6])\.\d{1,3})\b/)) && m[1]);
                return s && Number.isFinite(Number(s)) ? { s: Number(s).toFixed(3), unit: 'MHz' } : null;
            }
            case 'runway':
            case 'clearance': {
                const s = given || ((m = text.match(/\brunway\s+(\d{1,2}[LRC]?)\b/i)) && m[1]) || '';
                if (role === 'runway' && !s) return null;
                return { s: s.toUpperCase(), unit: 'runway' };
            }
            default:
                return null;
        }
    }

    // ------------------------------------------------------------ writing

    // "4 000 ft", "FL 110" or "GND", the way the controller says a level:
    // in feet on the QNH up to the transition altitude, a flight level above.
    function levelText(alt, o) {
        return AltitudeScale.readingText(AltitudeScale.reading(alt, { qnh: o.qnh, ta: o.ta }));
    }

    function headingText(h) {
        return String(Math.round(h) % 360 || 360).padStart(3, '0') + '°';
    }

    // ", 2 min later", ", 40 s later": the delay since the instruction.
    function later(ms) {
        const sec = Math.max(0, ms / 1000);
        if (sec < 55) return `, ${Math.max(10, Math.round(sec / 10) * 10)} s later`;
        return `, ${Math.round(sec / 60)} min later`;
    }

    // ------------------------------------------------------------ helpers

    // The points with a value, from the instruction to the end of the window.
    function within(points, key) {
        return points.filter((p) => p[key] != null);
    }

    // The point to compare the last one with: the one nearest to `sec`
    // before it, at least a third of that span back (the API leaves out
    // repeated positions, so points can be far apart). null when the track
    // is too short to tell a movement.
    function before(list, sec) {
        const last = list[list.length - 1];
        let best = null;
        for (const p of list) {
            const back = last.t - p.t;
            if (back < (sec * 1000) / 3) continue;
            if (!best || Math.abs(back - sec * 1000) < Math.abs(last.t - best.t - sec * 1000)) best = p;
        }
        return best;
    }

    // The first point within `near` of the target, the points of the
    // holdSec after it, and whether the aircraft stayed there: every one of
    // them near, and spread over no more than the tolerance (an aircraft
    // turning or climbing slowly through the tolerance is not there).
    function reach(list, near, holdSec, dist, tol) {
        const i = list.findIndex(near);
        if (i < 0) return null;
        const hold = list.slice(i).filter((p) => p.t - list[i].t <= holdSec * 1000);
        const d = hold.map(dist);
        const held = hold.every(near) && Math.max(...d) - Math.min(...d) <= tol;
        return { i, p: list[i], hold, held };
    }

    // Where the aircraft went through the target, from the point before the
    // first near one: the time and value, interpolated between the two points
    // either side. dist is signed, value minus target. null if it did not.
    function crossing(list, i, dist) {
        for (let k = Math.max(1, i); k < list.length; k++) {
            const a = list[k - 1];
            const b = list[k];
            const da = dist(a);
            const db = dist(b);
            if (da === db || da * db > 0) continue;
            const f = da / (da - db);
            return { t: a.t + (b.t - a.t) * f, f, a, b };
        }
        return null;
    }

    function median(xs) {
        const s = [...xs].sort((a, b) => a - b);
        const k = s.length >> 1;
        return s.length % 2 ? s[k] : (s[k - 1] + s[k]) / 2;
    }

    // ------------------------------------------------------------ by kind

    function level(target, list, ctx) {
        const o = ctx.o;
        const pts = within(list, 'alt');
        if (!pts.length) return null;
        // The aircraft's level in the target's reference: a flight level on
        // the standard 1013 hPa, feet on the QNH (the standard one if none).
        const goal = target.unit === 'FL' ? target.n * 100 : target.n;
        const ref = (p) => (target.unit === 'FL' ? p.alt : AltitudeScale.reading(p.alt, { qnh: o.qnh, ta: Infinity }).ft);
        const last = pts[pts.length - 1];
        const dist = (p) => ref(p) - goal;
        const near = (p) => !p.ground && Math.abs(dist(p)) <= o.levelTolFt;
        const r = reach(pts, near, o.holdSec, dist, o.levelTolFt);
        if (r && r.p === pts[0] && r.p.t - ctx.at <= o.holdSec * 1000 && r.held) {
            // There from the start: the same fact, without a wait, up to the
            // last point it was still there.
            let k = 0;
            while (k + 1 < pts.length && near(pts[k + 1])) k++;
            const run = pts.slice(0, k + 1);
            return { text: `at ${levelText(median(run.map((p) => p.alt)), o)}${later(pts[k].t - ctx.at)}`, at: pts[k].t };
        }
        if (r && r.held) {
            return { text: `level at ${levelText(median(r.hold.map((p) => p.alt)), o)}${later(r.p.t - ctx.at)}`, at: r.p.t };
        }
        if (r) {
            const x = crossing(pts, r.i, dist);
            const a = x ? x.a : r.p;
            const b = x ? x.b : pts[r.i + 1] || r.p;
            const alt = x ? a.alt + (b.alt - a.alt) * x.f : r.p.alt;
            const t = x ? x.t : r.p.t;
            const way = b.alt >= a.alt ? 'climbing' : 'descending';
            return { text: `${way} through ${levelText(alt, o)}${later(t - ctx.at)}`, at: Math.round(t) };
        }
        if (last.ground) return { text: `on the ground${later(last.t - ctx.at)}`, at: last.t };
        const prev = before(pts, o.trendSec);
        const rate = prev ? ((last.alt - prev.alt) / (last.t - prev.t)) * 60000 : 0;
        if (Math.abs(rate) >= o.climbFtPerMin) {
            const way = rate > 0 ? 'climbing' : 'descending';
            return { text: `${way} through ${levelText(last.alt, o)}${ctx.endDelay(last)}`, at: last.t };
        }
        return { text: `at ${levelText(last.alt, o)}${later(last.t - ctx.at)}`, at: last.t };
    }

    function heading(target, list, ctx) {
        const o = ctx.o;
        const pts = within(list, 'hdg').filter((p) => !p.ground);
        if (!pts.length) return null;
        // Signed, around the target, so that 359° and 001° read as one.
        const dist = (p) => ((p.hdg - target.n + 540) % 360) - 180;
        const near = (p) => Math.abs(dist(p)) <= o.headingTolDeg;
        const last = pts[pts.length - 1];
        const r = reach(pts, near, o.holdSec, dist, o.headingTolDeg);
        if (r && r.held) {
            // The heading flown: the median of the points that held it.
            const h = target.n + median(r.hold.map(dist));
            return { text: `turned to ${headingText((h + 360) % 360)}`, at: r.p.t };
        }
        if (r) {
            const x = crossing(pts, r.i, dist);
            const h = x ? target.n : r.p.hdg;
            const t = x ? x.t : r.p.t;
            return { text: `turning through ${headingText((h + 360) % 360)}${later(t - ctx.at)}`, at: Math.round(t) };
        }
        const prev = before(pts, o.turnTrendSec);
        if (prev && ParGeometry.angleDiff(last.hdg, prev.hdg) >= o.turnDeg) {
            return { text: `heading ${headingText(last.hdg)}, still turning`, at: last.t };
        }
        return { text: `heading ${headingText(last.hdg)}${later(last.t - ctx.at)}`, at: last.t };
    }

    function speed(target, list, ctx) {
        const o = ctx.o;
        const pts = within(list, 'gs');
        if (!pts.length) return null;
        const dist = (p) => p.gs - target.n;
        const near = (p) => Math.abs(dist(p)) <= o.speedTolKt;
        const kt = (v) => `${Math.round(v)} kt`;
        const last = pts[pts.length - 1];
        const r = reach(pts, near, o.holdSec, dist, o.speedTolKt);
        if (r && r.held) return { text: `${kt(median(r.hold.map((p) => p.gs)))}${later(r.p.t - ctx.at)}`, at: r.p.t };
        if (r) {
            const x = crossing(pts, r.i, dist);
            const a = x ? x.a : r.p;
            const b = x ? x.b : pts[r.i + 1] || r.p;
            const t = x ? x.t : r.p.t;
            const way = b.gs >= a.gs ? 'accelerating' : 'slowing';
            return { text: `${way} through ${kt(x ? target.n : r.p.gs)}${later(t - ctx.at)}`, at: Math.round(t) };
        }
        const prev = before(pts, o.trendSec);
        const delta = prev ? last.gs - prev.gs : 0;
        if (Math.abs(delta) >= o.speedChangeKt) {
            return { text: `${delta < 0 ? 'slowing' : 'accelerating'}, ${kt(last.gs)}${ctx.endDelay(last)}`, at: last.t };
        }
        return { text: `${kt(last.gs)}${later(last.t - ctx.at)}`, at: last.t };
    }

    function frequency(target, ctx) {
        const list = ctx.o.transmissions;
        if (!Array.isArray(list)) return null;
        const next = list
            .map((x) => ({ t: timeOf(x && x.at), f: x && x.frequency }))
            .filter((x) => x.t != null && x.t > ctx.at)
            .sort((a, b) => a.t - b.t)[0];
        if (!next) return { text: 'no later transmission heard here', at: null };
        const f = Number(next.f);
        if (!Number.isFinite(f)) return { text: `next heard${later(next.t - ctx.at)}`, at: next.t };
        return { text: `next heard on ${f.toFixed(3)}${later(next.t - ctx.at)}`, at: next.t };
    }

    // Landed: a point on the ground after an airborne one. Took off: an
    // airborne point after one on the ground. The runway is named from the
    // frames when one has the ground point on its axis, the end being the
    // one whose direction the aircraft flew; without frames, when the track
    // at that moment is within runwayTolDeg of the instruction's runway
    // number times ten, by that number alone (a parallel runway's side cannot
    // be told from a heading).
    function runway(target, role, text, list, ctx) {
        const o = ctx.o;
        const pts = list.filter((p) => p.alt != null || p.ground);
        const wantLand = role === 'runway' || !/takeoff|take off/i.test(text);
        const wantTakeoff = role === 'runway' || /takeoff|take off/i.test(text);
        for (let i = 1; i < pts.length; i++) {
            const a = pts[i - 1];
            const b = pts[i];
            let event = null;
            if (wantLand && !a.ground && b.ground) event = { verb: 'landed on', onGround: b, dir: a.trk != null ? a.trk : b.trk, at: b.t };
            if (wantTakeoff && a.ground && !b.ground) event = { verb: 'took off from', onGround: a, dir: b.trk != null ? b.trk : a.trk, at: b.t };
            if (!event) continue;
            const name = runwayName(event, target.s, o);
            return name ? { text: `${event.verb} ${name}`, at: event.at } : null;
        }
        return null;
    }

    function runwayName(event, said, o) {
        const g = event.onGround;
        if (Array.isArray(o.frames) && g.lat != null && g.lon != null && event.dir != null) {
            for (const f of o.frames) {
                const { s, c } = ParGeometry.project(f, g.lat, g.lon);
                if (s < -o.runwayHalfWidthNM || s > f.lengthNM + o.runwayHalfWidthNM || Math.abs(c) > o.runwayHalfWidthNM) continue;
                for (const end of [f.le, f.he]) {
                    if (ParGeometry.angleDiff(event.dir, end.landingBearing) <= o.runwayTolDeg) return end.ident;
                }
            }
        }
        const m = String(said || '').match(/^(\d{1,2})/);
        if (!m || event.dir == null) return null;
        const n = Number(m[1]);
        return ParGeometry.angleDiff(event.dir, n * 10) <= o.runwayTolDeg ? String(n).padStart(2, '0') : null;
    }

    // ------------------------------------------------------------ then

    function describe(instruction, track, options) {
        if (!instruction) return null;
        const kind = KINDS[instruction.role];
        if (!kind) return null;
        const text = String(instruction.text || '');
        // A report of where the aircraft is, not an instruction to go there.
        if (/^\s*(passing|leaving)\b/i.test(text)) return null;
        const at = timeOf(instruction.at);
        if (at == null) return null;
        const target = valueOf(instruction);
        if (!target) return null;
        const o = Object.assign({}, DEFAULTS, options || {});
        const end = at + o.windowMin * 60000;
        const ctx = {
            o,
            at,
            // The delay is said when the track ends before the window does:
            // the aircraft was lost, or it is now.
            endDelay: (p) => (end - p.t > o.endGapSec * 1000 ? later(p.t - at) : ''),
        };
        if (kind === 'frequency') return withKind(frequency(target, ctx), kind);
        const list = (Array.isArray(track) ? track : [])
            .map(pointOf)
            .filter((p) => p && p.t >= at && p.t <= end)
            .sort((a, b) => a.t - b.t);
        if (!list.length) return null;
        let out = null;
        if (kind === 'level') out = level(target, list, ctx);
        else if (kind === 'heading') out = heading(target, list, ctx);
        else if (kind === 'speed') out = speed(target, list, ctx);
        else out = runway(target, instruction.role, text, list, ctx);
        return withKind(out, kind);
    }

    function withKind(out, kind) {
        return out ? { text: out.text, at: out.at, kind } : null;
    }

    return { describe, valueOf, DEFAULTS };
});
