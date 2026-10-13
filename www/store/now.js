// Right now, the column's default tab, and the fold of the column (design
// system, ContextColumn; migration step 5): the summary sentence, the arrival
// tables, the last half hour's events, the weather and the counters, from
// what the store already holds. Folded, the column gives the map its width
// and Right now becomes a card over the map (layout B).
//
// One domain of the Alpine store 'atc' (docs-fr/32, palier 5.1); see
// store/station.js for how the domains are assembled. The sentence and the
// tables are built by www/now/right-now.js, the METAR reading by
// www/now/metar-plain.js: pure, and tested under node.
(function () {
    const FOLD_KEY = 'columnFolded';

    function readFolded() {
        try { return localStorage.getItem(FOLD_KEY) === 'true'; } catch (e) { return false; }
    }

    window.AtcStore = window.AtcStore || [];
    window.AtcStore.push(() => ({
        // The column folded away (layout B), kept across a reload under its own key.
        columnFolded: readFolded(),
        // When the server started, for "since at least 09:12" while the
        // runway in use has not changed since (null until read).
        serverStartedAt: null,
        // Advances every 30 s: the half hour of events and the "in about
        // 2 min" follow the clock without re-rendering every second.
        nowTick: 0,
        // The weather card open under the buttons: null, 'trend', 'taf' or 'notam'.
        nowWeatherOpen: null,
        // The counters, refreshed every 5 s with the arrival sequences: read
        // live, they rewrote the column at every aircraft update (250 DOM
        // writes a second, measured 13/10).
        nowStats: { airborne: 0, heard: 0, ground: 0 },

        startRightNow() {
            if (this._rightNowStarted) return;
            this._rightNowStarted = true;
            this.fetchServerStart();
            setInterval(() => { this.nowTick++; }, 30000);
            this.refreshNowStats();
            setInterval(() => this.refreshNowStats(), 5000);
        },

        refreshNowStats() {
            const c = this.counts;
            const next = { airborne: c.air_active, heard: this.voiceAircraftCount, ground: c.ground_active };
            const n = this.nowStats;
            if (n.airborne !== next.airborne || n.heard !== next.heard || n.ground !== next.ground) this.nowStats = next;
        },

        // uptime_seconds from /server (the owner's) or the public /vitals.
        async fetchServerStart() {
            for (const path of ['/server', '/vitals']) {
                try {
                    const r = await fetch(`${API_BASE_URL}${path}`, { credentials: 'same-origin' });
                    if (!r.ok) continue;
                    const d = await r.json();
                    if (Number.isFinite(d.uptime_seconds)) {
                        this.serverStartedAt = new Date(Date.now() - d.uptime_seconds * 1000);
                        return;
                    }
                } catch (e) { /* try the next one */ }
            }
        },

        // "17:20" in the time the page shows (Settings › Display), "15:20Z" in UTC.
        clockText(date) {
            const d = date instanceof Date ? date : new Date(date);
            if (isNaN(d.getTime())) return '';
            if (this.showLocalDates) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
            return d.toISOString().substring(11, 16) + 'Z';
        },

        // The latest emergency or go-around of the last half hour, for the
        // folded card.
        _latestWarning() {
            const ev = window.Alpine && Alpine.store('events');
            if (!ev) return null;
            const w = ev.recent(30).find((e) => e.kind === 'emergency' || e.kind === 'go_around');
            if (!w) return null;
            const m = RightNow.eventMark(w.kind);
            return { mark: m.mark, cls: w.kind === 'emergency' ? 'cl-urgent' : 'cl-warn', text: ev.kinds[w.kind].label, at: w.at };
        },

        // { html, short, source }: see www/now/right-now.js.
        get rightNow() {
            this.nowTick;
            if (!window.RightNow) return { html: '', short: '', source: '' };
            return RightNow.summary({
                airports: this.followedAirports,
                sequences: this.arrivals,
                next: this.nextTouchdown,
                serverStarted: this.serverStartedAt,
                clock: (d) => this.clockText(d),
                warning: this._latestWarning(),
            });
        },

        get arrivalTables() {
            if (!window.RightNow) return [];
            return RightNow.tables(this.followedAirports, this.arrivals);
        },

        // At most five, newest first (EventList); the bell keeps the full log.
        get nowEvents() {
            this.nowTick;
            const ev = window.Alpine && Alpine.store('events');
            if (!ev) return [];
            return ev.recent(30).slice(0, 5).map((e) => Object.assign({ mark: RightNow.eventMark(e.kind) }, e));
        },

        // An empty list says what it has watched: the log of a closed page
        // does not grow, so "the last half hour" only once the page has been
        // open that long.
        get nowEmptyText() {
            this.nowTick;
            const ev = window.Alpine && Alpine.store('events');
            const opened = ev && ev.openedAt;
            if (!opened || Date.now() - opened.getTime() >= 30 * 60000) {
                return 'Nothing notable in the last half hour: no emergency, go-around or runway change.';
            }
            return `Nothing notable since ${this.clockText(opened)}, when this page opened.`;
        },

        // The airport the weather is fetched for: the reference airport.
        get weatherAirport() {
            const ref = (this.followedAirports || []).find((a) => a.principal);
            return (ref && ref.code) || this.stationAirportCode || '';
        },

        get metarRaw() {
            const m = this.getLatestMetar();
            return (m && m.metar) || '';
        },

        get metarPlain() {
            return window.MetarPlain ? MetarPlain.read(this.metarRaw) : '';
        },

        get metarTrend() {
            return (this.metar && Array.isArray(this.metar.trend)) ? this.metar.trend : [];
        },

        toggleNowWeather(kind) {
            this.nowWeatherOpen = this.nowWeatherOpen === kind ? null : kind;
        },

        // Fold or unfold the column; the map is measured again once it has its
        // new width.
        setColumnFolded(folded, tab) {
            this.columnFolded = !!folded;
            if (tab) this.columnTab = tab;
            try { localStorage.setItem(FOLD_KEY, String(this.columnFolded)); } catch (e) { /* this page only */ }
            this.remeasureMap();
        },

        // OpenLayers watches its container and usually measures itself; this
        // makes sure. The map is engine.getMap() (engine.map, read before 13/10,
        // does not exist: the phone's pane switch never remeasured it).
        remeasureMap() {
            setTimeout(() => {
                const e = this.mapManager && this.mapManager.engine;
                const m = e && typeof e.getMap === 'function' ? e.getMap() : null;
                if (m && typeof m.updateSize === 'function') m.updateSize();
            }, 50);
        },
    }));
})();
