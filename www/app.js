/**
 * Module: app
 * Why it exists:
 * - Main frontend runtime entry point for Co-ATC.
 * - Wires Alpine store state, API/WebSocket clients, map manager integration,
 *   audio/transcription flows, and operator-facing UI behaviors.
 *
 * Key responsibilities:
 * - Define runtime configuration and environment-derived endpoints.
 * - Initialize shared clients/services and attach browser lifecycle handlers.
 * - Orchestrate realtime aircraft updates, map rendering, and sidebar/detail views.
 *
 * Quirks / contracts:
 * - Assumes same-host API/WebSocket deployment by default (LAN/local first).
 * - Contains deliberate fallbacks for non-secure contexts used in local operations,
 *   including UUID generation and service-worker behavior constraints.
 * - Large by design today: this file remains the integration nexus while map-
 *   specific logic is progressively moved into dedicated modules.
 */

// UUID generation with fallback for non-secure contexts (e.g., HTTP on LAN)
function generateUUID() {
    if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
        return crypto.randomUUID();
    }
    // Fallback for non-secure contexts
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
        const r = Math.random() * 16 | 0;
        const v = c === 'x' ? r : (r & 0x3 | 0x8);
        return v.toString(16);
    });
}

// Base API URL - dynamically use the current port
const API_BASE_URL = `${window.location.protocol}//${window.location.hostname}:${window.location.port}/api/v1`;

// Configuration
const CONFIG = {
    // defaultCenter: [43.6777, -79.6248], // Will be fetched from API
    mapEngine: 'openlayers',
    mapAircraftWebGL: false,
    mapOverlays: null,
    aviationChartOverlayUrl: '',
    weatherRadarWmsUrl: '',
    weatherRadarWmsParams: {},
    airspaceOverlayGeoJsonUrl: '',
    defaultZoom: 10,
    dataUrl: `${API_BASE_URL}/aircraft`,
    wsUrl: `ws://${window.location.hostname}:${window.location.port}/api/v1/ws`, // WebSocket URL
    useRealData: true,
    useSampleData: true,
    rangeRings: [5, 10, 25, 50, 100],
    selectedFadeOpacity: 0.4, // Opacity for non-selected items when one is selected
    
    // Refresh intervals (in milliseconds)
    stationRefreshInterval: 30 * 60 * 1000,  // 30 minutes for station data
    weatherRefreshInterval: 30 * 60 * 1000,  // 30 minutes for weather data
};

// Initialize WebSocket client
const wsClient = new WebSocketClient(CONFIG.wsUrl);
window.wsClient = wsClient;

const tileCacheRuntimeStats = {
    supported: 'serviceWorker' in navigator,
    controlled: false,
    cacheName: null,
    hits: 0,
    misses: 0,
    networkFetches: 0,
    cacheWrites: 0,
    hitRatePct: 0,
    lastEventAt: null
};

function requestTileCacheStatsFromSW() {
    if (!('serviceWorker' in navigator)) return;
    if (!navigator.serviceWorker.controller) return;
    navigator.serviceWorker.controller.postMessage({ type: 'tile-cache-stats-request' });
}

function attachTileCacheSWListeners() {
    if (!('serviceWorker' in navigator)) return;

    navigator.serviceWorker.addEventListener('message', (event) => {
        if (!event.data || event.data.type !== 'tile-cache-stats') return;
        const data = event.data.data || {};

        const hits = Number.isFinite(data.hits) ? data.hits : 0;
        const misses = Number.isFinite(data.misses) ? data.misses : 0;
        const totalLookups = hits + misses;

        tileCacheRuntimeStats.controlled = !!navigator.serviceWorker.controller;
        tileCacheRuntimeStats.cacheName = data.cacheName || tileCacheRuntimeStats.cacheName;
        tileCacheRuntimeStats.hits = hits;
        tileCacheRuntimeStats.misses = misses;
        tileCacheRuntimeStats.networkFetches = Number.isFinite(data.networkFetches) ? data.networkFetches : 0;
        tileCacheRuntimeStats.cacheWrites = Number.isFinite(data.cacheWrites) ? data.cacheWrites : 0;
        tileCacheRuntimeStats.hitRatePct = totalLookups > 0 ? Number(((hits / totalLookups) * 100).toFixed(1)) : 0;
        tileCacheRuntimeStats.lastEventAt = Number.isFinite(data.lastEventAt) ? data.lastEventAt : null;

        if (window.Alpine && Alpine.store('atc')) {
            Alpine.store('atc').tileCacheStats = { ...tileCacheRuntimeStats };
        }
    });

    navigator.serviceWorker.addEventListener('controllerchange', () => {
        tileCacheRuntimeStats.controlled = !!navigator.serviceWorker.controller;
        requestTileCacheStatsFromSW();
    });
}

attachTileCacheSWListeners();

async function registerTileCacheServiceWorker() {
    if (!('serviceWorker' in navigator)) {
        return;
    }

    if (!window.isSecureContext && window.location.hostname !== 'localhost' && window.location.hostname !== '127.0.0.1') {
        console.warn('Service worker tile cache skipped: insecure context');
        return;
    }

    try {
        const registration = await navigator.serviceWorker.register('/sw.js', { scope: '/' });
        console.log('Tile cache service worker registered');

        if (navigator.serviceWorker.controller) {
            console.log('Tile cache service worker is controlling this page');
            tileCacheRuntimeStats.controlled = true;
            requestTileCacheStatsFromSW();
        } else {
            console.log('Tile cache service worker installed, will control after next reload');
            tileCacheRuntimeStats.controlled = false;
        }

        if (registration.waiting) {
            console.log('Tile cache service worker update is waiting to activate');
        }
    } catch (error) {
        console.warn('Tile cache service worker registration failed:', error);
    }
}

window.addEventListener('load', () => {
    registerTileCacheServiceWorker();
});

// Declare Audio client - will be initialized in alpine:init
let audioClient;
// Declare Map Manager - will be initialized in alpine:init
let mapManager;
// Declare Animation Engine - will be initialized in alpine:init
let animationEngine;

// Map instance and layers
let isAppInitialized = false; // Flag to ensure init() runs only once

