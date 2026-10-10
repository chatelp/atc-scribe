// The altitude filter of the aircraft list and the map: a floor and a ceiling
// on one slider, in feet below the transition altitude and in flight levels
// above, with the traffic of the moment drawn over it. It filters as the
// handles move. The scale is in altitude-scale.js.
//
// It writes the same two settings the two number boxes it replaces wrote
// (settings.minAltitude and settings.maxAltitude, 0 and 60 000 meaning no
// bound), which the list (app.js) and the map (map/core/visibility-rules.js)
// already read. Aircraft on the ground are not concerned: the Ground button
// shows or hides them.

// x-data="altitudeFilter()", a global factory like serverSettings() in app.js.
function altitudeFilter() {
    const S = window.AltitudeScale;
    const BINS = 48;
    const REFRESH_MS = 2000;

    function transitionAltitude() {
        const geo = window.ParGeometry;
        return (geo && geo.DEFAULTS && geo.DEFAULTS.transitionAltitudeFt) || S.DEFAULT_TA;
    }

    return {
        ta: 5000,
        list: [],
        lo: 0,
        hi: 0,
        counts: new Array(BINS).fill(0),
        airborne: 0,
        inBand: 0,
        _frame: null,
        _timer: null,

        init() {
            this.ta = transitionAltitude();
            this.list = S.stops(this.ta);
            const st = this.$store.atc.settings;
            this.lo = S.indexOf(this.list, st.minAltitude);
            this.hi = Math.max(this.lo + 1, S.indexOf(this.list, st.maxAltitude));
            this.refresh();
            this._timer = setInterval(() => this.refresh(), REFRESH_MS);
        },

        destroy() {
            clearInterval(this._timer);
        },

        get last() { return this.list.length - 1; },
        get min() { return this.list[this.lo]; },
        get max() { return this.list[this.hi]; },
        get loPct() { return (this.lo / this.last) * 100; },
        get hiPct() { return (this.hi / this.last) * 100; },
        get taPct() { return S.position(this.list, this.ta) * 100; },
        get active() { return this.min > S.FLOOR || this.max < S.NO_CEILING; },
        get band() { return S.bandLabel(this.min, this.max, this.ta); },
        get presets() { return S.presets(this.ta); },
        get peak() { return Math.max(1, ...this.counts); },

        // Scale marks under the track.
        get marks() {
            return [
                { at: 0, text: 'SFC' },
                { at: this.taPct, text: 'TA' },
                { at: S.position(this.list, 10000) * 100, text: 'FL100' },
                { at: S.position(this.list, 20000) * 100, text: 'FL200' },
                { at: S.position(this.list, 30000) * 100, text: 'FL300' },
                { at: 100, text: '∞' },
            ];
        },

        label(ft) { return S.label(ft, this.ta); },

        binInBand(i) {
            const centre = ((i + 0.5) / BINS) * this.last;
            return centre >= this.lo && centre <= this.hi;
        },

        altitudes() {
            const out = [];
            for (const a of Object.values(this.$store.atc.aircraft || {})) {
                if (!a || a.on_ground || !a.adsb || a.status === 'signal_lost') continue;
                const ft = Number(a.adsb.alt_baro);
                if (Number.isFinite(ft)) out.push(ft);
            }
            return out;
        },

        refresh() {
            const alts = this.altitudes();
            this.counts = S.histogram(this.list, alts, BINS);
            this.airborne = alts.length;
            this.inBand = alts.filter((ft) => ft >= this.min && ft <= this.max).length;
        },

        // While a handle moves: the two handles never cross, and the list
        // and the map follow at most once a frame.
        moveLo(value) {
            this.lo = Math.min(Number(value), this.hi - 1);
            this.$refs.lo.value = this.lo;
            this.apply();
        },

        moveHi(value) {
            this.hi = Math.max(Number(value), this.lo + 1);
            this.$refs.hi.value = this.hi;
            this.apply();
        },

        choose(p) {
            this.lo = S.indexOf(this.list, p.min);
            this.hi = Math.max(this.lo + 1, S.indexOf(this.list, p.max));
            this.apply();
            this.save();
        },

        isPreset(p) {
            return this.min === this.list[S.indexOf(this.list, p.min)] &&
                   this.max === this.list[S.indexOf(this.list, p.max)];
        },

        apply() {
            const atc = this.$store.atc;
            atc.settings.minAltitude = this.min;
            atc.settings.maxAltitude = this.max;
            this.inBand = this.altitudes().filter((ft) => ft >= this.min && ft <= this.max).length;
            if (this._frame) return;
            this._frame = requestAnimationFrame(() => {
                this._frame = null;
                atc.applyFilters();
                if (atc.mapManager) atc.mapManager.applyFiltersAndRefreshView();
            });
        },

        // When a handle is let go: the band is kept by this browser.
        save() {
            this.$store.atc.saveSettings();
        },
    };
}
