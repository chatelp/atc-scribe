/**
 * The arrival sequence, computed outside the PAR view (13/10, D77): which
 * aircraft are placed in each runway frame of a followed airport, and who
 * lands next on each end in use. The PAR view draws from the same placement;
 * the store runs it for every followed airport, PAR open or not, so that a
 * "right now" panel can show the sequence and the next touchdown.
 *
 * Pure functions over the store's data and ParGeometry: testable in node.
 */
(function (root, factory) {
    const api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;
    } else {
        root.ParArrivals = api;
    }
})(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    const DEFAULTS = Object.freeze({
        inUseMinProbability: 0.15, // a runway end below this share is not "in use"
        staleSeconds: 60,          // an aircraft not seen for longer is left out
        tightNM: 3,                // under this gap on the same final, amber (ICAO Doc 4444)
    });

    function endIdent(score) {
        return String((score && score.runway_end) || '').split('/').pop();
    }

    // The aircraft the page shows: positioned, fresh, and passing the map's
    // filters (the PAR applies them too; the sequence follows the same rule).
    function visibleAircraft(store, rules, staleSeconds) {
        const now = Date.now();
        const lastSeenCutoff = store.settings && store.settings.lastSeenMinutes
            ? new Date(now - store.settings.lastSeenMinutes * 60000) : null;
        const out = [];
        for (const a of Object.values(store.aircraft || {})) {
            const t = a && a.adsb;
            if (!t || t.lat == null || t.lon == null) continue;
            if (a.last_seen && now - new Date(a.last_seen).getTime() > staleSeconds * 1000) continue;
            if (rules && !rules.shouldShowAircraftOnMap(a, store, { searchTerm: store.searchTerm, lastSeenCutoff })) continue;
            out.push(a);
        }
        return out;
    }

    // place puts the airport's visible aircraft in its runway frames. state
    // holds the hysteresis (hex -> frame id) between calls; the caller keeps
    // one per airport. Returns null when the airport has no usable runway.
    function place(G, store, apt, runways, state, options) {
        const o = Object.assign({}, DEFAULTS, options || {});
        const opts = G.DEFAULTS;
        const previousFrame = state.previousFrame || (state.previousFrame = new Map());
        const frames = G.buildFrames(apt.code, runways, opts);
        if (!frames.length) return null;
        const byIdent = {};
        for (const f of frames) { byIdent[f.le.ident] = f.le; byIdent[f.he.ident] = f.he; }
        const inUse = (apt.runway_in_use || [])
            .map((r) => Object.assign({}, r, { end: endIdent(r) }))
            .filter((r) => byIdent[r.end])
            .map((r) => Object.assign(r, { bearing: byIdent[r.end].landingBearing }))
            .filter((r) => r.probability >= o.inUseMinProbability);

        const list = o.list || visibleAircraft(store, o.rules, o.staleSeconds);
        const est = G.estimateQnh(Object.values(store.aircraft || {}), apt.latitude, apt.longitude, opts);
        const qnh = est ? est.qnh : G.STD_HPA;
        const tl = G.transitionLevel(opts.transitionAltitudeFt, qnh);

        const placed = new Map(frames.map((f) => [f.id, []]));
        for (const a of list) {
            const t = a.adsb;
            // co-atc ties each aircraft to one followed airport (D62): one it
            // gives to another airport is not this one's, whatever its place
            // in the cones -- De Gaulle's finals lie in Orly's northern one.
            const ph = a.phase && a.phase.current && a.phase.current[0];
            if (ph && ph.airport && ph.airport !== apt.code) {
                previousFrame.delete(a.hex);
                continue;
            }
            // The server's runway judge (D69) has the last word: an aircraft it
            // puts on another airport's runway is not this one's.
            const rw = a.runway || {};
            const arrival = rw.arrival && rw.arrival.airport === apt.code ? rw.arrival.runway : null;
            const departure = rw.departure && rw.departure.airport === apt.code ? rw.departure.runway : null;
            if (!arrival && !departure && (rw.arrival || rw.departure)) {
                previousFrame.delete(a.hex);
                continue;
            }
            const onGround = !!a.on_ground;
            if (typeof t.alt_baro !== 'number' && !onGround) continue;
            const altFt = onGround ? null : G.qnhAltitude(t.alt_baro, qnh);
            const input = { lat: t.lat, lon: t.lon, altFt, track: t.track, onGround };
            let p = G.classify(frames, input, opts, previousFrame.get(a.hex));
            // An aircraft judged on a runway goes to that runway's frame while
            // its cone holds it, even as it crosses the neighbouring axis on
            // its way in: arriving on the approach side, departing past the
            // far end.
            const judged = (ident, departing) => {
                const f = ident && frames.find((q) => q.le.ident === ident || q.he.ident === ident);
                if (!f) return null;
                const key = f.le.ident === ident ? 'le' : 'he';
                const q = G.placeInFrame(f, input, opts);
                if (!q) return null;
                const ok = departing
                    ? q.side === (key === 'le' ? 'he' : 'le') && q.movingAway
                    : (q.side === key && !q.movingAway) || q.side === 'rwy';
                return ok ? Object.assign(q, { judged: departing ? 'departure' : 'arrival', runway: ident }) : null;
            };
            p = judged(arrival, false) || judged(departure, true) || p;
            if (!p) { previousFrame.delete(a.hex); continue; }
            previousFrame.set(a.hex, p.frame);
            placed.get(p.frame).push({ a, p, altFt });
        }
        return { opts, frames, byIdent, inUse, list, est, qnh, tl, placed };
    }

    // sequences lists, for each runway end, who comes in to it: every end in
    // use (even with nobody), and any other end with aircraft on its final.
    function sequences(G, placement, options) {
        const o = Object.assign({}, DEFAULTS, options || {});
        if (!placement) return [];
        const now = Date.now();
        const out = [];
        for (const f of placement.frames) {
            const items = placement.placed.get(f.id) || [];
            for (const endKey of ['le', 'he']) {
                const ident = f[endKey].ident;
                const inUse = placement.inUse.some((r) => r.end === ident);
                const seq = G.arrivalSequence(f, endKey, items);
                if (!seq.length && !inUse) continue;
                out.push({
                    frame: f.id, endKey, runway: ident, inUse,
                    items: seq.map((x) => ({
                        hex: x.a.hex,
                        callsign: ((x.a.flight || '').trim()) || String(x.a.hex || '').toUpperCase(),
                        type: (x.a.adsb && x.a.adsb.t) || (x.a.bsdb && x.a.bsdb.icao_type_code) || '',
                        distNM: x.distNM, gsKt: x.gsKt, etaSec: x.etaSec,
                        eta: x.etaSec != null ? new Date(now + x.etaSec * 1000) : null,
                        gapNM: x.gapNM, gapSec: x.gapSec,
                        tight: x.gapNM != null && x.gapNM < o.tightNM,
                    })),
                });
            }
        }
        // Ends in use first, then by runway name: a stable order for a panel.
        out.sort((p, q) => (q.inUse - p.inUse) || p.runway.localeCompare(q.runway));
        return out;
    }

    // nextTouchdown is the earliest estimated touchdown among the sequences,
    // or null: what "next landing in 2 min" reads.
    function nextTouchdown(seqs) {
        let best = null;
        for (const s of seqs || []) {
            for (const it of s.items) {
                if (it.etaSec == null) continue;
                if (!best || it.etaSec < best.etaSec) best = Object.assign({ runway: s.runway }, it);
            }
        }
        return best;
    }

    return { DEFAULTS, endIdent, visibleAircraft, place, sequences, nextTouchdown };
});
