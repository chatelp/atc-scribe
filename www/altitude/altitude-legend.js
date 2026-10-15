// The altitude legend on the sky (AltitudeLegend, owner 15/10): which colour
// an aircraft wears on the map from which altitude, and how busy each band
// and its levels are now. The population is the Traffic slider's, aircraft
// airborne now, on their reported altitude as their glyph's colour is.
// Refreshed with the counters, not at each position.

// x-data="altitudeLegend()", a global factory like altitudeFilter().
function altitudeLegend() {
    const B = window.AltitudeBands;
    const S = window.AltitudeScale;
    const REFRESH_MS = 5000;

    return {
        bands: [],
        peak: 1,
        total: 0,
        _timer: null,

        init() {
            this.refresh();
            this._timer = setInterval(() => this.refresh(), REFRESH_MS);
        },

        destroy() {
            clearInterval(this._timer);
        },

        get cruise() { return this.$store.atc.stationCruiseAltitudeFt; },

        // "1 000 ft", "3 000 ft", "FL 125", "FL 180".
        get bounds() {
            return B.bounds(this.cruise).map((ft) => S.label(ft, S.DEFAULT_TA));
        },

        // Over a light base map, the light palette on a light chip.
        get bright() {
            return !!(window.MapPalette && window.MapPalette.isBright());
        },

        refresh() {
            const alts = [];
            for (const a of Object.values(this.$store.atc.aircraft || {})) {
                if (!a || a.on_ground || !a.adsb || a.status === 'signal_lost') continue;
                const ft = Number(a.adsb.alt_baro);
                if (Number.isFinite(ft)) alts.push(ft);
            }
            const l = B.legend(alts, this.cruise);
            this.bands = l.bands;
            this.peak = l.peak;
            this.total = alts.length;
        },
    };
}
