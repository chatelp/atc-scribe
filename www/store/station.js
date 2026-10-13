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
        metarDetailsVisible: false,
        tafDetailsVisible: false,
        notamDetailsVisible: false,
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

        // The airports followed besides the reference one, each with its runway in use.
        otherAirports() {
            return this.followedAirports.filter(a => !a.principal);
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
        mobilePane: 'map',

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
                const level = !t.reachable ? 'bad' : t.status === 'ok' ? 'ok' : t.status === 'degraded' ? 'warn' : 'unknown';
                const text = !t.reachable ? 'Down' : t.status === 'ok' ? 'OK' : t.status === 'degraded' ? 'Degraded' : '?';
                lights.push({ key: 'stt', label: 'Transcription', level, text,
                    help: !t.reachable ? 'The transcription sidecar does not answer: transmissions are not transcribed. co-atc restarts it if it crashed.'
                        : t.status === 'degraded' ? `The sidecar runs without some of its models (${(t.degraded || []).join(', ')}): fewer aircraft are identified.`
                        : 'The transcription sidecar answers with all its models.' });
            }
            if (st.adsb) {
                const age = st.adsb.last_fetch ? (Date.now() - Date.parse(st.adsb.last_fetch)) / 1000 : Infinity;
                const fresh = st.adsb.ok && age < 60;
                lights.push({ key: 'adsb', label: 'ADS-B', level: fresh ? 'ok' : 'bad', text: fresh ? String(st.adsb.aircraft) : 'Stale',
                    help: fresh ? `The ADS-B receiver answers: ${st.adsb.aircraft} aircraft in sight now.`
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

        // Phone tabs: 'map', 'par' (the PAR view of the first airport followed,
        // or the one last chosen) or 'list'.
        showPane(which) {
            if (which === 'list') { this.mobilePane = 'list'; return; }
            this.mobilePane = 'map';
            if (which === 'par') {
                const apt = this.parAirport || (this.followedAirports[0] && this.followedAirports[0].code);
                if (apt) this.setMainView('par', apt);
            } else {
                this.setMainView('map');
            }
            // The map was display:none: OpenLayers must measure it again.
            setTimeout(() => {
                const m = this.mapManager && this.mapManager.engine && this.mapManager.engine.map;
                if (m && typeof m.updateSize === 'function') m.updateSize();
            }, 50);
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

        async fetchRunwayInUse() {
            try {
                const response = await fetch(this.stationApiUrl);
                if (!response.ok) return;
                const data = await response.json();
                this.runwayInUse = data.runway_in_use || null;
                if (Array.isArray(data.airports)) this.followedAirports = data.airports;
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
        
        // Extract runway end identifier from "05-23/05" format → "05"
        formatRunwayEnd(runwayId) {
            if (!runwayId) return '';
            const parts = runwayId.split('/');
            return parts.length === 2 ? parts[1] : runwayId;
        },

        // Check if two runway IDs represent parallel runways (same base number, different L/R/C).
        // "06L-24R/06L" and "06R-24L/06R" → both base "06" → parallel
        areParallelRunways(rwy1, rwy2) {
            const end1 = this.formatRunwayEnd(rwy1);
            const end2 = this.formatRunwayEnd(rwy2);
            if (!end1 || !end2 || end1 === end2) return false;
            const base1 = end1.replace(/[LRC]$/, '');
            const base2 = end2.replace(/[LRC]$/, '');
            return base1 === base2;
        },

        getLatestMetar() {
            if (!this.metar || !this.metar.trend || this.metar.trend.length === 0) {
                return null;
            }
            
            // Return the first (latest) METAR in the trend array
            return this.metar.trend[0];
        },
        
        // The METAR, TAF and NOTAM popups (kind: 'metar', 'taf', 'notam'): one open
        // at a time, placed above the button that opens it.
        toggleWeatherDetails(kind) {
            const open = !this[`${kind}DetailsVisible`];
            this.metarDetailsVisible = false;
            this.tafDetailsVisible = false;
            this.notamDetailsVisible = false;
            this[`${kind}DetailsVisible`] = open;
            if (!open) return;

            setTimeout(() => {
                const button = document.querySelector(`[data-${kind}-button]`);
                const popup = document.querySelector(`[data-${kind}-popup]`);

                if (button && popup) {
                    const rect = button.getBoundingClientRect();
                    popup.style.left = `${rect.left + (rect.width / 2)}px`;
                    popup.style.bottom = `${window.innerHeight - rect.top + 8}px`;
                    popup.style.transform = 'translateX(-50%)';
                    popup.style.transition = 'none';
                }
            }, 0);
        },
        toggleMetarDetails() { this.toggleWeatherDetails('metar'); },
        toggleTAFDetails() { this.toggleWeatherDetails('taf'); },
        toggleNOTAMDetails() { this.toggleWeatherDetails('notam'); },
    }));
})();