// Alpine.js data store
document.addEventListener('alpine:init', () => {
    Alpine.store('atc', {
        // State
        aircraft: [],
        filteredAircraft: [],
        searchTerm: '',
        // Computed aircraft counts — derived from the live aircraft store so they stay
        // in sync as WebSocket adds/updates/removes aircraft at runtime.
        get counts() {
            const now = Date.now();
            const lastSeenCutoff = now - ((this.settings?.lastSeenMinutes || 10) * 60 * 1000);
            let ground_active = 0, ground_total = 0, air_active = 0, air_total = 0;

            for (const ac of Object.values(this.aircraft)) {
                if (ac.last_seen && new Date(ac.last_seen).getTime() < lastSeenCutoff) continue;
                if (ac.on_ground) {
                    ground_total++;
                    if (ac.status === 'active') ground_active++;
                } else {
                    air_total++;
                    if (ac.status === 'active') air_active++;
                }
            }
            return { ground_active, ground_total, air_active, air_total };
        },
        visibleAircraftOnMap: new Set(), // Track aircraft visible on map for UI indicators
        audioFrequencies: [],
        clientID: generateUUID(), // Unique client ID for audio streams
        unmutedFrequencies: new Set(), // Set of unmuted frequency IDs
        audioElements: {}, // Map of frequency ID to audio element
        audioAnalysers: {}, // Map of frequency ID to analyser node
        audioDataArrays: {}, // Map of frequency ID to data array
        visualizationFrameIds: {}, // Map of frequency ID to animation frame ID
        sourceNodes: {},
        
        // Request throttling state
        pendingRequests: {
            aircraft: false,
            tracks: new Map(), // Map of hex -> boolean for pending tracks requests
            proximity: false,
        },

        // Clear pending requests for a specific aircraft
        clearPendingRequestsForAircraft(hex) {
            this.pendingRequests.tracks.delete(hex);
        },

        // Clear all pending requests
        clearAllPendingRequests() {
            this.pendingRequests.aircraft = false;
            this.pendingRequests.tracks.clear();
            this.pendingRequests.proximity = false;
        },

        // Internal state for tracking aircraft selection changes
        _previousSelectedHex: null,

        // Request timeout configuration
        REQUEST_TIMEOUT_MS: 5000, // 5 seconds

        // Create a fetch request with timeout
        async fetchWithTimeout(url, options = {}) {
            const controller = new AbortController();
            const timeoutId = setTimeout(() => controller.abort(), this.REQUEST_TIMEOUT_MS);
            
            try {
                const response = await fetch(url, {
                    ...options,
                    signal: controller.signal
                });
                clearTimeout(timeoutId);
                return response;
            } catch (error) {
                clearTimeout(timeoutId);
                if (error.name === 'AbortError') {
                    throw new Error(`Request timeout after ${this.REQUEST_TIMEOUT_MS}ms`);
                }
                throw error;
            }
        },
        radiosStarted: false, // Initialize radiosStarted to false
        wsConnection: null, // WebSocket connection
        transcriptions: [], // Array of transcription messages
        aircraftAlerts: [], // Array of aircraft movement alerts
        audioApiUrl: `${API_BASE_URL}/frequencies`,
        transcriptionSearchTerm: '', // For searching transcriptions
        showLostAircraftOnly: false, // Toggle for showing only lost aircraft
        originalTranscriptions: {}, // Store original transcriptions before filtering
        stationApiUrl: `${API_BASE_URL}/station`, // API URL for station data
        wxApiUrl: `${API_BASE_URL}/wx`, // API URL for weather data
        adsbSourceApiUrl: `${API_BASE_URL}/adsb/source`,
        stationLatitude: null,
        stationLongitude: null,
        stationElevationFeet: null,
        stationCruiseAltitudeFt: 18000,
        stationAirportCode: null,
        // Station override state
        stationOverride: {
            latitude: null,
            longitude: null,
            active: false,
            mapClickMode: false,
            autoUpdate: false,
            updateInterval: 60, // seconds
            geolocationStatus: null,
            geolocationWatchId: null
        },
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
        initialDataLoaded: false,
        connected: null, // null = initial state, true = connected, false = connection lost
        wsReconnectAttempt: 0,
        wsNextRetryDelayMs: null,
        wsConnectionState: 'idle',
        lastUpdate: null,
        settingsCollapsed: true, // Hide settings panel by default
        selectedAircraft: null,
        showSplashScreen: true, // Show splash screen by default
        splashScreenAudioPlayed: false, // Track if the welcome sound has been played
        connectionLostSoundPlayed: false, // Track if the connection lost sound has been played
        // coordinates removed as they're not needed
        currentTime: new Date().toLocaleTimeString(),
        zuluTime: new Date().toUTCString().match(/(\d{2}:\d{2}:\d{2})/)[0] + 'Z', // Initial Zulu Time
        showLocalDates: localStorage.getItem('showLocalDates') === 'true' || false, // Default to UTC (false)
        hoveredAircraft: null,
        sidebarTab: localStorage.getItem('sidebarAircraftTab') || 'active',
        sortColumn: 'callsign',
        sortDirection: 'asc',
        lastUpdateSeconds: 0, // For footer status
        timeUpdateIntervalId: null, // Store ID for the time update interval
        mapPerfUpdateIntervalId: null, // Interval for map performance stats polling
        mapPerformanceStats: {
            heapUsedMB: null,
            heapLimitMB: null,
            heapUsagePct: null,
            heapDeltaMB: null,
            animationFps: 0,
            animationFrameMs: 0,
            animationAnimatedThisFrame: 0,
            animationVisibleTargets: 0,
            animationMarkerUpdatesPerSec: 0,
            refreshPerSec: 0,
            avgRefreshMs: 0,
            markerOpsPerSec: 0,
            fullVisibilityPassesPerSec: 0,
            animationCorrectedPerSec: 0,
            animationPredictedPerSec: 0,
            animationDroppedFrames: 0,
            animationQuality: 0,
            wsRates: {
                total: 0,
                aircraft_update: 0,
                aircraft_predicted_state: 0,
                aircraft_added: 0,
                aircraft_removed: 0,
                phase_change: 0,
                transcription: 0,
                frequency_status: 0,
                parse_errors: 0
            },
            tileCache: { ...tileCacheRuntimeStats }
        },
        tileCacheStats: { ...tileCacheRuntimeStats },
        _previousHeapUsedMB: null,
        userSetVolumes: {}, // Initialize as empty object
        lastSignificantAudioTime: {}, // Stores timestamp for each freqId
        secondsSinceLastAudio: {},  // Stores formatted string for display (e.g., "5s")
        lastAudioUpdateIntervalId: null, // Interval ID for updating secondsSinceLastAudio
        frequencyTranscriptions: {}, // Stores transcriptions per frequency_id
        transcriptionViewerVisible: {}, // Stores visibility state for each frequency's viewer
        unreadTranscriptions: {}, // Unread count per frequency (only live WS messages while viewer is closed)
        _readDividerId: {}, // ID of the first already-read message when viewer opens with unread
        frequencyConnectionStatus: {}, // Tracks connection status per frequency (connecting, connected, failed)

        // Settings
        settings: {
            mapStyle: (() => {
                const savedStyle = localStorage.getItem('mapStyle') || 'vfr-sectional';
                return savedStyle === 'terminal' ? 'vfr-sectional' : savedStyle;
            })(),
            showLabels: JSON.parse(localStorage.getItem('showLabels')) ?? true,
            showPaths: JSON.parse(localStorage.getItem('showPaths')) ?? true,
            showRings: JSON.parse(localStorage.getItem('showRings')) ?? true,
            showAirports: JSON.parse(localStorage.getItem('showAirports')) ?? true,
            showHeliports: JSON.parse(localStorage.getItem('showHeliports')) ?? true,
            showNavaids: JSON.parse(localStorage.getItem('showNavaids')) ?? true,
            showAllRunways: JSON.parse(localStorage.getItem('showAllRunways')) ?? true,
            showAirspaceBoundaries: JSON.parse(localStorage.getItem('showAirspaceBoundaries')) ?? false,
            airspaceOpacity: (() => {
                const value = parseFloat(localStorage.getItem('airspaceOpacity'));
                return Number.isFinite(value) ? value : 0.5;
            })(),
            ringsOpacity: (() => {
                const value = parseFloat(localStorage.getItem('ringsOpacity'));
                return Number.isFinite(value) ? value : 1;
            })(),
            airportsOpacity: (() => {
                const value = parseFloat(localStorage.getItem('airportsOpacity'));
                return Number.isFinite(value) ? value : 1;
            })(),
            heliportsOpacity: (() => {
                const value = parseFloat(localStorage.getItem('heliportsOpacity'));
                return Number.isFinite(value) ? value : 1;
            })(),
            navaidsOpacity: (() => {
                const value = parseFloat(localStorage.getItem('navaidsOpacity'));
                return Number.isFinite(value) ? value : 1;
            })(),
            allRunwaysOpacity: (() => {
                const value = parseFloat(localStorage.getItem('allRunwaysOpacity'));
                return Number.isFinite(value) ? value : 1;
            })(),
            showNexrad: JSON.parse(localStorage.getItem('showNexrad') ?? localStorage.getItem('showWeatherRadar')) ?? false,
            nexradOpacity: (() => {
                const value = parseFloat(localStorage.getItem('nexradOpacity'));
                return Number.isFinite(value) ? value : 0.55;
            })(),
            showNoaaInfrared: JSON.parse(localStorage.getItem('showNoaaInfrared')) ?? false,
            noaaInfraredOpacity: (() => {
                const value = parseFloat(localStorage.getItem('noaaInfraredOpacity'));
                return Number.isFinite(value) ? value : 0.55;
            })(),
            showNoaaRadar: JSON.parse(localStorage.getItem('showNoaaRadar')) ?? false,
            noaaRadarOpacity: (() => {
                const value = parseFloat(localStorage.getItem('noaaRadarOpacity'));
                return Number.isFinite(value) ? value : 0.55;
            })(),
            minAltitude: parseInt(localStorage.getItem('minAltitude')) || 0,
            maxAltitude: parseInt(localStorage.getItem('maxAltitude')) || 60000,
            trailLength: parseInt(localStorage.getItem('trailLength')) || 2,
            tracksLimit: parseInt(localStorage.getItem('tracksLimit')) || 1000,
            lastSeenMinutes: (() => {
                const allowed = ['1', '2', '5', '10', '20'];
                const value = (localStorage.getItem('lastSeenMinutes') || '10').trim();
                return allowed.includes(value) ? value : '10';
            })(), // Compact retention presets for sidebar control
            listSort: localStorage.getItem('listSort') || localStorage.getItem('activeListSort') || localStorage.getItem('lostListSort') || 'callsign_az',
            statusFilters: JSON.parse(localStorage.getItem('statusFilters')) || { active: true, stale: true, signal_lost: true },
            showAirAircraft: JSON.parse(localStorage.getItem('showAirAircraft')) ?? true,
            showGroundAircraft: JSON.parse(localStorage.getItem('showGroundAircraft')) ?? true,
            showLocalDates: JSON.parse(localStorage.getItem('showLocalDates')) ?? false, // Default to UTC (false)
            phaseFilters: JSON.parse(localStorage.getItem('phaseFilters')) || { CRZ: true, CLB: true, DEP: true, APP: true, ARR: true, TAX: true, 'T/O': true, 'T/D': true, NEW: true, UNK: true },
            excludeOtherAirportsGrounded: JSON.parse(localStorage.getItem('excludeOtherAirportsGrounded')) ?? false, // Default to false (show all grounded aircraft)
            // Aircraft animation settings
            aircraftAnimation: {
                enabled: JSON.parse(localStorage.getItem('aircraftAnimationEnabled')) ?? true,
                interpolationFps: parseInt(localStorage.getItem('aircraftAnimationFps')) || 30,
                viewportCulling: JSON.parse(localStorage.getItem('aircraftAnimationViewportCulling')) ?? true
            },
        },

        // Add property to track previous settings for change detection
        previousSettings: {},
        needsFullReload: false,
        _settingsSaveTimeoutId: null,
        _settingsSaveDebounceMs: 300,
        _lastSettingsHash: null,
        
        // Simulation state
        showCreateSimulatedAircraft: false,
        simulationModal: {
            lat: 43.6777, // Default to CYYZ area
            lon: -79.6248,
            altitude: 5000,
            heading: Math.floor(Math.random() * 360), // Random heading
            speed: 250,
            verticalRate: 0,
            mapClickMode: false
        },

        // Caching properties for filteredAircraft performance optimization
        _filteredAircraftCache: null,
        _lastFilterHash: null,

        // Enhanced settings save with change detection for WebSocket
        saveSettings() {
            const previousSettings = { ...this.previousSettings };
            
            // Save to localStorage
            localStorage.setItem('mapStyle', this.settings.mapStyle);
            localStorage.setItem('showLabels', this.settings.showLabels);
            localStorage.setItem('showPaths', this.settings.showPaths);
            localStorage.setItem('showRings', this.settings.showRings);
            localStorage.setItem('minAltitude', this.settings.minAltitude);
            localStorage.setItem('maxAltitude', this.settings.maxAltitude);
            localStorage.setItem('trailLength', this.settings.trailLength);
            localStorage.setItem('tracksLimit', this.settings.tracksLimit);
            localStorage.setItem('lastSeenMinutes', this.settings.lastSeenMinutes);
            localStorage.setItem('listSort', this.settings.listSort);
            localStorage.setItem('statusFilters', JSON.stringify(this.settings.statusFilters));
            localStorage.setItem('showAirAircraft', this.settings.showAirAircraft);
            localStorage.setItem('showGroundAircraft', this.settings.showGroundAircraft);
            localStorage.setItem('showLocalDates', this.settings.showLocalDates);
            localStorage.setItem('phaseFilters', JSON.stringify(this.settings.phaseFilters));
            localStorage.setItem('excludeOtherAirportsGrounded', this.settings.excludeOtherAirportsGrounded);
            localStorage.setItem('showAirports', this.settings.showAirports);
            localStorage.setItem('showHeliports', this.settings.showHeliports);
            localStorage.setItem('showNavaids', this.settings.showNavaids);
            localStorage.setItem('showAllRunways', this.settings.showAllRunways);
            localStorage.setItem('showAirspaceBoundaries', this.settings.showAirspaceBoundaries);
            localStorage.setItem('airspaceOpacity', this.settings.airspaceOpacity);
            localStorage.setItem('ringsOpacity', this.settings.ringsOpacity);
            localStorage.setItem('airportsOpacity', this.settings.airportsOpacity);
            localStorage.setItem('heliportsOpacity', this.settings.heliportsOpacity);
            localStorage.setItem('navaidsOpacity', this.settings.navaidsOpacity);
            localStorage.setItem('allRunwaysOpacity', this.settings.allRunwaysOpacity);
            localStorage.setItem('showNexrad', this.settings.showNexrad);
            localStorage.setItem('nexradOpacity', this.settings.nexradOpacity);
            localStorage.setItem('showNoaaInfrared', this.settings.showNoaaInfrared);
            localStorage.setItem('noaaInfraredOpacity', this.settings.noaaInfraredOpacity);
            localStorage.setItem('showNoaaRadar', this.settings.showNoaaRadar);
            localStorage.setItem('noaaRadarOpacity', this.settings.noaaRadarOpacity);
            localStorage.setItem('showWeatherRadar', this.settings.showNexrad);

            // Save aircraft animation settings
            localStorage.setItem('aircraftAnimationEnabled', this.settings.aircraftAnimation.enabled);
            localStorage.setItem('aircraftAnimationFps', this.settings.aircraftAnimation.interpolationFps);
            localStorage.setItem('aircraftAnimationViewportCulling', this.settings.aircraftAnimation.viewportCulling);
            
            // Update animation engine configuration if it exists
            if (this.animationEngine) {
                this.animationEngine.updateConfig(this.settings.aircraftAnimation);
            }
            
            // Detect if server-side filter change occurred
            const serverSideChanged = (
                previousSettings.minAltitude !== this.settings.minAltitude ||
                previousSettings.maxAltitude !== this.settings.maxAltitude ||
                previousSettings.lastSeenMinutes !== this.settings.lastSeenMinutes ||
                previousSettings.excludeOtherAirportsGrounded !== this.settings.excludeOtherAirportsGrounded
            );
            
            if (serverSideChanged && Object.keys(previousSettings).length > 0) {
                this.needsFullReload = true;
            }
            
            // Store current settings for next comparison
            this.previousSettings = { ...this.settings };
        },

        // Debounced settings save to reduce frequent writes and reactivity churn
        queueSaveSettings() {
            const settingsHash = JSON.stringify(this.settings);
            if (this._lastSettingsHash === settingsHash) {
                return;
            }
            this._lastSettingsHash = settingsHash;

            if (this._settingsSaveTimeoutId) {
                clearTimeout(this._settingsSaveTimeoutId);
            }

            this._settingsSaveTimeoutId = setTimeout(() => {
                this._settingsSaveTimeoutId = null;
                this.saveSettings();
            }, this._settingsSaveDebounceMs);
        },

        // Enhanced settings change handlers for WebSocket
        onFilterChange() {
            this.queueSaveSettings();
            
            // Check if this is a server-side filter change that requires bulk reload
            if (this.needsFullReload) {
                console.log('Server-side filter change detected, requesting bulk reload via WebSocket...');
                this.requestInitialAircraftData(); // Use WebSocket bulk request
                this.needsFullReload = false;
            } else {
                // Simple filters like search, phase, air/ground can be handled client-side
                this._lastFilterHash = null;
                
                if (this.mapManager) {
                    this.mapManager.applyFiltersAndRefreshView();
                }
            }
        },

        // Helper method to check if a frequency is unmuted
        isUnmuted(frequencyId) {
            return this.unmutedFrequencies.has(frequencyId);
        },

        // MapManager instance
        mapManager: null, // Will be set in alpine:init

        // Computed
        get aircraftCount() {
            return Object.keys(this.aircraft).length;
        },

        isSignalLostForSidebar(aircraft) {
            if (!aircraft) return false;

            // Strict source of truth: classification is time-based from last ADS-B poll.
            const secondsSince = this.getSecondsSinceLastSeen(aircraft);
            return Number.isFinite(secondsSince) && secondsSince >= 60;
        },

        get activeAircraftCount() {
            this.currentTime;
            return this.filteredAircraft.filter((aircraft) => !this.isSignalLostForSidebar(aircraft)).length;
        },

        get lostAircraftCount() {
            this.currentTime;
            return this.filteredAircraft.filter((aircraft) => this.isSignalLostForSidebar(aircraft)).length;
        },

        get visibleSidebarAircraft() {
            this.currentTime;
            if (this.sidebarTab === 'lost') {
                return this.filteredAircraft.filter((aircraft) => this.isSignalLostForSidebar(aircraft));
            }
            return this.filteredAircraft.filter((aircraft) => !this.isSignalLostForSidebar(aircraft));
        },

        setSidebarTab(tab) {
            this.sidebarTab = tab === 'lost' ? 'lost' : 'active';
            localStorage.setItem('sidebarAircraftTab', this.sidebarTab);
        },

        getStatusColor(aircraft) {
            if (!aircraft || !aircraft.status) return 'bg-highlight'; // Default green
            
            // If aircraft is on ground, use a neutral gray color
            if (aircraft.on_ground) return 'bg-gray-400';
            
            switch (aircraft.status) {
                case 'active':
                    return 'bg-highlight'; // Green
                case 'stale':
                    return 'bg-warning';   // Yellow
                case 'signal_lost':
                    return 'bg-gray-500';  // Grey
                default:
                    return 'bg-highlight'; // Default green
            }
        },

        // Helper function to safely get current phase
        getCurrentPhase(aircraft) {
            if (!aircraft) return 'NEW';
            return (aircraft.phase && aircraft.phase.current && aircraft.phase.current.length > 0)
                ? aircraft.phase.current[0].phase
                : 'NEW';
        },

        get filteredAircraft() {
            // CRITICAL FIX: Maintain stable array reference to prevent full table re-renders
            if (!this._filteredAircraftCache) {
                this._filteredAircraftCache = [];
            }

            // If no filtering has been done yet, do it immediately (synchronously for first load)
            if (!this._lastFilterHash) {
                this._performFiltering();
                return this._filteredAircraftCache;
            }
 
            // Return stable array reference
            return this._filteredAircraftCache;
        },
        
        // Perform heavy filtering computation asynchronously
        _performFiltering() {
            // Create lightweight hash of current filter state (include ALL filter settings)
            const phaseFilterHash = this.settings.phaseFilters ? Object.entries(this.settings.phaseFilters).map(([k, v]) => `${k}:${v}`).join(',') : '';
            const filterHash = `${this.searchTerm}|${this.settings.showGroundAircraft}|${this.settings.showAirAircraft}|${this.settings.minAltitude}|${this.settings.maxAltitude}|${this.settings.lastSeenMinutes}|${this.settings.listSort}|${phaseFilterHash}|${Object.keys(this.aircraft).length}|${this.selectedAircraft?.hex}|${this.sortColumn}|${this.sortDirection}`;

            // Return cached result if nothing changed (but always filter if no cache exists)
            if (this._lastFilterHash === filterHash && this._filteredAircraftCache) {
                return;
            }

            const searchLower = this.searchTerm.toLowerCase();
            const now = new Date();
            const lastSeenMinutes = Number(this.settings.lastSeenMinutes) || 10;
            const lastSeenCutoff = new Date(now.getTime() - (lastSeenMinutes * 60 * 1000));

            const filtered = Object.values(this.aircraft).filter(aircraft => {
                // Filter by last seen time - hide aircraft not seen recently
                if (aircraft.last_seen) {
                    const lastSeenDate = new Date(aircraft.last_seen);
                    if (lastSeenDate < lastSeenCutoff) {
                        return false;
                    }
                }

                // Filter by search term - includes callsign, type, category, manufacturer
                if (searchLower) {
                    const callsign = (aircraft.flight || aircraft.hex).toLowerCase();
                    const type = (aircraft.adsb?.type || '').toLowerCase();
                    const category = (aircraft.adsb?.category || '').toLowerCase();
                    const manufacturer = (aircraft.bsdb?.manufacturer || '').toLowerCase();
                    const bsdbType = (aircraft.bsdb?.type || '').toLowerCase();

                    const matchesSearch = callsign.includes(searchLower) ||
                                        type.includes(searchLower) ||
                                        category.includes(searchLower) ||
                                        manufacturer.includes(searchLower) ||
                                        bsdbType.includes(searchLower);

                    if (!matchesSearch) return false;
                }

                // Filter by air/ground settings - both can be enabled/disabled independently
                const showThisAircraft = (aircraft.on_ground && this.settings.showGroundAircraft) ||
                                        (!aircraft.on_ground && this.settings.showAirAircraft);

                if (!showThisAircraft) {
                    return false;
                }

                // Filter by flight phase
                const currentPhase = this.getCurrentPhase(aircraft);

                if (this.settings.phaseFilters && this.settings.phaseFilters[currentPhase] === false) {
                    return false;
                }

                // Filter by altitude (client-side for immediate table update)
                // Only apply altitude filter to aircraft in the air
                if (!aircraft.on_ground && (!aircraft.adsb || aircraft.adsb.alt_baro < this.settings.minAltitude || aircraft.adsb.alt_baro > this.settings.maxAltitude)) {
                    return false;
                }

                return true;
            });

            const sortKey = this.settings.listSort || 'callsign_az';
            const sorted = filtered.sort((a, b) => this.compareAircraftForSidebar(a, b, sortKey));
            
            // CRITICAL FIX: Update array in-place to maintain stable reference and prevent full table re-render
            this._updateFilteredAircraftInPlace(sorted);
            this._lastFilterHash = filterHash;
            this._lastFilterTime = Date.now();
        },

        // Update filtered aircraft array in-place to maintain stable reference
        _updateFilteredAircraftInPlace(newFiltered) {
            if (!this._filteredAircraftCache) {
                this._filteredAircraftCache = [];
            }

            // Create maps for efficient lookups
            const currentMap = new Map(this._filteredAircraftCache.map((aircraft, index) => [aircraft.hex, { aircraft, index }]));
            const newMap = new Map(newFiltered.map(aircraft => [aircraft.hex, aircraft]));

            // Track aircraft that need animations
            const addedAircraft = [];
            const removedAircraft = [];

            // Remove aircraft that are no longer in the filtered list
            for (let i = this._filteredAircraftCache.length - 1; i >= 0; i--) {
                const aircraft = this._filteredAircraftCache[i];
                if (!newMap.has(aircraft.hex)) {
                    removedAircraft.push(aircraft);
                    this._filteredAircraftCache.splice(i, 1);
                }
            }

            // Update existing aircraft and add new ones
            const finalArray = [];
            for (const newAircraft of newFiltered) {
                const existingEntry = currentMap.get(newAircraft.hex);
                
                if (existingEntry) {
                    // Update existing aircraft in-place
                    Object.assign(existingEntry.aircraft, newAircraft);
                    finalArray.push(existingEntry.aircraft);
                } else {
                    // Add new aircraft
                    addedAircraft.push(newAircraft);
                    finalArray.push(newAircraft);
                }
            }

            // Replace array contents while maintaining reference
            this._filteredAircraftCache.length = 0;
            this._filteredAircraftCache.push(...finalArray);

            // Trigger animations for added/removed aircraft
            this._animateAircraftChanges(addedAircraft, removedAircraft);
        },

        // Animate aircraft appearing and disappearing
        _animateAircraftChanges(addedAircraft, removedAircraft) {
            // Animate new aircraft appearing
            addedAircraft.forEach(aircraft => {
                setTimeout(() => {
                    const row = document.querySelector(`tr[data-aircraft-hex="${aircraft.hex}"]`);
                    if (row) {
                        row.style.opacity = '0';
                        row.style.transform = 'translateX(-20px)';
                        row.style.transition = 'opacity 0.3s ease, transform 0.3s ease';
                        
                        // Trigger animation
                        requestAnimationFrame(() => {
                            row.style.opacity = '1';
                            row.style.transform = 'translateX(0)';
                        });
                    }
                }, 50);
            });

            // Animate removed aircraft disappearing
            removedAircraft.forEach(aircraft => {
                const row = document.querySelector(`tr[data-aircraft-hex="${aircraft.hex}"]`);
                if (row) {
                    row.style.transition = 'opacity 0.3s ease, transform 0.3s ease';
                    row.style.opacity = '0';
                    row.style.transform = 'translateX(20px)';
                }
            });
        },

        getSortValue(aircraft, column) {
            switch (column) {
                case 'callsign_az':
                case 'callsign':
                    return (aircraft.flight || aircraft.hex).toLowerCase();
                case 'phase':
                    return this.getCurrentPhase(aircraft);
                case 'altitude':
                    return (typeof aircraft.adsb?.alt_baro === 'number' && Number.isFinite(aircraft.adsb.alt_baro))
                        ? aircraft.adsb.alt_baro
                        : Number.NEGATIVE_INFINITY;
                case 'heading':
                    return this.getHeadingWithType(aircraft).value ?? -1;
                case 'speed':
                    return (typeof aircraft.adsb?.tas === 'number' && Number.isFinite(aircraft.adsb.tas))
                        ? aircraft.adsb.tas
                        : Number.NEGATIVE_INFINITY;
                case 'gs':
                    return (typeof aircraft.adsb?.gs === 'number' && Number.isFinite(aircraft.adsb.gs))
                        ? aircraft.adsb.gs
                        : Number.NEGATIVE_INFINITY;
                case 'distance':
                    return aircraft.distance ?? 999999; // Sort undefined distances to the end
                case 'last_seen': {
                    if (!aircraft.last_seen) return Number.NEGATIVE_INFINITY;
                    const ts = new Date(aircraft.last_seen).getTime();
                    return Number.isFinite(ts) ? ts : Number.NEGATIVE_INFINITY;
                }
                default:
                    return '';
            }
        },

        compareAircraftForSidebar(a, b, column) {
            const aValue = this.getSortValue(a, column);
            const bValue = this.getSortValue(b, column);

            const ascendingColumns = new Set(['distance', 'callsign_az', 'callsign']);
            const direction = ascendingColumns.has(column) ? 1 : -1;

            if (aValue < bValue) return -1 * direction;
            if (aValue > bValue) return 1 * direction;

            const aCallsign = (a.flight || a.hex || '').toLowerCase();
            const bCallsign = (b.flight || b.hex || '').toLowerCase();
            if (aCallsign < bCallsign) return -1;
            if (aCallsign > bCallsign) return 1;
            return 0;
        },

        // RESTORING createLabelContent
        createLabelContent(aircraft, callsign, altitude, verticalTrend) {
            const altitudeColorClass = 'text-white';
            // Use alt_baro consistently across all components (same as details panel and flight strip)
            const hasAlt = aircraft.adsb && typeof aircraft.adsb.alt_baro === 'number' && Number.isFinite(aircraft.adsb.alt_baro);
            const altitudeDisplay = hasAlt
                ? `${Math.round(aircraft.adsb.alt_baro / 100) * 100}`
                : '-';
            
            // Speed logic: prefer TAS when present, otherwise GS, otherwise '-'.
            const tasValue = aircraft.adsb && typeof aircraft.adsb.tas === 'number' && Number.isFinite(aircraft.adsb.tas)
                ? Math.round(aircraft.adsb.tas)
                : null;
            const gsValue = aircraft.adsb && typeof aircraft.adsb.gs === 'number' && Number.isFinite(aircraft.adsb.gs)
                ? Math.round(aircraft.adsb.gs)
                : null;
            const speedValue = tasValue !== null ? tasValue : (gsValue !== null ? gsValue : '-');
            const speedLabel = tasValue !== null ? 'TAS' : (gsValue !== null ? 'GS' : 'SPD');

            const statusColorClass = this.getStatusColor(aircraft);
            let lastSeenText = '';
            if (aircraft.last_seen) {
                const secondsAgo = Math.floor((new Date() - new Date(aircraft.last_seen)) / 1000);
                lastSeenText = `${secondsAgo}s`;
            }

            const altitudeTrendIconClass = this.getAltitudeTrendIcon(aircraft);
            const altitudeTrendColorClass = this.getAltitudeTrendClasses(aircraft);

            // Determine callsign color based on aircraft status
            let callsignColorClass = 'text-highlight'; // Default green for active aircraft
            if (aircraft.status === 'signal_lost') {
                callsignColorClass = 'text-red-400'; // Red for signal lost (matching table)
            } else if (aircraft.stale_position) {
                callsignColorClass = 'text-orange-400'; // Orange for stale position (GPS lost)
            } else if (aircraft.on_ground) {
                callsignColorClass = 'text-white'; // White for grounded aircraft
            }

            // Stale position badge — shown when aircraft lost GPS but marker remains at last known location
            const staleBadge = aircraft.stale_position
                ? '<span class="text-[7px] font-bold text-orange-400/80 ml-1" title="Last known position — GPS data lost">LP</span>'
                : '';

            // Create phase badge (identical to table formatting)
            let phaseBadge = '';
            const currentPhase = this.getCurrentPhase(aircraft);
            if (currentPhase) {
                const phaseClasses = {
                    'CRZ': 'bg-blue-500/20 text-blue-400 border border-blue-500/30',
                    'CLB': 'bg-lime-500/20 text-lime-400 border border-lime-500/30',
                    'DEP': 'bg-green-500/20 text-green-400 border border-green-500/30',
                    'APP': 'bg-yellow-500/20 text-yellow-400 border border-yellow-500/30',
                    'ARR': 'bg-pink-400/15 text-pink-300 border border-pink-400/25',
                    'TAX': 'bg-purple-500/20 text-purple-400 border border-purple-500/30',
                    'T/O': 'bg-orange-500/20 text-orange-400 border border-orange-500/30',
                    'T/D': 'bg-teal-500/20 text-teal-400 border border-teal-500/30',
                    'NEW': 'bg-gray-500/20 text-gray-400 border border-gray-500/30',
                    'UNK': 'bg-slate-500/15 text-slate-400 border border-slate-500/25'
                };
                const phaseClass = phaseClasses[currentPhase] || phaseClasses['NEW'];
                phaseBadge = `<span class="text-[8px] font-bold uppercase px-1.5 py-0.5 rounded ${phaseClass}">${currentPhase}</span>`;
            }

            // Create airline and type display on same line
            const aircraftType = aircraft.adsb?.t || '-';
            const airlineTypeDisplay = aircraft.airline ? `${aircraft.airline} (${aircraftType})` : aircraftType;

            const borderClass = aircraft.stale_position
                ? 'border border-orange-500/40 border-dashed'
                : 'border border-white/10';

            if (aircraft.on_ground) {
                return `
                    <div class="bg-black/80 backdrop-blur-sm ${borderClass} p-1.5 rounded text-[11px] whitespace-nowrap min-w-[140px] flex flex-col gap-1 transition-all duration-200 group cursor-pointer
                                hover:bg-black/90 hover:border-highlight/50 hover:shadow-[0_0_10px_rgba(76,175,80,0.1)]">
                        <div class="flex justify-between items-center">
                            <div class="flex items-center">
                                ${phaseBadge}
                                <span class="font-bold ${callsignColorClass} text-xs group-hover:text-white/90 ${phaseBadge ? 'ml-1.5' : ''}">${callsign}</span>${staleBadge}
                            </div>
                            <span class="text-text/70 text-[10px]" data-lastseen>${lastSeenText}</span>
                        </div>
                        <div class="grid grid-cols-2 gap-1 text-[10px]">
                            <div class="${altitudeColorClass}">ALT ${altitudeDisplay} <span class="${altitudeTrendIconClass} ${altitudeTrendColorClass}"></span></div>
                            <div>${speedLabel} ${speedValue}</div>
                        </div>
                    </div>
                `;
            }

            return `
                <div class="bg-black/80 backdrop-blur-sm ${borderClass} p-1.5 rounded text-[11px] whitespace-nowrap min-w-[140px] flex flex-col gap-1 transition-all duration-200 group cursor-pointer
                            hover:bg-black/90 hover:border-highlight/50 hover:shadow-[0_0_10px_rgba(76,175,80,0.1)]">
                    <div class="flex justify-between items-center">
                        <div class="flex items-center">
                            ${phaseBadge}
                            <span class="font-bold ${callsignColorClass} text-xs ${phaseBadge ? 'ml-1.5' : ''}">${callsign}</span>${staleBadge}
                        </div>
                        <span class="text-text/70 text-[10px]" data-lastseen>${lastSeenText}</span>
                    </div>
                    <div class="grid grid-cols-2 gap-1 text-[10px]">
                        <div class="${altitudeColorClass}">ALT ${altitudeDisplay} <span class="${altitudeTrendIconClass} ${altitudeTrendColorClass}"></span></div>
                        <div>${speedLabel} ${speedValue}</div>
                    </div>
                </div>
            `;
        },

        // RESTORING getAltitudeTrendClasses and getAltitudeTrendIcon
        getAltitudeTrendClasses(aircraft) {
            if (!aircraft || !aircraft.adsb || typeof aircraft.adsb.baro_rate === 'undefined') return 'text-text'; // Default
            if (aircraft.adsb.baro_rate > 100) return 'text-highlight'; // Climbing - green
            if (aircraft.adsb.baro_rate < -100) return 'text-danger'; // Descending - red
            return 'text-text'; // Level
        },

        getAltitudeTrendIcon(aircraft) {
            if (!aircraft || !aircraft.adsb || typeof aircraft.adsb.baro_rate === 'undefined') return 'fas fa-arrows-alt-h'; // Default - level
            if (aircraft.adsb.baro_rate > 100) return 'fas fa-arrow-up'; // Climbing
            if (aircraft.adsb.baro_rate < -100) return 'fas fa-arrow-down'; // Descending
            return 'fas fa-arrows-alt-h'; // Level
        },

        getAltitudeUnitLabel(aircraft) {
            const altitude = aircraft?.adsb?.alt_baro;
            if (typeof altitude !== 'number' || !Number.isFinite(altitude)) return '';

            const baroRate = aircraft?.adsb?.baro_rate;
            if (typeof baroRate === 'number' && Number.isFinite(baroRate)) {
                const roundedRate = Math.round(baroRate);
                if (roundedRate > 100) return `+${roundedRate} ft`;
                if (roundedRate < -100) return `${roundedRate} ft`;
            }

            return 'ft';
        },

        getATCDerivedMetrics(aircraft) {
            const adsbData = aircraft?.adsb || {};
            const serverDerived = adsbData.atc_derived || null;
            const num = (v) => (typeof v === 'number' && Number.isFinite(v) ? v : null);
            const normalizeHeading = (v) => ((v % 360) + 360) % 360;
            const signedAngleDiff = (a, b) => {
                let d = normalizeHeading(a) - normalizeHeading(b);
                while (d > 180) d -= 360;
                while (d <= -180) d += 360;
                return d;
            };

            const track = num(adsbData.track);
            const trueHeading = num(adsbData.true_heading);
            const magHeading = num(adsbData.mag_heading);
            const windDir = num(adsbData.wd);
            const windSpeed = num(adsbData.ws);
            const gs = num(adsbData.gs);
            const verticalRate = num(adsbData.baro_rate) ?? num(adsbData.geom_rate);
            const trackRate = num(adsbData.track_rate);

            const headingSource = serverDerived?.heading_source
                || (trueHeading !== null ? 'TRUE' : (magHeading !== null ? 'MAG' : (track !== null ? 'TRACK' : '-')));

            let trackHeadingError = '-';
            if (num(serverDerived?.track_heading_error_deg) !== null) {
                const drift = num(serverDerived.track_heading_error_deg);
                trackHeadingError = `${drift >= 0 ? '+' : ''}${drift.toFixed(1)}°`;
            } else {
                const referenceHeading = trueHeading ?? magHeading ?? track;
                if (track !== null && referenceHeading !== null) {
                    const drift = signedAngleDiff(track, referenceHeading);
                    trackHeadingError = `${drift >= 0 ? '+' : ''}${drift.toFixed(1)}°`;
                }
            }

            let headTailWind = '-';
            let crossWind = '-';
            if (num(serverDerived?.head_tailwind_kt) !== null && num(serverDerived?.crosswind_kt) !== null) {
                const headwind = num(serverDerived.head_tailwind_kt);
                const crosswindVal = num(serverDerived.crosswind_kt);
                const hwLabel = headwind >= 0 ? 'HW' : 'TW';
                const cwLabel = crosswindVal >= 0 ? 'R' : 'L';
                headTailWind = `${hwLabel} ${Math.abs(headwind).toFixed(1)} kt`;
                crossWind = `${cwLabel} ${Math.abs(crosswindVal).toFixed(1)} kt`;
            } else if (track !== null && windDir !== null && windSpeed !== null) {
                const rel = signedAngleDiff(windDir, track) * Math.PI / 180;
                const headwind = windSpeed * Math.cos(rel); // + headwind, - tailwind
                const crosswindVal = windSpeed * Math.sin(rel); // + from right, - from left
                const hwLabel = headwind >= 0 ? 'HW' : 'TW';
                const cwLabel = crosswindVal >= 0 ? 'R' : 'L';
                headTailWind = `${hwLabel} ${Math.abs(headwind).toFixed(1)} kt`;
                crossWind = `${cwLabel} ${Math.abs(crosswindVal).toFixed(1)} kt`;
            }

            let flightPathAngle = '-';
            if (num(serverDerived?.flight_path_angle_deg) !== null) {
                const fpa = num(serverDerived.flight_path_angle_deg);
                flightPathAngle = `${fpa >= 0 ? '+' : ''}${fpa.toFixed(2)}°`;
            } else if (verticalRate !== null && gs !== null && gs > 1) {
                const gsFeetPerMin = gs * 101.2686;
                const fpa = Math.atan2(verticalRate, gsFeetPerMin) * 180 / Math.PI;
                flightPathAngle = `${fpa >= 0 ? '+' : ''}${fpa.toFixed(2)}°`;
            }

            let climbGradient = '-';
            if (num(serverDerived?.climb_gradient_ft_nm) !== null) {
                const gradient = num(serverDerived.climb_gradient_ft_nm);
                climbGradient = `${gradient >= 0 ? '+' : ''}${gradient.toFixed(0)} ft/NM`;
            } else if (verticalRate !== null && gs !== null && gs > 1) {
                const gradient = (verticalRate * 60.0) / gs;
                climbGradient = `${gradient >= 0 ? '+' : ''}${gradient.toFixed(0)} ft/NM`;
            }

            let etaToStation = '-';
            if (num(serverDerived?.eta_station_sec) !== null) {
                const totalSec = Math.max(0, Math.round(num(serverDerived.eta_station_sec)));
                const whole = Math.floor(totalSec / 60);
                const sec = totalSec % 60;
                etaToStation = `${whole}m ${sec}s`;
            } else {
                const distanceNm = num(aircraft.distance);
                if (distanceNm !== null && gs !== null && gs > 30) {
                    const mins = (distanceNm / gs) * 60;
                    const whole = Math.floor(mins);
                    const sec = Math.round((mins - whole) * 60);
                    etaToStation = `${whole}m ${sec}s`;
                }
            }

            const turnRateSource = num(serverDerived?.turn_rate_deg_sec) ?? trackRate;
            const turnRateText = turnRateSource !== null ? `${turnRateSource >= 0 ? '+' : ''}${turnRateSource.toFixed(2)}°/s` : '-';

            return {
                headingSource,
                trackHeadingError,
                headTailWind,
                crossWind,
                flightPathAngle,
                climbGradient,
                etaToStation,
                turnRateText
            };
        },

        // RESTORING formatAircraftDetails
        formatAircraftDetails() {
            if (!this.selectedAircraft) return '';

            const aircraft = this.selectedAircraft;
            const adsbData = aircraft.adsb || {};
            
            // Calculate seconds since last seen
            let lastSeenText = '-';
            let lastSeenSeconds = '';
            if (aircraft.last_seen) {
                const secondsAgo = Math.floor((new Date() - new Date(aircraft.last_seen)) / 1000);
                lastSeenText = this.formatDate(aircraft.last_seen, true);
                lastSeenSeconds = `${secondsAgo}s`;
            }

            // Calculate first seen text with seconds ago
            let firstSeenText = '-';
            let firstSeenSeconds = '';
            if (aircraft.created_at) {
                const secondsAgo = Math.floor((new Date() - new Date(aircraft.created_at)) / 1000);
                firstSeenText = this.formatDate(aircraft.created_at, true);
                firstSeenSeconds = `${secondsAgo}s`;
            }

            // Calculate takeoff time text with seconds ago
            let takeoffTimeText = '-';
            let takeoffSeconds = '';
            if (aircraft.DateTookoff || aircraft.date_tookoff) {
                const takeoffTime = aircraft.DateTookoff || aircraft.date_tookoff;
                const secondsAgo = Math.floor((new Date() - new Date(takeoffTime)) / 1000);
                takeoffTimeText = this.formatDate(takeoffTime, true);
                takeoffSeconds = `${secondsAgo}s`;
            }

            // Calculate landing time text with seconds ago
            let landingTimeText = '-';
            let landingSeconds = '';
            if (aircraft.DateLanded || aircraft.date_landed) {
                const landingTime = aircraft.DateLanded || aircraft.date_landed;
                const secondsAgo = Math.floor((new Date() - new Date(landingTime)) / 1000);
                landingTimeText = this.formatDate(landingTime, true);
                landingSeconds = `${secondsAgo}s`;
            }

            // Get BSDB data if available - use it to enrich basic info
            const bsdbData = aircraft.bsdb || {};

            // Prefer BSDB data over ADSB data for type and registration
            const aircraftType = bsdbData.type || adsbData.t || '-';
            const aircraftReg = bsdbData.registration || adsbData.r || '-';
            const aircraftOperator = bsdbData.registered_owners || '-';
            const derived = this.getATCDerivedMetrics(aircraft);

            const fields = [
                ['Basic Info', [
                    ['Callsign', aircraft.flight?.trim() || '-'],
                    ['Airline', aircraft.airline || '-'],
                    ['Operator', aircraftOperator],
                    ['Country', aircraft.airline_country || '-'],
                    ['Hex', aircraft.hex],
                    ['Type', aircraftType],
                    ['Registration', aircraftReg],
                    ['Category', adsbData.category || '-'],
                    ['Squawk', adsbData.squawk || '-'],
                    ['First Seen', firstSeenText, firstSeenSeconds],
                    ['Last Seen', lastSeenText, lastSeenSeconds]
                ]],
                ['Status', [
                    ['On Ground', aircraft.on_ground ? 'Yes' : 'No'],
                    ['Phase', this.getCurrentPhase(aircraft)],
                    ['Takeoff Time', takeoffTimeText, takeoffSeconds],
                    ['Landing Time', landingTimeText, landingSeconds]
                ]],
                ['Position', [
                    ['Latitude', adsbData.lat?.toFixed(6) || '-'],
                    ['Longitude', adsbData.lon?.toFixed(6) || '-'],
                    ['Distance (NM)', aircraft.distance ?? '-'],
                    ['Altitude (Baro)', `${adsbData.alt_baro ?? '-'} ft`],
                    ['Altitude (Geom)', `${adsbData.alt_geom ?? '-'} ft`],
                    ['Vertical Rate', `${adsbData.baro_rate ?? '-'} ft/min`]
                ]],
                ['Speed & Direction', [
                    ['Ground Speed', `${adsbData.gs ?? '-'} kts`],
                    ['True Airspeed', `${adsbData.tas ?? '-'} kts`],
                    ['IAS', `${adsbData.ias ?? '-'} kts`],
                    ['TAS', `${adsbData.tas ?? '-'} kts`],
                    ['Mach', adsbData.mach ?? '-'],
                    ['Track', `${adsbData.track ?? '-'}°`],
                    ['Mag Heading', `${adsbData.mag_heading ?? '-'}°`],
                    ['True Heading', `${adsbData.true_heading ?? '-'}°`]
                ]],
                ['Navigation', [
                    ['Nav QNH', `${adsbData.nav_qnh ?? '-'} hPa`],
                    ['Nav Altitude MCP', `${adsbData.nav_altitude_mcp ?? '-'} ft`],
                    ['Nav Altitude FMS', `${adsbData.nav_altitude_fms ?? '-'} ft`],
                    ['Nav Heading', `${adsbData.nav_heading ?? '-'}°`]
                ]],
                ['Weather', [
                    ['Wind Direction', `${adsbData.wd ?? '-'}°`],
                    ['Wind Speed', `${adsbData.ws ?? '-'} kts`],
                    ['OAT', `${adsbData.oat ?? '-'}°C`],
                    ['TAT', `${adsbData.tat ?? '-'}°C`]
                ]],
                ['ATC Derived', [
                    ['Heading Source', derived.headingSource],
                    ['Track-Heading Error', derived.trackHeadingError],
                    ['Head/Tail Wind', derived.headTailWind],
                    ['Crosswind (L/R)', derived.crossWind],
                    ['Flight Path Angle', derived.flightPathAngle],
                    ['Climb Gradient', derived.climbGradient],
                    ['Turn Rate', derived.turnRateText],
                    ['ETA to Station', derived.etaToStation]
                ]],
                ['ADSB Info', [
                    ['Version', adsbData.version ?? '-'],
                    ['NIC', adsbData.nic ?? '-'],
                    ['NACp', adsbData.nac_p ?? '-'],
                    ['NACv', adsbData.nac_v ?? '-'],
                    ['SIL', adsbData.sil ?? '-'],
                    ['SIL Type', adsbData.sil_type || '-'],
                    ['GVA', adsbData.gva ?? '-'],
                    ['SDA', adsbData.sda ?? '-']
                ]],
                ['Signal', [
                    ['Messages', adsbData.messages ?? '-'],
                    ['Seen', `${adsbData.seen ?? '-'}s`], 
                    ['RSSI', `${adsbData.rssi ?? '-'} dBm`]
                ]]
            ];

            // Initialize collapsible sections state if not already done
            if (!this.collapsibleSections) {
                this.collapsibleSections = {};
                fields.forEach(([category]) => {
                    // Default to expanded, except for ADSB Info and Signal which are collapsed by default
                    this.collapsibleSections[category] = !['ADSB Info', 'Signal'].includes(category);
                });
            }
            
            return `
                <div class="grid grid-cols-2 gap-x-4 gap-y-2 text-xs">
                    ${fields.map(([category, items]) => `
                        <div class="col-span-2 mt-2 first:mt-0">
                            <div class="flex justify-between items-center mb-1 cursor-pointer relative pb-1"
                                 onclick="Alpine.store('atc').toggleSection('${category}')">
                                <h4 class="text-highlight font-bold text-[11px] uppercase tracking-wider">${category}</h4>
                                <i class="fas fa-chevron-${this.collapsibleSections[category] ? 'down' : 'right'} text-highlight/70 text-xs"></i>
                                <!-- Subtle green underline -->
                                <div class="absolute bottom-0 left-0 right-0 h-px bg-highlight/30"></div>
                            </div>
                            <div class="grid grid-cols-2 gap-x-4 gap-y-0.5 transition-all duration-300 overflow-hidden"
                                 style="${this.collapsibleSections[category] ? '' : 'max-height: 0; opacity: 0; margin: 0; padding: 0;'}">
                                ${items.map(([label, value, seconds]) => `
                                    <div class="contents">
                                        <div class="text-text/70">${label}:</div>
                                        <div class="font-mono">
                                            ${seconds ? `
                                                <span>${value}</span>
                                                <span class="text-text/70 text-[10px] ml-1">${seconds}</span>
                                            ` : value}
                                        </div>
                                    </div>
                                `).join('')}
                            </div>
                        </div>
                    `).join('')}
                </div>
            `;
        },

        // RESTORING toggleSort
        toggleSort(column) {
            if (this.sortColumn === column) {
                this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
            } else {
                this.sortColumn = column;
                this.sortDirection = 'asc';
            }
            // No need to explicitly call applyFilters here, as the sorted list is a computed property
            // that will react to changes in sortColumn or sortDirection.
            // However, if applyFilters also handles map updates, it might be needed if sorting should affect map directly.
            // For now, assuming filteredAircraft computed property handles table refresh.
        },

        // RESTORING toggleStatusFilter and toggleGroundedAircraft
        toggleStatusFilter(statusKey) {
            if (this.settings.statusFilters.hasOwnProperty(statusKey)) {
                this.settings.statusFilters[statusKey] = !this.settings.statusFilters[statusKey];
                this.saveSettings();
                // this.applyFilters(); // applyFilters calls mapManager.applyFiltersAndRefreshView()
                if (this.mapManager) this.mapManager.applyFiltersAndRefreshView();
            }
        },
        
        // Toggle collapsible sections in aircraft details
        toggleSection(category) {
            if (!this.collapsibleSections) {
                this.collapsibleSections = {};
            }
            this.collapsibleSections[category] = !this.collapsibleSections[category];
        },

        // Toggle Air aircraft visibility
        toggleAirAircraft() {
            console.log('[Alpine Store] toggleAirAircraft called, current state:', this.settings.showAirAircraft);
            this.settings.showAirAircraft = !this.settings.showAirAircraft;

            this.saveSettings();
            this.applyFilters();
            this.refreshAlertsDisplay(); // Refresh alerts based on new filter

            // Update map visibility including tracks
            if (this.mapManager) {
                this.mapManager.applyFiltersAndRefreshView();
            }
        },

        // Toggle Ground aircraft visibility
        toggleGroundAircraft() {
            //console.log('[Alpine Store] toggleGroundAircraft called, current state:', this.settings.showGroundAircraft);
            this.settings.showGroundAircraft = !this.settings.showGroundAircraft;

            this.saveSettings();
            this.applyFilters();
            this.refreshAlertsDisplay(); // Refresh alerts based on new filter

            // Update map visibility including tracks
            if (this.mapManager) {
                this.mapManager.applyFiltersAndRefreshView();
            }
        },

        // Toggle flight phase filter
        togglePhaseFilter(phase) {
            if (!this.settings.phaseFilters) {
                this.settings.phaseFilters = { CRZ: true, CLB: true, DEP: true, APP: true, ARR: true, TAX: true, 'T/O': true, 'T/D': true, NEW: true, UNK: true };
            }
            this.settings.phaseFilters[phase] = !this.settings.phaseFilters[phase];
            this.saveSettings();
            this.applyFilters();
            this.refreshAlertsDisplay(); // Refresh alerts based on new filter

            // Update map visibility including tracks
            if (this.mapManager) {
                this.mapManager.applyFiltersAndRefreshView();
            }
        },

        // Simulation methods
        async createSimulatedAircraft(lat, lon, altitude, heading, speed, verticalRate) {
            try {
                const response = await this.fetchWithTimeout(`${API_BASE_URL}/simulation/aircraft`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        lat: lat,
                        lon: lon,
                        altitude: altitude,
                        heading: heading,
                        speed: speed,
                        vertical_rate: verticalRate
                    })
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    throw new Error(`Failed to create simulated aircraft: ${errorText}`);
                }

                const result = await response.json();
                console.log('Created simulated aircraft:', result.aircraft);
                
                // Close the modal
                this.showCreateSimulatedAircraft = false;
                this.simulationModal.mapClickMode = false;
                
                return result.aircraft;
            } catch (error) {
                console.error('Error creating simulated aircraft:', error);
                throw error;
            }
        },

        // Set all simulation controls at once via WebSocket
        setSimulationControls(hex, heading, speed, verticalRate) {
            // Find the aircraft (this.aircraft is an object, not array)
            const aircraft = this.aircraft[hex];
            if (!aircraft) {
                console.error('Aircraft not found:', hex);
                return;
            }

            // Initialize simulation_controls if needed
            if (!aircraft.simulation_controls) {
                aircraft.simulation_controls = {};
            }

            // Update local values immediately for responsive UI
            aircraft.simulation_controls.target_heading = parseFloat(heading);
            aircraft.simulation_controls.target_speed = parseFloat(speed);
            aircraft.simulation_controls.target_vertical_rate = parseFloat(verticalRate);

            // Send update via WebSocket
            if (wsClient && wsClient.connection && wsClient.connection.readyState === WebSocket.OPEN) {
                const message = {
                    type: 'simulation_control_update',
                    data: {
                        hex: hex,
                        heading: parseFloat(heading),
                        speed: parseFloat(speed),
                        vertical_rate: parseFloat(verticalRate)
                    }
                };
                wsClient.connection.send(JSON.stringify(message));
                console.log(`Updated simulation controls via WebSocket for ${hex}: hdg=${heading} spd=${speed} vs=${verticalRate}`);
            } else {
                console.error('WebSocket not connected, cannot update simulation controls');
            }
        },

        async removeSimulatedAircraft(hex) {
            try {
                const response = await this.fetchWithTimeout(`${API_BASE_URL}/simulation/aircraft/${hex}`, {
                    method: 'DELETE'
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    throw new Error(`Failed to remove simulated aircraft: ${errorText}`);
                }

                console.log('Removed simulated aircraft:', hex);
            } catch (error) {
                console.error('Error removing simulated aircraft:', error);
                throw error;
            }
        },

        setSimulationPositionFromMap() {
            this.simulationModal.mapClickMode = !this.simulationModal.mapClickMode;
            if (this.simulationModal.mapClickMode) {
                console.log('Click on map to set simulated aircraft position');
                // Add map click handler
                if (this.mapManager && this.mapManager.map) {
                    this.mapManager.enableSimulationPositionMode();
                }
            } else {
                // Disable map click handler
                if (this.mapManager && this.mapManager.map) {
                    this.mapManager.disableSimulationPositionMode();
                }
            }
        },

        generateRandomPosition() {
            // Generate random position within 50 nautical miles of airport
            const centerLat = this.stationLatitude || 43.6777; // CYYZ default
            const centerLon = this.stationLongitude || -79.6248;
            
            // 50 nautical miles = ~0.833 degrees latitude
            const maxDistanceDeg = 0.833;
            
            // Random angle and distance
            const angle = Math.random() * 2 * Math.PI;
            const distance = Math.random() * maxDistanceDeg;
            
            // Calculate new position
            const latOffset = distance * Math.cos(angle);
            const lonOffset = distance * Math.sin(angle) / Math.cos(centerLat * Math.PI / 180);
            
            this.simulationModal.lat = centerLat + latOffset;
            this.simulationModal.lon = centerLon + lonOffset;
            
            console.log(`Generated random position: ${this.simulationModal.lat.toFixed(6)}, ${this.simulationModal.lon.toFixed(6)}`);
        },

        onMapClickForSimulation(lat, lon) {
            if (this.simulationModal.mapClickMode) {
                this.simulationModal.lat = lat;
                this.simulationModal.lon = lon;
                this.simulationModal.mapClickMode = false;
                
                // Disable map click mode
                if (this.mapManager && this.mapManager.map) {
                    this.mapManager.disableSimulationPositionMode();
                }
                
                console.log(`Set simulation position from map: ${lat.toFixed(6)}, ${lon.toFixed(6)}`);
            }
        },

        // Get phase color class (matches navigation bar colors)
        getPhaseColorClass(phase) {
            const phaseColorMap = {
                'NEW': 'text-gray-400',
                'TAX': 'text-purple-400',
                'T/O': 'text-orange-400',
                'CLB': 'text-lime-400',
                'DEP': 'text-green-400',
                'CRZ': 'text-blue-400',
                'ARR': 'text-pink-300',
                'APP': 'text-yellow-400',
                'T/D': 'text-teal-400',
                'UNK': 'text-slate-400'
            };
            return phaseColorMap[phase] || 'text-gray-400';
        },

        // Get phase icon class
        getPhaseIconClass(phase) {
            const phaseIconMap = {
                'NEW': 'fa-plane',
                'TAX': 'fa-taxi',
                'T/O': 'fa-plane-departure',
                'DEP': 'fa-plane-up',
                'CRZ': 'fa-plane',
                'ARR': 'fa-plane-down',
                'APP': 'fa-plane-arrival',
                'T/D': 'fa-plane-arrival'
            };
            return phaseIconMap[phase] || 'fa-plane';
        },

        // Check if phase alert should be shown based on phase filter settings
        shouldShowPhaseAlert(phase) {
            // Only show alerts for phases that are currently enabled in filters
            // phaseFilters[phase] === false means phase is filtered OUT
            // phaseFilters[phase] !== false means phase is enabled (true or undefined)
            return this.settings.phaseFilters && this.settings.phaseFilters[phase] !== false;
        },
        
        // Get count of grounded aircraft
        getGroundedAircraftCount() {
            return Object.values(this.aircraft).filter(aircraft => aircraft.on_ground).length;
        },
        
        // Get seconds since last seen for an aircraft
        getSecondsSinceLastSeen(aircraft) {
            if (!aircraft.last_seen) return 'Unknown';

            // Reactive dependency so Alpine reevaluates this value every second
            // when the app clock updates.
            this.currentTime;

            const lastSeen = new Date(aircraft.last_seen);
            const now = new Date();
            return Math.floor((now - lastSeen) / 1000);
        },

        shouldShowLastSeenBadge(aircraft) {
            const secondsSince = this.getSecondsSinceLastSeen(aircraft);
            return Number.isFinite(secondsSince) && secondsSince >= 5;
        },

        formatLastSeenAgo(aircraft) {
            if (!aircraft?.last_seen) return '—';

            const secondsAgo = Math.max(0, Math.floor((Date.now() - new Date(aircraft.last_seen).getTime()) / 1000));
            if (!Number.isFinite(secondsAgo)) return '—';
            if (secondsAgo < 60) return `${secondsAgo}s`;

            const minutesAgo = Math.floor(secondsAgo / 60);
            if (minutesAgo < 60) return `${minutesAgo}m`;

            const hoursAgo = Math.floor(minutesAgo / 60);
            if (hoursAgo < 24) return `${hoursAgo}h`;

            const daysAgo = Math.floor(hoursAgo / 24);
            return `${daysAgo}d`;
        },

        // Highlight search matches with red underline
        highlightSearchMatch(text) {
            if (!this.searchTerm || !text) return text;
            
            const searchLower = this.searchTerm.toLowerCase();
            const textLower = text.toLowerCase();
            const index = textLower.indexOf(searchLower);
            
            if (index === -1) return text;
            
            const before = text.substring(0, index);
            const match = text.substring(index, index + this.searchTerm.length);
            const after = text.substring(index + this.searchTerm.length);
            
            return `${before}<span class="border-b border-red-400">${match}</span>${after}`;
        },

        // Cycle to next aircraft in the filtered list
        cycleToNextAircraft() {
            const filtered = this.filteredAircraft;
            if (filtered.length === 0) return;
            
            if (!this.selectedAircraft) {
                // Select first aircraft
                this.selectedAircraft = filtered[0];
                if (this.mapManager) {
                    this.mapManager.updateVisualState(filtered[0].hex, true);
                    this.mapManager.centerOnAircraft(filtered[0]);
                }
                return;
            }
            
            // Find current aircraft index
            const currentIndex = filtered.findIndex(aircraft => aircraft.hex === this.selectedAircraft.hex);
            if (currentIndex === -1) {
                // Current aircraft not in filtered list, select first
                this.selectedAircraft = filtered[0];
            } else {
                // Select next aircraft (wrap around to beginning)
                const nextIndex = (currentIndex + 1) % filtered.length;
                this.selectedAircraft = filtered[nextIndex];
            }
            
            if (this.mapManager) {
                this.mapManager.updateVisualState(this.selectedAircraft.hex, true);
                this.mapManager.centerOnAircraft(this.selectedAircraft);
            }
        },

        // Cycle to previous aircraft in the filtered list
        cycleToPreviousAircraft() {
            const filtered = this.filteredAircraft;
            if (filtered.length === 0) return;
            
            if (!this.selectedAircraft) {
                // Select last aircraft
                this.selectedAircraft = filtered[filtered.length - 1];
                if (this.mapManager) {
                    this.mapManager.updateVisualState(filtered[filtered.length - 1].hex, true);
                    this.mapManager.centerOnAircraft(filtered[filtered.length - 1]);
                }
                return;
            }
            
            // Find current aircraft index
            const currentIndex = filtered.findIndex(aircraft => aircraft.hex === this.selectedAircraft.hex);
            if (currentIndex === -1) {
                // Current aircraft not in filtered list, select last
                this.selectedAircraft = filtered[filtered.length - 1];
            } else {
                // Select previous aircraft (wrap around to end)
                const prevIndex = currentIndex === 0 ? filtered.length - 1 : currentIndex - 1;
                this.selectedAircraft = filtered[prevIndex];
            }
            
            if (this.mapManager) {
                this.mapManager.updateVisualState(this.selectedAircraft.hex, true);
                this.mapManager.centerOnAircraft(this.selectedAircraft);
            }
        },

        // Properties for Aircraft Details Panel (moved from x-data in HTML)
        aircraftDetailsShowHistoryView: false,
        aircraftDetailsHistoryData: [],
        aircraftDetailsFutureData: [],
        aircraftDetailsHindcastData: [],
        aircraftDetailsHistoryCount: 0,
        aircraftDetailsHistoryLoading: false,
        aircraftDetailsHistoryRefreshInterval: null,
        aircraftDetailsCurrentAircraftHexForPanel: null, // New property
        
        // Properties for Proximity View
        showProximityView: false,
        proximityDistance: 5, // Default to 5 NM
        proximityAircraft: [],
        proximityLoading: false,
        proximityHighlightedAircraft: new Set(), // Set of aircraft hex codes highlighted in proximity view

        // Properties for Phase History (now always shown in Tracks tab)
        phaseHistoryData: [],
        phaseHistoryLoading: false,
        phaseHistoryAircraftHex: null,
        phaseHistoryRefreshInterval: null,
        highlightedAdsbId: null, // For highlighting specific rows in Tracks tab
        
        // Getter to ensure phaseHistoryData is always an array
        get safePhaseHistoryData() {
            return this.phaseHistoryData || [];
        },
        
        // Current time for reactive time calculations
        currentTimeForPhases: new Date(),

        // Methods for Aircraft Details Panel (moved from x-data in HTML)
        setupAircraftDetailsPanel() {
            if (!this.selectedAircraft) { // No aircraft selected, fully close and reset
                this.aircraftDetailsShowHistoryView = false;
                this.aircraftDetailsHistoryData = [];
                this.aircraftDetailsFutureData = [];
                this.aircraftDetailsHindcastData = [];
                this.aircraftDetailsHistoryCount = 0;
                this.aircraftDetailsStopHistoryRefresh();

                // Clear map trails for the previously selected aircraft
                if (this.mapManager && this.aircraftDetailsCurrentAircraftHexForPanel) {
                    this.mapManager.clearAircraftTrails(this.aircraftDetailsCurrentAircraftHexForPanel);
                }

                this.aircraftDetailsCurrentAircraftHexForPanel = null;
                this.showProximityView = false;
                this.stopProximityRefresh();
                this.clearProximityView();
                this.phaseHistoryData = [];
                this.phaseHistoryAircraftHex = null;
                this.stopPhaseHistoryRefresh();
                return;
            }

            if (this.selectedAircraft.hex !== this.aircraftDetailsCurrentAircraftHexForPanel) {
                // Store current view state
                const wasInHistoryView = this.aircraftDetailsShowHistoryView;
                const wasInProximityView = this.showProximityView;
                
                // Clear data but maintain view state
                this.aircraftDetailsHistoryData = [];
                this.aircraftDetailsHistoryCount = 0;
                this.aircraftDetailsStopHistoryRefresh();
                this.stopProximityRefresh();
                this.clearProximityView();
                this.phaseHistoryData = [];
                this.stopPhaseHistoryRefresh();
                
                // Clear map trails for the previous aircraft immediately (O(1) operation)
                if (this.mapManager && this.aircraftDetailsCurrentAircraftHexForPanel) {
                    this.mapManager.clearAircraftTrails(this.aircraftDetailsCurrentAircraftHexForPanel);
                }
                
                // Clear stale track data from the aircraft object to prevent showing previous aircraft's tracks
                if (this.selectedAircraft.historyData) {
                    delete this.selectedAircraft.historyData;
                }
                if (this.selectedAircraft.futureData) {
                    delete this.selectedAircraft.futureData;
                }
                
                // Clear store track data immediately to prevent showing stale data
                this.aircraftDetailsHistoryData = [];
                this.aircraftDetailsFutureData = [];
                this.aircraftDetailsHindcastData = [];
                
                // Update current aircraft hex
                this.aircraftDetailsCurrentAircraftHexForPanel = this.selectedAircraft.hex;
                
                // Reload data for the new aircraft based on current view
                // Restore the previous view state
                if (wasInHistoryView) {
                    this.aircraftDetailsShowHistoryView = true;
                    this.showProximityView = false;
                } else if (wasInProximityView) {
                    this.showProximityView = true;
                    this.aircraftDetailsShowHistoryView = false;
                    this.loadProximityData();
                    this.startProximityRefresh();
                } else {
                    // Reset to default view if no special view was active
                    this.aircraftDetailsShowHistoryView = false;
                    this.showProximityView = false;
                }
                
                // Phase history is loaded as part of the tracks API response
                this.phaseHistoryAircraftHex = this.selectedAircraft.hex;

                // Load tracks data (includes phase history) for map trails when aircraft is selected
                this.aircraftDetailsLoadTracks();
                
                // Start refresh interval to keep map trails updated
                this.aircraftDetailsStartHistoryRefresh();
            }
        },

        aircraftDetailsStartHistoryRefresh() {
            if (this.aircraftDetailsHistoryRefreshInterval) {
                clearInterval(this.aircraftDetailsHistoryRefreshInterval);
            }
            const refreshRate = 5000; // Fixed 5 second refresh for aircraft details history
            
            this.aircraftDetailsHistoryRefreshInterval = setInterval(() => {
                if (this.selectedAircraft) {
                    this.aircraftDetailsLoadTracks(true); // Pass true for isRefresh
                }
            }, refreshRate);
        },

        aircraftDetailsStopHistoryRefresh() {
            if (this.aircraftDetailsHistoryRefreshInterval) {
                clearInterval(this.aircraftDetailsHistoryRefreshInterval);
                this.aircraftDetailsHistoryRefreshInterval = null;
            }
        },

        async aircraftDetailsLoadTracks(isRefresh = false) {
            if (!this.selectedAircraft) return;

            const hex = this.selectedAircraft.hex;

            // Initialize tracks pending requests if needed
            if (!this.pendingRequests.tracks) {
                this.pendingRequests.tracks = new Map();
            }

            // Check if there's already a pending tracks request for this aircraft
            if (this.pendingRequests.tracks.get(hex)) {
                return;
            }

            // Set pending flag
            this.pendingRequests.tracks.set(hex, true);
            
            if (!isRefresh) {
                this.aircraftDetailsHistoryLoading = true;
            }
            
            try {
                // Add limit parameter to control track length
                const limit = this.settings.tracksLimit || 1000;
                const url = `${API_BASE_URL}/aircraft/${hex}/tracks?limit=${limit}`;

                const response = await this.fetchWithTimeout(url);

                if (!response.ok) {
                    this.aircraftDetailsHistoryData = [];
                    this.aircraftDetailsFutureData = [];
                this.aircraftDetailsHindcastData = [];
                    this.aircraftDetailsHistoryCount = 0;
                    this.aircraftDetailsHistoryLoading = false;
                    return;
                }

                const data = await response.json();

                // Split combined tracks response into history, future, and hindcast
                this.aircraftDetailsHistoryData = data.history || [];
                this.aircraftDetailsFutureData = data.future || [];
                this.aircraftDetailsHindcastData = data.hindcast || [];
                this.aircraftDetailsHistoryCount = this.aircraftDetailsHistoryData.length;

                // Populate phase history from the same response (avoids separate API call)
                const phaseHistory = data.phase_history || [];
                if (phaseHistory.length > 0) {
                    this.phaseHistoryData = phaseHistory.map((phase, index) => ({
                        ...phase,
                        is_current: index === 0,
                        id: phase.id !== undefined ? phase.id : `phase-${index}`
                    }));
                } else {
                    this.phaseHistoryData = [];
                }

                // Update tracks mini-map and main map trails
                if (this.mapManager && this.mapManager.updateTracksMiniMap) {
                    this.mapManager.updateTracksMiniMap();
                }
                if (this.mapManager && this.mapManager.updateFlightPaths) {
                    this.mapManager.updateFlightPaths();
                }
            } catch (error) {
                this.aircraftDetailsHistoryData = [];
                this.aircraftDetailsFutureData = [];
                this.aircraftDetailsHindcastData = [];
                this.aircraftDetailsHistoryCount = 0;
                this.phaseHistoryData = [];
            } finally {
                this.aircraftDetailsHistoryLoading = false;
                this.pendingRequests.tracks.delete(hex);
            }
        },
        
        // Proximity View Methods
        proximityRefreshInterval: null,
        
        startProximityRefresh() {
            // Clear any existing interval
            this.stopProximityRefresh();
            
            // Fixed refresh rate for proximity data
            const refreshRate = 5000; // 5 seconds
            
            // Set up new interval
            this.proximityRefreshInterval = setInterval(() => {
                if (this.showProximityView && this.selectedAircraft) {
                    this.loadProximityData(true); // Pass true for isRefresh
                }
            }, refreshRate);
        },
        
        stopProximityRefresh() {
            if (this.proximityRefreshInterval) {
                clearInterval(this.proximityRefreshInterval);
                this.proximityRefreshInterval = null;
            }
        },
        
        async loadProximityData(isRefresh = false) {
            if (!this.selectedAircraft) return;
            
            // Check if there's already a pending proximity request
            if (this.pendingRequests.proximity) {
                console.log('Proximity request already in progress, skipping...');
                return;
            }

            this.pendingRequests.proximity = true;
            
            if (!isRefresh) {
                this.proximityLoading = true;
            }
            
            try {
                // Build the URL with the distance_nm and ref_hex parameters
                const url = `${API_BASE_URL}/aircraft?distance_nm=${this.proximityDistance}&ref_hex=${this.selectedAircraft.hex}`;
                
                const response = await this.fetchWithTimeout(url);
                if (!response.ok) {
                    console.error(`Error fetching proximity data: ${response.status}`);
                    this.proximityAircraft = [];
                    this.proximityLoading = false;
                    return;
                }
                
                const data = await response.json();
                
                // Filter out the reference aircraft
                this.proximityAircraft = (data.aircraft || []).filter(aircraft =>
                    aircraft.hex !== this.selectedAircraft.hex
                );
                
                // Draw the proximity circle on the map
                this.drawProximityCircle();
                
                // Highlight the aircraft labels
                this.highlightProximityAircraft();
            } catch (error) {
                if (error.message.includes('timeout')) {
                    console.warn('Proximity request timed out after 5 seconds');
                } else {
                    console.error('Error fetching proximity data:', error);
                }
                this.proximityAircraft = [];
            } finally {
                this.proximityLoading = false;
                // Always clear the pending flag
                this.pendingRequests.proximity = false;
            }
        },

        clearProximityView() {
            // Stop the refresh interval
            this.stopProximityRefresh();
            
            // Remove the proximity circle from the map
            if (this.mapManager) {
                this.mapManager.removeProximityCircle();
            }
            
            // Remove the highlighting from aircraft labels
            this.removeProximityHighlighting();
            
            // Reset the proximity data
            this.proximityAircraft = [];
            
            // Clear the highlighted aircraft set
            this.proximityHighlightedAircraft.clear();
        },
        
        // Methods for handling hover effects on proximity aircraft
        highlightProximityAircraftOnHover(hex) {
            if (!this.mapManager) return;
            
            // Find the aircraft label element
            const markers = this.mapManager.markers[hex];
            if (!markers || !markers.label) return;
            
            const labelElement = markers.label.getElement();
            if (!labelElement) return;
            
            const labelDiv = labelElement.querySelector('div');
            if (!labelDiv) return;
            
            // Add the hover class
            labelDiv.classList.add('proximity-highlight-hover');
        },
        
        unhighlightProximityAircraftOnHover() {
            if (!this.mapManager) return;
            
            // Remove hover class from all aircraft labels
            Object.keys(this.mapManager.markers).forEach(hex => {
                const markers = this.mapManager.markers[hex];
                if (!markers || !markers.label) return;
                
                const labelElement = markers.label.getElement();
                if (!labelElement) return;
                
                const labelDiv = labelElement.querySelector('div');
                if (!labelDiv) return;
                
                // Remove the hover class but keep the proximity highlight class
                labelDiv.classList.remove('proximity-highlight-hover');
            });
        },

        drawProximityCircle() {
            if (!this.mapManager || !this.selectedAircraft || !this.selectedAircraft.adsb) return;
            
            const position = [this.selectedAircraft.adsb.lat, this.selectedAircraft.adsb.lon];
            this.mapManager.drawProximityCircle(position, this.proximityDistance);
        },

        highlightProximityAircraft() {
            if (!this.mapManager) return;
            
            // Create a set of hex codes for quick lookup
            const proximityHexSet = new Set(this.proximityAircraft.map(a => a.hex));
            
            // Call the map manager to highlight these aircraft
            this.mapManager.highlightProximityAircraft(proximityHexSet);
        },

        removeProximityHighlighting() {
            if (!this.mapManager) return;
            
            this.mapManager.removeProximityHighlighting();
        },

        // Show phase history for an aircraft (now always shown in Tracks tab)
        showPhaseHistory(hex) {
            if (!hex) return;

            // Set the aircraft hex for phase history
            this.phaseHistoryAircraftHex = hex;

            // Switch to tracks view to show phase history
            this.aircraftDetailsShowHistoryView = true;
            this.showProximityView = false;

            // Phase history is loaded as part of the tracks API response
            this.aircraftDetailsLoadTracks();
        },

        // Navigate to Tracks tab and highlight a specific row by ADSB ID
        navigateToTracksWithHighlight(adsbId) {
            if (!adsbId) return;
            
            // Switch to tracks view
            this.aircraftDetailsShowHistoryView = true;
            this.showProximityView = false;
            this.stopProximityRefresh();
            this.clearProximityView();
            
            // Store the ADSB ID to highlight
            this.highlightedAdsbId = adsbId;
            
            // Wait for the view to render, then scroll to the highlighted row
            setTimeout(() => {
                this.scrollToHighlightedRow(adsbId);
            }, 100);
            
            // Clear highlight after 30 seconds
            setTimeout(() => {
                this.highlightedAdsbId = null;
            }, 30000);
        },

        // Scroll to the highlighted row in the Tracks table
        scrollToHighlightedRow(adsbId) {
            if (!adsbId) return;
            
            // Wait a bit more for Alpine.js to render the highlighted row
            setTimeout(() => {
                // Find the history data and locate the index of the matching ADSB ID
                const historyData = this.aircraftDetailsHistoryData;
                if (!historyData || historyData.length === 0) return;
                
                let targetIndex = -1;
                for (let i = 0; i < historyData.length; i++) {
                    if (historyData[i].id && historyData[i].id == adsbId) {
                        targetIndex = i;
                        break;
                    }
                }
                
                if (targetIndex === -1) return;
                
                // Find the tracks view container and scroll to the appropriate position
                const tracksContainer = document.querySelector('[x-show="$store.atc.aircraftDetailsShowHistoryView"] .overflow-x-auto');
                if (!tracksContainer) return;
                
                // Calculate approximate row height and scroll position
                // Account for header rows (Future Predictions header + Historical Positions header)
                const futureDataLength = this.aircraftDetailsFutureData ? this.aircraftDetailsFutureData.length : 0;
                const headerRows = (futureDataLength > 0 ? 1 : 0) + 1; // Future header (if exists) + Historical header
                const totalRowsBeforeTarget = futureDataLength + headerRows + targetIndex;
                
                // Estimate row height (approximately 32px per row)
                const estimatedRowHeight = 32;
                const scrollPosition = totalRowsBeforeTarget * estimatedRowHeight;
                
                // Scroll to the calculated position
                tracksContainer.scrollTo({
                    top: scrollPosition,
                    behavior: 'smooth'
                });
            }, 300);
        },

        // Highlight a specific position on the map when hovering over history row
        highlightPositionOnMap(position) {
            if (!position || typeof position.lat !== 'number' || typeof position.lon !== 'number' || !this.mapManager) return;
            
            // Create a temporary marker for the highlighted position
            this.mapManager.showPositionHighlight(position.lat, position.lon, {
                altitude: position.altitude,
                timestamp: position.timestamp,
                speed_gs: position.speed_gs,
                speed_true: position.speed_true,
                heading: position.true_heading
            });
        },

        // Clear the position highlight from the map
        clearPositionHighlight() {
            if (!this.mapManager) return;
            this.mapManager.clearPositionHighlight();
        },

        // Phase history is now loaded from the tracks API response (aircraftDetailsLoadTracks).
        // No separate fetch or refresh needed.

        // Close phase history view (clears data)
        closePhaseHistory() {
            this.phaseHistoryData = [];
            this.phaseHistoryAircraftHex = null;
            this.stopPhaseHistoryRefresh();
        },

        // Stop automatic refresh for phase history
        stopPhaseHistoryRefresh() {
            if (this.phaseHistoryRefreshInterval) {
                clearInterval(this.phaseHistoryRefreshInterval);
                this.phaseHistoryRefreshInterval = null;
            }
        },
        
        aircraftDetailsGetAltitudeTrend(position, index, isFuture = false) {
            if (!position || typeof position !== 'object') {
                return 'fas fa-arrows-alt-h';
            }

            // Use vertical_speed if available (from new tracks API)
            if (position.vertical_speed !== undefined && position.vertical_speed !== null) {
                if (position.vertical_speed > 100) return 'fas fa-arrow-up';
                if (position.vertical_speed < -100) return 'fas fa-arrow-down';
                return 'fas fa-arrows-alt-h';
            }
            
            // Fallback to altitude difference calculation for old data
            const dataArray = isFuture ? this.aircraftDetailsFutureData : this.aircraftDetailsHistoryData;
            
            if (!dataArray || index === dataArray.length - 1) return 'fas fa-arrows-alt-h';
            const nextPosition = dataArray[index + 1]; // Next is actually previous in time for history, or next in time for future
            if (!nextPosition) return 'fas fa-arrows-alt-h';
            
            // Check if we're using the new PositionMinimal format or the old Position format
            const currentAlt = position.alt_baro !== undefined ? position.alt_baro : position.altitude;
            const nextAlt = nextPosition.alt_baro !== undefined ? nextPosition.alt_baro : nextPosition.altitude;
            if (typeof currentAlt !== 'number' || typeof nextAlt !== 'number') return 'fas fa-arrows-alt-h';
            
            const altDiff = currentAlt - nextAlt;
            if (altDiff > 100) return 'fas fa-arrow-up';
            if (altDiff < -100) return 'fas fa-arrow-down';
            return 'fas fa-arrows-alt-h';
        },

        aircraftDetailsGetAltitudeTrendClass(position, index, isFuture = false) {
            if (!position || typeof position !== 'object') {
                return isFuture ? 'text-highlight/70' : 'text-text';
            }

            const dataArray = isFuture ? this.aircraftDetailsFutureData : this.aircraftDetailsHistoryData;
            
            if (!dataArray || index === dataArray.length - 1) return isFuture ? 'text-highlight/70' : 'text-text';
            const nextPosition = dataArray[index + 1]; // Next is actually previous in time for history, or next in time for future
            if (!nextPosition) return isFuture ? 'text-highlight/70' : 'text-text';
            
            // Check if we're using the new PositionMinimal format or the old Position format
            const currentAlt = position.alt_baro !== undefined ? position.alt_baro : position.altitude;
            const nextAlt = nextPosition.alt_baro !== undefined ? nextPosition.alt_baro : nextPosition.altitude;
            if (typeof currentAlt !== 'number' || typeof nextAlt !== 'number') return isFuture ? 'text-highlight/70' : 'text-text';
            
            const altDiff = currentAlt - nextAlt;
            if (altDiff > 100) return isFuture ? 'text-green-400' : 'text-highlight';
            if (altDiff < -100) return isFuture ? 'text-red-400' : 'text-danger';
            return isFuture ? 'text-highlight/70' : 'text-text';
        },

        formatTrackAltitude(value) {
            if (typeof value !== 'number' || !Number.isFinite(value)) return '-';
            return Math.round(value / 100) * 100;
        },

        formatTrackSpeed(value) {
            if (typeof value !== 'number' || !Number.isFinite(value)) return '-';
            return Math.round(value);
        },

        formatTrackDistance(value) {
            if (typeof value !== 'number' || !Number.isFinite(value)) return '-';
            return value;
        },

        // Check if there's a significant time gap (>10 minutes) between this position and the previous one
        hasSignificantTimeGap(position, index, isFuture = false) {
            const dataArray = isFuture ? this.aircraftDetailsFutureData : this.aircraftDetailsHistoryData;
            
            if (index === 0) return false; // First item has no previous to compare
            
            const prevPosition = dataArray[index - 1];
            if (!prevPosition || !prevPosition.timestamp || !position.timestamp) {
                return false;
            }
            
            const currentTime = new Date(position.timestamp);
            const prevTime = new Date(prevPosition.timestamp);
            const timeDiffMinutes = Math.abs(currentTime - prevTime) / (1000 * 60); // Convert to minutes
            
            return timeDiffMinutes > 10; // Return true if gap is more than 10 minutes
        },

        // Methods
        async init() {
            if (isAppInitialized) {
                console.warn("Application already initialized. Skipping init() call.");
                return;
            }

            if (!document.getElementById('map')) {
                console.error("Map container #map not found in DOM. Aborting initialization.");
                return;
            }
            
            if (!audioClient || !this.mapManager) { // Ensure mapManager is also ready
                return; 
            }
            console.log("Alpine store init() invoked.");

            try {
                await this.fetchStationData();
                await this.fetchWeatherData();
                await this.fetchADSBSourceStatus();
                // Initialize map using MapManager
                if (this.mapManager && !this.mapManager.map) {
                    this.mapManager.initMap();

                    // Apply persisted map style/overlay visibility after overlay registry is initialized
                    this.applyMapDisplaySettings();
                    
                    // Draw range rings after map is initialized
                    this.updateStationRings();
                    
                    // Draw runways if data is available
                    if (this.runwayData) {
                        this.mapManager.drawRunways(this.runwayData);
                    }
                } else if (this.mapManager && this.mapManager.map) {
                     console.warn("Alpine store init: MapManager's map already initialized.");
                } else {
                    console.error("Alpine store init: mapManager not available for map initialization.");
                }

                this.fetchAudioFrequencies();

                // Fetch reference data (airports, navaids, runways) after map init
                this.fetchReferenceData();

                // CRITICAL FIX: Check server config to determine if WebSocket streaming is enabled
                await this.initAircraftDataSource();

                // Initialize previous settings for change detection
                this.previousSettings = { ...this.settings };

                // Manage the current time and Zulu time update interval
                if (this.timeUpdateIntervalId) {
                    clearInterval(this.timeUpdateIntervalId);
                }
                this.updateCurrentTime(); // Initial call
                this.timeUpdateIntervalId = setInterval(() => {
                    this.updateCurrentTime();
                    this.currentTimeForPhases = new Date(); // Update time for phase history
                    if (this.connected && this.lastUpdate) {
                        this.lastUpdateSeconds = Math.floor((new Date() - this.lastUpdate) / 1000);
                    }
                    // Purge stale aircraft from the store every 30s
                    // Uses the same lastSeenMinutes threshold the server uses on bulk load
                    this._staleCleanupCounter = (this._staleCleanupCounter || 0) + 1;
                    if (this._staleCleanupCounter >= 30) {
                        this._staleCleanupCounter = 0;
                        this._purgeStaleAircraft();
                    }
                }, 1000);

                // New interval for updating "seconds since last audio"
                if (this.lastAudioUpdateIntervalId) {
                    clearInterval(this.lastAudioUpdateIntervalId);
                }
                this.updateSecondsSinceLastAudio(); // Initial call
                this.lastAudioUpdateIntervalId = setInterval(() => {
                    this.updateSecondsSinceLastAudio();
                }, 1000);

                // Poll map performance metrics every 2s (debug only, lightweight)
                if (this.mapPerfUpdateIntervalId) {
                    clearInterval(this.mapPerfUpdateIntervalId);
                }
                this.updateMapPerformanceStats();
                this.mapPerfUpdateIntervalId = setInterval(() => {
                    this.updateMapPerformanceStats();
                }, 2000);

                // Watch for hover changes and update map visual state
                let previousHoveredHex = null;
                Alpine.effect(() => {
                    const currentHoveredAircraft = Alpine.store('atc').hoveredAircraft;
                    const previousStrip = previousHoveredHex
                        ? document.querySelector(`tr[data-aircraft-hex="${previousHoveredHex}"] .flight-strip`)
                        : null;
                    if (previousStrip) {
                        previousStrip.classList.remove('flight-card-map-hover');
                    }

                    if (previousHoveredHex && (!currentHoveredAircraft || previousHoveredHex !== currentHoveredAircraft.hex)) {
                        if (this.mapManager) this.mapManager.updateVisualState(previousHoveredHex, true);
                    }

                    if (currentHoveredAircraft) {
                        const currentStrip = document.querySelector(`tr[data-aircraft-hex="${currentHoveredAircraft.hex}"] .flight-strip`);
                        if (currentStrip) {
                            currentStrip.classList.add('flight-card-map-hover');
                        }
                        if (this.mapManager) this.mapManager.updateVisualState(currentHoveredAircraft.hex, true);
                        previousHoveredHex = currentHoveredAircraft.hex;
                    } else {
                        previousHoveredHex = null;
                    }
                });

                let previousSelectedHex = null; // Keep this to know if an aircraft was just selected from null
                Alpine.effect(() => {
                    const currentHex = this.selectedAircraft ? this.selectedAircraft.hex : null;
                    if (currentHex === previousSelectedHex) {
                        return;
                    }

                    this.setupAircraftDetailsPanel();

                    if (this.mapManager) {
                        this.mapManager.applyFiltersAndRefreshView();
                    }

                    previousSelectedHex = currentHex;
                });
                
                // Watch for changes to showLocalDates
                Alpine.effect(() => {
                    const showLocalDates = this.showLocalDates;
                    this.settings.showLocalDates = showLocalDates;
                    this.queueSaveSettings();
                });

                // Watch for changes to settings
                Alpine.effect(() => {
                    // Trigger effect on any setting changes without deep clone churn
                    JSON.stringify(this.settings);
                    this.queueSaveSettings();
                });
                
                isAppInitialized = true;
                console.log("Application initialization successful.");

                // Setup keyboard event listeners
                this.setupKeyboardEvents();
                
                audioClient.initAudioContext();

                // Setup cleanup handler for page unload (prevents memory leaks)
                window.addEventListener('beforeunload', () => {
                    this.cleanup();
                });

            } catch (error) {
                console.error("Error during application initialization:", error);
            }
        },

        // Centralized cleanup to prevent memory leaks during long sessions
        cleanup() {
            console.log('App cleanup: Clearing all intervals and resources...');

            // Clear all interval IDs
            const intervalIds = [
                'timeUpdateIntervalId',
                'lastAudioUpdateIntervalId',
                'mapPerfUpdateIntervalId',
                'stationRefreshInterval',
                'weatherRefreshInterval',
                'adsbSourceRefreshInterval',
                'aircraftDetailsHistoryRefreshInterval',
                'proximityRefreshInterval',
                'phaseHistoryRefreshInterval'
            ];

            intervalIds.forEach(id => {
                if (this[id]) {
                    clearInterval(this[id]);
                    this[id] = null;
                }
            });

            // Stop animation engine
            if (window.animationEngine) {
                window.animationEngine.stop();
            }

            // Release map resources/listeners/timers
            if (this.mapManager && this.mapManager.cleanup) {
                this.mapManager.cleanup();
            }

            // Disconnect WebSocket and clear listeners
            if (wsClient) {
                wsClient.clearAllListeners();
                wsClient.disconnect();
            }

            console.log('App cleanup: Complete');
        },

        processAircraftData(data) {
            // Store the current proximity highlighted aircraft before processing new data
            const proximityHexSet = this.mapManager ? this.mapManager.proximityHexSet : null;
            
            const now = new Date();
            const currentAircraftHexes = new Set();
            const newAircraftData = {};

            data.aircraft.forEach(aircraft => {
                if (!aircraft.adsb || !aircraft.adsb.lat || !aircraft.adsb.lon) return;
                currentAircraftHexes.add(aircraft.hex);
                newAircraftData[aircraft.hex] = aircraft;

                if (this.mapManager && this.mapManager.ensureMapObjects) {
                    this.mapManager.ensureMapObjects(aircraft);
                }
            });
            
            this.aircraft = newAircraftData;

            // Invalidate filter cache when aircraft data changes
            this._lastFilterHash = null;

            if (this.mapManager) {
                this.mapManager.removeStaleMarkers(currentAircraftHexes);
            }
            
            if (!this.initialDataLoaded) {
                this.initialDataLoaded = true;
            }

            if (this.selectedAircraft && (!this.aircraft[this.selectedAircraft.hex])) {
                this.selectedAircraft = null;
            } else if (this.selectedAircraft && this.aircraft[this.selectedAircraft.hex]) {
                this.selectedAircraft = this.aircraft[this.selectedAircraft.hex];
            }

            // Apply filters and refresh the view
            if (this.mapManager) {
                this.mapManager.applyFiltersAndRefreshView();
                
                // Re-apply proximity highlighting if it was active
                if (proximityHexSet && proximityHexSet.size > 0) {
                    // Wait a tiny bit for the DOM to update
                    setTimeout(() => {
                        this.mapManager.highlightProximityAircraft(proximityHexSet);
                    }, 50);
                }
            }
            
            // Refresh alerts display to ensure filtering is applied continuously
            this.refreshAlertsDisplay();
        },

        updateCurrentTime() {
            const now = new Date();
            this.currentTime = now.toLocaleTimeString();
            this.zuluTime = now.toUTCString().match(/(\d{2}:\d{2}:\d{2})/)[0] + 'Z';
        },
        
        // Toggle between local and UTC date display
        toggleDateFormat() {
            this.showLocalDates = !this.showLocalDates;
            this.settings.showLocalDates = this.showLocalDates;
            this.saveSettings();
        },
        
        // Format a date based on user preference (local or UTC)
        formatDate(dateString, timeOnly = false) {
            if (!dateString) return '-';
            
            const date = new Date(dateString);
            if (isNaN(date.getTime())) return 'Invalid Date';
            
            if (timeOnly) {
                // Only show hours, minutes, and seconds
                if (this.showLocalDates) {
                    return date.toLocaleTimeString();
                } else {
                    return date.toISOString().substring(11, 19) + 'Z';
                }
            } else {
                // Show full date and time
                if (this.showLocalDates) {
                    return date.toLocaleString();
                } else {
                    return date.toISOString().replace('T', ' ').substring(0, 19) + 'Z';
                }
            }
        },

        processSampleData() {
            this.processAircraftData({ aircraft: [] });
        },

        // Settings methods
        toggleLabels() {
            this.saveSettings();
            if (this.mapManager) this.mapManager.applyFiltersAndRefreshView();
        },

        togglePaths() {
            this.saveSettings();
            if (this.mapManager) this.mapManager.applyFiltersAndRefreshView();
        },

        toggleRings() {
            this.saveSettings();
            if (this.mapManager) {
                this.mapManager.toggleRings();
            }
        },

        setMapStyle() {
            if (this.settings.mapStyle === 'terminal') {
                this.settings.mapStyle = 'vfr-sectional';
            }
            this.saveSettings();
            if (this.mapManager && typeof this.mapManager.setMapStyle === 'function') {
                this.mapManager.setMapStyle(this.settings.mapStyle);
            }
        },

        toggleAnimation() {
            this.saveSettings();
            if (this.animationEngine) {
                if (this.settings.aircraftAnimation.enabled) {
                    this.animationEngine.start();
                    console.log('Aircraft animation enabled');
                } else {
                    this.animationEngine.stop();
                    console.log('Aircraft animation disabled');
                }
            }
        },

        toggleAirports() {
            this.saveSettings();
            if (this.mapManager) {
                this.mapManager.toggleLayerVisibility('airports', this.settings.showAirports);
            }
        },

        toggleHeliports() {
            this.saveSettings();
            if (this.mapManager) {
                this.mapManager.toggleLayerVisibility('heliports', this.settings.showHeliports);
            }
        },

        toggleNavaids() {
            this.saveSettings();
            if (this.mapManager) {
                this.mapManager.toggleLayerVisibility('navaids', this.settings.showNavaids);
            }
        },

        toggleAllRunways() {
            this.saveSettings();
            if (this.mapManager) {
                this.mapManager.toggleLayerVisibility('allRunways', this.settings.showAllRunways);
            }
        },

        toggleAirspaceBoundaries() {
            this.saveSettings();
            if (this.mapManager) {
                this.mapManager.toggleLayerVisibility('airspace-polygons', this.settings.showAirspaceBoundaries);
            }
        },

        toggleNexrad() {
            this.saveSettings();
            if (this.mapManager) {
                this.mapManager.toggleLayerVisibility('nexrad-radar', this.settings.showNexrad);
            }
        },

        toggleNoaaInfrared() {
            this.saveSettings();
            if (this.mapManager) {
                this.mapManager.toggleLayerVisibility('noaa-infrared', this.settings.showNoaaInfrared);
            }
        },

        toggleNoaaRadar() {
            this.saveSettings();
            if (this.mapManager) {
                this.mapManager.toggleLayerVisibility('noaa-radar', this.settings.showNoaaRadar);
            }
        },

        setNexradOpacity() {
            this.saveSettings();
            if (this.mapManager && typeof this.mapManager.setOverlayOpacity === 'function') {
                this.mapManager.setOverlayOpacity('nexrad-radar', this.settings.nexradOpacity);
            }
        },

        setNoaaInfraredOpacity() {
            this.saveSettings();
            if (this.mapManager && typeof this.mapManager.setOverlayOpacity === 'function') {
                this.mapManager.setOverlayOpacity('noaa-infrared', this.settings.noaaInfraredOpacity);
            }
        },

        setNoaaRadarOpacity() {
            this.saveSettings();
            if (this.mapManager && typeof this.mapManager.setOverlayOpacity === 'function') {
                this.mapManager.setOverlayOpacity('noaa-radar', this.settings.noaaRadarOpacity);
            }
        },

        setAirspaceOpacity() {
            this.saveSettings();
            if (this.mapManager && typeof this.mapManager.setLayerOpacity === 'function') {
                this.mapManager.setLayerOpacity('airspace-polygons', this.settings.airspaceOpacity);
            }
        },

        setRingsOpacity() {
            this.saveSettings();
            if (this.mapManager && typeof this.mapManager.setLayerOpacity === 'function') {
                this.mapManager.setLayerOpacity('rangeRings', this.settings.ringsOpacity);
            }
        },

        setAirportsOpacity() {
            this.saveSettings();
            if (this.mapManager && typeof this.mapManager.setLayerOpacity === 'function') {
                this.mapManager.setLayerOpacity('airports', this.settings.airportsOpacity);
            }
        },

        setHeliportsOpacity() {
            this.saveSettings();
            if (this.mapManager && typeof this.mapManager.setLayerOpacity === 'function') {
                this.mapManager.setLayerOpacity('heliports', this.settings.heliportsOpacity);
            }
        },

        setNavaidsOpacity() {
            this.saveSettings();
            if (this.mapManager && typeof this.mapManager.setLayerOpacity === 'function') {
                this.mapManager.setLayerOpacity('navaids', this.settings.navaidsOpacity);
            }
        },

        setAllRunwaysOpacity() {
            this.saveSettings();
            if (this.mapManager && typeof this.mapManager.setLayerOpacity === 'function') {
                this.mapManager.setLayerOpacity('allRunways', this.settings.allRunwaysOpacity);
            }
        },

        applyMapDisplaySettings() {
            if (!this.mapManager) return;

            if (typeof this.mapManager.setMapStyle === 'function') {
                this.mapManager.setMapStyle(this.settings.mapStyle);
            }

            this.mapManager.toggleLayerVisibility('airspace-polygons', this.settings.showAirspaceBoundaries);
            this.mapManager.toggleLayerVisibility('nexrad-radar', this.settings.showNexrad);
            this.mapManager.toggleLayerVisibility('noaa-infrared', this.settings.showNoaaInfrared);
            this.mapManager.toggleLayerVisibility('noaa-radar', this.settings.showNoaaRadar);
            this.mapManager.toggleLayerVisibility('rangeRings', this.settings.showRings);
            this.mapManager.toggleLayerVisibility('airports', this.settings.showAirports);
            this.mapManager.toggleLayerVisibility('heliports', this.settings.showHeliports);
            this.mapManager.toggleLayerVisibility('navaids', this.settings.showNavaids);
            this.mapManager.toggleLayerVisibility('allRunways', this.settings.showAllRunways);
            if (typeof this.mapManager.setOverlayOpacity === 'function') {
                this.mapManager.setOverlayOpacity('nexrad-radar', this.settings.nexradOpacity);
                this.mapManager.setOverlayOpacity('noaa-infrared', this.settings.noaaInfraredOpacity);
                this.mapManager.setOverlayOpacity('noaa-radar', this.settings.noaaRadarOpacity);
            }
            if (typeof this.mapManager.setLayerOpacity === 'function') {
                this.mapManager.setLayerOpacity('airspace-polygons', this.settings.airspaceOpacity);
                this.mapManager.setLayerOpacity('rangeRings', this.settings.ringsOpacity);
                this.mapManager.setLayerOpacity('airports', this.settings.airportsOpacity);
                this.mapManager.setLayerOpacity('heliports', this.settings.heliportsOpacity);
                this.mapManager.setLayerOpacity('navaids', this.settings.navaidsOpacity);
                this.mapManager.setLayerOpacity('allRunways', this.settings.allRunwaysOpacity);
            }
        },

        // Station override methods
        setStationFromMap() {
            this.stationOverride.mapClickMode = true;
            this.showMapClickIndicator();
        },

        async applyStationOverride() {
            if (!this.stationOverride.latitude || !this.stationOverride.longitude) {
                alert('Please enter valid coordinates');
                return;
            }

            try {
                const response = await fetch('/api/v1/station', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        latitude: this.stationOverride.latitude,
                        longitude: this.stationOverride.longitude
                    })
                });

                if (response.ok) {
                    this.stationOverride.active = true;
                    this.updateStationRings();
                    if (this.mapManager) {
                        this.mapManager.centerOnStation();
                    }
                    console.log('Station override applied');
                } else {
                    const error = await response.text();
                    alert('Failed to apply station override: ' + error);
                }
            } catch (error) {
                console.error('Failed to apply station override:', error);
                alert('Failed to apply station override: ' + error.message);
            }
        },

        async clearStationOverride() {
            try {
                const response = await fetch('/api/v1/station', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        latitude: null,
                        longitude: null
                    })
                });

                if (response.ok) {
                    this.stationOverride.latitude = null;
                    this.stationOverride.longitude = null;
                    this.stationOverride.active = false;
                    this.stationOverride.autoUpdate = false;
                    this.stopGeolocationWatch();
                    this.updateStationRings();
                    if (this.mapManager) {
                        this.mapManager.centerOnStation();
                    }
                    console.log('Station override cleared');
                } else {
                    const error = await response.text();
                    alert('Failed to clear station override: ' + error);
                }
            } catch (error) {
                console.error('Failed to clear station override:', error);
                alert('Failed to clear station override: ' + error.message);
            }
        },

        updateStationRings() {
            if (this.mapManager) {
                // Update store coordinates for range rings
                if (this.stationOverride.active) {
                    // Use override coordinates temporarily for range rings display
                    // (but don't overwrite the original station coordinates)
                } else {
                    // Use the original station coordinates (already fetched from API)
                    // No need to refetch - just use current stationLatitude/stationLongitude
                }

                // Trigger range rings update - addRangeRings() will use the correct coordinates
                this.mapManager.addRangeRings();
            }
        },

        showMapClickIndicator() {
            // Change cursor, show instruction overlay
            const mapElement = document.getElementById('map');
            if (mapElement) {
                mapElement.style.cursor = 'crosshair';
            }
        },

        hideMapClickIndicator() {
            const mapElement = document.getElementById('map');
            if (mapElement) {
                mapElement.style.cursor = '';
            }
        },

        // Geolocation methods
        useGeolocation() {
            if (!navigator.geolocation) {
                alert('Geolocation is not supported by this browser');
                return;
            }

            this.stationOverride.geolocationStatus = 'Getting location...';
            
            navigator.geolocation.getCurrentPosition(
                (position) => {
                    this.stationOverride.latitude = position.coords.latitude;
                    this.stationOverride.longitude = position.coords.longitude;
                    this.stationOverride.geolocationStatus = `Location acquired (±${Math.round(position.coords.accuracy)}m)`;
                    console.log('Geolocation acquired:', position.coords);
                },
                (error) => {
                    let errorMessage = 'Location access denied';
                    switch(error.code) {
                        case error.PERMISSION_DENIED:
                            errorMessage = 'Location access denied';
                            break;
                        case error.POSITION_UNAVAILABLE:
                            errorMessage = 'Location unavailable';
                            break;
                        case error.TIMEOUT:
                            errorMessage = 'Location request timeout';
                            break;
                    }
                    this.stationOverride.geolocationStatus = errorMessage;
                    console.error('Geolocation error:', error);
                },
                {
                    enableHighAccuracy: true,
                    timeout: 10000,
                    maximumAge: 60000
                }
            );
        },

        toggleGeolocationAutoUpdate() {
            if (this.stationOverride.autoUpdate) {
                this.startGeolocationWatch();
            } else {
                this.stopGeolocationWatch();
            }
        },

        startGeolocationWatch() {
            if (!navigator.geolocation) {
                this.stationOverride.autoUpdate = false;
                alert('Geolocation is not supported by this browser');
                return;
            }

            this.stopGeolocationWatch(); // Clear any existing watch

            this.stationOverride.geolocationStatus = 'Starting location tracking...';
            
            this.stationOverride.geolocationWatchId = navigator.geolocation.watchPosition(
                (position) => {
                    const newLat = position.coords.latitude;
                    const newLon = position.coords.longitude;
                    
                    // Only update if coordinates have changed significantly (>10m)
                    if (!this.stationOverride.latitude || !this.stationOverride.longitude ||
                        Math.abs(newLat - this.stationOverride.latitude) > 0.0001 ||
                        Math.abs(newLon - this.stationOverride.longitude) > 0.0001) {
                        
                        this.stationOverride.latitude = newLat;
                        this.stationOverride.longitude = newLon;
                        
                        // Auto-apply if override is already active
                        if (this.stationOverride.active) {
                            this.applyStationOverride();
                        }
                    }
                    
                    this.stationOverride.geolocationStatus = `Tracking location (±${Math.round(position.coords.accuracy)}m)`;
                },
                (error) => {
                    let errorMessage = 'Location tracking failed';
                    switch(error.code) {
                        case error.PERMISSION_DENIED:
                            errorMessage = 'Location access denied';
                            this.stationOverride.autoUpdate = false;
                            break;
                        case error.POSITION_UNAVAILABLE:
                            errorMessage = 'Location unavailable';
                            break;
                        case error.TIMEOUT:
                            errorMessage = 'Location request timeout';
                            break;
                    }
                    this.stationOverride.geolocationStatus = errorMessage;
                    console.error('Geolocation watch error:', error);
                },
                {
                    enableHighAccuracy: true,
                    timeout: 15000,
                    maximumAge: this.stationOverride.updateInterval * 1000
                }
            );
        },

        stopGeolocationWatch() {
            if (this.stationOverride.geolocationWatchId) {
                navigator.geolocation.clearWatch(this.stationOverride.geolocationWatchId);
                this.stationOverride.geolocationWatchId = null;
                this.stationOverride.geolocationStatus = 'Location tracking stopped';
            }
        },

        updateGeolocationInterval() {
            if (this.stationOverride.autoUpdate) {
                // Restart watch with new interval
                this.startGeolocationWatch();
            }
        },

        // Debug function to show animation stats
        getAnimationStats() {
            if (this.animationEngine) {
                const stats = this.animationEngine.getStats();
                console.log('Animation Engine Stats:', stats);
                return stats;
            }
            return null;
        },

        // Poll map performance stats from MapManager
        updateMapPerformanceStats() {
            if (!this.mapManager || !this.mapManager.getPerformanceStats) return;
            requestTileCacheStatsFromSW();
            const mapStats = this.mapManager.getPerformanceStats();
            const animationStats = this.animationEngine ? this.animationEngine.getStats() : null;
            const wsStats = wsClient && wsClient.getMessageRateStats
                ? wsClient.getMessageRateStats()
                : null;
            const smoothingEnabled = !!this.settings?.aircraftAnimation?.enabled;
            const measuredFps = animationStats && Number.isFinite(animationStats.measuredFps)
                ? Number(animationStats.measuredFps.toFixed(1))
                : 0;
            const animationFrameMs = smoothingEnabled && animationStats && Number.isFinite(animationStats.averageFrameTime)
                ? Number(animationStats.averageFrameTime.toFixed(2))
                : null;
            const animationVisibleTargets = smoothingEnabled && animationStats && Number.isFinite(animationStats.visibleTargets)
                ? animationStats.visibleTargets
                : null;
            const animationAnimatedThisFrame = smoothingEnabled && animationStats && Number.isFinite(animationStats.animatedThisFrame)
                ? animationStats.animatedThisFrame
                : null;
            const animationPredictedPerSec = smoothingEnabled && animationStats && Number.isFinite(animationStats.predictedPerSec)
                ? Number(animationStats.predictedPerSec.toFixed(1))
                : null;
            const animationCorrectedPerSec = smoothingEnabled && animationStats && Number.isFinite(animationStats.correctedPerSec)
                ? Number(animationStats.correctedPerSec.toFixed(1))
                : null;
            const animationMarkerUpdatesPerSec = smoothingEnabled && animationStats && Number.isFinite(animationStats.markerUpdatesPerSec)
                ? Number(animationStats.markerUpdatesPerSec.toFixed(1))
                : null;
            const animationQuality = smoothingEnabled && animationStats && Number.isFinite(animationStats.qualityLevel)
                ? Number(animationStats.qualityLevel.toFixed(2))
                : null;
            const animationDroppedFrames = smoothingEnabled && animationStats && Number.isFinite(animationStats.droppedFrames)
                ? animationStats.droppedFrames
                : null;

            const wsByType = wsStats?.byTypePerSec || {};
            const wsRates = {
                total: Number.isFinite(wsStats?.totalPerSec) ? wsStats.totalPerSec : 0,
                aircraft_update: Number.isFinite(wsByType.aircraft_update) ? wsByType.aircraft_update : 0,
                aircraft_predicted_state: Number.isFinite(wsByType.aircraft_predicted_state) ? wsByType.aircraft_predicted_state : 0,
                aircraft_added: Number.isFinite(wsByType.aircraft_added) ? wsByType.aircraft_added : 0,
                aircraft_removed: Number.isFinite(wsByType.aircraft_removed) ? wsByType.aircraft_removed : 0,
                phase_change: Number.isFinite(wsByType.phase_change) ? wsByType.phase_change : 0,
                transcription: Number.isFinite(wsByType.transcription) ? wsByType.transcription : 0,
                frequency_status: Number.isFinite(wsByType.frequency_status) ? wsByType.frequency_status : 0,
                parse_errors: Number.isFinite(wsStats?.parseErrorsPerSec) ? wsStats.parseErrorsPerSec : 0
            };

            const effectiveUpdateRate = Number((wsRates.aircraft_update + wsRates.aircraft_predicted_state).toFixed(1));
            const displayRenderRate = smoothingEnabled ? measuredFps : effectiveUpdateRate;

            const fullVisibilityPassesPerSec = Number.isFinite(mapStats?.windowSec) && mapStats.windowSec > 0
                ? Number((mapStats.fullVisibilityPasses / mapStats.windowSec).toFixed(2))
                : 0;

            const memory = performance && performance.memory ? performance.memory : null;
            const heapUsedMB = memory && Number.isFinite(memory.usedJSHeapSize)
                ? Number((memory.usedJSHeapSize / (1024 * 1024)).toFixed(1))
                : null;
            const heapLimitMB = memory && Number.isFinite(memory.jsHeapSizeLimit)
                ? Number((memory.jsHeapSizeLimit / (1024 * 1024)).toFixed(1))
                : null;
            const heapUsagePct = (heapUsedMB !== null && heapLimitMB && heapLimitMB > 0)
                ? Number(((heapUsedMB / heapLimitMB) * 100).toFixed(1))
                : null;
            const heapDeltaMB = (heapUsedMB !== null && this._previousHeapUsedMB !== null)
                ? Number((heapUsedMB - this._previousHeapUsedMB).toFixed(1))
                : null;
            this._previousHeapUsedMB = heapUsedMB;

            this.mapPerformanceStats = {
                ...mapStats,
                smoothingEnabled: smoothingEnabled,
                animationFps: displayRenderRate,
                animationFrameMs: animationFrameMs,
                animationVisibleTargets: animationVisibleTargets,
                animationAnimatedThisFrame: animationAnimatedThisFrame,
                animationPredictedPerSec: animationPredictedPerSec,
                animationCorrectedPerSec: animationCorrectedPerSec,
                animationMarkerUpdatesPerSec: animationMarkerUpdatesPerSec,
                animationQuality: animationQuality,
                animationDroppedFrames: animationDroppedFrames,
                heapUsedMB: heapUsedMB,
                heapLimitMB: heapLimitMB,
                heapUsagePct: heapUsagePct,
                heapDeltaMB: heapDeltaMB,
                fullVisibilityPassesPerSec: fullVisibilityPassesPerSec,
                wsWindowSec: Number.isFinite(wsStats?.windowSec) ? wsStats.windowSec : 0,
                wsRates: wsRates,
                tileCache: { ...this.tileCacheStats }
            };
        },

        applyFilters() {
            this.onFilterChange();
        },



        updateSecondsSinceLastAudio() {
            if (!audioClient) { console.warn("audioClient not ready in updateSecondsSinceLastAudio (store)"); return; }
            audioClient.updateSecondsSinceLastAudio(this.secondsSinceLastAudio); // Pass the store's object to be updated
        },

