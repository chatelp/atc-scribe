// The event log: what happened worth a look, kept in a list the owner opens
// when they want, instead of the upstream alerts bar that showed every phase
// change across the top of the map (audit 2.2, docs-fr/32-audit-produit.md).
//
// Events are few on purpose: an emergency squawk, a go-around, a change of
// runway in use, an aircraft of interest, the station switching channels, a
// stream lost or back. Everything else is on the map and in the lists already.
//
// Most events come from a scan of the aircraft the page holds; the station and
// stream events are reported by app.js through Alpine.store('events').add().

(function () {
    const MAX_EVENTS = 100;
    const SCAN_MS = 2000;
    // A runway in use is reported once it has held this long, so a lone
    // aircraft landing against the flow does not log two changes.
    const RUNWAY_HOLD_MS = 120000;

    const SQUAWKS = { '7500': 'hijack', '7600': 'radio failure', '7700': 'emergency' };

    // Types worth looking out of the window for: the very large and the rare.
    const INTEREST_TYPES = new Set(['A388', 'A3ST', 'A337', 'A124', 'A225', 'B748', 'B744', 'A400', 'C17']);

    const KINDS = {
        emergency: { label: 'Emergency squawk', icon: 'fa-triangle-exclamation', color: 'text-red-400' },
        go_around: { label: 'Go-around', icon: 'fa-rotate-left', color: 'text-amber-300' },
        runway: { label: 'Runway in use', icon: 'fa-road', color: 'text-sky-300' },
        interest: { label: 'Aircraft of interest', icon: 'fa-star', color: 'text-violet-300' },
        station: { label: 'Station channels', icon: 'fa-tower-broadcast', color: 'text-emerald-300' },
        stream: { label: 'Audio stream', icon: 'fa-plug-circle-exclamation', color: 'text-orange-300' },
    };
    const DEFAULT_SOUNDS = { emergency: true };

    function loadSounds() {
        try {
            const kept = JSON.parse(localStorage.getItem('eventSounds') || 'null');
            if (kept && typeof kept === 'object') return { ...kept };
        } catch (e) { /* none kept */ }
        return { ...DEFAULT_SOUNDS };
    }

    function callsign(a) {
        return ((a && a.flight) || '').trim() || (a && a.hex ? a.hex.toUpperCase() : '?');
    }

    function runwayEnd(id) {
        if (!id) return '';
        const parts = String(id).split('/');
        return parts.length === 2 ? parts[1] : String(id);
    }

    document.addEventListener('alpine:init', () => {
        Alpine.store('events', {
            kinds: KINDS,
            list: [],          // newest first
            unread: 0,
            open: false,
            soundsOpen: false,
            sounds: loadSounds(),

            _seen: new Set(),  // keys of events already logged
            _runways: {},      // airport code -> { shown, candidate, since }
            _timer: null,

            init() {
                this._timer = setInterval(() => this.scan(), SCAN_MS);
            },

            add({ kind, text, hex = null, key = null }) {
                if (key) {
                    if (this._seen.has(key)) return;
                    this._seen.add(key);
                }
                this.list.unshift({ id: Date.now() + Math.random(), at: new Date(), kind, text, hex });
                if (this.list.length > MAX_EVENTS) this.list.length = MAX_EVENTS;
                if (!this.open) this.unread++;
                if (this.sounds[kind]) this.ring();
            },

            ring() {
                try {
                    const a = new Audio('/sounds/airplane-ding-dong.mp3');
                    a.volume = 0.6;
                    a.play().catch(() => { /* no click on the page yet */ });
                } catch (e) { /* no sound */ }
            },

            toggleOpen() {
                this.open = !this.open;
                if (this.open) this.unread = 0;
            },

            clear() {
                this.list = [];
                this.unread = 0;
            },

            toggleSound(kind) {
                this.sounds[kind] = !this.sounds[kind];
                try { localStorage.setItem('eventSounds', JSON.stringify(this.sounds)); } catch (e) { /* lasts the page */ }
            },

            select(ev) {
                const atc = Alpine.store('atc');
                if (!ev.hex || !atc || !atc.aircraft[ev.hex]) return;
                atc.selectAircraftByHex(ev.hex);
            },

            time(ev) {
                return ev.at.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
            },

            scan() {
                const atc = Alpine.store('atc');
                if (!atc || !atc.aircraft) return;
                const now = Date.now();

                for (const a of Object.values(atc.aircraft)) {
                    if (!a || !a.hex) continue;
                    const squawk = a.adsb && a.adsb.squawk;
                    if (SQUAWKS[squawk]) {
                        this.add({
                            kind: 'emergency', hex: a.hex, key: `sq:${a.hex}:${squawk}`,
                            text: `${callsign(a)} squawks ${squawk}, ${SQUAWKS[squawk]}`,
                        });
                    }
                    const ga = a.runway && a.runway.go_around;
                    if (ga) {
                        this.add({
                            kind: 'go_around', hex: a.hex, key: `ga:${a.hex}:${ga.since}`,
                            text: `${callsign(a)} goes around, ${ga.airport} runway ${ga.runway}`,
                        });
                    }
                    const type = ((a.adsb && a.adsb.t) || (a.bsdb && a.bsdb.icao_type_code) || '').toUpperCase();
                    if (INTEREST_TYPES.has(type)) {
                        this.add({
                            kind: 'interest', hex: a.hex, key: `type:${a.hex}`,
                            text: `${callsign(a)}, a ${type}, is in range`,
                        });
                    }
                }

                for (const apt of atc.followedAirports || []) {
                    const top = apt.runway_in_use && apt.runway_in_use[0];
                    const end = runwayEnd(top && top.runway_end);
                    if (!apt.code || !end) continue;
                    const st = this._runways[apt.code];
                    if (!st) {
                        // The runway found at load is the starting point, not a change.
                        this._runways[apt.code] = { shown: end, candidate: null, since: 0 };
                        continue;
                    }
                    if (end === st.shown) {
                        st.candidate = null;
                    } else if (end !== st.candidate) {
                        st.candidate = end;
                        st.since = now;
                    } else if (now - st.since >= RUNWAY_HOLD_MS) {
                        this.add({ kind: 'runway', text: `${apt.code} runway in use now ${end} (was ${st.shown})` });
                        st.shown = end;
                        st.candidate = null;
                    }
                }
            },
        });
    });
})();
