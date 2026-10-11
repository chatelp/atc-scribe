/**
 * The scale of the altitude filter: where each altitude sits on the slider,
 * and how it is written. Pure functions, no DOM -- loaded as a script in the
 * page (window.AltitudeScale) and as a module by the tests
 * (node --test 'www/altitude/*.test.js').
 *
 * The slider moves over a list of stops rather than over feet, so that the
 * low levels, where the traffic around an airfield is, get the room: 100 ft
 * steps up to the transition altitude, 500 ft up to FL195, then flight levels
 * by ten up to FL450, and a last stop for "no ceiling". About half the travel
 * is below the transition altitude.
 *
 * Altitudes are read the way the controller says them (reading()): an
 * aircraft reports its pressure altitude on the standard 1013 hPa (alt_baro),
 * which is its flight level; below the transition altitude it is given in
 * feet on the local QNH instead, about 27 ft higher per hPa above standard.
 * The filter compares those readings, so its feet are QNH feet too.
 */
(function (root, factory) {
    const api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;
    } else {
        root.AltitudeScale = api;
    }
})(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    // The bounds the store has always meant as "no bound".
    const FLOOR = 0;
    const NO_CEILING = 60000;
    const DEFAULT_TA = 5000;
    const STD_HPA = 1013.25;
    // Height of one hPa near sea level, as in par/par-geometry.js.
    const FT_PER_HPA = 27.3;

    /**
     * An altitude as it is said: {kind: 'ft', ft} on the QNH at or below the
     * transition altitude, {kind: 'fl', ft, fl} above it, with ft the value
     * the filter compares, or {kind: 'gnd', ft: 0} for an aircraft that reports
 * the ground. ref: {qnh, ta}; a missing QNH is the standard one.
     */
    function reading(altBaro, ref) {
        if (typeof altBaro !== 'number' || !Number.isFinite(altBaro)) return null;
        const qnh = (ref && ref.qnh) || STD_HPA;
        const ta = (ref && ref.ta) || DEFAULT_TA;
        // ADS-B sends "ground" for an aircraft on the ground, read as 0.
        if (altBaro <= 0) return { kind: 'gnd', ft: 0 };
        const onQnh = altBaro + (qnh - STD_HPA) * FT_PER_HPA;
        if (onQnh <= ta) return { kind: 'ft', ft: onQnh };
        return { kind: 'fl', ft: altBaro, fl: Math.round(altBaro / 100) };
    }

    // "2 900 ft" or "FL120"; feet to the nearest `step`.
    function readingText(r, step = 100) {
        if (!r) return '-';
        if (r.kind === 'gnd') return 'GND';
        if (r.kind === 'fl') return 'FL' + String(r.fl).padStart(3, '0');
        return `${thousands(Math.round(r.ft / step) * step)} ft`;
    }

    function stops(transitionAltitudeFt = DEFAULT_TA) {
        const ta = Math.max(1000, Math.min(18000, Math.round(transitionAltitudeFt / 100) * 100));
        const out = [];
        for (let ft = 0; ft <= ta; ft += 100) out.push(ft);
        for (let ft = Math.floor(ta / 500) * 500 + 500; ft < 20000; ft += 500) out.push(ft);
        for (let ft = 20000; ft <= 45000; ft += 1000) out.push(ft);
        out.push(NO_CEILING);
        return out;
    }

    // The stop nearest to an altitude; a value past FL450 is "no ceiling".
    function indexOf(list, ft) {
        if (!Number.isFinite(ft)) return 0;
        if (ft > 45000) return list.length - 1;
        let best = 0;
        for (let i = 1; i < list.length - 1; i++) {
            if (Math.abs(list[i] - ft) < Math.abs(list[best] - ft)) best = i;
        }
        return best;
    }

    // Where an altitude falls along the slider, 0 to 1, between the stops.
    function position(list, ft) {
        const last = list.length - 1;
        if (ft <= list[0]) return 0;
        if (ft >= list[last - 1]) return (last - 1) / last;
        let i = 1;
        while (list[i] < ft) i++;
        const frac = (ft - list[i - 1]) / (list[i] - list[i - 1]);
        return (i - 1 + frac) / last;
    }

    function thousands(n) {
        return String(Math.round(n)).replace(/\B(?=(\d{3})+(?!\d))/g, ' ');
    }

    // "2 500 ft" at or below the transition altitude, "FL120" above it.
    function label(ft, transitionAltitudeFt = DEFAULT_TA) {
        if (ft >= NO_CEILING) return 'no ceiling';
        if (ft <= 0) return 'surface';
        if (ft <= transitionAltitudeFt) return `${thousands(ft)} ft`;
        return 'FL' + String(Math.round(ft / 100)).padStart(3, '0');
    }

    function bandLabel(min, max, transitionAltitudeFt = DEFAULT_TA) {
        if (min <= FLOOR && max >= NO_CEILING) return 'all altitudes';
        if (min <= FLOOR) return `up to ${label(max, transitionAltitudeFt)}`;
        if (max >= NO_CEILING) return `${label(min, transitionAltitudeFt)} and above`;
        return `${label(min, transitionAltitudeFt)} to ${label(max, transitionAltitudeFt)}`;
    }

    // How many airborne aircraft fall in each slice of the slider, for the
    // small histogram over it: `bins` equal slices of the travel.
    function histogram(list, altitudes, bins) {
        const counts = new Array(bins).fill(0);
        for (const ft of altitudes) {
            if (!Number.isFinite(ft)) continue;
            const p = position(list, ft);
            counts[Math.min(bins - 1, Math.floor(p * bins))]++;
        }
        return counts;
    }

    // Presets: a few bands a listener reaches for, in the order of the slider.
    function presets(transitionAltitudeFt = DEFAULT_TA) {
        return [
            { id: 'all', label: 'All', min: FLOOR, max: NO_CEILING, help: 'Every altitude' },
            { id: 'circuit', label: '≤ 3000 ft', min: FLOOR, max: 3000, help: 'Circuits, light aircraft, final approaches' },
            { id: 'ta', label: `≤ TA`, min: FLOOR, max: transitionAltitudeFt, help: `Below the transition altitude (${thousands(transitionAltitudeFt)} ft): altitudes on QNH` },
            { id: 'low', label: '≤ FL100', min: FLOOR, max: 10000, help: 'Arrivals and departures, below the 250 kt limit' },
            { id: 'mid', label: 'FL100–195', min: 10000, max: 19500, help: 'Climbing and descending traffic, up to the ceiling of visual flight' },
            { id: 'high', label: '≥ FL195', min: 19500, max: NO_CEILING, help: 'Upper airspace, en route (instrument flights only)' },
        ];
    }

    return { FLOOR, NO_CEILING, DEFAULT_TA, STD_HPA, FT_PER_HPA, reading, readingText, stops, indexOf, position, label, bandLabel, histogram, presets };
});
