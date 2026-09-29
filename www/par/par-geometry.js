/**
 * PAR view geometry: where an aircraft stands relative to a runway, and which
 * runway's frame it belongs to. Pure functions, no DOM -- loaded as a script in
 * the page (window.ParGeometry) and as a module by the tests
 * (node --test 'www/par/*.test.js').
 *
 * A frame is one physical runway. Its axis runs from the low end ("le") to the
 * high end ("he"): s is the distance along that axis from the le end, in NM,
 * positive towards he, and c the distance to the right of the axis, in NM.
 * Each end has a coverage cone outside it, like the scope of a Precision
 * Approach Radar: arrivals to that end come down it, and departures from the
 * other end climb out of it. See docs-fr/31-vue-par.md.
 */
(function (root, factory) {
    const api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;
    } else {
        root.ParGeometry = api;
    }
})(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    const FT_PER_NM = 6076.12;
    // Height of one hPa near sea level in the standard atmosphere.
    const FT_PER_HPA = 27.3;
    const STD_HPA = 1013.25;

    // Every default below is a setting (docs-fr/31, "Les réglages"); these are
    // the values used until the settings panel says otherwise.
    const DEFAULTS = Object.freeze({
        rangeNM: 20,
        halfWidthDeg: 20,
        // The PAR's 7 degrees suits arrivals; departures climb steeper -- all four
        // measured from De Gaulle on 29/09 went past 7, none past 15 (V6).
        elevApproachDeg: 7,
        elevDepartDeg: 15,
        ceilingFt: 15000,
        transitionAltitudeFt: 5000,
        slopeDeg: 3,
        thresholdCrossingFt: 50,
        elevToleranceDeg: 0.7,
        azToleranceDeg: 2.5,
        // An aircraft crossing the axis at more than this angle is passing
        // through the cone, not flying the runway: at 15 NM past Orly's 20 end
        // lies the final of De Gaulle's 08R, flown at 68 degrees to Orly's axis
        // (29/09, AFR1855). Close to the runway, where aircraft turn onto final
        // or away after take-off, the angle does not count.
        maxCrossingDeg: 60,
        crossingFreeNM: 2,
        // Keep an aircraft in its frame unless another axis is this much nearer,
        // so one flying between two runways does not flicker from one to the other.
        hysteresisNM: 0.3,
    });

    const rad = (d) => d * Math.PI / 180;
    const deg = (r) => r * 180 / Math.PI;

    function angleDiff(a, b) {
        return Math.abs(((a - b) % 360 + 540) % 360 - 180);
    }

    // Flat-earth offset of (lat, lon) from (lat0, lon0), in NM east and north.
    // Within the 20 NM of a cone the error is a few metres.
    function offsetNM(lat0, lon0, lat, lon) {
        return {
            x: (lon - lon0) * 60 * Math.cos(rad(lat0)),
            y: (lat - lat0) * 60,
        };
    }

    function bearingDeg(lat0, lon0, lat, lon) {
        const o = offsetNM(lat0, lon0, lat, lon);
        return (deg(Math.atan2(o.x, o.y)) + 360) % 360;
    }

    function endFrom(r, p) {
        return {
            ident: r[p + '_ident'],
            lat: Number(r[p + '_latitude']),
            lon: Number(r[p + '_longitude']),
            elevFt: Number(r[p + '_elevation_ft']) || 0,
            displacedFt: Number(r[p + '_displaced_ft']) || 0,
        };
    }

    function located(e) {
        return e.ident && Number.isFinite(e.lat) && Number.isFinite(e.lon) && !(e.lat === 0 && e.lon === 0);
    }

    /**
     * Frames for an airport's runways, as /api/v1/airports/{code} serves them.
     * A closed runway, or one whose ends are not both located, has no frame.
     * perRunway maps "LFPO 20" style keys to {slopeDeg, thresholdCrossingFt}.
     */
    function buildFrames(airport, runways, opts, perRunway) {
        const o = Object.assign({}, DEFAULTS, opts || {});
        const frames = [];
        for (const r of runways || []) {
            if (r.closed) continue;
            const le = endFrom(r, 'le');
            const he = endFrom(r, 'he');
            if (!located(le) || !located(he)) continue;
            const b = bearingDeg(le.lat, le.lon, he.lat, he.lon);
            const d = offsetNM(le.lat, le.lon, he.lat, he.lon);
            const lengthNM = Math.hypot(d.x, d.y);
            for (const e of [le, he]) {
                const own = (perRunway || {})[airport + ' ' + e.ident] || {};
                e.slopeDeg = own.slopeDeg || o.slopeDeg;
                e.thresholdCrossingFt = own.thresholdCrossingFt != null ? own.thresholdCrossingFt : o.thresholdCrossingFt;
            }
            // Landing thresholds, where the glide path comes down: past the
            // physical end by the displaced length.
            le.thresholdS = le.displacedFt / FT_PER_NM;
            he.thresholdS = lengthNM - he.displacedFt / FT_PER_NM;
            // The direction an aircraft landing on each end flies.
            le.landingBearing = b;
            he.landingBearing = (b + 180) % 360;
            frames.push({
                id: le.ident + '-' + he.ident,
                airport,
                bearing: b,
                lengthNM,
                le,
                he,
            });
        }
        return frames;
    }

    // s along the axis from the le end (NM), c to its right (NM).
    function project(frame, lat, lon) {
        const o = offsetNM(frame.le.lat, frame.le.lon, lat, lon);
        const b = rad(frame.bearing);
        return {
            s: o.x * Math.sin(b) + o.y * Math.cos(b),
            c: o.x * Math.cos(b) - o.y * Math.sin(b),
        };
    }

    function qnhAltitude(altBaro, qnh) {
        return altBaro + ((qnh || STD_HPA) - STD_HPA) * FT_PER_HPA;
    }

    // Lowest flight level at least 1000 ft above the transition altitude
    // with this QNH (AIP France ENR 1.7). The controller gives the actual one.
    function transitionLevel(transitionAltitudeFt, qnh) {
        const shift = ((qnh || STD_HPA) - STD_HPA) * FT_PER_HPA;
        let fl = Math.ceil((transitionAltitudeFt + 1000 - shift) / 1000) * 10;
        while (fl * 100 + shift < transitionAltitudeFt + 1000) fl += 10;
        return fl;
    }

    /**
     * The QNH the crews around the airport have set: the median of nav_qnh
     * among aircraft well below the transition altitude (they fly on QNH there),
     * within radiusNM of the airport. Null when none sends it.
     */
    function estimateQnh(aircraftList, lat, lon, opts) {
        const o = Object.assign({ radiusNM: 40 }, DEFAULTS, opts || {});
        const values = [];
        for (const a of aircraftList || []) {
            const t = a && a.adsb;
            if (!t || typeof t.alt_baro !== 'number' || t.lat == null || t.nav_qnh == null) continue;
            if (a.on_ground || t.alt_baro <= 0 || t.alt_baro > o.transitionAltitudeFt - 500) continue;
            if (t.nav_qnh < 940 || t.nav_qnh > 1060) continue;
            const d = offsetNM(lat, lon, t.lat, t.lon);
            if (Math.hypot(d.x, d.y) > o.radiusNM) continue;
            values.push(t.nav_qnh);
        }
        if (values.length === 0) return null;
        values.sort((p, q) => p - q);
        const m = values.length >> 1;
        const qnh = values.length % 2 ? values[m] : (values[m - 1] + values[m]) / 2;
        return { qnh, count: values.length };
    }

    // Altitude of the glide path to this end at axis position s, or null on the
    // wrong side of the threshold.
    function glidePathFt(frame, endKey, s) {
        const e = frame[endKey];
        const d = endKey === 'le' ? e.thresholdS - s : s - e.thresholdS;
        if (d < 0) return null;
        return e.elevFt + e.thresholdCrossingFt + d * FT_PER_NM * Math.tan(rad(e.slopeDeg));
    }

    /**
     * Where an aircraft stands in one frame, or null when it is in none of its
     * cones and not on the runway. input: {lat, lon, altFt (QNH), track, onGround}.
     */
    function placeInFrame(frame, input, opts) {
        const o = Object.assign({}, DEFAULTS, opts || {});
        const { s, c } = project(frame, input.lat, input.lon);
        const L = frame.lengthNM;
        const base = { frame: frame.id, s, c, altFt: input.altFt };

        if (s >= 0 && s <= L) {
            // Over the runway itself: on it (a taxiway runs 0.1 NM away), or
            // low above its axis on the way down or up.
            const elev = frame.le.elevFt + (frame.he.elevFt - frame.le.elevFt) * (s / (L || 1));
            if (input.onGround ? Math.abs(c) <= 0.04 : (Math.abs(c) <= 0.3 && input.altFt - elev <= 3000)) {
                return Object.assign(base, { side: 'rwy', dist: 0 });
            }
            return null;
        }
        if (input.onGround) return null;

        const side = s < 0 ? 'le' : 'he';
        const end = frame[side];
        const dist = s < 0 ? -s : s - L;
        if (dist > o.rangeNM) return null;
        if (deg(Math.atan2(Math.abs(c), dist)) > o.halfWidthDeg) return null;
        if (input.altFt > o.ceilingFt) return null;

        const outward = side === 'le' ? (frame.bearing + 180) % 360 : frame.bearing;
        if (Number.isFinite(input.track) && dist > o.crossingFreeNM) {
            // Along the axis either way is 0; square across it, 90.
            const d = angleDiff(input.track, frame.bearing);
            const crossing = Math.min(d, 180 - d);
            if (crossing > o.maxCrossingDeg) return null;
        }
        const movingAway = Number.isFinite(input.track) && angleDiff(input.track, outward) < 90;
        const limit = movingAway ? o.elevDepartDeg : o.elevApproachDeg;
        const elevation = deg(Math.atan2(input.altFt - end.elevFt, Math.max(dist, 0.1) * FT_PER_NM));
        if (elevation > limit) return null;

        return Object.assign(base, {
            side,
            dist,
            movingAway,
            elevationDeg: elevation,
            azimuthDeg: deg(Math.atan2(c, dist)) * (side === 'le' ? -1 : 1),
        });
    }

    /**
     * The frame an aircraft belongs to: among those whose cone or runway holds
     * it, the one whose axis is nearest. previousFrameId keeps it where it was
     * unless another axis is clearly nearer (hysteresisNM).
     */
    function classify(frames, input, opts, previousFrameId) {
        const o = Object.assign({}, DEFAULTS, opts || {});
        let best = null;
        let previous = null;
        for (const f of frames) {
            const p = placeInFrame(f, input, o);
            if (!p) continue;
            if (f.id === previousFrameId) previous = p;
            if (!best || Math.abs(p.c) < Math.abs(best.c)) best = p;
        }
        if (previous && Math.abs(previous.c) <= Math.abs(best.c) + o.hysteresisNM) return previous;
        return best;
    }

    return {
        FT_PER_NM,
        FT_PER_HPA,
        STD_HPA,
        DEFAULTS,
        angleDiff,
        bearingDeg,
        buildFrames,
        project,
        qnhAltitude,
        transitionLevel,
        estimateQnh,
        glidePathFt,
        placeInFrame,
        classify,
    };
});
