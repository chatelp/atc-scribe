// The station and its surroundings: the receiver's position, the airports
// followed and their runways in use, the weather, the ADS-B source, the
// receiver's state, the main view (map or PAR) and the phone's panes.
//
// One domain of the Alpine store 'atc' (docs-fr/32, palier 5.1): the entries
// below are the store's, moved from app.js as they were, and app.js assembles
// them with the rest. A method's `this` is the store. The entries are built
// when app.js assembles the store, inside alpine:init as they always were: by
// then the module-level names of app.js (generateUUID, API_BASE_URL, CONFIG,
// MAP_STYLES, wsClient, audioClient, mapManager) are defined, and a method
// resolves them when it runs.
(function () {
    window.AtcStore = window.AtcStore || [];
    window.AtcStore.push(() => ({
        stationApiUrl: `${API_BASE_URL}/station`, // API URL for station data
        wxApiUrl: `${API_BASE_URL}/wx`, // API URL for weather data
        adsbSourceApiUrl: `${API_BASE_URL}/adsb/source`,
        stationLatitude: null,
        // How altitudes are read (www/altitude/altitude-scale.js): the QNH the
        // crews around the station have set, from their ADS-B (nav_qnh), and the
        // transition altitude. Standard 1013 until an aircraft sends its setting.
        altimetry: { qnh: 1013.25, estimated: false, count: 0, ta: 5000 },
        // What co-atc depends on (GET /status, audit 3.6): the receiver as its
        // station link reports it, the transcription sidecar, the ADS-B feed.
        serverStatus: null,
        stationLongitude: null,
        stationElevationFeet: null,
        stationCruiseAltitudeFt: 18000,
        stationAirportCode: null,
        // Weather configuration flags from station config
        stationFetchMETAR: false,
        stationFetchTAF: false,
        stationFetchNOTAMs: false,
        metar: null,
        taf: null,
        notams: null,
        weatherLastUpdated: null,
        weatherFetchErrors: [],
        runwayData: null, // Store runway data
        // The arrival sequence of every followed airport, PAR open or not
        // (par/par-arrivals.js, 13/10, D77): airport code -> [{ runway, inUse,
        // items: [{ hex, callsign, distNM, etaSec, eta, gapNM, tight }] }].
        arrivals: {},
        arrivalsInterval: null,
        _arrivalState: {},        // airport code -> { previousFrame }
        _arrivalRunways: {},      // airport code -> Promise<runways>

        runwayInUse: null, // Active runway scores from traffic analysis
        followedAirports: [], // every airport followed, the reference one first, from /station (Q40)
        // The main screen shows the radar map or the PAR view of one followed
        // airport (docs-fr/31-vue-par.md); the choice is kept in this browser.
        mainView: localStorage.getItem('mainView') === 'par' ? 'par' : 'map',
        parAirport: localStorage.getItem('parAirport') || '',
        parView: null,
        runwayInUseInterval: null, // 60s polling interval
        airportsData: null, // Airport reference data
        heliportsData: null, // Heliport reference data
        navaidsData: null, // Navaid reference data
        allRunwaysData: null, // All runways within range
        stationRefreshInterval: null,
        weatherRefreshInterval: null,
        adsbSourceRefreshInterval: null,
        adsbSourceInfo: {
            source_type: null,
            mode: null,
            status: 'unknown',
            aircraft: { available: false, last_success_at: null, last_error: '', data: null },
            receiver: { available: false, last_success_at: null, last_error: '', data: null },
            stats: { available: false, last_success_at: null, last_error: '', data: null },
            updated_at: null,
        },

        // The airport an aircraft's phase was judged against, shown only when
        // several are followed: with one, it would repeat on every strip.
        getPhaseAirport(aircraft) {
            if (!aircraft || this.followedAirports.length < 2) return '';
            const cur = aircraft.phase && aircraft.phase.current && aircraft.phase.current[0];
            return (cur && cur.airport) || '';
        },

        // The QNH of the moment, as the PAR view estimates it, within 40 NM of
        // the station. Without any aircraft sending its setting, standard.
        updateAltimetry() {
            const G = window.ParGeometry;
            if (!G || this.stationLatitude == null || this.stationLongitude == null) return;
            const ta = (G.DEFAULTS && G.DEFAULTS.transitionAltitudeFt) || 5000;
            const est = G.estimateQnh(Object.values(this.aircraft), this.stationLatitude, this.stationLongitude, { transitionAltitudeFt: ta });
            const next = est
                ? { qnh: Math.round(est.qnh * 10) / 10, estimated: true, count: est.count, ta }
                : { qnh: G.STD_HPA || 1013.25, estimated: false, count: 0, ta };
            const a = this.altimetry;
            if (a.qnh !== next.qnh || a.estimated !== next.estimated || a.count !== next.count || a.ta !== next.ta) {
                this.altimetry = next;
                this._lastFilterHash = null;
            }
        },
        // On a phone the map and the list take turns (audit 4.2): 'map' or 'list'.
        // The phone's pane (PhoneShell, step 12): 'radio' (the thread, home),
        // 'sky' (the map or a PAR), 'traffic' or 'now' (the column).
        mobilePane: 'radio',

        // Range rings around the station: addRangeRings() reads the station's
        // coordinates from the store.
        updateStationRings() {
            if (this.mapManager) this.mapManager.addRangeRings();
        },

        // Select an aircraft by callsign (for clicking on transcription callsigns)
        async fetchStatus() {
            try {
                const r = await fetch(`${API_BASE_URL}/status`, { credentials: 'same-origin' });
                if (!r.ok) return;
                const st = await r.json();
                this.serverStatus = st;
                if (st.audio_archive) this.audioArchive = { available: !!st.audio_archive.available };
            } catch (e) { /* stays as it was */ }
        },

        // The three lights of the status banner, each with its help.
        // The antenna's line in the top bar (migration step 3, TopBar): what the
        // station has on air and since when, said, not switchable here; or why
        // there is nothing to listen to. Level: the StatusLight variant.
        get antennaLine() {
            const st = this.serverStatus;
            const rx = st && st.receiver;
            if (!rx) return { level: 'unknown', text: 'Antenna: no report' };
            if (rx.stale) return { level: 'unknown', text: `Antenna: no report since ${this.timelineTime(rx.reported_at)}` };
            const ago = (v) => {
                const ms = v ? Date.now() - Date.parse(v) : NaN;
                if (!Number.isFinite(ms) || ms < 0) return '';
                const m = Math.round(ms / 60000);
                if (m < 1) return 'just now';
                if (m < 60) return `${m} min ago`;
                const h = Math.floor(m / 60);
                // Past two days, in days ("126 h 27 ago" read badly, design review 14/10).
                if (h >= 48) return `${Math.floor(h / 24)} days ago`;
                return `${h} h ${String(m % 60).padStart(2, '0')} ago`;
            };
            const since = rx.since ? ago(rx.since) : '';
            if (rx.state === 'on_air') {
                return { level: 'ok', text: `On air: ${rx.title || rx.label || 'the station\'s selection'}${since ? `, chosen at the station ${since}` : ''}` };
            }
            if (rx.state === 'switching') return { level: 'switching', text: `Switching channels at the station${since ? `, ${since}` : ''}` };
            if (rx.state === 'lent') return { level: 'warn', text: `Receiver lent${rx.detail ? ` (${rx.detail})` : ''}${since ? `, ${since}` : ''}: no audio` };
            if (rx.state === 'off') return { level: 'off', text: `Receiver off${since ? `, ${since}` : ''}` };
            return { level: 'unknown', text: `Antenna: ${rx.label || rx.state || '?'}` };
        },

        get statusLights() {
            const st = this.serverStatus;
            if (!st) return [];
            const lights = [];
            const rx = st.receiver;
            const when = (v) => v ? this.timelineTime(v) : '';
            if (!rx) {
                lights.push({ key: 'rx', label: 'Antenna', level: 'unknown', text: '?',
                    help: 'Nothing reports the receiver\'s state: Clairance only sees its audio streams. On the owner\'s station, radio-ctl-sync reports it.' });
            } else {
                const by = { on_air: ['ok', 'On air'], lent: ['warn', 'Lent'], switching: ['warn', 'Switching'], off: ['bad', 'Off'], unknown: ['unknown', '?'] };
                let [level, text] = by[rx.state] || ['unknown', '?'];
                if (rx.stale) { level = 'unknown'; text = '?'; }
                const parts = [rx.label, rx.title, rx.since ? `since ${when(rx.since)}` : '', rx.detail].filter(Boolean);
                lights.push({ key: 'rx', label: 'Antenna', level, text,
                    help: (rx.stale ? `No report since ${when(rx.reported_at)}: the station link has stopped. Last known: ` : '') + parts.join(' · ') + '.' });
            }
            if (st.transcription) {
                const t = st.transcription;
                // A server that runs without transcription (no source transcribes,
                // no sidecar configured; fiche H, 13/10) says status "off" without
                // "reachable": neutral, not a failure.
                const off = t.status === 'off';
                // A test instance's stand-in sidecar answers "ok" too: it must
                // not wear the same green as a real model (owner, 15/10).
                const fake = !off && t.reachable && t.backend === 'fake';
                const level = off ? 'off' : !t.reachable ? 'bad' : fake ? 'warn' : t.status === 'ok' ? 'ok' : t.status === 'degraded' ? 'warn' : 'unknown';
                const text = off ? 'Off' : !t.reachable ? 'Down' : fake ? 'Simulated' : t.status === 'ok' ? 'OK' : t.status === 'degraded' ? 'Degraded' : '?';
                const runsOn = [t.model, t.backend].filter(Boolean).join(', ');
                lights.push({ key: 'stt', label: 'Transcription', level, text,
                    help: off ? 'The server runs without transcription: no source transcribes and no sidecar is configured. Turn "transcribe audio" on for a source once a sidecar answers.'
                        : !t.reachable ? 'The transcription sidecar does not answer: transmissions are not transcribed. co-atc restarts it if it crashed.'
                        : fake ? 'A stand-in sidecar answers, for testing: no model listens, the texts are made up. Nothing heard here says what the radio said, and the matching figures mean nothing.'
                        : t.status === 'degraded' ? `The sidecar runs without some of its models (${(t.degraded || []).join(', ')}): fewer aircraft are identified.`
                        : 'The transcription sidecar answers with all its models' + (runsOn ? ` (${runsOn}).` : '.') });
            }
            if (st.adsb) {
                const age = st.adsb.last_fetch ? (Date.now() - Date.parse(st.adsb.last_fetch)) / 1000 : Infinity;
                const fresh = st.adsb.ok && age < 60;
                // messages_per_second: the receiver's own counter (0 when the source has
                // none), a health figure the aircraft count does not give.
                const rate = Number(st.adsb.messages_per_second) || 0;
                const rateText = rate > 0 ? ` It decodes about ${rate} messages a second.` : '';
                // Every fetch failing (failing_since, 14/10): since when, why, and
                // what to do if it lasts. After a network cut a server once kept
                // failing for 12 h while the station answered; a restart cured it.
                const failingMin = st.adsb.failing_since ? Math.floor((Date.now() - Date.parse(st.adsb.failing_since)) / 60000) : null;
                const unreachable = !fresh && failingMin !== null;
                lights.push({ key: 'adsb', label: 'ADS-B', level: fresh ? 'ok' : 'bad',
                    text: fresh ? (rate > 0 ? `${st.adsb.aircraft} · ${rate}/s` : String(st.adsb.aircraft))
                        : unreachable ? `Unreachable · ${failingMin < 1 ? '<1' : failingMin} min` : 'Stale',
                    help: fresh ? `The ADS-B receiver answers: ${st.adsb.aircraft} aircraft in sight now.${rateText}`
                        : unreachable ? `Clairance cannot read the station's ADS-B since ${when(st.adsb.failing_since)}: ${st.adsb.failing_why || 'the station cannot be read'}. Positions on the map are not moving. If the station answers elsewhere (its own map, another device) and this lasts more than a few minutes, restart Clairance: after a network cut it may not find the station again by itself.`
                        : `No fresh ADS-B data${st.adsb.last_fetch ? ' since ' + when(st.adsb.last_fetch) : ''}: positions on the map are not moving.` });
            }
            return lights;
        },

        // In place of an empty radio bar: why there is nothing to listen to.
        get receiverMessage() {
            const rx = this.serverStatus && this.serverStatus.receiver;
            const since = rx && rx.since ? ` since ${this.timelineTime(rx.since)}` : '';
            if (rx && !rx.stale) {
                const said = rx.label ? ` (${rx.label})` : '';
                if (rx.state === 'lent') return `The receiver is lent${said}${since}: no audio until it is back on the air.`;
                if (rx.state === 'off') return `The receiver is off${said}${since}.`;
                if (rx.state === 'switching') return `The receiver is switching channels${said}: the streams come back in a moment.`;
            }
            return this.audioFrequencies.length ? '' : 'No frequency connected.';
        },

        // The phone's tab bar (PhoneShell, step 12, decision 5): the thread is
        // home; Sky shows the map or a PAR, chosen in its own top row.
        phoneTabs: [
            { id: 'radio', label: 'Radio', icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" aria-hidden="true"><path d="M3 6.5a5.5 5.5 0 0 1 10 0M5 8.5a3 3 0 0 1 6 0"/><circle cx="8" cy="11" r="1.2"/></svg>' },
            { id: 'sky', label: 'Sky', icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" aria-hidden="true"><circle cx="8" cy="8" r="6"/><circle cx="8" cy="8" r="3"/><path d="M8 8 12.2 3.8"/></svg>' },
            { id: 'traffic', label: 'Traffic', icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" aria-hidden="true"><path d="M3 4h10M3 8h10M3 12h10"/></svg>' },
            { id: 'now', label: 'Right now', icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" aria-hidden="true"><circle cx="8" cy="8" r="6"/><path d="M8 4.5V8l2.5 1.5"/></svg>' },
        ],

        // The column is the Traffic and the Right now panes on the phone.
        get phoneColumn() {
            return this.mobilePane === 'traffic' || this.mobilePane === 'now';
        },

        showPane(which) {
            this.mobilePane = which;
            if (which === 'traffic' || which === 'now') this.columnTab = which;
        },

        async fetchStationData() {
            try {
                const response = await fetch(this.stationApiUrl);
                if (!response.ok) {
                    console.error(`HTTP error fetching station data! Status: ${response.status}`);
                    // Fallback to some default if needed, or handle error appropriately
                    this.stationLatitude = 13.6777; // Default fallback
                    this.stationLongitude = -79.6248; // Default fallback
                    this.stationElevationFeet = 569; // Default fallback
                    return;
                }
                const data = await response.json();
                this.stationLatitude = data.latitude;
                this.stationLongitude = data.longitude;
                this.applyDefaultMapStyle();
                this.stationElevationFeet = data.elevation_feet;
                this.stationCruiseAltitudeFt = Number.isFinite(data.cruise_altitude_ft) ? data.cruise_altitude_ft : 18000;
                this.stationAirportCode = data.airport_code;
                
                // Range rings and layers follow the coordinates just fetched.
                if (this.mapManager && this.mapManager.map) {
                    this.updateStationRings();
                    this.applyMapDisplaySettings();
                }

                // Store weather configuration flags
                this.stationFetchMETAR = data.fetch_metar;
                this.stationFetchTAF = data.fetch_taf;
                this.stationFetchNOTAMs = data.fetch_notams;
                
                // Every airport followed; its runways are drawn, not only the
                // reference airport's.
                this.followedAirports = Array.isArray(data.airports) ? data.airports : [];
                if (this.settings.showProcedures) this.drawProcedureLayer();
                const followedRunways = this.followedAirports.map(a => a.runways).filter(Boolean);

                // Store runway data if available
                if (followedRunways.length > 0 || data.runways) {
                    this.runwayData = followedRunways.length > 0 ? followedRunways : data.runways;
                    console.log('Runway data loaded:', this.runwayData);

                    // Draw runways on the map if mapManager is initialized
                    if (this.mapManager) {
                        this.mapManager.drawRunways(this.runwayData);
                    }
                }

                // Store runway-in-use scores
                if (data.runway_in_use) {
                    this.runwayInUse = data.runway_in_use;
                }

                console.log('Station data loaded:', data);

                // Center map on station coordinates after loading
                if (this.mapManager) {
                    this.mapManager.centerOnStation();
                }

                // Setup refresh interval if not already set (less frequent since station data is static)
                if (!this.stationRefreshInterval) {
                    this.stationRefreshInterval = setInterval(() => {
                        console.log('Refreshing station data...');
                        this.fetchStationData();
                    }, CONFIG.stationRefreshInterval);
                }

                // Setup runway-in-use polling (60s) — separate from station refresh
                if (!this.runwayInUseInterval) {
                    this.runwayInUseInterval = setInterval(() => this.fetchRunwayInUse(), 60000);
                }
                if (!this.arrivalsInterval) {
                    this.arrivalsInterval = setInterval(() => this.updateArrivals(), 5000);
                    setTimeout(() => this.updateArrivals(), 3000);
                }
                if (!this.altimetryInterval) {
                    this.altimetryInterval = setInterval(() => this.updateAltimetry(), 30000);
                    setTimeout(() => this.updateAltimetry(), 5000);
                }
                if (!this.statusInterval) {
                    this.fetchStatus();
                    // The receiver may be lent, a disk unplugged, the sidecar
                    // restarted while the page is open.
                    this.statusInterval = setInterval(() => this.fetchStatus(), 15000);
                }
            } catch (error) {
                console.error('Error fetching station data:', error);
                // Fallback to some default if needed, or handle error appropriately
                this.stationLatitude = 43.6777; // Default fallback on error
                this.stationLongitude = -79.6248; // Default fallback on error
                this.stationElevationFeet = 569;    // Default fallback on error
            }
        },

        async fetchADSBSourceStatus() {
            try {
                const response = await fetch(this.adsbSourceApiUrl);
                if (!response.ok) {
                    console.error(`HTTP error fetching ADS-B source status! Status: ${response.status}`);
                    return;
                }

                const data = await response.json();
                this.adsbSourceInfo = {
                    source_type: data.source_type || null,
                    mode: data.mode || null,
                    status: data.status || 'unknown',
                    aircraft: data.aircraft || { available: false, data: null },
                    receiver: data.receiver || { available: false, data: null },
                    stats: data.stats || { available: false, data: null },
                    updated_at: data.updated_at || null,
                };

                if (!this.adsbSourceRefreshInterval) {
                    this.adsbSourceRefreshInterval = setInterval(() => {
                        this.fetchADSBSourceStatus();
                    }, 5000);
                }
            } catch (error) {
                console.error('Error fetching ADS-B source status:', error);
            }
        },

        getAdsbReceiverSummary() {
            const receiver = this.adsbSourceInfo?.receiver?.data;
            if (!receiver || typeof receiver !== 'object') {
                return null;
            }

            const fullVersion = typeof receiver.version === 'string' ? receiver.version : null;
            const version = fullVersion ? fullVersion.split(' git:')[0] : null;

            return {
                version,
                refreshMs: Number.isFinite(receiver.refresh) ? receiver.refresh : null,
                historySeconds: Number.isFinite(receiver.history) ? receiver.history : null,
                readsb: receiver.readsb === true,
                zstd: receiver.zstd === true,
                binCraft: receiver.binCraft === true,
            };
        },

        getAdsbStatsSummary() {
            const stats = this.adsbSourceInfo?.stats?.data;
            if (!stats || typeof stats !== 'object') {
                return null;
            }

            const period = stats.last1min || stats.last5min || stats.last15min || null;
            const local = period?.local || null;
            const tracks = period?.tracks || null;

            const messagesPerMin = Number.isFinite(period?.messages) ? period.messages : null;
            const positionsPerMin = Number.isFinite(period?.position_count_total) ? period.position_count_total : null;
            const tracksAll = Number.isFinite(tracks?.all) ? tracks.all : null;
            const signalDb = Number.isFinite(local?.signal) ? local.signal : null;
            const noiseDb = Number.isFinite(local?.noise) ? local.noise : null;

            return {
                aircraftWithPos: Number.isFinite(stats.aircraft_with_pos) ? stats.aircraft_with_pos : null,
                aircraftWithoutPos: Number.isFinite(stats.aircraft_without_pos) ? stats.aircraft_without_pos : null,
                gainDb: Number.isFinite(stats.gain_db) ? stats.gain_db : null,
                estimatedPpm: Number.isFinite(stats.estimated_ppm) ? stats.estimated_ppm : null,
                messagesPerMin,
                positionsPerMin,
                tracksAll,
                signalDb,
                noiseDb,
                snrDb: Number.isFinite(signalDb) && Number.isFinite(noiseDb)
                    ? Number((signalDb - noiseDb).toFixed(1))
                    : null,
            };
        },

        formatSignedNumber(value, decimals = 1) {
            if (!Number.isFinite(value)) return '—';
            const abs = Math.abs(value).toFixed(decimals);
            if (value > 0) return `+${abs}`;
            if (value < 0) return `-${abs}`;
            return Number(abs).toFixed(decimals);
        },

        setMainView(view, airport) {
            this.mainView = view === 'par' ? 'par' : 'map';
            if (airport) this.parAirport = airport;
            try {
                localStorage.setItem('mainView', this.mainView);
                localStorage.setItem('parAirport', this.parAirport);
            } catch (e) { /* the choice lasts for this page only */ }
            if (!this.parView) return;
            if (this.mainView === 'par') this.parView.show(this.parAirport);
            else this.parView.hide();
        },

        // The runways of a followed airport, as the PAR reads them, fetched once.
        runwaysOf(code) {
            if (!this._arrivalRunways[code]) {
                this._arrivalRunways[code] = fetch(`${API_BASE_URL}/airports/${encodeURIComponent(code)}`, { credentials: 'same-origin' })
                    .then((r) => (r.ok ? r.json() : null))
                    .then((d) => (d && Array.isArray(d.runways) ? d.runways : []))
                    .catch(() => { delete this._arrivalRunways[code]; return []; });
            }
            return this._arrivalRunways[code];
        },

        // Who lands next, for every followed airport: the same placement the
        // PAR draws, run every few seconds whether the PAR is open or not.
        async updateArrivals() {
            const A = window.ParArrivals, G = window.ParGeometry;
            if (!A || !G) return;
            const next = {};
            for (const apt of this.followedAirports || []) {
                if (!apt || !apt.code) continue;
                const runways = await this.runwaysOf(apt.code);
                if (!runways.length) continue;
                const state = this._arrivalState[apt.code] || (this._arrivalState[apt.code] = {});
                const placement = A.place(G, this, apt, runways, state, { rules: window.MapVisibilityRules });
                next[apt.code] = A.sequences(G, placement);
            }
            this.arrivals = next;
        },

        // The earliest estimated touchdown among the followed airports, or null.
        get nextTouchdown() {
            const A = window.ParArrivals;
            if (!A) return null;
            let best = null;
            for (const [code, seqs] of Object.entries(this.arrivals || {})) {
                const n = A.nextTouchdown(seqs);
                if (n && (!best || n.etaSec < best.etaSec)) best = Object.assign({ airport: code }, n);
            }
            return best;
        },

        async fetchRunwayInUse() {
            try {
                const response = await fetch(this.stationApiUrl);
                if (!response.ok) return;
                const data = await response.json();
                this.runwayInUse = data.runway_in_use || null;
                if (Array.isArray(data.airports)) this.followedAirports = data.airports;
                // The Procedures layer follows the runways in use (Q56).
                if (this.settings.showProcedures) this.drawProcedureLayer();
            } catch (e) { /* silent */ }
        },

        async fetchReferenceData() {
            // Fetch airports, navaids, and all runways in parallel
            try {
                const [airportsRes, heliportsRes, navaidsRes, runwaysRes] = await Promise.all([
                    fetch(`${API_BASE_URL}/airports`),
                    fetch(`${API_BASE_URL}/heliports`),
                    fetch(`${API_BASE_URL}/navaids`),
                    fetch(`${API_BASE_URL}/runways`)
                ]);

                if (airportsRes.ok) {
                    this.airportsData = await airportsRes.json();
                    console.log(`Airports loaded: ${this.airportsData.length}`);
                    if (this.mapManager) {
                        this.mapManager.drawAirports(this.airportsData);
                        if (!this.settings.showAirports) {
                            this.mapManager.toggleLayerVisibility('airports', false);
                        }
                    }
                }

                if (heliportsRes.ok) {
                    this.heliportsData = await heliportsRes.json();
                    console.log(`Heliports loaded: ${this.heliportsData.length}`);
                    if (this.mapManager) {
                        this.mapManager.drawHeliports(this.heliportsData);
                        if (!this.settings.showHeliports) {
                            this.mapManager.toggleLayerVisibility('heliports', false);
                        }
                    }
                }

                if (navaidsRes.ok) {
                    this.navaidsData = await navaidsRes.json();
                    console.log(`Navaids loaded: ${this.navaidsData.length}`);
                    if (this.mapManager) {
                        this.mapManager.drawNavaids(this.navaidsData);
                        if (!this.settings.showNavaids) {
                            this.mapManager.toggleLayerVisibility('navaids', false);
                        }
                    }
                }

                if (runwaysRes.ok) {
                    this.allRunwaysData = await runwaysRes.json();
                    console.log(`All runways loaded: ${this.allRunwaysData.length}`);
                    if (this.mapManager) {
                        this.mapManager.drawAllRunways(this.allRunwaysData);
                        if (!this.settings.showAllRunways) {
                            this.mapManager.toggleLayerVisibility('allRunways', false);
                        }
                    }
                }
            } catch (error) {
                console.error('Error fetching reference data:', error);
            }
        },

        async fetchWeatherData() {
            try {
                const response = await fetch(this.wxApiUrl);
                if (!response.ok) {
                    console.error(`HTTP error fetching weather data! Status: ${response.status}`);
                    return;
                }
                
                const data = await response.json();
                
                // Store weather data
                this.metar = data.metar;
                this.taf = data.taf;
                this.notams = data.notams;
                this.weatherLastUpdated = data.last_updated;
                this.weatherFetchErrors = data.fetch_errors || [];
                
                console.log('Weather data loaded:', data);
                
                // Setup weather refresh interval if not already set
                if (!this.weatherRefreshInterval) {
                    this.weatherRefreshInterval = setInterval(() => {
                        console.log('Refreshing weather data...');
                        this.fetchWeatherData();
                    }, CONFIG.weatherRefreshInterval);
                }
            } catch (error) {
                console.error('Error fetching weather data:', error);
            }
        },
        
        getLatestMetar() {
            if (!this.metar || !this.metar.trend || this.metar.trend.length === 0) {
                return null;
            }
            
            // Return the first (latest) METAR in the trend array
            return this.metar.trend[0];
        },
            }));
})();
