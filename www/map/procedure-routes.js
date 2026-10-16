/**
 * The published routes of the followed airports' departures and arrivals on
 * the map (ProcedureRoute, Q56): which route a chip asks for, where it
 * starts, what the sky's note and the help say, the constraints' words, and
 * which routes the layer draws for the runways in use. Pure functions, no
 * DOM -- loaded as a script in the page (window.ProcedureRoutes) and as a
 * module by the tests. The routes come from GET /api/v1/procedure-routes
 * (internal/aip/routes.go), whole ones alone.
 */
(function (root, factory) {
    const api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;
    } else {
        root.ProcedureRoutes = api;
    }
})(typeof self !== 'undefined' ? self : this, function () {
    // A runway end below this share of the evidence is not in use (the same
    // floor as the PAR's arrivals, par/par-arrivals.js).
    const IN_USE_MIN = 0.15;
    const NNBSP = ' ';

    // The runway ends in use at an airport, from /station's runway_in_use
    // ("08R-26L/26L" -> "26L").
    function inUse(airport) {
        const list = (airport && airport.runway_in_use) || [];
        return list
            .filter((r) => (r.probability ?? 0) >= IN_USE_MIN)
            .map((r) => String(r.runway_end || '').split('/').pop())
            .filter(Boolean);
    }

    function heading(end) {
        const n = parseInt(String(end), 10);
        return Number.isFinite(n) ? n * 10 : null;
    }

    function angle(a, b) {
        const d = Math.abs(a - b) % 360;
        return d > 180 ? 360 - d : d;
    }

    // East, west, north or south, from a runway end's number.
    function direction(end) {
        const h = heading(end);
        if (h === null) return '';
        if (h >= 45 && h < 135) return 'east';
        if (h >= 135 && h < 225) return 'south';
        if (h >= 225 && h < 315) return 'west';
        return 'north';
    }

    // Whether a route serves the configuration of the runways in use: one of
    // its runways points the way one in use does (26L in use: 26L, 26R, 27L,
    // 27R).
    function servesConfiguration(route, ends) {
        return (route.runways || []).some((rw) => ends.some((e) => angle(heading(rw), heading(e)) <= 30));
    }

    // The runways a route is drawn from or to: the one of its runways in use
    // when the runways in use say which, all of them otherwise.
    function runwaysFor(route, ends) {
        const rws = route.runways || [];
        const used = rws.filter((rw) => ends.includes(rw));
        return used.length === 1 ? used : rws;
    }

    function orList(xs) {
        if (xs.length <= 1) return xs.join('');
        return xs.slice(0, -1).join(', ') + ' or ' + xs[xs.length - 1];
    }

    // "ELCOB 7G · departure from 26R · published route".
    function note(route, runways) {
        const way = route.kind === 'SID' ? 'departure from ' : 'arrival to ';
        return `${route.name} · ${way}${orList(runways)} · published route`;
    }

    // What the route is, for the note's and the layer's help.
    function help(route, runways) {
        const what = route.kind === 'SID' ? `a departure from ${orList(runways)}` : `an arrival to ${orList(runways)}`;
        return `The published route of ${route.name}, ${what}. The controller often clears an aircraft direct to a later point: ` +
            'the route is what was published, not the path flown.';
    }

    const LAYER_HELP = 'Published routes: the controller often clears an aircraft direct to a later point, so a route is not the path flown.';
    const OUTSIDE = 'Not for this station: published routes come from the French eAIP (aerodromes LF…).';
    const UNKNOWN_RUNWAY = 'Shown once the runway in use is known.';

    // Whether the French eAIP covers an airport (internal/aip, Covered).
    function covered(code) {
        return /^LF[A-Z]{2}$/.test(String(code || '').toUpperCase());
    }

    // The route a chip names: its designation, at a followed airport (the
    // one whose runways in use it serves first).
    function find(routes, designator, airports) {
        const same = (routes || []).filter((r) => r.name === designator);
        if (same.length <= 1) return same[0] || null;
        for (const a of airports || []) {
            const ends = inUse(a);
            const hit = same.find((r) => r.airport === a.code && servesConfiguration(r, ends));
            if (hit) return hit;
        }
        return same[0];
    }

    function feet(n) {
        return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, NNBSP) + ' ft';
    }

    function altText(v) {
        if (!v) return '';
        if (/^FL\d{3}$/.test(v)) return 'FL ' + v.slice(2);
        const n = parseInt(v, 10);
        return Number.isFinite(n) ? feet(n) : '';
    }

    // A point's constraints, in the brand's numbers: "≥ FL 060", "≤ 4 000 ft",
    // "≤ 250 kt", "FL 070" when the altitude is exact.
    function constraints(leg) {
        const out = [];
        const min = altText(leg.min_alt), max = altText(leg.max_alt);
        if (min && max && min === max) out.push(min);
        else {
            if (min) out.push('≥ ' + min);
            if (max) out.push('≤ ' + max);
        }
        if (leg.max_kt) out.push('≤ ' + leg.max_kt + ' kt');
        return out;
    }

    // The points of a route in the order flown, the runways' thresholds
    // first for a departure: [{ lat, lon, name, named, constraints }] per
    // drawn line (a departure from two runways draws two starts that meet
    // at the first point).
    function lines(route, runways) {
        const legs = (route.legs || []).map((l) => ({
            lat: l.lat, lon: l.lon, name: l.point, named: !!l.named, constraints: constraints(l),
        }));
        if (route.kind !== 'SID') return [legs];
        const starts = (route.thresholds || []).filter((t) => runways.includes(t.runway));
        if (!starts.length) return [legs];
        return starts.map((t, i) => [{ lat: t.lat, lon: t.lon, name: t.runway, runway: true, constraints: [] }]
            .concat(i === 0 ? legs : legs.slice(0, 1)));
    }

    // What the layer draws for one followed airport: its routes of a kind
    // serving the runways in use, and the row's words. state: 'outside',
    // 'unknown' (no runway in use yet) or 'shown'.
    function layer(routes, airports, kind) {
        const shown = (airports || []).filter((a) => covered(a.code));
        if (!shown.length) return { state: 'outside', text: OUTSIDE, routes: [] };
        const out = [];
        const said = [];
        for (const a of shown) {
            const ends = inUse(a);
            if (!ends.length) continue;
            const rs = (routes || []).filter((r) => r.airport === a.code && r.kind === kind && servesConfiguration(r, ends));
            if (!rs.length) continue;
            out.push(...rs);
            said.push(`${a.code} ${direction(ends[0])} (${ends.join(', ')})`);
        }
        if (!said.length) return { state: 'unknown', text: UNKNOWN_RUNWAY, routes: [] };
        return { state: 'shown', text: said.join(' · ') + ', from the runways in use', routes: out };
    }

    // The side of a point its name goes, away from the route (a name is never
    // crossed by it): the unit vector, in screen pixels (y down), opposite
    // the bisector of the two segments meeting there.
    function labelSide(prev, at, next) {
        const unit = (x, y) => { const n = Math.hypot(x, y) || 1; return [x / n, y / n]; };
        const a = prev ? unit(prev[0] - at[0], prev[1] - at[1]) : null;
        const b = next ? unit(next[0] - at[0], next[1] - at[1]) : null;
        if (a && b) {
            const s = unit(a[0] + b[0], a[1] + b[1]);
            // A straight line: the bisector is undefined; go to its left.
            if (Math.hypot(a[0] + b[0], a[1] + b[1]) < 0.2) return unit(-b[1], b[0]);
            return [-s[0], -s[1]];
        }
        const d = a || b;
        if (!d) return [1, 0];
        // An end: across the line, on its left.
        return unit(-d[1], d[0]);
    }

    return {
        inUse, direction, servesConfiguration, runwaysFor, note, help, find, constraints, lines, layer, labelSide, covered,
        LAYER_HELP, OUTSIDE, UNKNOWN_RUNWAY,
    };
});