// Initialize WebSocket for all aircraft data and alerts
async initAircraftDataSource() {
    console.log('[AIRCRAFT DATA] Initializing WebSocket for aircraft updates and alerts...');
    this.initWebSocket();
},

        // Initialize WebSocket connection
        initWebSocket() {
            if (!wsClient) {
                console.error("wsClient not available during initWebSocket. This shouldn't happen.");
                return;
            }

            if (wsClient.enableAutoReconnect) {
                wsClient.enableAutoReconnect();
            }
            if (wsClient.resetReconnectAttempts) {
                wsClient.resetReconnectAttempts();
            }
            if (wsClient.clearAllListeners) {
                wsClient.clearAllListeners();
            }

            const updateWsStatus = (status) => {
                this.wsConnectionState = status?.state || 'idle';
                this.wsReconnectAttempt = Number.isFinite(status?.reconnectAttempts) ? status.reconnectAttempts : 0;
                this.wsNextRetryDelayMs = Number.isFinite(status?.nextRetryDelayMs) ? status.nextRetryDelayMs : null;
            };

            updateWsStatus(wsClient.getConnectionStatus ? wsClient.getConnectionStatus() : null);

            // Add event listeners
            wsClient.addEventListener('transcription', (data) => {
                this.handleTranscriptionMessage(data);
            });
            
            wsClient.addEventListener('transcription_update', (data) => {
                this.handleTranscriptionUpdateMessage(data);
            });

            // Add event listeners for phase changes
            wsClient.addEventListener('phase_change', (data) => {
                this.handlePhaseChangeMessage(data);
            });

            // Add event listener for clearance events
            wsClient.addEventListener('clearance_issued', (data) => {
                this.handleClearanceIssued(data);
            });

            // Add new aircraft streaming handlers
            wsClient.addEventListener('aircraft_added', (data) => {
                this.handleAircraftAdded(data);
            });
            
            wsClient.addEventListener('aircraft_update', (data) => {
                this.handleAircraftUpdate(data);
            });

            wsClient.addEventListener('aircraft_predicted_state', (data) => {
                this.handleAircraftPredictedState(data);
            });
            
            wsClient.addEventListener('aircraft_removed', (data) => {
                this.handleAircraftRemoved(data);
            });
            
            // Add bulk data response handler
            wsClient.addEventListener('aircraft_bulk_response', (data) => {
                this.handleBulkAircraftData(data);
            });

            // Add frequency status change handler
            wsClient.addEventListener('frequency_status', (data) => {
                this.handleFrequencyStatusChange(data);
            });

            // Aircraft events are now handled as phase changes (T/O and T/D)
            
            wsClient.addEventListener('open', () => {
                console.log('App.js: WebSocket connection now open.');
                this.connected = true;
                this.wsReconnectAttempt = 0;
                this.wsNextRetryDelayMs = null;
                // Reset the connection lost sound flag when connection is re-established
                this.connectionLostSoundPlayed = false;
                
                // Request initial aircraft data via WebSocket (this will send filter update)
                console.log('WebSocket connected, requesting initial aircraft data...');
                this.requestInitialAircraftData();
            });
            
            wsClient.addEventListener('close', (event) => {
                console.log('App.js: WebSocket connection closed.', event);
                
                // Only set connected to false if we previously had a successful connection
                // This prevents the CONNECTION LOST overlay from showing on initial page load
                if (this.connected === true) {
                    console.log('Connection was previously established and is now lost');
                    this.connected = false;
                    
                    // Play connection lost sound once when connection is lost
                    this.playConnectionLostSound();
                }
                
                // Note: Reconnection is now handled by WebSocketClient internally
                // No need to manually initiate reconnection here
            });

            wsClient.addEventListener('state_change', (status) => {
                updateWsStatus(status);

                if (status?.state === 'reconnecting' || status?.state === 'closed') {
                    this.connected = false;
                }
                if (status?.state === 'connecting' && this.connected === false) {
                    // keep false while recovering to retain clear UX
                }
            });

            wsClient.addEventListener('reconnect_scheduled', (payload) => {
                this.wsReconnectAttempt = Number.isFinite(payload?.attempt) ? payload.attempt : this.wsReconnectAttempt;
                this.wsNextRetryDelayMs = Number.isFinite(payload?.delayMs) ? payload.delayMs : this.wsNextRetryDelayMs;
                this.connected = false;
            });
            
            wsClient.addEventListener('error', (event) => {
                console.error('App.js: WebSocket error:', event);
            });
            
            // Connect to the WebSocket server if not already connected or trying
            if (wsClient.connection === null || wsClient.connection.readyState === WebSocket.CLOSED) {
                 console.log("App.js: Attempting to connect WebSocket...");
                wsClient.connect();
            } else if (wsClient.connection.readyState === WebSocket.CONNECTING) {
                console.log("App.js: WebSocket is already connecting.");
                this.connected = null;
            } else if (wsClient.connection.readyState === WebSocket.OPEN) {
                console.log("App.js: WebSocket is already open.");
                this.connected = true;
            }
        },
        
        // Handle transcription message
        handleTranscriptionMessage(data) {
            // Add the transcription to the array
            this.transcriptions.push(data);
            
            // Keep only the last 100 transcriptions
            if (this.transcriptions.length > 100) {
                this.transcriptions.shift();
            }
            
            // Store transcription by frequency_id
            const freqId = data.frequency_id;
            
            // Initialize arrays if they don't exist
            if (!this.frequencyTranscriptions[freqId]) {
                this.frequencyTranscriptions[freqId] = [];
            }
            if (!this.originalTranscriptions[freqId]) {
                this.originalTranscriptions[freqId] = [];
            }
            
            // Always add to the original transcriptions array
            this.originalTranscriptions[freqId].unshift(data);
            
            // Limit stored original transcriptions per frequency
            if (this.originalTranscriptions[freqId].length > 99) {
                this.originalTranscriptions[freqId].pop();
            }
            
            // If there's an active search, check if the new transcription matches
            if (this.transcriptionSearchTerm) {
                const searchTerm = this.transcriptionSearchTerm.toLowerCase();
                const shouldInclude =
                    (data.text && data.text.toLowerCase().includes(searchTerm)) ||
                    (data.content_processed && data.content_processed.toLowerCase().includes(searchTerm)) ||
                    (data.callsign && data.callsign.toLowerCase().includes(searchTerm)) ||
                    (data.speaker_type && data.speaker_type.toLowerCase().includes(searchTerm));
                
                // Only add to the visible list if it matches the search
                if (shouldInclude) {
                    this.frequencyTranscriptions[freqId].unshift(data);
                }
            } else {
                // No search active, add normally
                this.frequencyTranscriptions[freqId].unshift(data);
            }

            // Limit stored visible transcriptions per frequency
            if (this.frequencyTranscriptions[freqId].length > 99) {
                this.frequencyTranscriptions[freqId].pop();
            }

            // Increment unread count only when viewer is closed
            if (!this.transcriptionViewerVisible[freqId]) {
                this.unreadTranscriptions[freqId] = (this.unreadTranscriptions[freqId] || 0) + 1;
            }
        },
        
        // Filter transcriptions based on search term
        filterTranscriptions(frequencyId) {
            if (!this.frequencyTranscriptions[frequencyId]) {
                return;
            }
            
            // If search term is empty, restore from original transcriptions
            if (!this.transcriptionSearchTerm || this.transcriptionSearchTerm.trim() === '') {
                if (this.originalTranscriptions[frequencyId]) {
                    // Copy the original transcriptions to the visible array
                    this.frequencyTranscriptions[frequencyId] = [...this.originalTranscriptions[frequencyId]];
                } else {
                    // Fallback to API if original transcriptions aren't available
                    this.fetchTranscriptionsForFrequency(frequencyId);
                }
                return;
            }
            
            const searchTerm = this.transcriptionSearchTerm.toLowerCase();
            
            // Use original transcriptions as the source for filtering
            if (this.originalTranscriptions[frequencyId]) {
                const originalData = [...this.originalTranscriptions[frequencyId]];
                
                // Create a filtered copy of the transcriptions
                const filtered = originalData.filter(transcription => {
                    // Search in the text content
                    if (transcription.text && transcription.text.toLowerCase().includes(searchTerm)) {
                        return true;
                    }
                    
                    // Search in the processed content if available
                    if (transcription.content_processed &&
                        transcription.content_processed.toLowerCase().includes(searchTerm)) {
                        return true;
                    }
                    
                    // Search in the callsign if available
                    if (transcription.callsign &&
                        transcription.callsign.toLowerCase().includes(searchTerm)) {
                        return true;
                    }
                    
                    // Search in the speaker type if available
                    if (transcription.speaker_type &&
                        transcription.speaker_type.toLowerCase().includes(searchTerm)) {
                        return true;
                    }
                    
                    return false;
                });
                
                // Replace the array with the filtered version
                this.frequencyTranscriptions[frequencyId] = filtered;
            } else {
                // Fallback to API if original transcriptions aren't available
                this.fetchTranscriptionsForFrequency(frequencyId).then(data => {
                    // Store as original and then filter
                    this.originalTranscriptions[frequencyId] = [...data];
                    this.filterTranscriptions(frequencyId); // Call again now that we have original data
                });
            }
        },
        
        // Handle transcription update messages (processed transcriptions)
        handleTranscriptionUpdateMessage(data) {
            console.log('Received transcription update:', data);
            
            if (!data.id) {
                console.error('Received transcription update without ID:', data);
                return;
            }
            
            // Find the original transcription in the array by ID only
            const index = this.transcriptions.findIndex(t => t.id === data.id);
                
            if (index !== -1) {
                // Update the existing transcription with processed content
                const updatedTranscription = {
                    ...this.transcriptions[index],
                    content_processed: data.content_processed,
                    speaker_type: data.speaker_type,
                    callsign: data.callsign,
                    is_processed: true
                };
                
                // Replace the transcription in the array
                this.transcriptions.splice(index, 1, updatedTranscription);
                
                // Update the frequency transcriptions array
                if (!this.frequencyTranscriptions[data.frequency_id]) {
                    this.frequencyTranscriptions[data.frequency_id] = [];
                }
                
                // Find the transcription in the frequency-specific array by ID only
                const freqIndex = this.frequencyTranscriptions[data.frequency_id].findIndex(t => t.id === data.id);
                
                if (freqIndex !== -1) {
                    console.log(`Found transcription at index ${freqIndex}, updating...`);
                    // Create a new object with all properties from the original and the update
                    const updatedFreqTranscription = {
                        ...this.frequencyTranscriptions[data.frequency_id][freqIndex],
                        content_processed: data.content_processed,
                        speaker_type: data.speaker_type,
                        callsign: data.callsign,
                        is_processed: true
                    };
                    
                    // Replace the transcription in the array
                    this.frequencyTranscriptions[data.frequency_id].splice(freqIndex, 1, updatedFreqTranscription);
                } else {
                    console.warn(`Could not find transcription with id ${data.id} in frequency ${data.frequency_id} array`);
                }
                
                // Also update in the originalTranscriptions array
                if (this.originalTranscriptions[data.frequency_id]) {
                    const origIndex = this.originalTranscriptions[data.frequency_id].findIndex(t => t.id === data.id);
                    if (origIndex !== -1) {
                        // Create a new object with all properties from the original and the update
                        const updatedOrigTranscription = {
                            ...this.originalTranscriptions[data.frequency_id][origIndex],
                            content_processed: data.content_processed,
                            speaker_type: data.speaker_type,
                            callsign: data.callsign,
                            is_processed: true
                        };
                        
                        // Replace the transcription in the array
                        this.originalTranscriptions[data.frequency_id].splice(origIndex, 1, updatedOrigTranscription);
                    } else {
                        console.warn(`Could not find transcription with id ${data.id} in original transcriptions array for frequency ${data.frequency_id}`);
                    }
                }
            } else {
                console.warn(`Could not find transcription with id ${data.id} in main transcriptions array`);
            }
        },

        // Handle phase change messages
        handlePhaseChangeMessage(data) {
            console.log('Received phase change:', data);
            
            // Create alert message for phase change
            const message = `${data.flight || data.hex} changed phase: ${data.transition}`;
            
            // Add to alerts using existing alert system
            this.addPhaseChangeAlert(data);
            
            // Trigger visual effect on the map for takeoff/landing (T/O and T/D phases)
            if ((data.phase === 'T/O' || data.phase === 'T/D') && this.mapManager) {
                const eventType = data.phase === 'T/O' ? 'takeoff' : 'landing';
                this.mapManager.showTakeoffLandingEffect(data.hex, eventType, data.phase);
            }
            
            // Log to console for debugging
            console.log(`Phase change alert: ${message}`);
        },

        // Handle clearance issued messages
        handleClearanceIssued(data) {
            console.log('Clearance issued:', data);
            
            // Update aircraft clearances if currently selected
            const selectedAircraft = this.selectedAircraft;
            if (selectedAircraft && selectedAircraft.flight === data.callsign) {
                // Refresh aircraft details to show new clearance
                this.refreshSelectedAircraftDetails();
            }
            
            // Show alert for clearance
            this.showClearanceAlert(data);
            
            // Log to console for debugging
            console.log(`Clearance issued: ${data.callsign} → ${data.clearance_type.toUpperCase()} CLEARANCE`);
        },

        // Show clearance alert
        showClearanceAlert(clearanceData) {
            const alertText = `${clearanceData.callsign} → ${clearanceData.clearance_type.toUpperCase()} CLEARANCE`;
            let alertClass;
            
            switch(clearanceData.clearance_type) {
                case 'takeoff':
                    alertClass = 'alert-takeoff';
                    break;
                case 'landing':
                    alertClass = 'alert-landing';
                    break;
                case 'approach':
                    alertClass = 'alert-approach';
                    break;
                default:
                    alertClass = 'alert-clearance';
            }
            
            // Add to alerts system (reusing existing alert infrastructure)
            this.addAlert(alertText, alertClass, clearanceData.callsign);
        },

        // Refresh selected aircraft details
        refreshSelectedAircraftDetails() {
            if (this.selectedAircraft) {
                // Re-fetch aircraft data to get updated clearances
                this.selectAircraft(this.selectedAircraft.hex);
            }
        },

        // Aircraft events are now handled as phase changes (T/O and T/D)
        
        // Highlight search term in text with a subtle red underline
        formatTimeAgo(timestamp) {
            if (!timestamp) return '';
            // Reading currentTimeForPhases triggers Alpine reactivity every second
            const now = this.currentTimeForPhases || new Date();
            const ts = new Date(timestamp);
            const diffS = Math.floor((now - ts) / 1000);
            if (diffS < 60) return diffS + 's';
            const diffM = Math.floor(diffS / 60);
            if (diffM < 60) return diffM + 'm';
            const diffH = Math.floor(diffM / 60);
            if (diffH < 24) return diffH + 'h ' + (diffM % 60) + 'm';
            return ts.toLocaleDateString();
        },

        highlightSearchTerm(text) {
            if (!text || !this.transcriptionSearchTerm || this.transcriptionSearchTerm.trim() === '') {
                return text;
            }
            
            // Escape special characters for regex
            const escapeRegExp = (string) => {
                return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
            };
            
            const searchTerm = escapeRegExp(this.transcriptionSearchTerm.trim());
            const regex = new RegExp(`(${searchTerm})`, 'gi');
            
            // Replace matches with the same text but with a red underline
            return text.replace(regex, '<span class="border-b border-red-400">${1}</span>');
        },
        
        // Handle aircraft movement message
        handleAircraftMessage(data) {
            if (data && data.movement) {
                this.addAircraftAlert(data);
            }
        },
        
        // Handle status update message
        handleStatusUpdateMessage(data) {
            if (!data || !data.hex) return;

            // Skip new_aircraft alerts - handled by phase changes
            if (data.new_status === 'new_aircraft') return;

            const aircraft = this.aircraft[data.hex];
            if (!aircraft) return;

            if (data.new_status === 'signal_lost') {
                const secondsSince = this.getSecondsSinceLastSeen(aircraft);
                if (!Number.isFinite(secondsSince) || secondsSince < 60) {
                    return;
                }
            }

            // Update status
            aircraft.status = data.new_status;

            // For signal_lost: skip alerts for CRZ phase or grounded aircraft (expected/noisy)
            if (data.new_status === 'signal_lost') {
                const currentPhase = aircraft.phase?.current?.[0]?.phase;
                if (currentPhase === 'CRZ' || data.on_ground) return;
            }

            this.addStatusAlert(data);
        },
        
        // Request initial aircraft data via WebSocket
        requestInitialAircraftData() {
            console.log('Requesting initial aircraft data');
            // Request bulk data for initial load
            // Only send last_seen_minutes to server to avoid loading stale aircraft
            // All other filtering is done client-side
            if (wsClient) {
                wsClient.requestBulkAircraftData({
                    last_seen_minutes: Number(this.settings.lastSeenMinutes) || 10
                });
            }
        },

        requestInitialAircraftDataThrottled(reason = 'resync', hex = '') {
            const now = Date.now();
            if (!this._lastUnknownAircraftResyncAt) {
                this._lastUnknownAircraftResyncAt = 0;
            }
            if (!this._unknownAircraftResyncCooldownMs) {
                this._unknownAircraftResyncCooldownMs = 5000;
            }

            if ((now - this._lastUnknownAircraftResyncAt) < this._unknownAircraftResyncCooldownMs) {
                return;
            }

            this._lastUnknownAircraftResyncAt = now;
            console.warn(`Requesting bulk aircraft resync (${reason})`, hex || '');
            this.requestInitialAircraftData();
        },

        // Handle bulk aircraft data response (initial load on WebSocket connect)
        handleBulkAircraftData(data) {
            console.log(`Received bulk aircraft data: ${data.count} aircraft`);

            // Clear current aircraft data
            this.aircraft = {};
            
            // Process each aircraft
            if (data.aircraft && Array.isArray(data.aircraft)) {
                for (const aircraft of data.aircraft) {
                    // Calculate distance
                    this.calculateAircraftDistance(aircraft);
                    
                    // Update animation engine with aircraft data
                    if (this.animationEngine) {
                        this.animationEngine.updateAircraft(aircraft);
                    }
                    
                    // Add to aircraft map (all aircraft from bulk response are pre-filtered)
                    this.aircraft[aircraft.hex] = aircraft;
                }
            }

            // Update connection status
            this.connected = true;
            this.lastUpdate = new Date();
            this.lastUpdateSeconds = 0;
            
            // Update map with all aircraft
            if (this.mapManager) {
                // Clear existing markers that are no longer in the current aircraft set
                const currentHexes = new Set(Object.keys(this.aircraft));
                this.mapManager.removeStaleMarkers(currentHexes);
                
                // Update all aircraft markers
                for (const aircraft of Object.values(this.aircraft)) {
                    if (this.mapManager.ensureMapObjects) {
                        this.mapManager.ensureMapObjects(aircraft);
                    }
                }
                
                // Refresh view and apply filters
                this.mapManager.applyFiltersAndRefreshView();
            }
            
            // Invalidate filtered aircraft cache
            this._lastFilterHash = null;
            
            console.log('Bulk aircraft data processing complete');
        },

        // Throttling state for performance optimization
        pendingMapUpdates: new Set(),
        mapUpdateThrottleId: null,
        cacheInvalidationPending: false,
        _lastUnknownAircraftResyncAt: 0,
        _unknownAircraftResyncCooldownMs: 5000,
        _predictedMinApplyIntervalMs: 1000,
        _predictedMinConfidence: 0.65,
        
        // Filtering throttling state to prevent main thread blocking
        _filteringScheduled: false,
        _lastFilterTime: null,
        // Performance-optimized aircraft handlers (throttling handled by queueMapUpdate/queueCacheInvalidation)
        handleAircraftAdded(data) {
            if (this.wsUpdatesPaused) {
                console.log(`[PAUSED] Skipping aircraft added: ${data.aircraft?.flight || data.hex}`);
                return;
            }

            if (data.aircraft) {
                const observedAtMs = Date.parse(data.observed_at || '');
                const payloadLastSeenMs = Date.parse(data.aircraft.last_seen || '');
                const realObservedAt = Number.isFinite(observedAtMs)
                    ? observedAtMs
                    : (Number.isFinite(payloadLastSeenMs) ? payloadLastSeenMs : null);
                if (Number.isFinite(realObservedAt)) {
                    data.aircraft._lastRealObservedAt = realObservedAt;
                    data.aircraft.last_seen = new Date(realObservedAt).toISOString();
                }
                if (data.aircraft.adsb) {
                    data.aircraft.adsb.source = 'real';
                }

                // Update last update timestamp
                this.lastUpdate = new Date();
                this.lastUpdateSeconds = 0;

                // Apply distance calculation
                this.calculateAircraftDistance(data.aircraft);

                // ALWAYS add to aircraft store - filtering is done at display level
                this.aircraft[data.aircraft.hex] = data.aircraft;

                // Update animation engine with new aircraft data
                if (this.animationEngine) {
                    this.animationEngine.updateAircraft(data.aircraft);
                }

                // Queue for throttled map update (map will handle visibility based on filters)
                this.queueMapUpdate(data.aircraft.hex);

                // Queue cache invalidation
                this.queueCacheInvalidation();
            }
        },

        handleAircraftUpdate(data) {
            if (this.wsUpdatesPaused) {
                console.log(`[PAUSED] Skipping aircraft update: ${data.hex}`);
                return;
            }

            const hex = data.hex;
            let existing = this.aircraft[hex];
            const observedAtMs = Date.parse(data.observed_at || '');
            const realObservedAt = Number.isFinite(observedAtMs) ? observedAtMs : null;
            const realObservedAtIso = Number.isFinite(realObservedAt)
                ? new Date(realObservedAt).toISOString()
                : null;

            // Update last update timestamp
            this.lastUpdate = new Date();
            this.lastUpdateSeconds = 0;

            // Handle delta update (new efficient format)
            if (data.delta) {
                // If aircraft doesn't exist locally, avoid creating sparse placeholders.
                // Request a bulk sync so details panel fields stay complete.
                if (!existing) {
                    this.requestInitialAircraftDataThrottled('delta_without_local_aircraft', hex);
                    return;
                }

                if (Number.isFinite(realObservedAt)) {
                    existing._lastRealObservedAt = realObservedAt;
                }
                if (existing.adsb) {
                    existing.adsb.source = 'real';
                }

                // Heal previously sparse entries (delta streams may not include all metadata fields).
                if (!existing.created_at) {
                    this.requestInitialAircraftDataThrottled('sparse_local_aircraft', hex);
                }

                // Check if this update contains filter-relevant changes BEFORE applying
                const hasFilterRelevantChanges =
                    data.delta.status !== undefined ||
                    data.delta.phase !== undefined ||
                    data.delta.on_ground !== undefined ||
                    data.delta.flight !== undefined;

                this.applyDelta(existing, data.delta, realObservedAtIso);
                this.calculateAircraftDistance(existing);

                // Update animation engine with delta
                if (this.animationEngine) {
                    this.animationEngine.updateAircraftDelta(hex, data.delta);
                }

                // Queue map update - map will handle visibility based on filters
                this.queueMapUpdate(hex);

                // Update selected aircraft panel if this is the selected one
                if (this.selectedAircraft && this.selectedAircraft.hex === hex) {
                    this.setupAircraftDetailsPanel();
                }

                // PERFORMANCE: Only invalidate cache if filter-relevant data changed
                // Position/altitude/speed updates don't affect which aircraft are shown
                if (hasFilterRelevantChanges) {
                    this.queueCacheInvalidation();
                }
            }
            // Fallback: full aircraft object (backward compatibility)
            else if (data.aircraft) {
                const payloadLastSeenMs = Date.parse(data.aircraft.last_seen || '');
                const effectiveObservedAt = Number.isFinite(realObservedAt)
                    ? realObservedAt
                    : (Number.isFinite(payloadLastSeenMs) ? payloadLastSeenMs : null);
                if (Number.isFinite(effectiveObservedAt)) {
                    data.aircraft._lastRealObservedAt = effectiveObservedAt;
                    data.aircraft.last_seen = new Date(effectiveObservedAt).toISOString();
                }
                if (data.aircraft.adsb) {
                    data.aircraft.adsb.source = 'real';
                }

                // ALWAYS store aircraft data - filtering is done at display level
                this.aircraft[data.aircraft.hex] = data.aircraft;
                this.calculateAircraftDistance(data.aircraft);

                if (this.animationEngine) {
                    this.animationEngine.updateAircraft(data.aircraft);
                }

                // Queue map update - map will handle visibility based on filters
                this.queueMapUpdate(hex);

                if (this.selectedAircraft && this.selectedAircraft.hex === data.aircraft.hex) {
                    this.selectedAircraft = data.aircraft;
                    this.setupAircraftDetailsPanel();
                }

                // Full object replacement always needs cache invalidation
                this.queueCacheInvalidation();
            }
        },

        handleAircraftPredictedState(data) {
            if (this.wsUpdatesPaused || !data || !data.hex || !data.delta) {
                return;
            }

            const aircraft = this.aircraft[data.hex];
            if (!aircraft || aircraft.status === 'signal_lost') {
                return;
            }

            const confidence = Number(data.delta.confidence);
            if (Number.isFinite(confidence) && confidence < this._predictedMinConfidence) {
                return;
            }

            const preservedLastSeen = aircraft.last_seen;
            const preservedRealObservedAt = aircraft._lastRealObservedAt;

            if (!aircraft.adsb) {
                aircraft.adsb = {};
            }

            const basedOnMs = Date.parse(data.based_on || '');
            const predictedAtMs = Date.parse(data.predicted_at || '');
            const latestRealObservedAt = Number.isFinite(aircraft._lastRealObservedAt) ? aircraft._lastRealObservedAt : null;

            // Real ADS-B updates always win over predicted updates.
            if (Number.isFinite(basedOnMs) && Number.isFinite(latestRealObservedAt) && basedOnMs < latestRealObservedAt) {
                return;
            }

            const predictionTs = Number.isFinite(predictedAtMs)
                ? predictedAtMs
                : (Number.isFinite(basedOnMs) ? basedOnMs : Date.now());
            const lastAppliedPredictionTs = Number.isFinite(aircraft._lastPredictedAppliedAt)
                ? aircraft._lastPredictedAppliedAt
                : null;
            if (Number.isFinite(lastAppliedPredictionTs) &&
                (predictionTs - lastAppliedPredictionTs) < this._predictedMinApplyIntervalMs) {
                return;
            }

            const delta = data.delta;
            if (delta.lat !== undefined) aircraft.adsb.lat = delta.lat;
            if (delta.lon !== undefined) aircraft.adsb.lon = delta.lon;
            if (delta.alt_baro !== undefined) aircraft.adsb.alt_baro = delta.alt_baro;
            if (delta.gs !== undefined) aircraft.adsb.gs = delta.gs;
            if (delta.track !== undefined) aircraft.adsb.track = delta.track;
            if (delta.true_heading !== undefined) aircraft.adsb.true_heading = delta.true_heading;
            if (delta.mag_heading !== undefined) aircraft.adsb.mag_heading = delta.mag_heading;
            aircraft.adsb.source = 'predicted';

            aircraft._lastPredictedObservedAt = Number.isFinite(basedOnMs) ? basedOnMs : Date.now();
            aircraft._lastPredictedAppliedAt = predictionTs;

            // Predicted updates must never advance real timing fields.
            aircraft.last_seen = preservedLastSeen;
            aircraft._lastRealObservedAt = preservedRealObservedAt;

            this.calculateAircraftDistance(aircraft);

            if (this.animationEngine) {
                this.animationEngine.updateAircraft(aircraft);
            }

            this.queueMapUpdate(data.hex);
        },

        // Apply delta updates to an existing aircraft object
        // PERFORMANCE: Only set values that have actually changed to minimize Alpine reactivity triggers
        applyDelta(aircraft, delta, realObservedAtIso = null) {
            // Initialize adsb object if needed
            if (!aircraft.adsb) aircraft.adsb = {};

            // Preserve last known position when GPS drops out (ADS-B → mode_s transition).
            // If the delta sends lat=0 and lon=0 but the aircraft previously had a valid position,
            // keep the old lat/lon and flag the position as stale so the map can show
            // the marker at its last known location with a distinct visual style.
            const hadValidPosition = aircraft.adsb.lat && aircraft.adsb.lat !== 0 || aircraft.adsb.lon && aircraft.adsb.lon !== 0;
            const deltaDropsPosition = delta.lat === 0 && delta.lon === 0;

            if (hadValidPosition && deltaDropsPosition) {
                // Don't overwrite — mark position as stale
                aircraft.stale_position = true;
            } else {
                // Apply position normally
                if (delta.lat !== undefined && aircraft.adsb.lat !== delta.lat) aircraft.adsb.lat = delta.lat;
                if (delta.lon !== undefined && aircraft.adsb.lon !== delta.lon) aircraft.adsb.lon = delta.lon;
                // Clear stale flag if we got valid position data back
                if (delta.lat !== undefined && delta.lon !== undefined && (delta.lat !== 0 || delta.lon !== 0)) {
                    aircraft.stale_position = false;
                }
            }
            if (delta.alt_baro !== undefined && aircraft.adsb.alt_baro !== delta.alt_baro) aircraft.adsb.alt_baro = delta.alt_baro;
            if (delta.track !== undefined && aircraft.adsb.track !== delta.track) aircraft.adsb.track = delta.track;
            if (delta.gs !== undefined && aircraft.adsb.gs !== delta.gs) aircraft.adsb.gs = delta.gs;
            if (delta.tas !== undefined && aircraft.adsb.tas !== delta.tas) aircraft.adsb.tas = delta.tas;
            if (delta.baro_rate !== undefined && aircraft.adsb.baro_rate !== delta.baro_rate) aircraft.adsb.baro_rate = delta.baro_rate;
            if (delta.mag_heading !== undefined && aircraft.adsb.mag_heading !== delta.mag_heading) aircraft.adsb.mag_heading = delta.mag_heading;
            if (delta.true_heading !== undefined && aircraft.adsb.true_heading !== delta.true_heading) aircraft.adsb.true_heading = delta.true_heading;
            if (delta.source !== undefined && aircraft.adsb.source !== delta.source) aircraft.adsb.source = delta.source;
            if (delta.atc_derived !== undefined && aircraft.adsb.atc_derived !== delta.atc_derived) aircraft.adsb.atc_derived = delta.atc_derived;

            // Apply full adsb object if provided — preserve last known position if new object has no GPS
            if (delta.adsb !== undefined) {
                const oldLat = aircraft.adsb?.lat;
                const oldLon = aircraft.adsb?.lon;
                const oldHadPosition = (oldLat && oldLat !== 0) || (oldLon && oldLon !== 0);
                aircraft.adsb = delta.adsb;
                const newHasPosition = (delta.adsb.lat && delta.adsb.lat !== 0) || (delta.adsb.lon && delta.adsb.lon !== 0);
                if (oldHadPosition && !newHasPosition) {
                    aircraft.adsb.lat = oldLat;
                    aircraft.adsb.lon = oldLon;
                    aircraft.stale_position = true;
                } else if (newHasPosition) {
                    aircraft.stale_position = false;
                }
            }

            // Apply top-level aircraft properties - only if value changed
            if (delta.flight !== undefined && aircraft.flight !== delta.flight) aircraft.flight = delta.flight;
            if (delta.status !== undefined && aircraft.status !== delta.status) aircraft.status = delta.status;
            if (delta.on_ground !== undefined && aircraft.on_ground !== delta.on_ground) aircraft.on_ground = delta.on_ground;
            if (delta.phase !== undefined && aircraft.phase !== delta.phase) aircraft.phase = delta.phase;
            if (delta.distance !== undefined && aircraft.distance !== delta.distance) aircraft.distance = delta.distance;

            // Apply BSDB (BaseStation.sqb enrichment) data if provided
            if (delta.bsdb !== undefined) aircraft.bsdb = delta.bsdb;

            // Only update last_seen when the delta contains actual ADS-B sensor data
            // (not for status-only changes like signal_lost which would reset the timer)
            const hasAdsbData = delta.lat !== undefined || delta.lon !== undefined ||
                delta.alt_baro !== undefined || delta.gs !== undefined ||
                delta.track !== undefined || delta.baro_rate !== undefined ||
                delta.tas !== undefined || delta.adsb !== undefined;
            if (hasAdsbData && realObservedAtIso) {
                aircraft.last_seen = realObservedAtIso;
            }
        },

        handleAircraftRemoved(data) {
            if (this.wsUpdatesPaused) {
                console.log(`[PAUSED] Skipping aircraft removed: ${data.hex}`);
                return;
            }

            const aircraft = this.aircraft[data.hex];
            if (aircraft) {
                // Don't delete from the store. A "removed" stream event means the target
                // is absent from the current poll, not necessarily past signal-lost timeout.
                // Signal-lost classification is derived strictly from last_seen age.

                // Remove from map and animation (no marker/trail for non-broadcasting aircraft)
                if (this.mapManager) {
                    this.mapManager.removeAircraft(data.hex);
                }
                if (this.animationEngine) {
                    this.animationEngine.removeAircraft(data.hex);
                }

                // Queue cache invalidation
                this.queueCacheInvalidation();
            }
        },

        // Purge aircraft from the store whose last_seen exceeds the lastSeenMinutes setting.
        // This mirrors the server-side filter used on bulk load, keeping client and server in sync.
        _purgeStaleAircraft() {
            const cutoff = Date.now() - ((Number(this.settings.lastSeenMinutes) || 10) * 60 * 1000);
            let purged = 0;
            for (const hex of Object.keys(this.aircraft)) {
                const ac = this.aircraft[hex];
                if (ac.last_seen && new Date(ac.last_seen).getTime() < cutoff) {
                    delete this.aircraft[hex];
                    if (this.mapManager) this.mapManager.removeAircraft(hex);
                    if (this.animationEngine) this.animationEngine.removeAircraft(hex);
                    if (this.selectedAircraft && this.selectedAircraft.hex === hex) {
                        this.selectedAircraft = null;
                    }
                    purged++;
                }
            }
            if (purged > 0) {
                this.queueCacheInvalidation();
            }
        },

        // Queue map updates for throttling
        queueMapUpdate(hex) {
            if (this.wsUpdatesPaused) {
                return;
            }

            this.pendingMapUpdates.add(hex);

            // Apply immediately on each websocket update to avoid perceived buffering/jumps
            this.processPendingMapUpdates();
        },


        // Process all pending map updates in a batch
        processPendingMapUpdates() {
            if (this.pendingMapUpdates.size === 0) return;

            // Process ALL pending updates in one batch - map updates are lightweight
            const aircraftToUpdate = Array.from(this.pendingMapUpdates);
            this.pendingMapUpdates.clear();

            // Update specific aircraft markers instead of full refresh
            aircraftToUpdate.forEach(hex => {
                const aircraft = this.aircraft[hex];
                if (aircraft && this.mapManager) {
                    this.mapManager.updateSingleAircraft(hex, aircraft);
                }
            });

            // TRAILS FIX: Throttled call to update flight paths (draws trail polylines)
            if (this.mapManager && this.settings.showPaths && !this._trailUpdatePending) {
                this._trailUpdatePending = true;
                setTimeout(() => {
                    this._trailUpdatePending = false;
                    if (this.mapManager) {
                        this.mapManager.updateFlightPaths();
                    }
                }, 500); // Update trails every 500ms max
            }
        },

        // Queue cache invalidation to reduce frequency - VERY AGGRESSIVE
        queueCacheInvalidation() {
            if (!this.cacheInvalidationPending) {
                this.cacheInvalidationPending = true;
                
                // Keep sidebar and map state responsive while still batching bursts.
                setTimeout(() => {
                    // Don't null the cache, just invalidate the hash to trigger re-filtering
                    // This maintains the stable array reference to prevent full table re-renders
                    this._lastFilterHash = null;
                    this._performFiltering();
                    this.cacheInvalidationPending = false;
                }, 120);
            }
        },

        // Request new aircraft data when settings change
        // Note: Server-side filtering has been removed. All filtering is done client-side.
        // Settings are just saved locally; server broadcasts all updates.
        requestNewAircraftData() {
            console.log('Settings changed, saving locally');
            this.saveSettings();
        },

        calculateAircraftDistance(aircraft) {
            if (aircraft.adsb && aircraft.adsb.lat && aircraft.adsb.lon &&
                this.stationLatitude && this.stationLongitude) {
                const distanceNM = this.haversineDistance(
                    aircraft.adsb.lat, aircraft.adsb.lon,
                    this.stationLatitude, this.stationLongitude
                );
                aircraft.distance = Math.round(distanceNM * 10) / 10;
            }
        },

        haversineDistance(lat1, lon1, lat2, lon2) {
            const R = 3440.065; // Earth's radius in nautical miles
            const dLat = this.toRadians(lat2 - lat1);
            const dLon = this.toRadians(lon2 - lon1);
            const a = Math.sin(dLat/2) * Math.sin(dLat/2) +
                      Math.cos(this.toRadians(lat1)) * Math.cos(this.toRadians(lat2)) *
                      Math.sin(dLon/2) * Math.sin(dLon/2);
            const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1-a));
            return R * c;
        },

        toRadians(degrees) {
            return degrees * (Math.PI/180);
        },

        aircraftPassesFilters(aircraft) {
            // Apply all current filters to determine if aircraft should be displayed
            const searchLower = this.searchTerm.toLowerCase();
            
            // Search filter - includes callsign, type, category, manufacturer
            if (searchLower) {
                const callsign = (aircraft.flight || aircraft.hex).toLowerCase();
                const type = (aircraft.adsb?.type || '').toLowerCase();
                const category = (aircraft.adsb?.category || '').toLowerCase();
                const manufacturer = (aircraft.bsdb?.manufacturer || '').toLowerCase();
                const bsdbType = (aircraft.bsdb?.type || '').toLowerCase();

                const matchesSearch = callsign.includes(searchLower) ||
                                    type.includes(searchLower) ||
                                    category.includes(searchLower) ||
                                    manufacturer.includes(searchLower) ||
                                    bsdbType.includes(searchLower);

                if (!matchesSearch) return false;
            }
            
            // Air/Ground filter
            const showThisAircraft = (aircraft.on_ground && this.settings.showGroundAircraft) ||
                                   (!aircraft.on_ground && this.settings.showAirAircraft);
            
            if (!showThisAircraft) return false;
            
            // Phase filter
            const currentPhase = this.getCurrentPhase(aircraft);
            if (this.settings.phaseFilters && this.settings.phaseFilters[currentPhase] === false) {
                return false;
            }
            
            // Altitude filter (for air aircraft)
            if (!aircraft.on_ground && aircraft.adsb &&
                (aircraft.adsb.alt_baro < this.settings.minAltitude ||
                 aircraft.adsb.alt_baro > this.settings.maxAltitude)) {
                return false;
            }
            
            return true;
        },
        
        // Select an aircraft by callsign (for clicking on transcription callsigns)
        selectAircraftByCallsign(callsign) {
            if (!callsign) return;
            
            console.log(`Attempting to select aircraft with callsign: ${callsign}`);
            
            // Normalize the callsign for comparison (trim whitespace, uppercase)
            const normalizedCallsign = callsign.trim().toUpperCase();
            
            // Find the aircraft with the matching callsign
            const foundAircraft = Object.values(this.aircraft).find(aircraft => {
                const aircraftCallsign = (aircraft.flight || '').trim().toUpperCase();
                return aircraftCallsign === normalizedCallsign;
            });
            
            if (foundAircraft) {
                console.log(`Found aircraft with callsign ${callsign}:`, foundAircraft);

                // Set as selected aircraft
                this.selectedAircraft = foundAircraft;

                // Update aircraft details panel
                this.setupAircraftDetailsPanel();
                
                // Update map to highlight the aircraft
                if (this.mapManager) {
                    this.mapManager.updateVisualState(foundAircraft.hex, true);
                    
                    // Center map on the aircraft if it has coordinates
                    if (foundAircraft.adsb && foundAircraft.adsb.lat && foundAircraft.adsb.lon && this.mapManager.centerOnAircraft) {
                        this.mapManager.centerOnAircraft(foundAircraft);
                    }
                }
            } else {
                console.warn(`Could not find aircraft with callsign: ${callsign}`);
            }
        },
        
        // Select an aircraft by hex ID (for right-clicking on alerts)
        selectAircraftByHex(hex) {
            if (!hex) return;
            
            console.log(`Attempting to select aircraft with hex: ${hex}`);
            
            // Find the aircraft with the matching hex
            const foundAircraft = this.aircraft[hex];
            
            if (foundAircraft) {
                console.log(`Found aircraft with hex ${hex}:`, foundAircraft);
                
                // Check if we need to enable the appropriate filter to show the aircraft
                const needsGroundFilter = foundAircraft.on_ground && !this.settings.showGroundAircraft;
                const needsAirFilter = !foundAircraft.on_ground && !this.settings.showAirAircraft;
                
                if (needsGroundFilter) {
                    this.settings.showGroundAircraft = true;
                    this.saveSettings();
                    console.log('Enabled Ground filter to show selected aircraft');
                    // Apply the new filter
                    if (this.mapManager) {
                        this.mapManager.applyFiltersAndRefreshView();
                    }
                } else if (needsAirFilter) {
                    this.settings.showAirAircraft = true;
                    this.saveSettings();
                    console.log('Enabled Air filter to show selected aircraft');
                    // Apply the new filter
                    if (this.mapManager) {
                        this.mapManager.applyFiltersAndRefreshView();
                    }
                }
                
                // Set as selected aircraft
                this.selectedAircraft = foundAircraft;
                
                // Update aircraft details panel
                this.setupAircraftDetailsPanel();
                
                // Update map to highlight the aircraft
                if (this.mapManager) {
                    this.mapManager.updateVisualState(foundAircraft.hex, true);
                    
                    // Center map on the aircraft if it has coordinates
                    if (foundAircraft.adsb && foundAircraft.adsb.lat && foundAircraft.adsb.lon && this.mapManager.centerOnAircraft) {
                        this.mapManager.centerOnAircraft(foundAircraft);
                    }
                }
            } else {
                console.warn(`Could not find aircraft with hex: ${hex}`);
            }
        },
        
        // Add aircraft alert
        addAircraftAlert(data) {
            // Create a unique ID for the alert
            const alertId = Date.now() + '-' + data.hex;
            
            // Add the alert to the array for tracking
            this.aircraftAlerts.push({
                id: alertId,
                hex: data.hex,
                flight: data.flight || data.hex,
                movement: data.movement,
                timestamp: data.timestamp || new Date().toISOString()
            });
            
            // Get the alerts container
            const alertsContainer = document.getElementById('alerts-container');
            if (!alertsContainer) return;
            
            // Hide the "None" text
            const noAlertsText = document.getElementById('no-alerts-text');
            if (noAlertsText) {
                noAlertsText.style.display = 'none';
            }
            
            // Create the alert element
            const alertElement = document.createElement('div');
            alertElement.id = alertId;
            alertElement.className = `inline-flex items-center text-xs px-1.5 py-0.5 rounded ${data.movement === 'tookoff' ? 'text-blue-300' : 'text-green-300'} cursor-pointer hover:bg-black/50`;
            
            // Add left-click event to dismiss the alert
            alertElement.addEventListener('click', () => {
                this.removeAircraftAlert(alertId);
            });
            
            // Add right-click event to select the aircraft and center the map on it
            alertElement.addEventListener('contextmenu', (e) => {
                e.preventDefault(); // Prevent the default context menu
                this.selectAircraftByHex(data.hex);
            });
            
            // Create the icon
            const icon = document.createElement('i');
            icon.className = `fas fa-xs mr-1 ${data.movement === 'tookoff' ? 'fa-plane-departure' : 'fa-plane-arrival'}`;
            alertElement.appendChild(icon);
            
            // Create the text
            const text = document.createElement('span');
            text.textContent = data.flight || data.hex;
            alertElement.appendChild(text);
            
            // Add the alert to the container
            alertsContainer.appendChild(alertElement);
            
            // Remove the alert after 60 seconds
            setTimeout(() => {
                this.removeAircraftAlert(alertId);
            }, 60000);
        },

        // Check if an alert should be shown based on Air/Ground filter settings
        shouldShowAlert(hex) {
            const aircraft = this.aircraft[hex];
            if (!aircraft) return true; // Show alert if we don't have aircraft data yet
            
            // Check if aircraft matches current Air/Ground filter settings
            const showThisAircraft = (aircraft.on_ground && this.settings.showGroundAircraft) ||
                                   (!aircraft.on_ground && this.settings.showAirAircraft);
            
            return showThisAircraft;
        },

        // Refresh alerts display based on current Air/Ground filter settings and phase filters
        refreshAlertsDisplay() {
            const alertsContainer = document.getElementById('alerts-container');
            if (!alertsContainer) return;

            // Check each alert to see if it should be visible
            this.aircraftAlerts.forEach(alert => {
                const alertElement = document.getElementById(alert.id);
                if (alertElement) {
                    let shouldShow = this.shouldShowAlert(alert.hex);
                    
                    // Additional check for phase change alerts
                    if (alert.type === 'phase_change' && alert.data && alert.data.phase) {
                        shouldShow = shouldShow && this.shouldShowPhaseAlert(alert.data.phase);
                    }
                    
                    if (shouldShow) {
                        alertElement.style.display = 'inline-flex';
                    } else {
                        alertElement.style.display = 'none';
                    }
                }
            });

            // Check if any alerts are visible to show/hide "None" text
            const visibleAlerts = this.aircraftAlerts.filter(alert => {
                const alertElement = document.getElementById(alert.id);
                return alertElement && alertElement.style.display !== 'none';
            });

            const noAlertsText = document.getElementById('no-alerts-text');
            if (noAlertsText) {
                noAlertsText.style.display = visibleAlerts.length === 0 ? 'block' : 'none';
            }
        },
        
        // Add status alert
        addStatusAlert(data) {
            // Check if we should show this alert based on Air/Ground filter settings
            if (!this.shouldShowAlert(data.hex)) {
                return; // Don't show alert if aircraft doesn't match current filter
            }
            
            // Create a unique ID for the alert
            const alertId = Date.now() + '-status-' + data.hex;
            
            // Add the alert to the array for tracking
            this.aircraftAlerts.push({
                id: alertId,
                hex: data.hex,
                type: 'status',
                status: data.new_status,
                timestamp: new Date()
            });
            
            // Get the alerts container
            const alertsContainer = document.getElementById('alerts-container');
            if (!alertsContainer) return;
            
            // Hide the "None" text
            const noAlertsText = document.getElementById('no-alerts-text');
            if (noAlertsText) {
                noAlertsText.style.display = 'none';
            }
            
            // Create the alert element
            const alertElement = document.createElement('div');
            alertElement.id = alertId;
            
            // Set color based on status
            let colorClass = 'text-yellow-300'; // Default for stale
            let iconClass = 'fa-exclamation-triangle';
            let statusText = data.new_status.toUpperCase();
            
            // Add left-click event to dismiss the alert
            alertElement.addEventListener('click', () => {
                this.removeAircraftAlert(alertId);
            });
            
            // Add right-click event to select the aircraft and center the map on it
            alertElement.addEventListener('contextmenu', (e) => {
                e.preventDefault(); // Prevent the default context menu
                this.selectAircraftByHex(data.hex);
            });
            
            if (data.new_status === 'signal_lost') {
                colorClass = 'text-gray-400'; // Use grey for signal_lost
                iconClass = 'fa-ban';
                statusText = ''; // No additional text, just callsign
            }
            
            alertElement.className = `inline-flex items-center text-xs px-1.5 py-0.5 rounded ${colorClass} cursor-pointer hover:bg-black/50`;
            
            // Add icon
            const icon = document.createElement('i');
            icon.className = `fas ${iconClass} fa-xs mr-1`;
            alertElement.appendChild(icon);
            
            // Add text
            const text = document.createElement('span');
            text.textContent = `${data.flight || data.hex}${statusText}`;
            alertElement.appendChild(text);
            
            // Add the alert to the container
            alertsContainer.appendChild(alertElement);
            
            // Remove the alert after 60 seconds
            setTimeout(() => {
                this.removeAircraftAlert(alertId);
            }, 60000);
        },

        // Add phase change alert
        addPhaseChangeAlert(data) {
            // Check if we should show this alert based on Air/Ground filter settings
            if (!this.shouldShowAlert(data.hex)) {
                return; // Don't show alert if aircraft doesn't match current filter
            }

            // Check if we should show this alert based on phase filter settings
            if (!this.shouldShowPhaseAlert(data.phase)) {
                return; // Don't show alert if phase is filtered out
            }
            
            // Create a unique ID for the alert
            const alertId = Date.now() + '-phase-' + data.hex;

            // Add the alert to the array for tracking
            this.aircraftAlerts.push({
                id: alertId,
                hex: data.hex,
                type: 'phase_change',
                data: data
            });

            // Get the alerts container
            const alertsContainer = document.getElementById('alerts-container');
            if (!alertsContainer) return;

            // Hide the "None" text
            const noAlertsText = document.getElementById('no-alerts-text');
            if (noAlertsText) {
                noAlertsText.style.display = 'none';
            }

            // Use centralized color and icon mapping
            const colorClass = this.getPhaseColorClass(data.phase);
            const iconClass = this.getPhaseIconClass(data.phase);
            const displayText = `${data.flight || data.hex} → ${data.phase}`;

            // Create the alert element
            const alertElement = document.createElement('div');
            alertElement.id = alertId;
            alertElement.className = `inline-flex items-center text-xs px-1.5 py-0.5 rounded ${colorClass} cursor-pointer hover:bg-black/50`;

            // Add left-click event to dismiss the alert
            alertElement.addEventListener('click', () => {
                this.removeAircraftAlert(alertId);
            });

            // Add right-click event to select the aircraft and center the map on it
            alertElement.addEventListener('contextmenu', (e) => {
                e.preventDefault(); // Prevent the default context menu
                this.selectAircraftByHex(data.hex);
            });

            // Create icon
            const icon = document.createElement('i');
            icon.className = `fas ${iconClass} fa-xs mr-1`;
            alertElement.appendChild(icon);

            // Create text
            const text = document.createElement('span');
            text.textContent = displayText;
            alertElement.appendChild(text);

            // Add the alert to the container
            alertsContainer.appendChild(alertElement);

            // Remove the alert after 60 seconds
            setTimeout(() => {
                this.removeAircraftAlert(alertId);
            }, 60000);
        },

        // Aircraft events are now handled as phase changes (T/O and T/D)

        // Remove aircraft alert
        removeAircraftAlert(alertId) {
            // Remove from the array
            this.aircraftAlerts = this.aircraftAlerts.filter(alert => alert.id !== alertId);
            
            // Remove from the DOM
            const alertElement = document.getElementById(alertId);
            if (alertElement) {
                alertElement.remove();
            }
            
            // Show the "None" text if there are no alerts
            if (this.aircraftAlerts.length === 0) {
                const noAlertsText = document.getElementById('no-alerts-text');
                if (noAlertsText) {
                    noAlertsText.style.display = 'block';
                }
            }
        },

        // Clear all aircraft alerts
        clearAllAircraftAlerts() {
            // Remove all alerts from DOM
            this.aircraftAlerts.forEach(alert => {
                const alertElement = document.getElementById(alert.id);
                if (alertElement) {
                    alertElement.remove();
                }
            });
            
            // Clear the array
            this.aircraftAlerts = [];
            
            // Show the "None" text
            const noAlertsText = document.getElementById('no-alerts-text');
            if (noAlertsText) {
                noAlertsText.style.display = 'block';
            }
        },

        getTranscriptionCount(frequencyId) {
            return this.unreadTranscriptions[frequencyId] || 0;
        },

        toggleTranscriptionViewer(frequencyId) {
            // Ensure the viewer state for this frequency is initialized
            if (this.transcriptionViewerVisible[frequencyId] === undefined) {
                this.transcriptionViewerVisible[frequencyId] = false;
            }
            
            // Toggle the state
            const newState = !this.transcriptionViewerVisible[frequencyId];
            this.transcriptionViewerVisible[frequencyId] = newState;

            // When opening: set read divider if there are unread messages, then clear count
            if (newState) {
                const unreadCount = this.unreadTranscriptions[frequencyId] || 0;
                const txns = this.frequencyTranscriptions[frequencyId];
                if (unreadCount > 0 && txns && txns.length > unreadCount) {
                    this._readDividerId[frequencyId] = txns[unreadCount]?.id;
                } else {
                    delete this._readDividerId[frequencyId];
                }
                this.unreadTranscriptions[frequencyId] = 0;
            }

            // When closing: clear the read divider
            if (!newState) {
                delete this._readDividerId[frequencyId];
            }

            // If closing the viewer, reset the search term
            if (!newState && this.transcriptionSearchTerm) {
                this.transcriptionSearchTerm = '';
                // Reload the original transcriptions if needed
                this.fetchTranscriptionsForFrequency(frequencyId);
            }
            
            // Position the transcription viewer correctly if it's being opened
            if (this.transcriptionViewerVisible[frequencyId]) {
                // Execute immediately with no delay
                const freqElement = document.querySelector(`[data-freq-id="${frequencyId}"]`);
                const viewer = document.querySelector(`[data-viewer-id="${frequencyId}"]`);
                
                if (freqElement && viewer) {
                    const rect = freqElement.getBoundingClientRect();
                    viewer.style.left = `${rect.left}px`;
                    viewer.style.width = `${rect.width}px`;
                    viewer.style.bottom = `${window.innerHeight - rect.top + 8}px`;
                    // Ensure no transitions or animations
                    viewer.style.transition = 'none';
                    viewer.style.transform = 'none';
                }
            }
        },
        
        // Fetch transcriptions for a specific frequency
        async fetchTranscriptionsForFrequency(frequencyId) {
            try {
                const response = await fetch(`${API_BASE_URL}/transcriptions/frequency/${frequencyId}`);
                if (response.ok) {
                    const data = await response.json();
                    if (data && data.transcriptions) {
                        // Normalize API field names to match WS format
                        const normalized = data.transcriptions.map(t => ({
                            ...t,
                            timestamp: t.created_at || t.timestamp,
                            text: t.content || t.text,
                        }));
                        // Sort newest first
                        this.frequencyTranscriptions[frequencyId] = normalized.sort((a, b) =>
                            new Date(b.timestamp) - new Date(a.timestamp)
                        );
                    }
                }
                return this.frequencyTranscriptions[frequencyId] || [];
            } catch (error) {
                console.error('Error fetching transcriptions:', error);
                return this.frequencyTranscriptions[frequencyId] || [];
            }
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
                this.stationElevationFeet = data.elevation_feet;
                this.stationCruiseAltitudeFt = Number.isFinite(data.cruise_altitude_ft) ? data.cruise_altitude_ft : 18000;
                this.stationAirportCode = data.airport_code;
                
                // Check if station override is active and restore state
                if (data.override_active) {
                    this.stationOverride.latitude = data.latitude;
                    this.stationOverride.longitude = data.longitude;
                    this.stationOverride.active = true;
                    console.log('Station override restored from server:', {
                        lat: data.latitude,
                        lon: data.longitude
                    });
                    
                    // Update station rings to the override coordinates
                    if (this.mapManager) {
                        this.updateStationRings();
                    }
                } else {
                    // Ensure override state is cleared if not active
                    this.stationOverride.active = false;
                    this.stationOverride.latitude = null;
                    this.stationOverride.longitude = null;
                    
                    // Update station rings to use the new coordinates from API
                    if (this.mapManager && this.mapManager.map) {
                        this.updateStationRings();
                        this.applyMapDisplaySettings();
                    }
                }
                
                // Store weather configuration flags
                this.stationFetchMETAR = data.fetch_metar;
                this.stationFetchTAF = data.fetch_taf;
                this.stationFetchNOTAMs = data.fetch_notams;
                
                // Store runway data if available
                if (data.runways) {
                    this.runwayData = data.runways;
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

        async fetchRunwayInUse() {
            try {
                const response = await fetch(this.stationApiUrl);
                if (!response.ok) return;
                const data = await response.json();
                this.runwayInUse = data.runway_in_use || null;
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
        
        // Toggle METAR details visibility
        toggleMetarDetails() {
            // Initialize if undefined
            if (this.metarDetailsVisible === undefined) {
                this.metarDetailsVisible = false;
            }
            
            // Toggle the state
            this.metarDetailsVisible = !this.metarDetailsVisible;
            
            // Close other popups
            this.tafDetailsVisible = false;
            this.notamDetailsVisible = false;
            
            // Position the popup correctly if it's being opened
            if (this.metarDetailsVisible) {
                setTimeout(() => {
                    const metarElement = document.querySelector('[data-metar-button]');
                    const metarPopup = document.querySelector('[data-metar-popup]');
                    
                    if (metarElement && metarPopup) {
                        const rect = metarElement.getBoundingClientRect();
                        metarPopup.style.left = `${rect.left + (rect.width / 2)}px`;
                        metarPopup.style.bottom = `${window.innerHeight - rect.top + 8}px`;
                        metarPopup.style.transform = 'translateX(-50%)';
                        metarPopup.style.transition = 'none';
                    }
                }, 0);
            }
        },
        
        // Toggle TAF details visibility
        toggleTAFDetails() {
            // Initialize if undefined
            if (this.tafDetailsVisible === undefined) {
                this.tafDetailsVisible = false;
            }
            
            // Toggle the state
            this.tafDetailsVisible = !this.tafDetailsVisible;
            
            // Close other popups
            this.metarDetailsVisible = false;
            this.notamDetailsVisible = false;
            
            // Position the popup correctly if it's being opened
            if (this.tafDetailsVisible) {
                setTimeout(() => {
                    const tafElement = document.querySelector('[data-taf-button]');
                    const tafPopup = document.querySelector('[data-taf-popup]');
                    
                    if (tafElement && tafPopup) {
                        const rect = tafElement.getBoundingClientRect();
                        tafPopup.style.left = `${rect.left + (rect.width / 2)}px`;
                        tafPopup.style.bottom = `${window.innerHeight - rect.top + 8}px`;
                        tafPopup.style.transform = 'translateX(-50%)';
                        tafPopup.style.transition = 'none';
                    }
                }, 0);
            }
        },
        
        // Toggle NOTAM details visibility
        toggleNOTAMDetails() {
            // Initialize if undefined
            if (this.notamDetailsVisible === undefined) {
                this.notamDetailsVisible = false;
            }
            
            // Toggle the state
            this.notamDetailsVisible = !this.notamDetailsVisible;
            
            // Close other popups
            this.metarDetailsVisible = false;
            this.tafDetailsVisible = false;
            
            // Position the popup correctly if it's being opened
            if (this.notamDetailsVisible) {
                setTimeout(() => {
                    const notamElement = document.querySelector('[data-notam-button]');
                    const notamPopup = document.querySelector('[data-notam-popup]');
                    
                    if (notamElement && notamPopup) {
                        const rect = notamElement.getBoundingClientRect();
                        notamPopup.style.left = `${rect.left + (rect.width / 2)}px`;
                        notamPopup.style.bottom = `${window.innerHeight - rect.top + 8}px`;
                        notamPopup.style.transform = 'translateX(-50%)';
                        notamPopup.style.transition = 'none';
                    }
                }, 0);
            }
        },
        
        // Get the TAF data
        getTAF() {
            return this.taf;
        },
        
        // Get the NOTAM data
        getNOTAMs() {
            return this.notams;
        },
        
        // Get the count of NOTAMs
        getNOTAMCount() {
            if (!this.notams || !Array.isArray(this.notams)) {
                return 0;
            }
            return this.notams.length;
        },
        
        // Get the count of TAF decoded items
        getTAFCount() {
            if (!this.taf || !this.taf.decoded || !Array.isArray(this.taf.decoded)) {
                return 0;
            }
            return this.taf.decoded.length;
        },

        async fetchAudioFrequencies() {
            try {
                const response = await fetch(this.audioApiUrl);
                if (!response.ok) {
                    console.error(`HTTP error fetching frequencies! Status: ${response.status}`);
                    this.audioFrequencies = [];
                    return;
                }
                const data = await response.json();
                if (data && data.frequencies) {
                    this.audioFrequencies = data.frequencies;

                    // Populate connection status from API response
                    data.frequencies.forEach(freq => {
                        if (freq.status && freq.status !== 'available') {
                            this.frequencyConnectionStatus[freq.id] = {
                                status: freq.status,
                                error: freq.last_error || null,
                                lastUpdate: Date.now()
                            };
                            console.log(`Frequency ${freq.id} initial status: ${freq.status}`, freq.last_error || '');
                        }
                    });

                    // Load historical transcriptions per frequency
                    if (data.transcriptions) {
                        for (const [freqId, txns] of Object.entries(data.transcriptions)) {
                            if (txns && txns.length > 0) {
                                // Normalize API field names to match WS format and mark as historical
                                const normalized = txns.map(t => ({
                                    ...t,
                                    timestamp: t.created_at,
                                    text: t.content,
                                    _historical: true
                                }));
                                this.frequencyTranscriptions[freqId] = normalized;
                                this.originalTranscriptions[freqId] = [...normalized];
                            }
                        }
                    }

                    // DO NOT connect to all frequencies immediately here.
                    // Let the user click "Start Radios"
                    // this.connectToAllFrequencies();
                    // Instead, just prepare them (create elements, setup viz graph)
                    this.prepareAllFrequencies();
                } else {
                    this.audioFrequencies = [];
                }
            } catch (error) {
                console.error('Error fetching audio frequencies:', error);
                this.audioFrequencies = [];
            }
        },

        // Renamed from connectToAllFrequencies to reflect its new role
        prepareAllFrequencies() {
            // REMOVED: this.initAudioContext(); // Ensure audio context is ready - This was causing the error.
            // The audioClient.prepareFrequency (called below) handles context initialization.
            this.audioFrequencies.forEach(freq => {
                this.prepareFrequency(freq); // This will now also setup visualization graph
            });
        },

        prepareFrequency(frequency) {
            if (!audioClient) { console.warn("audioClient not ready in prepareFrequency"); return; }
            audioClient.prepareFrequency(frequency);
        },

        connectToFrequency(frequency) {
            if (!audioClient) { console.warn("audioClient not ready in connectToFrequency"); return; }
            audioClient.connectToFrequency(frequency);
        },

        startAllRadios() {
            if (!audioClient) { console.warn("audioClient not ready in startAllRadios"); return; }
            audioClient.startAllRadios();
        },

        toggleMute(frequency) {
            if (!audioClient) { console.warn("audioClient not ready in toggleMute"); return; }
            audioClient.toggleMute(frequency);
        },

        cleanupFrequency(frequencyId) {
            if (!audioClient) { console.warn("audioClient not ready in cleanupFrequency"); return; }
            audioClient.cleanupFrequency(frequencyId);
        },

        // Handle frequency connection status changes from WebSocket
        handleFrequencyStatusChange(data) {
            if (!data || !data.frequency_id) {
                console.warn('Invalid frequency status data:', data);
                return;
            }

            const { frequency_id, status, error } = data;
            console.log(`Frequency ${frequency_id} status changed to: ${status}`, error ? `(${error})` : '');

            // Update the status tracking object
            this.frequencyConnectionStatus[frequency_id] = {
                status: status,
                error: error || null,
                lastUpdate: Date.now()
            };
        },

        // Check if a frequency is in a failed state
        isFrequencyFailed(frequencyId) {
            const statusInfo = this.frequencyConnectionStatus[frequencyId];
            return statusInfo && statusInfo.status === 'failed';
        },

        // Check if a frequency is connecting
        isFrequencyConnecting(frequencyId) {
            const statusInfo = this.frequencyConnectionStatus[frequencyId];
            return statusInfo && statusInfo.status === 'connecting';
        },

        // Check if a frequency is connected
        isFrequencyConnected(frequencyId) {
            const statusInfo = this.frequencyConnectionStatus[frequencyId];
            return statusInfo && statusInfo.status === 'connected';
        },

        // Get the connection status for a frequency
        getFrequencyStatus(frequencyId) {
            const statusInfo = this.frequencyConnectionStatus[frequencyId];
            return statusInfo ? statusInfo.status : 'unknown';
        },

        // Get error message for a frequency
        getFrequencyError(frequencyId) {
            const statusInfo = this.frequencyConnectionStatus[frequencyId];
            return statusInfo ? statusInfo.error : null;
        },

        // Play welcome sound
        playWelcomeSound() {
            if (this.splashScreenAudioPlayed) return;
            
            try {
                const audio = new Audio('/sounds/airplane-ding-dong.mp3');
                audio.volume = 0.7; // Set volume to 70%
                audio.play().then(() => {
                    console.log('[Alpine Store] Welcome sound played successfully');
                    this.splashScreenAudioPlayed = true;
                }).catch(err => {
                    console.error('[Alpine Store] Error playing welcome sound:', err);
                });
            } catch (err) {
                console.error('[Alpine Store] Error creating audio element:', err);
            }
        },
        
        // Close splash screen and play sound
        closeSplashScreen() {
            // Play the welcome sound when user clicks the button
            this.playWelcomeSound();
            
            // Hide the splash screen
            this.showSplashScreen = false;
            
            // Now that the splash screen is closed, we can start showing connection lost messages if needed
            // This ensures the connection lost overlay never appears during initial loading
            this.initialDataLoaded = true;
            
            console.log('[Alpine Store] Splash screen closed');
        },
        
        // Setup keyboard event listeners
        setupKeyboardEvents() {
            document.addEventListener('keydown', (e) => {
                // Skip if user is typing in an input field
                const isInInputField = e.target.tagName === 'INPUT' || 
                                     e.target.tagName === 'TEXTAREA' || 
                                     e.target.contentEditable === 'true';
                
                // ESC key to close aircraft details
                if (e.key === 'Escape' && this.selectedAircraft) {
                    const previousHex = this.selectedAircraft.hex;
                    this.selectedAircraft = null;
                    this.aircraftDetailsShowHistoryView = false;
                    this.showProximityView = false;
                    this.aircraftDetailsHistoryData = [];
                    this.aircraftDetailsFutureData = [];
                this.aircraftDetailsHindcastData = [];
                    this.aircraftDetailsHistoryCount = 0;
                    this.phaseHistoryData = [];
                    this.phaseHistoryAircraftHex = null;
                    this.aircraftDetailsStopHistoryRefresh();
                    this.stopProximityRefresh();
                    this.stopPhaseHistoryRefresh();
                    this.clearProximityView();

                    // Clear map trails for the deselected aircraft
                    if (this.mapManager && previousHex) {
                        this.mapManager.clearAircraftTrails(previousHex);
                    }
                }
                
                // TAB key for aircraft navigation (only when not in input fields)
                if (e.key === 'Tab' && !isInInputField) {
                    e.preventDefault();
                    
                    if (e.shiftKey) {
                        this.cycleToPreviousAircraft();
                    } else {
                        this.cycleToNextAircraft();
                    }
                }
                
                // Skip other hotkeys if user is typing in an input field
                if (isInInputField) return;
                
                // Air/Ground filter hotkeys
                if (e.key.toLowerCase() === 'a') {
                    e.preventDefault();
                    this.toggleAirAircraft();
                    console.log('[Hotkey] Toggled Air filter:', this.settings.showAirAircraft);
                }
                
                if (e.key.toLowerCase() === 'g') {
                    e.preventDefault();
                    this.toggleGroundAircraft();
                    console.log('[Hotkey] Toggled Ground filter:', this.settings.showGroundAircraft);
                }
                
                // Flight phase hotkeys (1-9, 0) - matches UI filter bar order
                const phaseKeys = {
                    '1': 'NEW',   // New
                    '2': 'TAX',   // Taxi
                    '3': 'T/O',   // Takeoff
                    '4': 'CLB',   // Climb
                    '5': 'DEP',   // Departure
                    '6': 'CRZ',   // Cruise
                    '7': 'ARR',   // Arrival
                    '8': 'APP',   // Approach
                    '9': 'T/D',   // Touchdown
                    '0': 'UNK'    // Unknown
                };
                
                if (phaseKeys[e.key]) {
                    e.preventDefault();
                    const phase = phaseKeys[e.key];
                    this.togglePhaseFilter(phase);
                    console.log(`[Hotkey] Toggled ${phase} phase filter:`, this.settings.phaseFilters[phase]);
                }
            });
        },

        // Play connection lost sound
        playConnectionLostSound() {
            if (this.connectionLostSoundPlayed) return;
            
            try {
                // const audio = new Audio('/sounds/airbus_retard.mp3');
                // audio.volume = 0.8; // Set volume to 80%
                // audio.play().then(() => {
                //     console.log('[Alpine Store] Connection lost sound played successfully');
                //     this.connectionLostSoundPlayed = true;
                // }).catch(err => {
                //     console.error('[Alpine Store] Error playing connection lost sound:', err);
                // });
                if (audioClient) {
                    audioClient.playRetardSound();
                    this.connectionLostSoundPlayed = true; // Assume it plays successfully
                } else {
                    console.error('[Alpine Store] audioClient not available to play connection lost sound.');
                }
            } catch (err) {
                console.error('[Alpine Store] Error initiating connection lost sound:', err);
            }
        },

        // Heading helpers for map icon rotation and label suffixes
        normalizeHeading(value) {
            if (!Number.isFinite(value)) return null;
            return ((value % 360) + 360) % 360;
        },

        hasHeadingValue(value) {
            return value !== undefined && value !== null && Number.isFinite(value);
        },

        _isLikelyDefaultZeroHeading(value, adsbLike) {
            if (!this.hasHeadingValue(value)) return false;
            if (Math.abs(value) > 0.0001) return false;

            const track = this.hasHeadingValue(adsbLike?.track) ? this.normalizeHeading(adsbLike.track) : null;
            const trueHeading = this.hasHeadingValue(adsbLike?.true_heading) ? this.normalizeHeading(adsbLike.true_heading) : null;
            const magHeading = this.hasHeadingValue(adsbLike?.mag_heading) ? this.normalizeHeading(adsbLike.mag_heading) : null;

            const hasNonZeroTrack = track !== null && Math.abs(track) > 0.0001;
            const hasNonZeroTrue = trueHeading !== null && Math.abs(trueHeading) > 0.0001;
            const hasNonZeroMag = magHeading !== null && Math.abs(magHeading) > 0.0001;

            return hasNonZeroTrack || hasNonZeroTrue || hasNonZeroMag;
        },

        _getResolvedHeadingCandidates(adsbLike) {
            const magHeading = this.hasHeadingValue(adsbLike?.mag_heading)
                ? this.normalizeHeading(adsbLike.mag_heading)
                : null;
            const track = this.hasHeadingValue(adsbLike?.track)
                ? this.normalizeHeading(adsbLike.track)
                : null;
            const trueHeading = this.hasHeadingValue(adsbLike?.true_heading)
                ? this.normalizeHeading(adsbLike.true_heading)
                : null;

            const magIsUsable = magHeading !== null && !this._isLikelyDefaultZeroHeading(magHeading, adsbLike);
            const trueIsUsable = trueHeading !== null && !this._isLikelyDefaultZeroHeading(trueHeading, adsbLike);
            const trackIsUsable = track !== null;

            return {
                magHeading,
                track,
                trueHeading,
                magIsUsable,
                trackIsUsable,
                trueIsUsable
            };
        },

        // Get heading with fallback priority: magnetic -> track -> true
        getHeadingWithFallback(aircraft) {
            if (!aircraft?.adsb) return null;

            const candidates = this._getResolvedHeadingCandidates(aircraft.adsb);
            if (candidates.magIsUsable) return candidates.magHeading;
            if (candidates.trackIsUsable) return candidates.track;
            if (candidates.trueIsUsable) return candidates.trueHeading;
            return null;
        },

        getHeadingWithTypeFromAdsb(adsbLike) {
            if (!adsbLike) return { value: null, type: null };

            const candidates = this._getResolvedHeadingCandidates(adsbLike);
            if (candidates.magIsUsable) return { value: candidates.magHeading, type: 'magnetic' };
            if (candidates.trackIsUsable) return { value: candidates.track, type: 'track' };
            if (candidates.trueIsUsable) return { value: candidates.trueHeading, type: 'true' };

            return { value: null, type: null };
        },

        getHeadingWithType(aircraft) {
            if (!aircraft?.adsb) return { value: null, type: null };
            return this.getHeadingWithTypeFromAdsb(aircraft.adsb);
        },

        getHeadingDisplayTextFromAdsb(adsbLike) {
            const heading = this.getHeadingWithTypeFromAdsb(adsbLike);
            if (heading.value === null) return '-';
            return `${Math.round(heading.value)}°`;
        },

        getHeadingDisplayText(aircraft) {
            const heading = this.getHeadingWithType(aircraft);
            if (heading.value === null) return '-';
            return `${Math.round(heading.value)}°`;
        },

        formatPredictionTime(position) {
            if (!position || !position.timestamp) return '';
            const secs = Math.max(0, Math.round((new Date(position.timestamp).getTime() - Date.now()) / 1000));
            const m = Math.floor(secs / 60);
            const s = secs % 60;
            if (s === 0 && m > 0) return `+${m} min`;
            if (m === 0) return `+${secs}s`;
            return `+${m}:${String(s).padStart(2, '0')}`;
        },

        getFutureDataForTable() {
            // Show every 3rd point (15s intervals from 5s granularity data)
            return this.aircraftDetailsFutureData.filter((_, i) => (i + 1) % 3 === 0);
        },

        // Build flat array for history table rendering from server-deduplicated data.
        // Server provides skipped_before/skipped_after on positions; we convert to
        // { type: 'row', position, originalIndex, beforeDivider } and { type: 'divider', count } entries.
        getFilteredHistoryData() {
            const data = this.aircraftDetailsHistoryData;
            if (!data || data.length === 0) return [];

            const result = [];
            for (let i = 0; i < data.length; i++) {
                const pos = data[i];

                // Divider before this row (skipped duplicates between prev and this)
                if (pos.skipped_before > 0) {
                    // Mark the previous row as being before a divider (suppress its border)
                    if (result.length > 0 && result[result.length - 1].type === 'row') {
                        result[result.length - 1].beforeDivider = true;
                    }
                    result.push({ type: 'divider', count: pos.skipped_before });
                }

                result.push({ type: 'row', position: pos, originalIndex: i, beforeDivider: false });

                // Trailing divider after last row
                if (pos.skipped_after > 0) {
                    result[result.length - 1].beforeDivider = true;
                    result.push({ type: 'divider', count: pos.skipped_after });
                }
            }

            return result;
        },

        getHeadingSuffix(type) {
            switch (type) {
                case 'magnetic':
                    return 'hdg(m)';
                case 'track':
                    return 'hdg(trk)';
                case 'true':
                    return 'hdg(t)';
                default:
                    return '-';
            }
        },

        // WebSocket debugging state
        wsUpdatesPaused: false,
        // Removed mapUpdatesDisabled and dataOnlyMode debug parameters

        // Toggle WebSocket updates for troubleshooting
        toggleWebSocketUpdates() {
            this.wsUpdatesPaused = !this.wsUpdatesPaused;
            console.log(`WebSocket updates ${this.wsUpdatesPaused ? 'PAUSED' : 'RESUMED'}`);

            if (this.wsUpdatesPaused) {
                // Clear any pending updates when pausing
                this.cleanupThrottling();
                // PERFORMANCE: Stop animation engine when updates are paused
                if (this.animationEngine) {
                    this.animationEngine.stop();
                    console.log('Animation engine STOPPED');
                }
            } else {
                // Resume animation engine when updates resume
                if (this.animationEngine) {
                    this.animationEngine.start();
                    console.log('Animation engine STARTED');
                }
            }
        },

        // Removed toggleMapUpdates() and toggleDataOnlyMode() debug methods

        // Clean up throttling mechanisms to prevent memory leaks
        cleanupThrottling() {
            if (this.mapUpdateThrottleId) {
                clearTimeout(this.mapUpdateThrottleId);
                this.mapUpdateThrottleId = null;
            }
            this.pendingMapUpdates.clear();
            this.cacheInvalidationPending = false;
        }
    });

    // Initialize audioClient AFTER the store is defined
    audioClient = new AudioClient(Alpine.store('atc'));
    
    // Initialize OpenLayers map manager only
    if (!window.OpenLayersMapManager || typeof window.OpenLayersMapManager.createOpenLayersMapManager !== 'function') {
        throw new Error('OpenLayersMapManager is unavailable; OpenLayers runtime bootstrap failed');
    }
    mapManager = window.OpenLayersMapManager.createOpenLayersMapManager(Alpine.store('atc'), CONFIG);
    Alpine.store('atc').mapManager = mapManager; // Make mapManager accessible in the store

    // Initialize Aircraft Animation Engine
    animationEngine = new AircraftAnimationEngine(mapManager, Alpine.store('atc'));
    Alpine.store('atc').animationEngine = animationEngine; // Make animation engine accessible in the store
    animationEngine.initialize();

    // Now initialize the store's own logic
    Alpine.store('atc').init();

    // Watch for aircraft selection changes to clear pending requests and trails
    Alpine.effect(() => {
        const store = Alpine.store('atc');
        const currentSelectedHex = store.selectedAircraft?.hex || null;
        const previousSelectedHex = store._previousSelectedHex || null;

        if (currentSelectedHex === previousSelectedHex) {
            return;
        }

        // If aircraft selection changed
        if (previousSelectedHex && previousSelectedHex !== currentSelectedHex) {
            // Clear pending requests for the previous aircraft
            store.clearPendingRequestsForAircraft(previousSelectedHex);

            // Clear trails for the previous aircraft when deselecting
            if (store.mapManager) {
                store.mapManager.clearAircraftTrails(previousSelectedHex);
            }
        }

        // Store current selection for next comparison
        store._previousSelectedHex = currentSelectedHex;

        // Refresh map visibility when selected aircraft changes to show/hide aircraft based on filters
        if (store.mapManager) {
            store.mapManager.applyFiltersAndRefreshView();
        }
    });
});
