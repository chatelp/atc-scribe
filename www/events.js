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

    const SQUAWKS = { '7500': 'unlawful interference squawk', '7600': 'radio failure squawk', '7700': 'emergency squawk' };
    const COMPASS = ['north', 'north-east', 'east', 'south-east', 'south', 'south-west', 'west', 'north-west'];

    // ", FL 110, 18 NM north-east": where the aircraft is, from the station,
    // in the brand's numbers (EventList). Each part only when known.
    function whereText(a, atc, withLevel) {
        const parts = [];
        if (withLevel && atc && typeof atc.altitudeReading === 'function') {
            const r = atc.altitudeReading(a);
            if (r && r.kind === 'fl') parts.push('FL ' + String(r.fl).padStart(3, '0'));
            else if (r && r.kind === 'ft') parts.push(String(Math.round(r.ft / 100) * 100).replace(/\B(?=(\d{3})+(?!\d))/g, '\u202f') + ' ft');
            else if (r && r.kind === 'gnd') parts.push('on the ground');
        }
        const t = a.adsb || {};
        if (Number.isFinite(a.distance) && atc && atc.stationLatitude != null && t.lat != null) {
            const r = Math.PI / 180;
            const y = Math.sin((t.lon - atc.stationLongitude) * r) * Math.cos(t.lat * r);
            const x = Math.cos(atc.stationLatitude * r) * Math.sin(t.lat * r) -
                Math.sin(atc.stationLatitude * r) * Math.cos(t.lat * r) * Math.cos((t.lon - atc.stationLongitude) * r);
            const deg = (Math.atan2(y, x) / r + 360) % 360;
            parts.push(`${Math.round(a.distance)} NM ${COMPASS[Math.round(deg / 45) % 8]}`);
        }
        return parts.length ? ', ' + parts.join(', ') : '';
    }

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

    // The log outlives a reload (13/10): the bell showed an empty list after
    // every refresh, and "what happened in the last half hour" needs the past.
    // Kept in this browser, newest first, bounded like the list itself; the
    // keys come back too, so a restored event is not logged a second time.
    const LOG_KEY = 'eventLog';

    function loadLog() {
        try {
            const kept = JSON.parse(localStorage.getItem(LOG_KEY) || 'null');
            if (!Array.isArray(kept)) return [];
            return kept
                .filter((e) => e && e.kind && e.text && e.at)
                .filter((e) => !sameDirectionChange(e))
                .slice(0, MAX_EVENTS)
                .map((e) => ({ ...e, at: new Date(e.at) }))
                .filter((e) => !Number.isNaN(e.at.getTime()));
        } catch (e) { return []; }
    }

    // A runway line logged before 13/10 for the lead passing between two
    // runways of the same direction ("LFPG runway in use now 26L (was 27R)")
    // was not a change: it is dropped when the log is read back.
    function sameDirectionChange(e) {
        if (e.kind !== 'runway') return false;
        const m = /runway in use now (\S+) \(was (\S+)\)/.exec(e.text);
        return !!m && sameDirection(m[1], m[2]);
    }

    function saveLog(list) {
        try {
            localStorage.setItem(LOG_KEY, JSON.stringify(list.map((e) => ({ ...e, at: e.at.toISOString() }))));
        } catch (e) { /* lasts the page */ }
    }

    function sameDay(a, b) {
        return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
    }

    function callsign(a) {
        return ((a && a.flight) || '').trim() || (a && a.hex ? a.hex.toUpperCase() : '?');
    }

    // "an A337", "a B744": the article as the type code is read aloud, letter
    // by letter (A, E, F, H, I, L, M, N, O, R, S, X start with a vowel sound).
    function article(type) {
        return /^[AEFHILMNORSX]/i.test(String(type || '')) ? 'an' : 'a';
    }

    function runwayEnd(id) {
        if (!id) return '';
        const parts = String(id).split('/');
        return parts.length === 2 ? parts[1] : String(id);
    }

    // A runway end's direction, from its number (the heading in tens of
    // degrees): 26L -> "west". Two ends one number apart at most are flown the
    // same way (De Gaulle's 26L and 27R land together): the lead passing
    // between them is not a change of runway. Same rule as the server
    // (internal/adsb/runway_tracker.go) and Right now (www/now/right-now.js).
    const POINTS = ['north', 'north-east', 'east', 'south-east', 'south', 'south-west', 'west', 'north-west'];

    function runwayNumber(end) {
        const n = parseInt(String(end || '').replace(/[LRC]$/, ''), 10);
        return n >= 1 && n <= 36 ? n : null;
    }

    function sameDirection(a, b) {
        const na = runwayNumber(a), nb = runwayNumber(b);
        if (na == null || nb == null) return false;
        let d = Math.abs(na - nb);
        if (d > 18) d = 36 - d;
        return d <= 1;
    }

    function direction(end) {
        const n = runwayNumber(end);
        return n == null ? '' : POINTS[Math.round((n * 10) % 360 / 45) % 8];
    }

    document.addEventListener('alpine:init', () => {
        const restored = loadLog();
        Alpine.store('events', {
            kinds: KINDS,
            list: restored,    // newest first; what this browser logged before, too
            unread: 0,
            unreadUrgent: 0,   // of which emergencies: the counter turns urgent for those only
            open: false,
            soundsOpen: false,
            sounds: loadSounds(),

            _seen: new Set(restored.map((e) => e.key).filter(Boolean)),  // keys of events already logged
            _runways: {},      // airport code -> { shown, candidate, since }
            // When this page opened: an empty list is honest only about what it
            // watched, the log of a closed page does not grow.
            openedAt: new Date(),
            _timer: null,

            init() {
                this._timer = setInterval(() => this.scan(), SCAN_MS);
            },

            add({ kind, text, hex = null, key = null }) {
                if (key) {
                    if (this._seen.has(key)) return;
                    this._seen.add(key);
                }
                this.list.unshift({ id: Date.now() + Math.random(), at: new Date(), kind, text, hex, key });
                if (this.list.length > MAX_EVENTS) this.list.length = MAX_EVENTS;
                saveLog(this.list);
                if (!this.open) {
                    this.unread++;
                    if (kind === 'emergency') this.unreadUrgent++;
                }
                if (this.sounds[kind]) this.ring();
            },

            // The events of the last few minutes, newest first: what a "right
            // now" summary reads.
            recent(minutes = 30) {
                const since = Date.now() - minutes * 60000;
                return this.list.filter((e) => e.at.getTime() >= since);
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
                if (this.open) { this.unread = 0; this.unreadUrgent = 0; }
            },

            clear() {
                this.list = [];
                this.unread = 0;
                this.unreadUrgent = 0;
                saveLog(this.list);
            },

            // "Ring a bell for" (EventList, a-journal): five choices, the
            // station's channels and its audio streams under one.
            soundGroups: [
                { key: 'emergency', label: 'Emergencies', kinds: ['emergency'] },
                { key: 'go_around', label: 'Go-arounds', kinds: ['go_around'] },
                { key: 'runway', label: 'Runway changes', kinds: ['runway'] },
                { key: 'interest', label: 'Aircraft of interest', kinds: ['interest'] },
                { key: 'station', label: 'Station and audio', kinds: ['station', 'stream'] },
            ],

            soundGroupOn(group) {
                return group.kinds.every((k) => !!this.sounds[k]);
            },

            toggleSoundGroup(group) {
                const on = !this.soundGroupOn(group);
                for (const k of group.kinds) this.sounds[k] = on;
                try { localStorage.setItem('eventSounds', JSON.stringify(this.sounds)); } catch (e) { /* lasts the page */ }
            },

            // The mark of an event's kind, as Right now shows it (■ ▲ ●).
            markOf(ev) {
                return window.RightNow && typeof window.RightNow.eventMark === 'function'
                    ? window.RightNow.eventMark(ev.kind) : { mark: '', cls: '' };
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

            // The page's clock (UTC or local, Settings > Display), as Right now.
            time(ev) {
                const atc = window.Alpine && Alpine.store('atc');
                const t = atc && typeof atc.clockText === 'function'
                    ? atc.clockText(ev.at)
                    : ev.at.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
                // A restored event from another day says which one.
                return sameDay(ev.at, new Date()) ? t : `${ev.at.toLocaleDateString([], { day: '2-digit', month: '2-digit' })} ${t}`;
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
                            // "7700 AFR6TZ, emergency squawk, FL 110, 18 NM north-east."
                            text: `${squawk} ${callsign(a)}, ${SQUAWKS[squawk]}${whereText(a, atc, true)}.`,
                        });
                    }
                    const ga = a.runway && a.runway.go_around;
                    if (ga) {
                        this.add({
                            kind: 'go_around', hex: a.hex, key: `ga:${a.hex}:${ga.since}`,
                            // "Go-around EZY14LM on 27R at LFPG, now climbing back."
                            text: `Go-around ${callsign(a)} on ${ga.runway} at ${ga.airport}, now climbing back.`,
                        });
                    }
                    const type = ((a.adsb && a.adsb.t) || (a.bsdb && a.bsdb.icao_type_code) || '').toUpperCase();
                    if (INTEREST_TYPES.has(type)) {
                        this.add({
                            kind: 'interest', hex: a.hex, key: `type:${a.hex}`,
                            // "AFR18GH, an A388, is in range, 34 NM west."
                            text: `${callsign(a)}, ${article(type)} ${type}, is in range${whereText(a, atc, false)}.`,
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
                    if (end === st.shown || sameDirection(end, st.shown)) {
                        // The same runway, or the lead passing within the same
                        // direction: followed silently.
                        st.shown = end;
                        st.candidate = null;
                    } else if (end !== st.candidate) {
                        st.candidate = end;
                        st.since = now;
                    } else if (now - st.since >= RUNWAY_HOLD_MS) {
                        const way = direction(end);
                        this.add({ kind: 'runway', text: way
                            ? `${apt.code} switched to landing ${way} (${end}, was ${st.shown})`
                            : `${apt.code} runway in use now ${end} (was ${st.shown})` });
                        st.shown = end;
                        st.candidate = null;
                    }
                }
            },
        });
    });
})();
