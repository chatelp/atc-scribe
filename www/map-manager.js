class MapManager {
    
    // Initialize tracks mini-map
    initTracksMiniMap(containerId, retryCount = 0) {
        // Clean up any existing mini-map first
        this.cleanupTracksMiniMap();
        
        setTimeout(() => {
            const container = document.getElementById(containerId);
            if (!container) {
                console.warn('Mini-map container not found:', containerId);
                return;
            }
            
            // Ensure container is visible and has dimensions
            if (container.offsetWidth === 0 || container.offsetHeight === 0) {
                if (retryCount < 5) { // Limit retries to prevent infinite loops
                    console.warn(`Mini-map container has no dimensions, retrying... (${retryCount + 1}/5)`);
                    setTimeout(() => this.initTracksMiniMap(containerId, retryCount + 1), 500);
                } else {
                    console.error('Mini-map container failed to get dimensions after 5 retries, giving up');
                }
                return;
            }
            
            try {
                // Create mini-map instance
                this.tracksMiniMap = L.map(containerId, {
                    zoomControl: false,
                    attributionControl: false,
                    dragging: true,
                    scrollWheelZoom: true,
                    doubleClickZoom: true,
                    boxZoom: true,
                    keyboard: true
                });
                
                // Add dark tile layer (same as main map)
                L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
                    maxZoom: 18,
                    opacity: 1.0,
                    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>'
                }).addTo(this.tracksMiniMap);
                
                // Force map to recognize its container size
                setTimeout(() => {
                    if (this.tracksMiniMap) {
                        this.tracksMiniMap.invalidateSize();
                    }
                }, 100);
            } catch (error) {
                console.error('Error creating mini-map:', error);
                this.tracksMiniMap = null;
                return;
            }
            
            // Set initial view
            const store = Alpine.store('atc');
            if (store.selectedAircraft && store.selectedAircraft.adsb) {
                this.tracksMiniMap.setView([
                    store.selectedAircraft.adsb.lat,
                    store.selectedAircraft.adsb.lon
                ], 12);
            } else {
                // Default view
                this.tracksMiniMap.setView([43.6777, -79.6248], 10);
            }
            
            // Initialize layers for tracks
            this.tracksMiniMapLayers = {
                hindcast: L.layerGroup().addTo(this.tracksMiniMap),
                historical: L.layerGroup().addTo(this.tracksMiniMap),
                future: L.layerGroup().addTo(this.tracksMiniMap),
                current: L.layerGroup().addTo(this.tracksMiniMap)
            };
            
            // Update tracks when data changes
            this.updateTracksMiniMap();
        }, 500);
    }
    
    // Update tracks mini-map with current data
    updateTracksMiniMap() {
        if (!this.tracksMiniMap || !this.tracksMiniMapLayers) return;
        
        const store = Alpine.store('atc');
        if (!store.aircraftDetailsShowHistoryView) return;
        
        // Check if map container still exists
        if (!this.tracksMiniMap.getContainer() || !document.body.contains(this.tracksMiniMap.getContainer())) {
            this.tracksMiniMap = null;
            this.tracksMiniMapLayers = null;
            return;
        }
        
        // Clear existing layers
        this.tracksMiniMapLayers.hindcast.clearLayers();
        this.tracksMiniMapLayers.historical.clearLayers();
        this.tracksMiniMapLayers.future.clearLayers();
        this.tracksMiniMapLayers.current.clearLayers();
        
        const aircraft = store.selectedAircraft;
        if (!aircraft) return;
        
        // Add current position (or last known position if signal lost)
        if (aircraft.adsb && aircraft.adsb.lat && aircraft.adsb.lon) {
            const isSignalLost = aircraft.status === 'signal_lost';
            const currentMarker = L.circleMarker([aircraft.adsb.lat, aircraft.adsb.lon], {
                radius: 6,
                fillColor: isSignalLost ? '#F44336' : '#4CAF50',
                color: isSignalLost ? '#F44336' : '#4CAF50',
                weight: 2,
                opacity: 1,
                fillOpacity: 0.8
            }).addTo(this.tracksMiniMapLayers.current);
        }
        
        // Add historical track
        const historyData = store.aircraftDetailsHistoryData || [];
        const historyPoints = historyData
            .filter(pos => pos.lat && pos.lon)
            .map(pos => [pos.lat, pos.lon]);

        if (historyPoints.length > 1) {
            L.polyline(historyPoints, {
                color: '#888888',
                weight: 1,
                opacity: 0.5,
                dashArray: '3, 3'
            }).addTo(this.tracksMiniMapLayers.historical);
        }

        if (historyPoints.length > 0) {
            // Add markers for historical points
            historyPoints.forEach(point => {
                L.circleMarker(point, {
                    radius: 2,
                    fillColor: '#888888',
                    color: '#888888',
                    weight: 1,
                    opacity: 0.5,
                    fillOpacity: 0.3
                }).addTo(this.tracksMiniMapLayers.historical);
            });
        }

        // Add hindcast (predicted pre-coverage path)
        const hindcastData = store.aircraftDetailsHindcastData || [];
        if (hindcastData.length > 0) {
            const hindcastPoints = hindcastData
                .filter(pos => pos.lat && pos.lon)
                .map(pos => [pos.lat, pos.lon]);

            // Connect hindcast to first history point for visual continuity
            if (hindcastPoints.length > 0 && historyPoints.length > 0) {
                hindcastPoints.push(historyPoints[0]);
            }

            if (hindcastPoints.length > 1) {
                L.polyline(hindcastPoints, {
                    color: '#00BCD4',
                    weight: 1.5,
                    opacity: 0.5,
                    dashArray: '4, 6'
                }).addTo(this.tracksMiniMapLayers.hindcast);
            }

            // Add markers for hindcast points (excluding the connection point)
            hindcastData.filter(pos => pos.lat && pos.lon).forEach(pos => {
                L.circleMarker([pos.lat, pos.lon], {
                    radius: 2,
                    fillColor: '#00BCD4',
                    color: '#00BCD4',
                    weight: 1,
                    opacity: 0.4,
                    fillOpacity: 0.2
                }).addTo(this.tracksMiniMapLayers.hindcast);
            });
        }

        // Add future predictions
        const futureData = store.aircraftDetailsFutureData || [];
        if (futureData.length > 0) {
            const futurePoints = futureData
                .filter(pos => pos.lat && pos.lon)
                .map(pos => [pos.lat, pos.lon]);
            
            if (futurePoints.length > 1) {
                L.polyline(futurePoints, {
                    color: '#FFC107',
                    weight: 2,
                    opacity: 0.8,
                    dashArray: '10, 5'
                }).addTo(this.tracksMiniMapLayers.future);
            }
            
            // Add markers for future points
            futurePoints.forEach(point => {
                L.circleMarker(point, {
                    radius: 3,
                    fillColor: '#FFC107',
                    color: '#FFC107',
                    weight: 1,
                    opacity: 0.8,
                    fillOpacity: 0.6
                }).addTo(this.tracksMiniMapLayers.future);
            });
        }
        
        // Fit map to show all points
        const allPoints = [];
        if (aircraft.adsb && aircraft.adsb.lat && aircraft.adsb.lon) {
            allPoints.push([aircraft.adsb.lat, aircraft.adsb.lon]);
        }
        historyData.forEach(pos => {
            if (pos.lat && pos.lon) allPoints.push([pos.lat, pos.lon]);
        });
        hindcastData.forEach(pos => {
            if (pos.lat && pos.lon) allPoints.push([pos.lat, pos.lon]);
        });
        futureData.forEach(pos => {
            if (pos.lat && pos.lon) allPoints.push([pos.lat, pos.lon]);
        });
        
        if (allPoints.length > 0) {
            try {
                if (allPoints.length === 1) {
                    this.tracksMiniMap.setView(allPoints[0], 12);
                } else {
                    this.tracksMiniMap.fitBounds(allPoints, { padding: [10, 10] });
                }
            } catch (error) {
                console.warn('Error updating mini-map view:', error);
                // Fallback to a simple setView if fitBounds fails
                if (allPoints.length > 0) {
                    this.tracksMiniMap.setView(allPoints[0], 10);
                }
            }
        }
    }
    
    // Clean up tracks mini-map
    cleanupTracksMiniMap() {
        if (this.tracksMiniMap) {
            try {
                this.tracksMiniMap.off();
                this.tracksMiniMap.remove();
            } catch (error) {
                console.warn('Error cleaning up mini-map:', error);
            }
            this.tracksMiniMap = null;
            this.tracksMiniMapLayers = null;
        }
    }

    constructor(store, L, CONFIG) {
        this.store = store;
        this.L = L;
        this.CONFIG = CONFIG;

        this.map = null;
        this.layers = {
            aircraft: this.L.layerGroup(),
            trails: this.L.layerGroup(),
            rangeRings: this.L.layerGroup(),
            runways: this.L.layerGroup(),
            airports: this.L.layerGroup(),
            heliports: this.L.layerGroup(),
            navaids: this.L.layerGroup(),
            allRunways: this.L.layerGroup(),
        };
        this.markers = {}; // Stores Leaflet marker objects { hex: { aircraft: marker, label: labelMarker } }
        this.trails = {}; // Trails managed by MapManager
        this.proximityCircle = null; // For proximity visualization
        this.proximityHexSet = null; // Set of aircraft hex codes in proximity
        this.proximityRefHex = null; // Reference aircraft hex for proximity
        this.proximityDistanceNM = null; // Distance in NM for proximity circle

        // Reference data rendering state
        this.runwayData = null;
        this.airportsData = null;
        this.heliportsData = null;
        this.navaidsData = null;
        this.allRunwaysData = null;

        // Marker pooling for performance (reduces GC pressure)
        this.markerPool = {
            aircraft: [],
            labels: [],
            maxSize: 50  // Maximum pool size per type
        };

        // Trail version tracking for optimized updates
        this.trailVersions = {};

        // Keep longer trail history than current display window so increasing trail length can redraw immediately
        this._trailRetentionMinutes = 10;

        // Trail layer tracking - maps hex to array of layers for O(1) removal
        this.trailLayersByHex = {};

        // Coalesced refresh scheduling
        this._refreshScheduled = false;
        this._refreshThrottleTimer = null;
        this._refreshThrottleMs = 120;
        this._lastRefreshRunAt = 0;

        // Per-aircraft caches to avoid redundant layer/DOM work
        this._markerVisibilityState = {};
        this._visualStateVersionByHex = {};
        this._flightCardHoverStateByHex = {};

        // Lightweight map performance counters (cumulative)
        this._perfCounters = {
            refreshRequested: 0,
            refreshCoalesced: 0,
            refreshExecuted: 0,
            refreshDurationMs: 0,
            markerAdds: 0,
            markerRemoves: 0,
            labelAdds: 0,
            labelRemoves: 0,
            visualStateApplied: 0,
            visualStateSkipped: 0,
            trailLayersAdded: 0,
            trailLayersRemoved: 0,
            singleAircraftUpdates: 0,
            fullVisibilityPasses: 0
        };
        this._perfSnapshot = {
            timestamp: performance.now(),
            counters: { ...this._perfCounters }
        };
    }

    // O(1) trail layer removal helper - removes all trail layers for a specific aircraft
    _removeTrailLayersByHex(hex) {
        const layers = this.trailLayersByHex[hex];
        if (layers && layers.length > 0) {
            this._perfCounters.trailLayersRemoved += layers.length;
            for (let i = 0; i < layers.length; i++) {
                this.layers.trails.removeLayer(layers[i]);
            }
            this.trailLayersByHex[hex] = [];
        }
    }

    // Helper to add a trail layer and track it for O(1) removal
    _addTrailLayer(hex, layer) {
        layer.addTo(this.layers.trails);
        this._perfCounters.trailLayersAdded++;
        if (!this.trailLayersByHex[hex]) {
            this.trailLayersByHex[hex] = [];
        }
        this.trailLayersByHex[hex].push(layer);
    }

    // Public method to clear rendered trails for an aircraft.
    // By default this only removes map layers and keeps trail history in memory.
    clearAircraftTrails(hex, options = {}) {
        if (!hex) return;
        const purgeData = options.purgeData === true;

        // Remove trail layers from map (O(1) operation)
        this._removeTrailLayersByHex(hex);
        // Clear layer tracking
        delete this.trailLayersByHex[hex];
        // Invalidate version cache so trails will be redrawn when needed
        delete this.trailVersions[hex];

        // Optional full data purge (for real aircraft removal / hard cleanup only)
        if (purgeData) {
            delete this.trails[hex];
        }
    }

    // Clean up orphaned trail data for aircraft no longer in the store
    cleanupOrphanedTrails() {
        const storeAircraftHexes = new Set(Object.keys(this.store.aircraft || {}));

        // Clean up trails for aircraft no longer in the store
        Object.keys(this.trails).forEach(hex => {
            if (!storeAircraftHexes.has(hex)) {
                // Aircraft is no longer in the store - clean up completely
                this._removeTrailLayersByHex(hex);
                delete this.trailLayersByHex[hex];
                delete this.trailVersions[hex];
                delete this.trails[hex];
            }
        });

        // Also check trailLayersByHex for orphaned entries
        Object.keys(this.trailLayersByHex).forEach(hex => {
            if (!storeAircraftHexes.has(hex)) {
                this._removeTrailLayersByHex(hex);
                delete this.trailLayersByHex[hex];
                delete this.trailVersions[hex];
            }
        });
    }

    // Create a new marker (pooling disabled - was causing glitches)
    _getPooledMarker(position, icon) {
        // Always create new markers - pooling caused visibility glitches
        return this.L.marker(position, { icon: icon, riseOnHover: true });
    }

    // Create a new label marker (pooling disabled - was causing glitches)
    _getPooledLabel(position, icon) {
        // Always create new markers - pooling caused visibility glitches
        return this.L.marker(position, { icon: icon, interactive: true });
    }

    // Release marker (pooling disabled - just remove from map)
    _releaseMarker(marker, type = 'aircraft') {
        // Clean up the marker properly
        marker.off(); // Remove all event listeners
        if (marker._map) {
            marker.removeFrom(marker._map);
        }
        // Let GC handle the marker (pooling disabled)
        return false;
    }

    initMap() {
        // This function is now primarily guarded by the `if (!this.map)` check in the store's init().
        const mapContainer = document.getElementById('map');
        if (mapContainer && mapContainer._leaflet_id) {
             console.warn("MapManager.initMap called, but map container already has _leaflet_id. Current map instance:", this.map);
             if (!this.map) throw new Error("Map container already initialized by Leaflet, but MapManager's 'map' is null.");
             return; 
        }
        if (this.map) {
            console.warn("MapManager.initMap called, but 'this.map' variable is already set. Not re-initializing.");
            return;
        }

        console.log("MapManager: Initializing Leaflet map on #map element...");
        
        // Determine the center coordinates - use override if active, otherwise station coordinates
        const centerLat = this.store.stationOverride.active
            ? (this.store.stationOverride.latitude || this.store.stationLatitude || 43.6777)
            : (this.store.stationLatitude || 43.6777);
        const centerLon = this.store.stationOverride.active
            ? (this.store.stationOverride.longitude || this.store.stationLongitude || -79.6248)
            : (this.store.stationLongitude || -79.6248);
            
        console.log(`MapManager: Centering map on station coordinates: ${centerLat}, ${centerLon}`);

        // Canvas renderer for vector layers (circles, polylines)
        // padding: 0.5 = 50% buffer beyond viewport — enough for smooth pan without excess overdraw
        this.canvasRenderer = this.L.canvas({ padding: 0.5 });

        this.map = this.L.map('map', {
            center: [centerLat, centerLon],
            zoom: this.CONFIG.defaultZoom,
            zoomControl: false,
            attributionControl: false,
            keyboard: false,  // Disable Leaflet's default keyboard navigation to prevent conflicts with custom hotkeys
            preferCanvas: true,  // Use Canvas renderer by default for vector layers
            renderer: this.canvasRenderer  // Set default renderer
        });
        
        // Add event listeners for viewport changes to update visible aircraft list
        // PERFORMANCE: Throttle to prevent excessive updates during rapid pan/zoom
        this._visibleListUpdateTimeout = null;
        this.map.on('moveend zoomend', () => {
            if (this._visibleListUpdateTimeout) return;
            this._visibleListUpdateTimeout = setTimeout(() => {
                this._visibleListUpdateTimeout = null;
                this.updateVisibleAircraftList();
            }, 250); // 250ms throttle
        });

        this.L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
            attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> &copy; <a href="https://carto.com/attributions">CARTO</a>',
            subdomains: 'abcd',
            maxZoom: 19
        }).addTo(this.map);

        this.layers.allRunways.addTo(this.map);
        this.layers.airports.addTo(this.map);
        this.layers.heliports.addTo(this.map);
        this.layers.navaids.addTo(this.map);
        this.layers.aircraft.addTo(this.map);
        this.layers.trails.addTo(this.map);
        this.layers.runways.addTo(this.map);
        
        // Only add range rings layer and rings if the setting is enabled
        if (this.store.settings.showRings) {
            this.layers.rangeRings.addTo(this.map);
            this.addRangeRings();
        }
        
        this.map.on('click', (e) => {
            // Check if click is on the map itself (not on markers/labels)
            // Include canvas element class for Canvas renderer
            const target = e.originalEvent.target;
            const isMapClick = target.classList.contains('leaflet-container') ||
                              target.classList.contains('leaflet-tile') ||
                              target.classList.contains('leaflet-pane') ||
                              target.classList.contains('leaflet-zoom-animated') ||
                              target.tagName === 'CANVAS';
            
            if (isMapClick) {
                // Check if we're in station override map click mode
                if (this.store.stationOverride.mapClickMode) {
                    // Set coordinates from map click
                    this.store.stationOverride.latitude = e.latlng.lat;
                    this.store.stationOverride.longitude = e.latlng.lng;
                    this.store.stationOverride.mapClickMode = false;
                    
                    // Remove click indicator
                    this.store.hideMapClickIndicator();
                    
                    console.log('Station coordinates set from map:', e.latlng);
                    return; // Don't process other click actions
                }
                
                // Existing click behavior - deselect aircraft
                if (this.store.selectedAircraft) {
                    this.store.selectedAircraft = null;
                    // Visual update will be triggered by Alpine.effect in app.js watching selectedAircraft
                }
            }
        });

        this.map.on('dblclick', (e) => {
            // Check if double-click is on the map itself (not on markers/labels)
            const target = e.originalEvent.target;
            const isMapClick = target.classList.contains('leaflet-container') ||
                              target.classList.contains('leaflet-tile') ||
                              target.classList.contains('leaflet-pane') ||
                              target.classList.contains('leaflet-zoom-animated') ||
                              target.tagName === 'CANVAS';

            if (isMapClick && this.store.searchTerm) {
                this.store.searchTerm = '';
                this.store.applyFilters();
            }
        });

        // Start periodic label refresh timer to keep lastSeen timestamps updating
        this.startLabelRefreshTimer();
    }

    // Refresh stale aircraft labels periodically (only for aircraft without recent updates)
    startLabelRefreshTimer() {
        // Clear any existing timer
        if (this.labelRefreshTimer) {
            clearInterval(this.labelRefreshTimer);
        }

        // Refresh stale labels every 5 seconds (much less aggressive than every 1 second)
        this.labelRefreshTimer = setInterval(() => {
            this.refreshStaleLabels();
        }, 5000);
    }

    // Update only stale aircraft labels (those without recent WebSocket updates)
    refreshStaleLabels() {
        if (!this.store.settings.showLabels) return;

        const now = Date.now();
        const staleThreshold = 5000; // Only update labels for aircraft not updated in 5+ seconds
        const fadeStartThreshold = 10000; // Start opacity fade after 10s

        // Batch DOM updates in a single frame
        requestAnimationFrame(() => {
            const hexes = Object.keys(this.markers);
            for (let i = 0; i < hexes.length; i++) {
                const hex = hexes[i];
                const markerInfo = this.markers[hex];
                const aircraft = this.store.aircraft[hex];

                if (!markerInfo || !markerInfo.label || !aircraft || !aircraft.last_seen) continue;

                // Skip if label is not on map (not visible)
                if (!this.layers.aircraft.hasLayer(markerInfo.label)) continue;

                // Only update stale aircraft (not recently updated via WebSocket)
                const lastSeenTime = new Date(aircraft.last_seen).getTime();
                const timeSinceUpdate = now - lastSeenTime;
                if (timeSinceUpdate < staleThreshold) continue;

                // Update lastSeen text directly in DOM for stale aircraft
                const labelElement = markerInfo.label.getElement();
                if (labelElement) {
                    if (!markerInfo.lastSeenSpan || !markerInfo.lastSeenSpan.isConnected) {
                        markerInfo.lastSeenSpan = labelElement.querySelector('[data-lastseen]');
                    }
                    const lastSeenSpan = markerInfo.lastSeenSpan;
                    if (lastSeenSpan) {
                        const secondsAgo = Math.floor(timeSinceUpdate / 1000);
                        const newText = `${secondsAgo}s`;
                        if (lastSeenSpan.textContent !== newText) {
                            lastSeenSpan.textContent = newText;
                        }
                    }
                }

                // Re-evaluate visual state for fading aircraft (opacity + red label)
                if (timeSinceUpdate >= fadeStartThreshold) {
                    this.updateVisualState(hex);
                }
            }
        });
    }

    addRangeRings() {
        // Safety check: ensure map is initialized before adding range rings
        if (!this.map) {
            console.warn('MapManager: Cannot add range rings - map not initialized yet');
            return;
        }
        
        this.layers.rangeRings.clearLayers();
        // Use override coordinates if active, otherwise use station coordinates from API
        const centerLatLng = this.store.stationOverride.active
            ? [this.store.stationOverride.latitude || 0, this.store.stationOverride.longitude || 0]
            : [this.store.stationLatitude || 0, this.store.stationLongitude || 0];
        this.CONFIG.rangeRings.forEach(radius => {
            const circle = this.L.circle(centerLatLng, {
                radius: radius * 1852,
                color: 'rgba(115, 115, 115, 0.5)',  // neutral-600 with 50% opacity
                weight: 1,
                fill: false,
                renderer: this.canvasRenderer  // Explicit Canvas rendering
            }).addTo(this.layers.rangeRings);

            const point = this.map.latLngToLayerPoint(centerLatLng);
            const circlePoint = this.map.latLngToLayerPoint(
                this.L.latLng(
                    centerLatLng[0],
                    centerLatLng[1] + (radius * 1852) / 111320
                )
            );
            const labelPoint = this.map.layerPointToLatLng({
                x: point.x,
                y: point.y - (point.y - circlePoint.y)
            });

            this.L.marker(labelPoint, {
                icon: this.L.divIcon({
                    html: `${radius} NM`,
                    className: 'text-neutral-600/50 text-[10px] bg-transparent border-0 shadow-none pointer-events-none'
                })
            }).addTo(this.layers.rangeRings);
        });
    }

    // Method to center map on current station coordinates
    centerOnStation() {
        if (!this.map) return;
        
        const centerLat = this.store.stationOverride.active
            ? (this.store.stationOverride.latitude || this.store.stationLatitude || 43.6777)
            : (this.store.stationLatitude || 43.6777);
        const centerLon = this.store.stationOverride.active
            ? (this.store.stationOverride.longitude || this.store.stationLongitude || -79.6248)
            : (this.store.stationLongitude || -79.6248);
            
        console.log(`MapManager: Centering map on station coordinates: ${centerLat}, ${centerLon}`);
        this.map.setView([centerLat, centerLon], this.CONFIG.defaultZoom);
    }

    // Generate simple future trajectory for active aircraft based on current vector
    generateFutureTrajectory(aircraft) {
        const currentLat = aircraft?.adsb?.lat;
        const currentLon = aircraft?.adsb?.lon;
        const groundSpeed = aircraft?.adsb?.gs;
        const track = aircraft?.adsb?.track;

        const hasValidPosition = typeof currentLat === 'number' && Number.isFinite(currentLat)
            && typeof currentLon === 'number' && Number.isFinite(currentLon);
        const hasValidSpeed = typeof groundSpeed === 'number' && Number.isFinite(groundSpeed) && groundSpeed >= 5;
        const hasValidTrack = typeof track === 'number' && Number.isFinite(track);

        if (!hasValidPosition || !hasValidSpeed || !hasValidTrack) {
            return [];
        }

        // Convert ground speed from knots to degrees per minute (approximate)
        const speedDegreesPerMinute = (groundSpeed * 0.000277778) / 60; // knots to degrees/minute

        // Generate future positions for next 5 minutes
        const futurePoints = [];
        for (let minutes = 1; minutes <= 5; minutes++) {
            // Calculate distance traveled in degrees
            const distanceDegrees = speedDegreesPerMinute * minutes;
            
            // Convert track to radians
            const trackRadians = (track * Math.PI) / 180;
            
            // Calculate new position
            const deltaLat = distanceDegrees * Math.cos(trackRadians);
            const deltaLon = distanceDegrees * Math.sin(trackRadians) / Math.cos((currentLat * Math.PI) / 180);
            
            const futureLat = currentLat + deltaLat;
            const futureLon = currentLon + deltaLon;
            
            futurePoints.push([futureLat, futureLon]);
        }

        return futurePoints;
    }

    _ensureLeafletObjects(aircraft) {
        // Skip aircraft without a valid numeric GPS position (covers mode_s, partial deltas, etc.)
        const lat = aircraft.adsb?.lat;
        const lon = aircraft.adsb?.lon;
        const hasPosition = typeof lat === 'number' && typeof lon === 'number' && (lat !== 0 || lon !== 0);
        if (!hasPosition) return;

        const isStalePosition = aircraft.stale_position === true;
        const now = new Date();

        // Only add trail points for aircraft with live (non-stale) positions
        if (!isStalePosition) {
            if (!this.trails[aircraft.hex]) {
                this.trails[aircraft.hex] = [];
            }
            this.trails[aircraft.hex].push({
                lat: lat,
                lon: lon,
                alt_baro: (typeof aircraft.adsb?.alt_baro === 'number' && Number.isFinite(aircraft.adsb.alt_baro))
                    ? aircraft.adsb.alt_baro
                    : null,
                time: now,
                isHistorical: false
            });
        }

        // Prune old trail points (only if trail data exists)
        if (this.trails[aircraft.hex]) {
            const selectedTrailLength = Number(this.store.settings.trailLength);
            const retentionMinutes = Math.max(
                this._trailRetentionMinutes,
                Number.isFinite(selectedTrailLength) ? selectedTrailLength : 2
            );
            const cutoffTime = new Date(now.getTime() - (retentionMinutes * 60 * 1000));
            // PERF: splice-based pruning avoids allocating a new array (filter creates a copy every call)
            const trail = this.trails[aircraft.hex];
            let pruneIdx = 0;
            while (pruneIdx < trail.length && trail[pruneIdx].time < cutoffTime) pruneIdx++;
            if (pruneIdx > 0) trail.splice(0, pruneIdx);

            // Hard limit on trail points to prevent memory leaks during long flights
            if (trail.length > 500) {
                trail.splice(0, trail.length - 500);
            }
        }

        const position = [lat, lon];
        const heading = this.store.getHeadingWithFallback(aircraft);
        const iconClass = this.getAircraftIconClass(aircraft);
        const animationOwnsPose = this.store?.settings?.aircraftAnimation?.enabled === true;

        if (!this.markers[aircraft.hex]) {
            // --- NEW MARKER ---
            // Build icon + label only when creating a new marker (not on every update)
            const icon = this.createAircraftIcon(aircraft);
            const callsign = (aircraft.flight || aircraft.hex).trim();
            const altitude = (aircraft.adsb && typeof aircraft.adsb.alt_baro === 'number') ? aircraft.adsb.alt_baro : null;
            const currentSpeed = (aircraft.adsb && typeof aircraft.adsb.tas === 'number')
                ? aircraft.adsb.tas
                : ((aircraft.adsb && typeof aircraft.adsb.gs === 'number') ? aircraft.adsb.gs : null);
            const currentPhase = this.getCurrentPhase(aircraft);
            const currentStatus = aircraft.status || 'active';
            const currentTrend = this.getVerticalTrend(aircraft);
            const isStale = aircraft.stale_position ? '1' : '0';
            const altitudeKey = altitude === null ? 'na' : Math.round(altitude / 100);
            const speedKey = currentSpeed === null ? 'na' : Math.round(currentSpeed / 10);
            const labelVersion = `${callsign}_${altitudeKey}_${speedKey}_${currentPhase}_${currentStatus}_${currentTrend}_${isStale}`;
            const newLabelContent = this.store.createLabelContent(aircraft, callsign, altitude, currentTrend);
            const labelContentIcon = this.L.divIcon({
                html: newLabelContent,
                className: this.getLabelClassName(aircraft),
                iconSize: [130, 40],
                iconAnchor: [-8, 2]
            });

            const marker = this._getPooledMarker(position, icon);
            const label = this._getPooledLabel(position, labelContentIcon);

            this.markers[aircraft.hex] = {
                aircraft: marker,
                label: label,
                lastLat: position[0],
                lastLon: position[1],
                lastHeading: heading,
                lastIconClass: iconClass,
                lastLabelContent: newLabelContent,
                lastLabelVersion: labelVersion,
                lastAltitude: altitude,
                created: Date.now(),
                lastSeenSpan: null
            };

            // Apply initial CSS rotation after DOM insertion
            requestAnimationFrame(() => {
                this._applyAircraftRotation(marker, heading);
            });

            marker.on('mouseover', () => {
                Alpine.store('atc').hoveredAircraft = aircraft;
                this.updateVisualState(aircraft.hex, true);
            });
            marker.on('mouseout', () => {
                Alpine.store('atc').hoveredAircraft = null;
                this.updateVisualState(aircraft.hex, true);
            });
            marker.on('click', (e) => {
                this.L.DomEvent.stopPropagation(e);
                Alpine.store('atc').selectedAircraft = aircraft;
                this.updateVisualState(aircraft.hex, true);
            });
            label.on('mouseover', () => {
                Alpine.store('atc').hoveredAircraft = aircraft;
                this.updateVisualState(aircraft.hex, true);
            });
            label.on('mouseout', () => {
                Alpine.store('atc').hoveredAircraft = null;
                this.updateVisualState(aircraft.hex, true);
            });
            label.on('click', (e) => {
                this.L.DomEvent.stopPropagation(e);
                Alpine.store('atc').selectedAircraft = aircraft;
                this.updateVisualState(aircraft.hex, true);
            });
        } else {
            // --- UPDATE EXISTING MARKER ---
            const existing = this.markers[aircraft.hex];

            // Apply every real ADS-B position delta immediately to avoid marker "stick then jump"
            const positionChanged = existing.lastLat !== position[0] || existing.lastLon !== position[1];

            if (positionChanged && !animationOwnsPose) {
                existing.aircraft.setLatLng(position);
                existing.label.setLatLng(position);
            }
            if (positionChanged) {
                existing.lastLat = position[0];
                existing.lastLon = position[1];
            }

            // Update icon size when category group changes
            if (existing.lastIconClass !== iconClass) {
                existing.aircraft.setIcon(this.createAircraftIcon(aircraft));
                existing.lastIconClass = iconClass;
                if (!animationOwnsPose) {
                    requestAnimationFrame(() => {
                        this._applyAircraftRotation(existing.aircraft, heading);
                    });
                }
            }

            // Keep icon aligned with direction of travel over long sessions
            const headingChanged = Math.abs(this._minimalAngleDiffDeg(existing.lastHeading || 0, heading)) > 0.5;
            if (headingChanged && !animationOwnsPose) {
                this._applyAircraftRotation(existing.aircraft, heading);
                existing.lastHeading = heading;
            } else if (headingChanged) {
                existing.lastHeading = heading;
            }

            // PERF: Compute label version FIRST — only build DOM label if data actually changed.
            // This avoids createLabelContent() + L.divIcon() allocation on ~90-95% of updates.
            const currentAltitude = (aircraft.adsb && typeof aircraft.adsb.alt_baro === 'number') ? aircraft.adsb.alt_baro : null;
            const currentSpeed = (aircraft.adsb && typeof aircraft.adsb.tas === 'number')
                ? aircraft.adsb.tas
                : ((aircraft.adsb && typeof aircraft.adsb.gs === 'number') ? aircraft.adsb.gs : null);
            const currentPhase = this.getCurrentPhase(aircraft);
            const currentStatus = aircraft.status || 'active';
            const currentCallsign = (aircraft.flight || aircraft.hex).trim();
            const currentTrend = this.getVerticalTrend(aircraft);
            const isStale = aircraft.stale_position ? '1' : '0';
            const altitudeKey = currentAltitude === null ? 'na' : Math.round(currentAltitude / 100);
            const speedKey = currentSpeed === null ? 'na' : Math.round(currentSpeed / 10);
            const labelVersion = `${currentCallsign}_${altitudeKey}_${speedKey}_${currentPhase}_${currentStatus}_${currentTrend}_${isStale}`;

            if (existing.lastLabelVersion !== labelVersion) {
                const newLabelContent = this.store.createLabelContent(aircraft, currentCallsign, currentAltitude, currentTrend);
                const labelContentIcon = this.L.divIcon({
                    html: newLabelContent,
                    className: this.getLabelClassName(aircraft),
                    iconSize: [130, 40],
                    iconAnchor: [-8, 2]
                });
                existing.label.setIcon(labelContentIcon);
                existing.lastLabelContent = newLabelContent;
                existing.lastLabelVersion = labelVersion;
                existing.lastSeenSpan = null;

                // setIcon replaces the DOM element - re-apply visual state
                this.updateVisualState(aircraft.hex);
            }
        }
    }

    getVerticalTrend(aircraft) {
        const verticalRate = aircraft.adsb ? aircraft.adsb.baro_rate : null;
        if (typeof verticalRate !== 'number') return 'level';
        if (verticalRate > 100) return 'climbing';
        if (verticalRate < -100) return 'descending';
        return 'level';
    }

    getLabelClassName(aircraft) {
        // FIXED: Remove 'aircraft-label' class that was causing duplicate labels
        let finalLabelClassName = '';
        if (aircraft.status === 'signal_lost') {
            finalLabelClassName += 'aircraft-label-signal-lost';
        } else if (aircraft.status === 'stale') {
            finalLabelClassName += 'aircraft-label-inactive';
        }
        return finalLabelClassName;
    }

    _normalizeHeading(heading) {
        const n = Number(heading);
        if (!Number.isFinite(n)) return 0;
        return ((n % 360) + 360) % 360;
    }

    _minimalAngleDiffDeg(a, b) {
        let d = this._normalizeHeading(a) - this._normalizeHeading(b);
        while (d > 180) d -= 360;
        while (d <= -180) d += 360;
        return d;
    }

    _applyAircraftRotation(marker, heading) {
        const markerElement = marker?.getElement?.();
        if (!markerElement) return;

        const iconContainer = markerElement.querySelector('.aircraft-icon-container');
        if (!iconContainer) return;

        const normalizedHeading = this._normalizeHeading(heading);
        const currentTransform = iconContainer.style.transform || '';
        const translatePart = currentTransform.match(/translate\([^)]+\)/)?.[0] || '';

        iconContainer.style.transform = `${translatePart ? `${translatePart} ` : ''}rotate(${normalizedHeading}deg)`;
        iconContainer.style.transformOrigin = 'center';
        if (!iconContainer.style.transition) {
            iconContainer.style.transition = 'transform 0.2s ease';
        }
    }

    getAircraftWakeCategory(aircraft) {
        const category = (aircraft?.adsb?.category || '').toString().toUpperCase().trim();
        return category;
    }

    _normalizeAircraftTypeText(value) {
        const text = (value || '').toString().toUpperCase();
        return text
            .replace(/[\/_.,-]+/g, ' ')
            .replace(/\s+/g, ' ')
            .trim();
    }

    _compactAircraftTypeText(value) {
        return this._normalizeAircraftTypeText(value).replace(/[^A-Z0-9]/g, '');
    }

    _getAircraftTypeCandidates(aircraft) {
        const rawCandidates = [
            aircraft?.bsdb?.type,
            aircraft?.adsb?.type,
            aircraft?.adsb?.t,
            aircraft?.type,
            aircraft?.aircraft_type,
            aircraft?.model
        ];

        if (aircraft?.bsdb?.manufacturer && aircraft?.bsdb?.type) {
            rawCandidates.push(`${aircraft.bsdb.manufacturer} ${aircraft.bsdb.type}`);
        }

        const unique = new Set();
        const candidates = [];

        for (const value of rawCandidates) {
            const normalized = this._normalizeAircraftTypeText(value);
            if (!normalized || unique.has(normalized)) continue;

            unique.add(normalized);
            candidates.push({
                normalized,
                compact: this._compactAircraftTypeText(normalized)
            });
        }

        return candidates;
    }

    _matchesAircraftTypePattern(candidates, patterns) {
        return candidates.some(candidate => {
            return patterns.some(pattern => {
                if (pattern.normalized && pattern.normalized.test(candidate.normalized)) return true;
                if (pattern.compact && pattern.compact.test(candidate.compact)) return true;
                return false;
            });
        });
    }

    _getSpecificAircraftIconClass(aircraft) {
        const candidates = this._getAircraftTypeCandidates(aircraft);
        if (!candidates.length) return null;

        const match = (patterns) => this._matchesAircraftTypePattern(candidates, patterns);

        if (match([
            { normalized: /\bA\s*3\s*8\s*0\b|\bAIRBUS\s+A\s*380\b/, compact: /A380/ }
        ])) return 'a380';

        if (match([
            { normalized: /\bA\s*3\s*4\s*0\b|\bAIRBUS\s+A\s*340\b/, compact: /A340/ }
        ])) return 'a340';

        if (match([
            { normalized: /\bA\s*3\s*3\s*0\b|\bAIRBUS\s+A\s*330\b/, compact: /A330/ }
        ])) return 'a330';

        if (match([
            { normalized: /\bA\s*3\s*(1[89]|2[01])\b|\bAIRBUS\s+A\s*3\s*(1[89]|2[01])\b|\bA20N\b|\bA21N\b/, compact: /A31[89]|A320|A321|A20N|A21N/ }
        ])) return 'a320';

        if (match([
            { normalized: /\bB\s*7\s*8\s*7\b|\bBOEING\s+787\b|\b78[89]\b/, compact: /B787|78[89]|78X/ }
        ])) return 'b787';

        if (match([
            { normalized: /\bB\s*7\s*7\s*7\b|\bBOEING\s+777\b|\b77[0-9W]\b/, compact: /B777|77[0-9W]/ }
        ])) return 'b777';

        if (match([
            { normalized: /\bB\s*7\s*6\s*7\b|\bBOEING\s+767\b|\b76[0-9]\b/, compact: /B767|76[0-9]/ }
        ])) return 'b767';

        if (match([
            { normalized: /\bB\s*7\s*4\s*7\b|\bBOEING\s+747\b|\b74[0-9]\b/, compact: /B747|74[0-9]/ }
        ])) return 'b747';

        if (match([
            { normalized: /\bB\s*7\s*3\s*7\b|\bBOEING\s+737\b|\b73[0-9]\b/, compact: /B737|73[0-9]/ }
        ])) return 'b737';

        if (match([
            { normalized: /\bC\s*1\s*3\s*0\b|\bHERCULES\b/, compact: /C130|HERCULES/ }
        ])) return 'c130';

        if (match([
            { normalized: /\bCRJ\b|\bCANADAIR\s+REGIONAL\s+JET\b|\bCL\s*[- ]?65\b/, compact: /CRJ|CL65/ }
        ])) return 'crjx';

        if (match([
            { normalized: /\bDH\s*8\s*A\b|\bDHC\s*8\b|\bDASH\s*8\b|\bQ\s*400\b/, compact: /DH8A|DHC8|DASH8|Q400/ }
        ])) return 'dh8a';

        if (match([
            { normalized: /\bE\s*1\s*9\s*5\b/, compact: /E195/ }
        ])) return 'e195';

        if (match([
            { normalized: /\bERJ\b|\bEMBRAER\s+REGIONAL\s+JET\b|\bE\s*1\s*7\s*[05]\b/, compact: /ERJ|E170|E175/ }
        ])) return 'erj';

        if (match([
            { normalized: /\bF\s*1\s*0\s*0\b|\bFOKKER\s*100\b/, compact: /F100|FOKKER100/ }
        ])) return 'f100';

        if (match([
            { normalized: /\bFA\s*7\s*X\b|\bFALCON\s*7\s*X\b/, compact: /FA7X|FALCON7X/ }
        ])) return 'fa7x';

        if (match([
            { normalized: /\bGLF\s*5\b|\bGULFSTREAM\s*(V|5|G\s*5\s*0\s*0)\b/, compact: /GLF5|GULFSTREAMV|GULFSTREAM5|G500|GV/ }
        ])) return 'glf5';

        if (match([
            { normalized: /\bLEAR\s*JET\b|\bLEARJET\b|\bLJ\s*\d{2}\b/, compact: /LEARJET|LJ\d{2}/ }
        ])) return 'learjet';

        if (match([
            { normalized: /\bMD\s*1\s*1\b|\bMCDONNELL\s+DOUGLAS\s+11\b/, compact: /MD11/ }
        ])) return 'md11';

        if (match([
            { normalized: /\bCESSNA\b|\bC\s*1\s*5\s*2\b|\bC\s*1\s*7\s*2\b|\bC\s*2\s*0\s*8\b/, compact: /CESSNA|C152|C172|C208/ }
        ])) return 'cessna';

        return null;
    }

    getAircraftIconClass(aircraft) {
        const specificTypeClass = this._getSpecificAircraftIconClass(aircraft);
        if (specificTypeClass) return specificTypeClass;

        const category = this.getAircraftWakeCategory(aircraft);

        if (category === 'A1') return 'a1';
        if (category === 'A2') return 'a2';
        if (category === 'A3') return 'a3';
        if (category === 'A4') return 'a4';
        if (category === 'A5') return 'a5';
        if (category === 'A6') return 'a6';
        if (category === 'A7') return 'a7';

        return 'unknown';
    }

    _getAircraftIconDimensions(iconClass) {
        const scale = 0.85;
        const scaled = (size) => Math.max(12, Math.round(size * scale));

        switch (iconClass) {
            case 'a1': return { size: scaled(24), stroke: 1.15 };
            case 'a2': return { size: scaled(28), stroke: 1.2 };
            case 'a3': return { size: scaled(31), stroke: 1.28 };
            case 'a4': return { size: scaled(34), stroke: 1.34 };
            case 'a5': return { size: scaled(36), stroke: 1.38 };
            case 'a6': return { size: scaled(28), stroke: 1.2 };
            case 'a7': return { size: scaled(28), stroke: 1.2 };
            case 'a320':
            case 'a330':
            case 'a340':
            case 'a380':
            case 'b737':
            case 'b747':
            case 'b767':
            case 'b777':
            case 'b787':
            case 'c130':
            case 'cessna':
            case 'crjx':
            case 'dh8a':
            case 'e195':
            case 'erj':
            case 'f100':
            case 'fa7x':
            case 'glf5':
            case 'learjet':
            case 'md11':
                return { size: scaled(30), stroke: 1.2 };
            case 'unknown':
            default:
                return { size: scaled(26), stroke: 1.2 };
        }
    }

    _getAircraftAssetPath(iconClass) {
        switch (iconClass) {
            case 'a1': return 'assets/a1.svg';
            case 'a2': return 'assets/a2.svg';
            case 'a3': return 'assets/a3.svg';
            case 'a4': return 'assets/a4.svg';
            case 'a5': return 'assets/a5.svg';
            case 'a6': return 'assets/a6.svg';
            case 'a7': return 'assets/a7.svg';
            case 'a320': return 'assets/aircraft/a320.svg';
            case 'a330': return 'assets/aircraft/a330.svg';
            case 'a340': return 'assets/aircraft/a340.svg';
            case 'a380': return 'assets/aircraft/a380.svg';
            case 'b737': return 'assets/aircraft/b737.svg';
            case 'b747': return 'assets/aircraft/b747.svg';
            case 'b767': return 'assets/aircraft/b767.svg';
            case 'b777': return 'assets/aircraft/b777.svg';
            case 'b787': return 'assets/aircraft/b787.svg';
            case 'c130': return 'assets/aircraft/c130.svg';
            case 'cessna': return 'assets/aircraft/cessna.svg';
            case 'crjx': return 'assets/aircraft/crjx.svg';
            case 'dh8a': return 'assets/aircraft/dh8a.svg';
            case 'e195': return 'assets/aircraft/e195.svg';
            case 'erj': return 'assets/aircraft/erj.svg';
            case 'f100': return 'assets/aircraft/f100.svg';
            case 'fa7x': return 'assets/aircraft/fa7x.svg';
            case 'glf5': return 'assets/aircraft/glf5.svg';
            case 'learjet': return 'assets/aircraft/learjet.svg';
            case 'md11': return 'assets/aircraft/md11.svg';
            case 'unknown':
            default:
                return 'assets/a0.svg';
        }
    }

    getAircraftIconForClass(iconClass = 'unknown') {
        if (!this._aircraftIconsBySize) {
            this._aircraftIconsBySize = {};
        }

        const { size } = this._getAircraftIconDimensions(iconClass);
        const cacheKey = `${iconClass}:${size}`;

        if (this._aircraftIconsBySize[cacheKey]) {
            return this._aircraftIconsBySize[cacheKey];
        }

        const half = size / 2;
        const iconSrc = this._getAircraftAssetPath(iconClass);

        const svg = `<div class="aircraft-icon-container">
            <img src="${iconSrc}" alt="" aria-hidden="true" width="${size}" height="${size}" style="display:block; width:100%; height:100%; max-width:${size}px; max-height:${size}px; object-fit: contain;" />
        </div>`;

        this._aircraftIconsBySize[cacheKey] = this.L.divIcon({
            html: svg,
            className: 'aircraft-marker',
            iconSize: [size, size],
            iconAnchor: [half, half]
        });

        return this._aircraftIconsBySize[cacheKey];
    }

    createAircraftIcon(aircraft) {
        const iconClass = this.getAircraftIconClass(aircraft);
        return this.getAircraftIconForClass(iconClass);
    }

    updateFlightPaths() {
        // Don't clear all layers - only update changed trails
        if (!this.store.settings.showPaths) {
            this.layers.trails.clearLayers();
            // Keep trail history in memory so re-enabling paths restores immediately
            this.trailVersions = {};
            this.trailLayersByHex = {};
            return;
        }

        // Clean up orphaned trails first (trails for aircraft no longer in the store)
        this.cleanupOrphanedTrails();

        // Track which trails need updates
        const updatedTrails = new Set();

        Object.keys(this.store.aircraft).forEach(hex => {
            const aircraftData = this.store.aircraft[hex];
            if (!aircraftData) return;

            // Check if this aircraft is currently selected
            const isSelectedAircraft = this.store.selectedAircraft && this.store.selectedAircraft.hex === hex;

            // Trail visibility follows marker visibility - if marker isn't on map, trail shouldn't show
            // This ensures trails respect ALL filters (search, ground state, altitude, phase, last seen)
            const markerInfo = this.markers[hex];
            const markerIsOnMap = markerInfo && this.layers.aircraft.hasLayer(markerInfo.aircraft);

            // Show trail if marker is visible OR if aircraft is selected (selected always shows)
            const shouldShowTrail = markerIsOnMap || isSelectedAircraft;

            // If aircraft marker isn't visible and isn't selected, remove trail layers
            // from the map but KEEP the trail data so it redraws instantly when visible again
            if (!shouldShowTrail) {
                this._removeTrailLayersByHex(hex);
                delete this.trailLayersByHex[hex];
                delete this.trailVersions[hex];
                return;
            }

            const trail = this.trails[hex];
            const selectedTrailLength = Number(this.store.settings.trailLength);
            const trailLengthMinutes = Number.isFinite(selectedTrailLength) ? selectedTrailLength : 2;
            const trailCutoffTimeMs = Date.now() - (trailLengthMinutes * 60 * 1000);
            const visibleTrailPointCount = trail
                ? trail.reduce((count, point) => {
                    if (!point?.time) return count;
                    return point.time.getTime() >= trailCutoffTimeMs ? count + 1 : count;
                }, 0)
                : 0;

            // For selected aircraft, we may have history data even if no real-time trail exists
            // (e.g., stale/signal_lost aircraft that were selected after going inactive)
            const hasHistoryData = isSelectedAircraft && this.store.aircraftDetailsHistoryData && this.store.aircraftDetailsHistoryData.length > 0;
            const hasFutureData = isSelectedAircraft && this.store.aircraftDetailsFutureData && this.store.aircraftDetailsFutureData.length > 0;

            // Only skip if there's no trail AND no history data for selected aircraft
            if ((!trail || trail.length < 2) && !hasHistoryData) return;

            // Version-based change detection for trail updates
            const lastTrailPoint = trail && trail.length > 0 ? trail[trail.length - 1] : { lat: 0, lon: 0 };
            // Include history/future data lengths in version key so trails redraw when data arrives
            const historyLen = hasHistoryData ? this.store.aircraftDetailsHistoryData.length : 0;
            const futureLen = hasFutureData ? this.store.aircraftDetailsFutureData.length : 0;
            const trailVersion = `${trail ? trail.length : 0}_${visibleTrailPointCount}_${trailLengthMinutes}_${lastTrailPoint.lat.toFixed(5)}_${lastTrailPoint.lon.toFixed(5)}_${isSelectedAircraft}_${historyLen}_${futureLen}`;

            // Skip update if trail hasn't changed
            if (this.trailVersions[hex] === trailVersion) {
                return;
            }

            // Remove old trail for this aircraft only - O(1) operation
            this._removeTrailLayersByHex(hex);

            // Update version after removing old trail
            this.trailVersions[hex] = trailVersion;
            
            let currentOpacity = 0.7;

            if (this.store.selectedAircraft && !isSelectedAircraft) {
                currentOpacity = this.CONFIG.selectedFadeOpacity;      
            } else if (isSelectedAircraft) {
                currentOpacity = 0.7;
            }

            // Draw real-time trail if we have trail data
            if (trail && trail.length >= 2) {
                const currentPoints = trail
                    .filter(point => {
                        if (typeof point.lat !== 'number' || typeof point.lon !== 'number') return false;
                        if (!point.time) return false;
                        return point.time.getTime() >= trailCutoffTimeMs;
                    })
                    .map(point => [point.lat, point.lon]);
                if (currentPoints.length < 2) return;

                const polyline = this.L.polyline(currentPoints, {
                    weight: 2,
                    color: this.getAircraftColor(hex),
                    opacity: currentOpacity,
                    aircraftHex: hex, // Add identifier for cleanup
                    renderer: this.canvasRenderer  // Canvas for performance
                });
                this._addTrailLayer(hex, polyline);
            }

            // Show history trail for selected aircraft (works for all status types including stale/signal_lost)
            if (hasHistoryData) {
                const validPositions = this.store.aircraftDetailsHistoryData.filter(position => position.lat && position.lon);

                // Limit to max 30 points for performance
                const maxHistoryPoints = 30;
                let historyPoints;

                if (validPositions.length <= maxHistoryPoints) {
                    historyPoints = validPositions.map(position => [position.lat, position.lon]);
                } else {
                    const step = Math.floor(validPositions.length / maxHistoryPoints);
                    historyPoints = [];
                    for (let i = 0; i < validPositions.length; i += step) {
                        historyPoints.push([validPositions[i].lat, validPositions[i].lon]);
                    }
                    // Always include the last point
                    const lastPosition = validPositions[validPositions.length - 1];
                    historyPoints.push([lastPosition.lat, lastPosition.lon]);
                }

                // Create the history trail polyline
                if (historyPoints.length >= 2) {
                    const historyPolyline = this.L.polyline(historyPoints, {
                        color: this.getAircraftColor(hex),
                        weight: 2,
                        opacity: 0.7,
                        lineJoin: 'round',
                        dashArray: '4, 4',
                        aircraftHex: hex,
                        renderer: this.canvasRenderer
                    });
                    this._addTrailLayer(hex, historyPolyline);
                }

                // Add markers at key minute intervals (show every minute up to 5, then every 2 minutes)
                if (validPositions.length > 0) {
                    const now = new Date();
                    const shownMinutes = new Set();

                    validPositions.forEach((position, index) => {
                        const posTime = new Date(position.timestamp);
                        const minutesAgo = Math.round((now - posTime) / (1000 * 60));

                        // Show markers at: 1, 2, 3, 4, 5, then every 2 minutes (6, 8, 10...)
                        const shouldShow = (minutesAgo <= 5) ||
                                          (minutesAgo > 5 && minutesAgo % 2 === 0) ||
                                          (index === 0) || // Always show first
                                          (index === validPositions.length - 1); // Always show last

                        if (shouldShow && !shownMinutes.has(minutesAgo)) {
                            shownMinutes.add(minutesAgo);

                            const alt = position.altitude !== undefined ?
                                (position.altitude === 0 ? 'GND' : Math.round(position.altitude/100)*100) : '';

                            // Circle marker
                            const marker = this.L.circleMarker([position.lat, position.lon], {
                                radius: index === 0 || index === validPositions.length - 1 ? 4 : 3,
                                color: '#888888', fillColor: '#888888',
                                fillOpacity: 0.6, opacity: 0.6, weight: 1,
                                aircraftHex: hex, renderer: this.canvasRenderer
                            });
                            this._addTrailLayer(hex, marker);

                            // Time/altitude label
                            const label = this.L.marker([position.lat, position.lon], {
                                icon: this.L.divIcon({
                                    html: `<div style="color:#888;font-size:10px;opacity:0.8;">-${minutesAgo}m ${alt}</div>`,
                                    className: 'altitude-label-container',
                                    iconSize: [60, 16], iconAnchor: [-5, 0]
                                }),
                                aircraftHex: hex
                            });
                            this._addTrailLayer(hex, label);
                        }
                    });
                }
            }

            // Show future trajectories only for active aircraft (not stale or signal_lost)
            // Stale/signal_lost aircraft should only show history, not predictions
            const isActiveAircraft = aircraftData.status === 'active';
            const showFutureTrajectory = isActiveAircraft && (
                (isSelectedAircraft && this.store && this.store.aircraftDetailsFutureData && this.store.aircraftDetailsFutureData.length > 0) ||
                (aircraftData.adsb && aircraftData.adsb.gs > 5) // Only for moving active aircraft
            );
            
            if (showFutureTrajectory && aircraftData.adsb && aircraftData.adsb.lat && aircraftData.adsb.lon) {
                let futureDataPoints = [];
                let usingServerData = false;

                // Use server-provided future data if available, otherwise generate trajectory
                if (isSelectedAircraft && this.store.aircraftDetailsFutureData && this.store.aircraftDetailsFutureData.length > 0) {
                    futureDataPoints = this.store.aircraftDetailsFutureData
                        .filter(position => position.lat && position.lon)
                        .map(position => [position.lat, position.lon]);
                    usingServerData = true;
                } else if (aircraftData.adsb) {
                    // Fall back to generated trajectory for non-selected or while data is loading
                    futureDataPoints = this.generateFutureTrajectory(aircraftData);
                }

                if (futureDataPoints.length > 0) {
                    // For server-provided data, use prediction points directly (they start
                    // near the aircraft). Prepending the live position causes jank because
                    // the live position updates every ~1s while predictions refresh every ~5s.
                    // For client-generated trajectories, prepend current position as anchor.
                    let futurePoints;
                    if (usingServerData) {
                        futurePoints = futureDataPoints;
                    } else {
                        const currentPosition = [aircraftData.adsb.lat, aircraftData.adsb.lon];
                        futurePoints = [currentPosition, ...futureDataPoints];
                    }

                    // Create the future trajectory polyline
                    if (futurePoints.length >= 2) {
                        const futurePolyline = this.L.polyline(futurePoints, {
                            color: this.getAircraftColor(hex),
                            weight: 2,
                            opacity: isSelectedAircraft ? 0.5 : 0.3,
                            lineJoin: 'round',
                            dashArray: '3, 7',
                            aircraftHex: hex,
                            renderer: this.canvasRenderer
                        });
                        this._addTrailLayer(hex, futurePolyline);
                    }

                    // Add time markers for future positions (selected aircraft only)
                    // Show markers every 15s (every 3rd point at 5s granularity)
                    if (isSelectedAircraft && this.store.aircraftDetailsFutureData && this.store.aircraftDetailsFutureData.length > 0) {
                        this.store.aircraftDetailsFutureData.forEach((position, index) => {
                            if ((index + 1) % 3 !== 0) return;

                            const timeLabel = this.store.formatPredictionTime(position);
                            const alt = position.altitude !== undefined ?
                                (position.altitude === 0 ? 'GND' : Math.round(position.altitude/100)*100) : '';

                            // Circle marker (last one slightly larger)
                            const isLast = index === this.store.aircraftDetailsFutureData.length - 1;
                            const marker = this.L.circleMarker([position.lat, position.lon], {
                                radius: isLast ? 4 : 3,
                                color: '#666666', fillColor: '#666666',
                                fillOpacity: 0.5, opacity: 0.5, weight: 1,
                                aircraftHex: hex, renderer: this.canvasRenderer
                            });
                            this._addTrailLayer(hex, marker);

                            // Time/altitude label
                            const label = this.L.marker([position.lat, position.lon], {
                                icon: this.L.divIcon({
                                    html: `<div style="color:#666;font-size:10px;opacity:0.7;">${timeLabel} ${alt}</div>`,
                                    className: 'altitude-label-container',
                                    iconSize: [60, 16], iconAnchor: [-5, 0]
                                }),
                                aircraftHex: hex
                            });
                            this._addTrailLayer(hex, label);
                        });
                    }
                }
            }
            
            updatedTrails.add(hex);
        });
    }

    getAircraftColor(hex) {
        let hash = 0;
        for (let i = 0; i < hex.length; i++) {
            hash = hex.charCodeAt(i) + ((hash << 5) - hash);
        }
        const r = (hash & 0xFF0000) >> 16;
        const g = (hash & 0x00FF00) >> 8;
        const b = hash & 0x0000FF;
        return `rgb(${r}, ${g}, ${b})`;
    }

    _getAltitudeColorBand(aircraftData) {
        const altitude = Number(aircraftData?.adsb?.alt_baro);
        if (!Number.isFinite(altitude)) return 'cruise';

        const configuredCruise = Number(this.store?.stationCruiseAltitudeFt);
        const cruiseAltitudeFt = Number.isFinite(configuredCruise) && configuredCruise > 0
            ? configuredCruise
            : 18000;

        if (altitude >= cruiseAltitudeFt) return 'cruise';
        if (altitude > 12500) return 'high';
        if (altitude >= 3000) return 'mid';
        if (altitude >= 1000) return 'low';
        return 'very-low';
    }

    updateVisualState(hex) {
        const aircraftData = this.store.aircraft[hex];
        const markers = this.markers[hex];
        if (!markers || !aircraftData) return;

        // Check if this aircraft is in proximity view
        const isInProximity = this.proximityHexSet && this.proximityHexSet.has(hex);
        
        // Only apply green highlighting to the selected aircraft, never to proximity aircraft
        const isActuallySelected = Alpine.store('atc').selectedAircraft && Alpine.store('atc').selectedAircraft.hex === hex && !isInProximity;
        const isHovered = Alpine.store('atc').hoveredAircraft?.hex === hex && !isInProximity;
        const isFadedDueToOtherSelection = Alpine.store('atc').selectedAircraft && !isActuallySelected && !isInProximity;

        let targetOpacity = 1.0;
        const isStalePosition = aircraftData.stale_position === true;

        // Calculate time-based opacity: gradual fade from 10s to 60s since last ADS-B data
        let ageBasedOpacity = 1.0;
        let isAgeStale = false;   // true when opacity fade starts (>10s)
        let isAgeLost = false;    // true when fully stale (>=60s) — label style changes
        if (aircraftData.last_seen) {
            const ageSeconds = (Date.now() - new Date(aircraftData.last_seen).getTime()) / 1000;
            if (ageSeconds > 10) {
                isAgeStale = true;
                // Linear fade from 1.0 at 10s to 0.4 at 60s
                const fadeProgress = Math.min(1, (ageSeconds - 10) / 50);
                ageBasedOpacity = 1.0 - fadeProgress * 0.6;
            }
            if (ageSeconds >= 60) {
                isAgeLost = true;
            }
        }

        if (isFadedDueToOtherSelection) {
            targetOpacity = this.CONFIG.selectedFadeOpacity;
        } else {
            if (isActuallySelected || isHovered || isInProximity) {
                targetOpacity = isStalePosition ? 0.6 : ageBasedOpacity;
            } else {
                if (isStalePosition) {
                    targetOpacity = 0.35;
                } else if (isAgeStale) {
                    targetOpacity = ageBasedOpacity;
                } else {
                    targetOpacity = 1.0;
                }
            }
        }

        // Only apply green highlighting to selected/hovered aircraft that are NOT in proximity
        const isHighlightedForStyle = (isActuallySelected || isHovered) && !isInProximity;
        const altitudeBand = this._getAltitudeColorBand(aircraftData);

        // Bucket opacity to 1 decimal place to avoid excessive re-renders from continuous fade
        const opacityBucket = Math.round(targetOpacity * 10) / 10;
        const visualVersion = `${opacityBucket}_${isInProximity ? 1 : 0}_${isHighlightedForStyle ? 1 : 0}_${altitudeBand}_${isAgeLost ? 1 : 0}`;
        if (this._visualStateVersionByHex[hex] === visualVersion) {
            this._perfCounters.visualStateSkipped++;
            return;
        }
        this._visualStateVersionByHex[hex] = visualVersion;
        this._perfCounters.visualStateApplied++;

        const checkAircraftIcon = (apply = false) => {
            const element = markers.aircraft.getElement();
            if (!element) return true;

            const allBandClasses = [
                'altitude-band-cruise',
                'altitude-band-high',
                'altitude-band-mid',
                'altitude-band-low',
                'altitude-band-very-low'
            ];
            const targetClass = `altitude-band-${altitudeBand}`;

            const hasTargetClass = element.classList.contains(targetClass);
            const hasOtherBandClass = allBandClasses.some(cls => cls !== targetClass && element.classList.contains(cls));
            const stateMatches = hasTargetClass && !hasOtherBandClass;

            if (stateMatches) return true;
            if (!apply) return false;

            allBandClasses.forEach(cls => element.classList.remove(cls));
            element.classList.add(targetClass);
            return true;
        };
        this._updateElementStyle(markers.aircraft.getElement(), {}, {}, checkAircraftIcon);

        const checkLabelStyle = (apply = false) => {
            const element = markers.label.getElement();
            const labelDiv = element?.querySelector('div') || element;

            if (!labelDiv) return true; // No element, no changes needed

            const isProximity = this.proximityHexSet && this.proximityHexSet.has(hex);
            const hasProximityClass = labelDiv.classList.contains('proximity-highlight');
            const hasHoverClass = labelDiv.classList.contains('aircraft-label-table-hover');
            const hasAgeLostClass = labelDiv.classList.contains('aircraft-label-age-stale');
            const currentBg = labelDiv.style.backgroundColor;
            const currentBorder = labelDiv.style.borderColor;

            // Determine desired state
            let desiredProximity = isProximity;
            let desiredHover = !isProximity && isHighlightedForStyle;
            let desiredBg = desiredHover ? 'rgba(76, 175, 80, 0.1)' : '';
            let desiredBorder = desiredHover ? 'rgba(76, 175, 80, 0.3)' : '';

            // Check if current state matches desired state
            const stateMatches = (hasProximityClass === desiredProximity) &&
                                 (hasHoverClass === desiredHover) &&
                                 (hasAgeLostClass === isAgeLost) &&
                                 (currentBg === desiredBg) &&
                                 (currentBorder === desiredBorder);

            if (stateMatches) return true; // No changes needed
            if (!apply) return false; // Changes needed but not applying yet

            // Apply age-lost class (dashed border + white text) at 60s
            if (isAgeLost) {
                labelDiv.classList.add('aircraft-label-age-stale');
            } else {
                labelDiv.classList.remove('aircraft-label-age-stale');
            }

            // Apply changes
            if (desiredProximity) {
                labelDiv.classList.remove('aircraft-label-table-hover');
                labelDiv.classList.add('proximity-highlight');
                labelDiv.style.backgroundColor = '';
                labelDiv.style.borderColor = '';
            } else if (desiredHover) {
                labelDiv.classList.remove('proximity-highlight');
                labelDiv.classList.add('aircraft-label-table-hover');
                labelDiv.style.backgroundColor = desiredBg;
                labelDiv.style.borderColor = desiredBorder;
                this._updateFlightCardHover(hex, true);
            } else {
                labelDiv.classList.remove('proximity-highlight');
                labelDiv.classList.remove('aircraft-label-table-hover');
                labelDiv.style.backgroundColor = '';
                labelDiv.style.borderColor = '';
                this._updateFlightCardHover(hex, false);
            }
            return true;
        };
        this._updateElementStyle(markers.label.getElement(), {}, {}, checkLabelStyle);

        markers.aircraft.setOpacity(targetOpacity);
        if (markers.label && markers.label.setOpacity) {
            markers.label.setOpacity(targetOpacity);
        }
    }
    
    _updateElementStyle(element, highlightedStyle, normalStyle, checkFn) {
        // Check if changes are needed (apply=false returns true if no changes needed)
        if (checkFn()) return true;
        // Apply changes when needed
        checkFn(true);
    }

    // Helper function to update flight card hover effect in table
    _updateFlightCardHover(hex, isHovered) {
        if (this._flightCardHoverStateByHex[hex] === isHovered) {
            return;
        }
        this._flightCardHoverStateByHex[hex] = isHovered;

        // Find the flight card element in the table by aircraft hex
        const flightCardElement = document.querySelector(`[data-aircraft-hex="${hex}"]`);
        if (flightCardElement) {
            if (isHovered) {
                flightCardElement.classList.add('flight-card-map-hover');
            } else {
                flightCardElement.classList.remove('flight-card-map-hover');
            }
        }
    }

    // Helper function to safely get current phase
    getCurrentPhase(aircraft) {
        return (aircraft && aircraft.phase && aircraft.phase.current && aircraft.phase.current.length > 0)
            ? aircraft.phase.current[0].phase
            : 'NEW';
    }

    updateMapMarkerAndLabelVisibility() {
        this._perfCounters.fullVisibilityPasses++;
        const searchLower = this.store.searchTerm.toLowerCase();

        // Get current map bounds for viewport culling
        const mapBounds = this.map ? this.map.getBounds() : null;
        const currentZoom = this.map ? this.map.getZoom() : 10;

        // Enable viewport culling when zoomed in (zoom level > 11)
        const useViewportCulling = currentZoom > 11 && mapBounds;

        // Calculate last seen cutoff time
        const now = new Date();
        const lastSeenCutoff = new Date(now.getTime() - (this.store.settings.lastSeenMinutes * 60 * 1000));

        Object.keys(this.store.aircraft).forEach(hex => {
            const aircraft = this.store.aircraft[hex];
            let markerInfo = this.markers[hex];

            // If aircraft exists in store but has no marker, create it now
            // This ensures aircraft that arrived while filtered still get markers
            if (!markerInfo) {
                this._ensureLeafletObjects(aircraft);
                markerInfo = this.markers[hex];
                if (!markerInfo) return; // Still failed to create marker, skip
            }

            // Filter by last seen time
            let isVisibleByLastSeen = true;
            if (aircraft.last_seen) {
                const lastSeenDate = new Date(aircraft.last_seen);
                isVisibleByLastSeen = lastSeenDate >= lastSeenCutoff;
            }

            const callsign = (aircraft.flight || aircraft.hex).toLowerCase();
            const type = (aircraft.adsb?.type || '').toLowerCase();
            const category = (aircraft.adsb?.category || '').toLowerCase();

            const matchesSearch = searchLower === '' ||
                                callsign.includes(searchLower) ||
                                type.includes(searchLower) ||
                                category.includes(searchLower);

            // Filter by air/ground state - both can be enabled/disabled independently
            const isVisibleByGroundState = (aircraft.on_ground && this.store.settings.showGroundAircraft) ||
                                          (!aircraft.on_ground && this.store.settings.showAirAircraft);

            // Only apply altitude filter to aircraft in the air
            const isVisibleByAltitude = aircraft.on_ground ||
                (aircraft.adsb && aircraft.adsb.alt_baro >= this.store.settings.minAltitude &&
                aircraft.adsb.alt_baro <= this.store.settings.maxAltitude);

            // Filter by flight phase - matching the table logic in app.js
            const currentPhase = this.getCurrentPhase(aircraft);

            const isVisibleByPhase = !(this.store.settings.phaseFilters && this.store.settings.phaseFilters[currentPhase] === false);

            // Check if this aircraft is currently selected
            const isSelectedAircraft = this.store.selectedAircraft && this.store.selectedAircraft.hex === aircraft.hex;

            // Check if aircraft is in viewport (when viewport culling is enabled)
            let isInViewport = true;
            if (useViewportCulling && aircraft.adsb && aircraft.adsb.lat && aircraft.adsb.lon) {
                const position = L.latLng(aircraft.adsb.lat, aircraft.adsb.lon);
                isInViewport = mapBounds.contains(position);
            }

            // Aircraft should be visible if it matches ALL filters AND is in viewport OR if it's selected
            const shouldBeVisibleOnMap = ((matchesSearch && isVisibleByGroundState && isVisibleByAltitude && isVisibleByPhase && isVisibleByLastSeen && isInViewport) || isSelectedAircraft);

            const visibilityState = this._markerVisibilityState[hex] || {
                markerVisible: this.layers.aircraft.hasLayer(markerInfo.aircraft),
                labelVisible: markerInfo.label && this.layers.aircraft.hasLayer(markerInfo.label)
            };
            const markerIsOnMap = !!visibilityState.markerVisible;
            const labelIsOnMap = !!visibilityState.labelVisible;

            if (shouldBeVisibleOnMap && !markerIsOnMap) {
                markerInfo.aircraft.addTo(this.layers.aircraft);
                visibilityState.markerVisible = true;
                this._perfCounters.markerAdds++;
                
                // CRITICAL FIX: Apply heading rotation when aircraft becomes visible again
                const heading = this.store.getHeadingWithFallback(aircraft);
                requestAnimationFrame(() => {
                    this._applyAircraftRotation(markerInfo.aircraft, heading);
                });
            } else if (!shouldBeVisibleOnMap && markerIsOnMap) {
                this.layers.aircraft.removeLayer(markerInfo.aircraft);
                visibilityState.markerVisible = false;
                this._perfCounters.markerRemoves++;
            }

            // Labels should be visible if labels are enabled AND (aircraft matches filters OR is selected)
            const shouldLabelBeVisible = this.store.settings.showLabels && shouldBeVisibleOnMap;
            if (markerInfo.label) {
                if (shouldLabelBeVisible && !labelIsOnMap) {
                    markerInfo.label.addTo(this.layers.aircraft);
                    visibilityState.labelVisible = true;
                    this._perfCounters.labelAdds++;
                } else if (!shouldLabelBeVisible && labelIsOnMap) {
                    this.layers.aircraft.removeLayer(markerInfo.label);
                    visibilityState.labelVisible = false;
                    this._perfCounters.labelRemoves++;
                }
            }

            this._markerVisibilityState[hex] = visibilityState;
            
            if (shouldBeVisibleOnMap) {
                 this.updateVisualState(hex);
            }
        });
    }

    // Method to be called by the store's toggleRings
    toggleRings() {
        if (this.store.settings.showRings) {
            if (!this.map.hasLayer(this.layers.rangeRings)) {
                this.layers.rangeRings.addTo(this.map);
            }
            this.addRangeRings(); // Re-adds/updates rings
        } else {
            this.layers.rangeRings.clearLayers();
            if (this.map.hasLayer(this.layers.rangeRings)) {
                this.map.removeLayer(this.layers.rangeRings);
            }
        }
    }

    // Efficient single aircraft update for WebSocket performance
    updateSingleAircraft(hex, aircraft) {
        if (!aircraft) return;
        this._perfCounters.singleAircraftUpdates++;

        // _ensureLeafletObjects handles all updates: position, label content (with version checking), etc.
        this._ensureLeafletObjects(aircraft);

        const markerInfo = this.markers[hex];
        if (!markerInfo) return;

        // Immediately update lastSeen text in DOM (without full label regeneration)
        if (markerInfo.label && aircraft.last_seen) {
            const labelElement = markerInfo.label.getElement();
            if (labelElement) {
                if (!markerInfo.lastSeenSpan || !markerInfo.lastSeenSpan.isConnected) {
                    markerInfo.lastSeenSpan = labelElement.querySelector('[data-lastseen]');
                }
                const lastSeenSpan = markerInfo.lastSeenSpan;
                if (lastSeenSpan) {
                    const secondsAgo = Math.floor((Date.now() - new Date(aircraft.last_seen).getTime()) / 1000);
                    lastSeenSpan.textContent = `${secondsAgo}s`;
                }
            }
        }

        // Update visual state (color, rotation, etc.)
        this.updateVisualState(hex);

        // Check visibility for this specific aircraft
        this.updateSingleAircraftVisibility(hex, aircraft);
    }
    
    // Update visibility for a single aircraft (performance optimized)
    updateSingleAircraftVisibility(hex, aircraft) {
        const markerInfo = this.markers[hex];
        if (!markerInfo) return;
        
        const searchLower = this.store.searchTerm.toLowerCase();
        const callsign = (aircraft.flight || aircraft.hex).toLowerCase();
        const type = (aircraft.adsb?.type || '').toLowerCase();
        const category = (aircraft.adsb?.category || '').toLowerCase();
        
        const matchesSearch = searchLower === '' ||
                            callsign.includes(searchLower) ||
                            type.includes(searchLower) ||
                            category.includes(searchLower);
        
        const isVisibleByGroundState = (aircraft.on_ground && this.store.settings.showGroundAircraft) ||
                                      (!aircraft.on_ground && this.store.settings.showAirAircraft);
        
        const isVisibleByAltitude = aircraft.on_ground ||
            (aircraft.adsb && aircraft.adsb.alt_baro >= this.store.settings.minAltitude &&
             aircraft.adsb.alt_baro <= this.store.settings.maxAltitude);
        
        const currentPhase = this.getCurrentPhase(aircraft);
        const isVisibleByPhase = !(this.store.settings.phaseFilters && this.store.settings.phaseFilters[currentPhase] === false);
        
        const isSelectedAircraft = this.store.selectedAircraft && this.store.selectedAircraft.hex === aircraft.hex;
        const shouldBeVisibleOnMap = (matchesSearch && isVisibleByGroundState && isVisibleByAltitude && isVisibleByPhase) || isSelectedAircraft;
        
        const visibilityState = this._markerVisibilityState[hex] || {
            markerVisible: this.layers.aircraft.hasLayer(markerInfo.aircraft),
            labelVisible: markerInfo.label && this.layers.aircraft.hasLayer(markerInfo.label)
        };
        const markerIsOnMap = !!visibilityState.markerVisible;
        const labelIsOnMap = !!visibilityState.labelVisible;
        
        // Update marker visibility
        if (shouldBeVisibleOnMap && !markerIsOnMap) {
            markerInfo.aircraft.addTo(this.layers.aircraft);
            visibilityState.markerVisible = true;
            this._perfCounters.markerAdds++;

            const heading = this.store.getHeadingWithFallback(aircraft);
            requestAnimationFrame(() => {
                this._applyAircraftRotation(markerInfo.aircraft, heading);
            });
        } else if (!shouldBeVisibleOnMap && markerIsOnMap) {
            this.layers.aircraft.removeLayer(markerInfo.aircraft);
            visibilityState.markerVisible = false;
            this._perfCounters.markerRemoves++;
        }
        
        // Update label visibility
        const shouldLabelBeVisible = this.store.settings.showLabels && shouldBeVisibleOnMap;
        if (markerInfo.label) {
            if (shouldLabelBeVisible && !labelIsOnMap) {
                markerInfo.label.addTo(this.layers.aircraft);
                visibilityState.labelVisible = true;
                this._perfCounters.labelAdds++;
            } else if (!shouldLabelBeVisible && labelIsOnMap) {
                this.layers.aircraft.removeLayer(markerInfo.label);
                visibilityState.labelVisible = false;
                this._perfCounters.labelRemoves++;
            }
        }

        this._markerVisibilityState[hex] = visibilityState;
    }

    // Centralized place for map-related updates when filters change
    applyFiltersAndRefreshView(options = {}) {
        this._perfCounters.refreshRequested++;
        if (options.immediate === true) {
            if (this._refreshThrottleTimer) {
                clearTimeout(this._refreshThrottleTimer);
                this._refreshThrottleTimer = null;
            }
            this._refreshScheduled = false;
            this._applyFiltersAndRefreshViewNow();
            return;
        }

        if (this._refreshScheduled) {
            this._perfCounters.refreshCoalesced++;
            return;
        }

        const now = performance.now();
        const elapsed = now - this._lastRefreshRunAt;
        const waitMs = Math.max(0, this._refreshThrottleMs - elapsed);

        this._refreshScheduled = true;
        if (waitMs > 0) {
            this._perfCounters.refreshCoalesced++;
            this._refreshThrottleTimer = setTimeout(() => {
                this._refreshThrottleTimer = null;
                requestAnimationFrame(() => {
                    this._refreshScheduled = false;
                    this._applyFiltersAndRefreshViewNow();
                });
            }, waitMs);
            return;
        }

        requestAnimationFrame(() => {
            this._refreshScheduled = false;
            this._applyFiltersAndRefreshViewNow();
        });
    }

    _applyFiltersAndRefreshViewNow() {
        const refreshStart = performance.now();
        this.updateMapMarkerAndLabelVisibility();
        this.updateFlightPaths();
        this.updateVisibleAircraftList();
        
        // Update proximity circle if active
        this.updateProximityCircle();

        this._perfCounters.refreshExecuted++;
        this._perfCounters.refreshDurationMs += (performance.now() - refreshStart);
        this._lastRefreshRunAt = performance.now();
    }
    
    // Update list of aircraft visible on map for UI indicators
    // PERFORMANCE: Only update store if the set actually changed to prevent Alpine re-renders
    updateVisibleAircraftList() {
        if (!this.map) return;

        const mapBounds = this.map.getBounds();
        const currentZoom = this.map.getZoom();
        const useViewportCulling = currentZoom > 11 && mapBounds;

        // Track which aircraft are currently visible on the map
        const visibleAircraftHexes = new Set();

        Object.keys(this.store.aircraft).forEach(hex => {
            const aircraft = this.store.aircraft[hex];
            const markerInfo = this.markers[hex];

            if (!markerInfo || !this.layers.aircraft.hasLayer(markerInfo.aircraft)) {
                return; // Aircraft marker not on map
            }

            // If viewport culling is enabled, check if aircraft is in bounds
            if (useViewportCulling && aircraft.adsb && aircraft.adsb.lat && aircraft.adsb.lon) {
                const position = L.latLng(aircraft.adsb.lat, aircraft.adsb.lon);
                if (mapBounds.contains(position)) {
                    visibleAircraftHexes.add(hex);
                }
            } else if (!useViewportCulling) {
                // When not using viewport culling, all rendered aircraft are "visible"
                visibleAircraftHexes.add(hex);
            }
        });

        // PERFORMANCE: Only update store if the set actually changed
        // Compare sets to avoid unnecessary Alpine reactivity triggers on map pan/zoom
        const currentSet = this.store.visibleAircraftOnMap;
        if (currentSet && currentSet.size === visibleAircraftHexes.size) {
            let setsEqual = true;
            for (const hex of visibleAircraftHexes) {
                if (!currentSet.has(hex)) {
                    setsEqual = false;
                    break;
                }
            }
            if (setsEqual) return; // No change, skip update
        }

        this.store.visibleAircraftOnMap = visibleAircraftHexes;
    }

    // Helper to remove markers for aircraft that are no longer present
    removeStaleMarkers(currentAircraftHexes) {
        Object.keys(this.markers).forEach(hex => {
            if (!currentAircraftHexes.has(hex)) {
                if (this.markers[hex]) {
                    if (this.layers.aircraft.hasLayer(this.markers[hex].aircraft)) {
                        this.layers.aircraft.removeLayer(this.markers[hex].aircraft);
                    }
                    if (this.markers[hex].label && this.layers.aircraft.hasLayer(this.markers[hex].label)) {
                        this.layers.aircraft.removeLayer(this.markers[hex].label);
                    }
                    delete this.markers[hex];
                }
                delete this.trails[hex]; // Also remove its trail data from MapManager
                delete this.trailVersions[hex]; // Also remove trail version tracking
                this._removeTrailLayersByHex(hex); // Remove trail layers from map
                delete this.trailLayersByHex[hex]; // Clean up tracking
                delete this._markerVisibilityState[hex];
                delete this._visualStateVersionByHex[hex];
                delete this._flightCardHoverStateByHex[hex];
            }
        });
    }

    // Helper to remove a single aircraft marker
    removeAircraft(hex) {
        if (this.markers[hex]) {
            // Try to pool the markers for reuse
            if (this.markers[hex].aircraft) {
                if (this.layers.aircraft.hasLayer(this.markers[hex].aircraft)) {
                    this.layers.aircraft.removeLayer(this.markers[hex].aircraft);
                }
                this._releaseMarker(this.markers[hex].aircraft, 'aircraft');
            }
            if (this.markers[hex].label) {
                if (this.layers.aircraft.hasLayer(this.markers[hex].label)) {
                    this.layers.aircraft.removeLayer(this.markers[hex].label);
                }
                this._releaseMarker(this.markers[hex].label, 'labels');
            }
            delete this.markers[hex];
        }
        delete this.trails[hex]; // Also remove its trail data from MapManager
        delete this.trailVersions[hex]; // Also remove trail version tracking
        this._removeTrailLayersByHex(hex); // Remove trail layers from map
        delete this.trailLayersByHex[hex]; // Clean up tracking
        delete this._markerVisibilityState[hex];
        delete this._visualStateVersionByHex[hex];
        delete this._flightCardHoverStateByHex[hex];
    }

    // Force cleanup of aircraft markers (prevents overlapping labels)
    forceCleanupAircraft(hex) {
        //console.log(`[MAP] Force cleanup for ${hex}`);
        
        // Remove from our tracking
        if (this.markers[hex]) {
            this.removeAircraft(hex);
        }
        
        // SAFETY: Scan all layers to remove any orphaned markers for this aircraft
        const layersToCheck = [this.layers.aircraft];
        layersToCheck.forEach(layer => {
            layer.eachLayer(marker => {
                // Check if this marker belongs to our aircraft (by checking attached data or position)
                if (marker.options && marker.options.aircraftHex === hex) {
                    console.log(`[MAP] Found orphaned marker for ${hex}, removing...`);
                    layer.removeLayer(marker);
                } else if (marker._tooltip && marker._tooltip._content && marker._tooltip._content.includes(hex)) {
                    // Fallback: check tooltip content for aircraft hex
                    console.log(`[MAP] Found orphaned marker with tooltip for ${hex}, removing...`);
                    layer.removeLayer(marker);
                }
            });
        });
    }

    // Proximity Visualization Methods
    drawProximityCircle(position, distanceNM) {
        // Remove any existing proximity circle
        this.removeProximityCircle();
        
        // Convert nautical miles to meters
        const radiusMeters = distanceNM * 1852; // 1 NM = 1852 meters
        
        // Store the reference aircraft hex and distance for updates
        this.proximityRefHex = this.store.selectedAircraft?.hex;
        this.proximityDistanceNM = distanceNM;
        
        // Determine if there are aircraft in proximity (excluding the reference aircraft)
        const hasAircraftInProximity = this.proximityHexSet && this.proximityHexSet.size > 0;
        
        // Create a new circle
        this.proximityCircle = this.L.circle(position, {
            radius: radiusMeters,
            color: '#EB8C00', // Less saturated orange color
            fillColor: '#EB8C00',
            fillOpacity: 0.08, // Slightly less fill opacity
            weight: 2,
            dashArray: '5, 5', // Dashed line
            interactive: false, // Make sure the circle doesn't interfere with clicks
            renderer: this.canvasRenderer  // Canvas for performance
        }).addTo(this.map);
        
        // Ensure the circle is below aircraft markers
        if (this.proximityCircle.getElement()) {
            this.proximityCircle.getElement().style.zIndex = '300'; // Even lower than before (300 vs 400)
        }
        
        // Don't change the map view
    }

    removeProximityCircle() {
        if (this.proximityCircle) {
            this.map.removeLayer(this.proximityCircle);
            this.proximityCircle = null;
        }
        // Clear reference properties
        this.proximityRefHex = null;
        this.proximityDistanceNM = null;
    }
    
    updateProximityCircle() {
        // If we have a proximity circle and reference aircraft
        if (this.proximityCircle && this.proximityRefHex && this.proximityDistanceNM) {
            // Get the current position of the reference aircraft
            const aircraft = this.store.aircraft[this.proximityRefHex];
            if (aircraft && aircraft.adsb && aircraft.adsb.lat && aircraft.adsb.lon) {
                const position = [aircraft.adsb.lat, aircraft.adsb.lon];
                
                // Update the circle position
                this.proximityCircle.setLatLng(position);
            }
        }
    }

    highlightProximityAircraft(proximityHexSet) {
        // Remove any existing highlighting first
        this.removeProximityHighlighting();
        
        // Store the set for later use
        this.proximityHexSet = proximityHexSet;
        
        // First, ensure ALL aircraft labels are on top of the circle
        Object.keys(this.markers).forEach(hex => {
            if (this.markers[hex].label) {
                const labelElement = this.markers[hex].label.getElement();
                if (labelElement) {
                    labelElement.style.zIndex = '1000'; // Ensure all labels are on top of the circle
                }
            }
            
            // Also bring all aircraft markers to front
            if (this.markers[hex].aircraft) {
                const markerElement = this.markers[hex].aircraft.getElement();
                if (markerElement) {
                    markerElement.style.zIndex = '1000';
                }
            }
        });
        
        // Then apply highlighting to the aircraft in proximity
        Object.keys(this.markers).forEach(hex => {
            if (proximityHexSet.has(hex) && this.markers[hex].label) {
                // Store the hex in the store's proximityHighlightedAircraft set
                if (!this.store.proximityHighlightedAircraft) {
                    this.store.proximityHighlightedAircraft = new Set();
                }
                this.store.proximityHighlightedAircraft.add(hex);
                
                // Always apply proximity highlighting, even to selected/hovered aircraft
                // This ensures proximity highlighting takes precedence
                
                const labelElement = this.markers[hex].label.getElement();
                if (labelElement) {
                    const labelDiv = labelElement.querySelector('div');
                    if (labelDiv) {
                        // Remove any existing classes that might affect the style
                        labelDiv.classList.remove('selected');
                        labelDiv.classList.remove('hovered');
                        
                        // Add our custom proximity highlight class
                        labelDiv.classList.add('proximity-highlight');
                        
                        // Set styles directly
                        labelDiv.style.opacity = '1'; // Full opacity
                        labelDiv.style.zIndex = '1500'; // Higher z-index
                        labelDiv.style.borderColor = ''; // Remove any border color to use the one from CSS
                    }
                }
                
                // Make the aircraft icon fully opaque and bring it to the very front
                if (this.markers[hex].aircraft) {
                    const aircraftElement = this.markers[hex].aircraft.getElement();
                    if (aircraftElement) {
                        aircraftElement.style.opacity = '1'; // Full opacity
                        aircraftElement.style.zIndex = '1500'; // Higher z-index
                    }
                }
                
                // We can't directly access trail elements as they're in a layer group
                // Instead, we'll redraw the trails with higher opacity when in proximity view
            }
        });
        
        // We no longer need to update the circle animation
    }

    removeProximityHighlighting() {
        if (!this.proximityHexSet) return;
        
        // Remove highlighting from all aircraft labels
        Object.keys(this.markers).forEach(hex => {
            if (this.proximityHexSet.has(hex) && this.markers[hex].label) {
                // Remove from the store's proximityHighlightedAircraft set
                if (this.store.proximityHighlightedAircraft) {
                    this.store.proximityHighlightedAircraft.delete(hex);
                }
                
                // Check if this is the selected aircraft
                const isSelectedAircraft = this.store.selectedAircraft && this.store.selectedAircraft.hex === hex;
                
                const labelElement = this.markers[hex].label.getElement();
                if (labelElement) {
                    const labelDiv = labelElement.querySelector('div');
                    if (labelDiv) {
                        // Remove proximity classes
                        labelDiv.classList.remove('proximity-highlight');
                        labelDiv.classList.remove('proximity-highlight-hover');
                        
                        if (!isSelectedAircraft) {
                            // Only reset these styles for non-selected aircraft
                            // For selected aircraft, we want to keep the green highlighting
                            labelDiv.style.animation = ''; // Reset animation
                            
                            // Now call updateVisualState to restore the proper state
                            this.updateVisualState(hex, true);
                        }
                    }
                }
                
                // Reset aircraft icon z-index
                if (this.markers[hex].aircraft) {
                    const aircraftElement = this.markers[hex].aircraft.getElement();
                    if (aircraftElement) {
                        aircraftElement.style.zIndex = '1000'; // Reset to normal z-index
                    }
                }
                
                // We don't need to reset trail elements as they're managed by the layer group
            }
        });
        
        // Clear the set
        this.proximityHexSet = null;
    }
    
    updateProximityCircleAnimation(hasAircraftInProximity) {
        if (!this.proximityCircle || typeof this.proximityCircle.getElement !== 'function') return;
        
        const circleElement = this.proximityCircle.getElement();
        if (!circleElement) return;
        
        if (hasAircraftInProximity) {
            // Add pulsing animation class if there are aircraft in proximity
            circleElement.classList.add('proximity-circle-alert');
            circleElement.classList.remove('proximity-circle-normal');
        } else {
            // Remove pulsing animation class if no aircraft in proximity
            circleElement.classList.remove('proximity-circle-alert');
            circleElement.classList.add('proximity-circle-normal');
        }
    }

    // Draw runways and extended centerlines
    drawRunways(runwayData) {
        if (!this.map) {
            console.warn('MapManager: Cannot draw runways - map not initialized yet');
            return;
        }
        if (!runwayData || !runwayData.runway_thresholds || !runwayData.runway_extensions) {
            console.warn("No runway data available");
            return;
        }

        // Store data — rendering is driven by _renderAllRefData() via the shared zoom listener
        this.runwayData = runwayData;
        this.updateRunwayDisplay();
        this._ensureRefDataListeners();
    }

    updateRunwayDisplay() {
        if (!this.map || !this.runwayData) return;
        this.layers.runways.clearLayers();

        const zoom = this.map.getZoom();
        if (zoom < 9) return; // Don't render home runways below zoom 9

        const showLabels = zoom >= 10;
        const showExtensions = zoom >= 11;
        const showDistanceMarkers = zoom >= 13;
        const lineWeight = Math.max(4, Math.min(8, zoom - 6));

        for (const runwayId in this.runwayData.runway_thresholds) {
            const thresholds = this.runwayData.runway_thresholds[runwayId];
            const extensions = this.runwayData.runway_extensions[runwayId];
            const ends = Object.keys(thresholds);
            if (ends.length !== 2) continue;

            const end1 = ends[0], end2 = ends[1];

            // Runway line — canvas polyline, no culling
            this.L.polyline([
                [thresholds[end1].latitude, thresholds[end1].longitude],
                [thresholds[end2].latitude, thresholds[end2].longitude]
            ], {
                color: '#FFFFFF', weight: lineWeight, opacity: 0.8,
                renderer: this.canvasRenderer
            }).addTo(this.layers.runways);

            if (showLabels) {
                this.L.marker([thresholds[end1].latitude, thresholds[end1].longitude], {
                    icon: this.L.divIcon({
                        html: `<div class="runway-label">${end1}</div>`,
                        className: 'runway-label-container', iconSize: [30, 20]
                    }), interactive: false
                }).addTo(this.layers.runways);
                this.L.marker([thresholds[end2].latitude, thresholds[end2].longitude], {
                    icon: this.L.divIcon({
                        html: `<div class="runway-label">${end2}</div>`,
                        className: 'runway-label-container', iconSize: [30, 20]
                    }), interactive: false
                }).addTo(this.layers.runways);
            }

            if (showExtensions && extensions) {
                // Helper to draw one runway end's extension
                const drawExtension = (endId) => {
                    const pts = extensions[endId];
                    if (!pts || pts.length === 0) return;

                    this.L.polyline(
                        pts.map(p => [p.latitude, p.longitude]),
                        { color: '#76C76C', weight: 1.5, opacity: 0.4,
                          dashArray: '8, 12', renderer: this.canvasRenderer }
                    ).addTo(this.layers.runways);

                    if (showDistanceMarkers) {
                        for (let j = 0; j < pts.length; j++) {
                            const p = pts[j];
                            if (p.distance <= 0 || p.distance % 2 !== 0) continue;
                            this.L.circleMarker([p.latitude, p.longitude], {
                                radius: 1.5, color: '#76C76C', fillColor: '#76C76C',
                                fillOpacity: 0.6, opacity: 0.6, weight: 1,
                                renderer: this.canvasRenderer
                            }).addTo(this.layers.runways);
                            this.L.marker([p.latitude, p.longitude], {
                                icon: this.L.divIcon({
                                    html: `<div class="runway-distance-label">${p.distance}</div>`,
                                    className: 'runway-distance-label-container',
                                    iconSize: [20, 16], iconAnchor: [10, -5]
                                }), interactive: false
                            }).addTo(this.layers.runways);
                        }
                    }
                };
                drawExtension(end1);
                drawExtension(end2);
            }
        }
    }

    // Draw airport markers on the map
    // --- Reference data rendering (airports, heliports, navaids, runways) ---
    // Performance: viewport culling, canvas markers, lazy popups, single zoom+move listener

    _ensureRefDataListeners() {
        if (this._refDataListenersBound) return;
        this._refDataListenersBound = true;
        this._lastRefZoom = this.map.getZoom();
        this._lastRefCenter = this.map.getCenter();

        // Single coalesced handler for both zoom and pan events.
        // Zoom change → full rebuild (canvas markers + DOM labels).
        // Pan only  → rebuild ONLY if panned > 30 % of viewport (distance gating).
        //             This eliminates ~95 % of pan-triggered rebuilds while still
        //             filling in labels that scroll into view on large pans.
        this.map.on('zoomend moveend', () => {
            clearTimeout(this._refTimeout);
            this._refTimeout = setTimeout(() => {
                const zoom = this.map.getZoom();
                if (zoom !== this._lastRefZoom) {
                    this._lastRefZoom = zoom;
                    this._lastRefCenter = this.map.getCenter();
                    this._renderAllRefData();
                    return;
                }
                // Pan only — check if we moved far enough to warrant a label refresh
                const center = this.map.getCenter();
                const b = this.map.getBounds();
                const threshold = Math.max(
                    (b.getEast()  - b.getWest())  * 0.3,
                    (b.getNorth() - b.getSouth()) * 0.3
                );
                if (Math.abs(center.lng - this._lastRefCenter.lng) > threshold ||
                    Math.abs(center.lat - this._lastRefCenter.lat) > threshold) {
                    this._lastRefCenter = center;
                    this._renderAllRefData();
                }
            }, 120);
        });
    }

    _renderAllRefData() {
        this._renderAirports();
        this._renderHeliports();
        this._renderNavaids();
        this._renderAllRunways();
        // Home-airport runways share the same zoom listener
        if (this.runwayData) this.updateRunwayDisplay();
    }

    // Viewport bounds with generous padding for DOM label culling.
    // Canvas markers are added WITHOUT culling (the renderer clips internally).
    // Labels use this padded bounds so they cover ~50 % extra in each direction,
    // surviving moderate pans without a rebuild.
    _getViewBounds() {
        return this.map.getBounds().pad(0.5);
    }

    drawAirports(airports) {
        if (!this.map) return;
        this.airportsData = airports;
        this._renderAirports();
        this._ensureRefDataListeners();
    }

    _renderAirports() {
        if (!this.map || !this.airportsData) return;
        this.layers.airports.clearLayers();

        const zoom = this.map.getZoom();
        if (zoom < 8) return;

        const typeCfg = {
            'large_airport':  { color: '#60A5FA', radius: 6, minZoom: 8 },
            'medium_airport': { color: '#34D399', radius: 5, minZoom: 9 },
            'small_airport':  { color: '#A78BFA', radius: 4, minZoom: 10 },
            'seaplane_base':  { color: '#2DD4BF', radius: 3, minZoom: 11 },
        };
        const showLabels = zoom >= 12;
        // Canvas markers: no viewport culling — renderer clips internally.
        // DOM labels: viewport-culled with generous padding to survive moderate pans.
        const labelBounds = showLabels ? this._getViewBounds() : null;

        for (let i = 0, len = this.airportsData.length; i < len; i++) {
            const ap = this.airportsData[i];
            const cfg = typeCfg[ap.type] || typeCfg['small_airport'];
            if (zoom < cfg.minZoom) continue;

            this.L.circleMarker([ap.latitude, ap.longitude], {
                radius: cfg.radius, color: cfg.color, fillColor: cfg.color,
                fillOpacity: 0.6, opacity: 0.8, weight: 1, renderer: this.canvasRenderer
            }).bindPopup(() => this._airportPopup(ap), { className: 'ref-popup-container', maxWidth: 300 })
              .addTo(this.layers.airports);

            if (showLabels && labelBounds.contains([ap.latitude, ap.longitude])) {
                this.L.marker([ap.latitude, ap.longitude], {
                    icon: this.L.divIcon({
                        html: `<div class="airport-label">${ap.ident}</div>`,
                        className: 'airport-label-container',
                        iconSize: [40, 14], iconAnchor: [20, -8]
                    }),
                    interactive: false
                }).addTo(this.layers.airports);
            }
        }
    }

    _airportPopup(ap) {
        const freqRows = (ap.frequencies || []).map(f =>
            `<tr><td class="ref-popup-type">${f.type}</td><td>${f.description}</td><td class="ref-popup-freq">${f.frequency_mhz}</td></tr>`
        ).join('');
        const freqTable = freqRows
            ? `<table class="ref-popup-table"><tr><th>Type</th><th>Desc</th><th>MHz</th></tr>${freqRows}</table>` : '';
        return `<div class="ref-popup">
            <div class="ref-popup-title">${ap.name}</div>
            <div class="ref-popup-subtitle">${ap.ident} &middot; ${ap.type.replace(/_/g, ' ')} &middot; ${ap.elevation_ft || '?'} ft</div>
            ${ap.municipality ? `<div class="ref-popup-detail">${ap.municipality}, ${ap.iso_country}</div>` : ''}
            ${ap.iata_code ? `<div class="ref-popup-detail">IATA: ${ap.iata_code}</div>` : ''}
            ${freqTable}
        </div>`;
    }

    drawHeliports(heliports) {
        if (!this.map) return;
        this.heliportsData = heliports;
        this._renderHeliports();
        this._ensureRefDataListeners();
    }

    _renderHeliports() {
        if (!this.map || !this.heliportsData) return;
        this.layers.heliports.clearLayers();

        const zoom = this.map.getZoom();
        if (zoom < 11) return;

        const showLabels = zoom >= 12;
        const labelBounds = showLabels ? this._getViewBounds() : null;

        for (let i = 0, len = this.heliportsData.length; i < len; i++) {
            const hp = this.heliportsData[i];

            this.L.circleMarker([hp.latitude, hp.longitude], {
                radius: 4, color: '#FB923C', fillColor: '#FB923C',
                fillOpacity: 0.5, opacity: 0.8, weight: 1.5, renderer: this.canvasRenderer
            }).bindPopup(() => this._heliportPopup(hp), { className: 'ref-popup-container', maxWidth: 300 })
              .addTo(this.layers.heliports);

            if (showLabels && labelBounds.contains([hp.latitude, hp.longitude])) {
                this.L.marker([hp.latitude, hp.longitude], {
                    icon: this.L.divIcon({
                        html: `<div class="heliport-label">${hp.ident}</div>`,
                        className: 'heliport-label-container',
                        iconSize: [40, 14], iconAnchor: [20, -8]
                    }),
                    interactive: false
                }).addTo(this.layers.heliports);
            }
        }
    }

    _heliportPopup(hp) {
        const freqRows = (hp.frequencies || []).map(f =>
            `<tr><td class="ref-popup-type">${f.type}</td><td>${f.description}</td><td class="ref-popup-freq">${f.frequency_mhz}</td></tr>`
        ).join('');
        const freqTable = freqRows
            ? `<table class="ref-popup-table"><tr><th>Type</th><th>Desc</th><th>MHz</th></tr>${freqRows}</table>` : '';
        return `<div class="ref-popup">
            <div class="ref-popup-title">${hp.name}</div>
            <div class="ref-popup-subtitle">${hp.ident} &middot; heliport &middot; ${hp.elevation_ft || '?'} ft</div>
            ${hp.municipality ? `<div class="ref-popup-detail">${hp.municipality}, ${hp.iso_country}</div>` : ''}
            ${freqTable}
        </div>`;
    }

    drawNavaids(navaids) {
        if (!this.map) return;
        this.navaidsData = navaids;
        this._renderNavaids();
        this._ensureRefDataListeners();
    }

    // Pre-cached navaid SVGs (built once per type+size, reused across renders)
    _navaidSVG(type, size = 18) {
        const key = `${type}_${size}`;
        if (!this._navaidSVGCache) this._navaidSVGCache = {};
        if (this._navaidSVGCache[key]) return this._navaidSVGCache[key];

        const s = size, h = s / 2;
        const colors = {
            'VOR': '#60A5FA', 'VOR-DME': '#60A5FA', 'VORTAC': '#818CF8',
            'NDB': '#F59E0B', 'NDB-DME': '#F59E0B',
            'DME': '#A78BFA', 'TACAN': '#818CF8',
        };
        const c = colors[type] || '#9CA3AF';
        const hex = (r) => {
            let pts = [];
            for (let i = 0; i < 6; i++) {
                const a = Math.PI / 6 + i * Math.PI / 3;
                pts.push(`${(h + r * Math.cos(a)).toFixed(1)},${(h - r * Math.sin(a)).toFixed(1)}`);
            }
            return pts.join(' ');
        };

        let svg;
        switch (type) {
            case 'VOR':
                svg = `<svg width="${s}" height="${s}" viewBox="0 0 ${s} ${s}"><polygon points="${hex(h * 0.8)}" fill="none" stroke="${c}" stroke-width="1.5"/></svg>`; break;
            case 'VOR-DME':
                svg = `<svg width="${s}" height="${s}" viewBox="0 0 ${s} ${s}"><polygon points="${hex(h * 0.8)}" fill="none" stroke="${c}" stroke-width="1.5"/><circle cx="${h}" cy="${h}" r="2" fill="${c}"/></svg>`; break;
            case 'VORTAC':
                svg = `<svg width="${s}" height="${s}" viewBox="0 0 ${s} ${s}"><polygon points="${hex(h * 0.7)}" fill="none" stroke="${c}" stroke-width="1.5"/>${[90,210,330].map(a=>{const ar=a*Math.PI/180;return `<line x1="${(h+h*0.72*Math.cos(ar)).toFixed(1)}" y1="${(h-h*0.72*Math.sin(ar)).toFixed(1)}" x2="${(h+h*0.95*Math.cos(ar)).toFixed(1)}" y2="${(h-h*0.95*Math.sin(ar)).toFixed(1)}" stroke="${c}" stroke-width="2"/>`;}).join('')}</svg>`; break;
            case 'NDB': {
                const dots = [0,60,120,180,240,300].map(a=>{const ar=a*Math.PI/180;return `<circle cx="${(h+h*0.7*Math.cos(ar)).toFixed(1)}" cy="${(h-h*0.7*Math.sin(ar)).toFixed(1)}" r="1" fill="${c}" opacity="0.6"/>`;}).join('');
                svg = `<svg width="${s}" height="${s}" viewBox="0 0 ${s} ${s}"><circle cx="${h}" cy="${h}" r="3" fill="${c}"/>${dots}</svg>`; break;
            }
            case 'NDB-DME': {
                const dots2 = [0,60,120,180,240,300].map(a=>{const ar=a*Math.PI/180;return `<circle cx="${(h+h*0.7*Math.cos(ar)).toFixed(1)}" cy="${(h-h*0.7*Math.sin(ar)).toFixed(1)}" r="1" fill="${c}" opacity="0.6"/>`;}).join('');
                svg = `<svg width="${s}" height="${s}" viewBox="0 0 ${s} ${s}"><circle cx="${h}" cy="${h}" r="3" fill="${c}"/>${dots2}<rect x="${h-2}" y="${h-2}" width="4" height="4" fill="none" stroke="${c}" stroke-width="0.8"/></svg>`; break;
            }
            case 'DME':
                svg = `<svg width="${s}" height="${s}" viewBox="0 0 ${s} ${s}"><rect x="${h-h*0.5}" y="${h-h*0.5}" width="${h}" height="${h}" fill="none" stroke="${c}" stroke-width="1.5"/></svg>`; break;
            case 'TACAN':
                svg = `<svg width="${s}" height="${s}" viewBox="0 0 ${s} ${s}">${[90,210,330].map(a=>{const ar=a*Math.PI/180;return `<line x1="${(h+h*0.2*Math.cos(ar)).toFixed(1)}" y1="${(h-h*0.2*Math.sin(ar)).toFixed(1)}" x2="${(h+h*0.8*Math.cos(ar)).toFixed(1)}" y2="${(h-h*0.8*Math.sin(ar)).toFixed(1)}" stroke="${c}" stroke-width="2"/>`;}).join('')}<circle cx="${h}" cy="${h}" r="2" fill="${c}"/></svg>`; break;
            default:
                svg = `<svg width="${s}" height="${s}" viewBox="0 0 ${s} ${s}"><circle cx="${h}" cy="${h}" r="4" fill="none" stroke="${c}" stroke-width="1.5"/></svg>`;
        }
        this._navaidSVGCache[key] = svg;
        return svg;
    }

    // Navaid canvas color map for zoom 9-10 (fast circleMarkers, no DOM)
    _navaidCanvasColor(type) {
        const m = { 'VOR':'#60A5FA','VOR-DME':'#60A5FA','VORTAC':'#818CF8','NDB':'#F59E0B','NDB-DME':'#F59E0B','DME':'#A78BFA','TACAN':'#818CF8' };
        return m[type] || '#9CA3AF';
    }

    _renderNavaids() {
        if (!this.map || !this.navaidsData) return;
        this.layers.navaids.clearLayers();

        const zoom = this.map.getZoom();
        if (zoom < 9) return;

        const useSVG = zoom >= 11;
        const showLabels = zoom >= 12;
        // DOM elements (SVG icons + labels) are viewport-culled; canvas markers are not.
        const domBounds = (useSVG || showLabels) ? this._getViewBounds() : null;

        for (let i = 0, len = this.navaidsData.length; i < len; i++) {
            const nav = this.navaidsData[i];
            const pos = [nav.latitude, nav.longitude];

            if (useSVG) {
                // At high zoom: always add a tiny canvas marker for the popup hit target
                const c = this._navaidCanvasColor(nav.type);
                this.L.circleMarker(pos, {
                    radius: 1, color: c, fillColor: c,
                    fillOpacity: 0, opacity: 0, weight: 0,
                    renderer: this.canvasRenderer
                }).bindPopup(() => this._navaidPopup(nav), { className: 'ref-popup-container', maxWidth: 280 })
                  .addTo(this.layers.navaids);

                // SVG divIcon — viewport-culled (DOM element)
                if (domBounds.contains(pos)) {
                    this.L.marker(pos, {
                        icon: this.L.divIcon({
                            html: this._navaidSVG(nav.type),
                            className: 'navaid-icon-container',
                            iconSize: [18, 18], iconAnchor: [9, 9]
                        }),
                        interactive: false
                    }).addTo(this.layers.navaids);
                }
            } else {
                // Zoom 9-10: canvas circleMarkers only — no viewport culling needed
                const c = this._navaidCanvasColor(nav.type);
                this.L.circleMarker(pos, {
                    radius: 3, color: c, fillColor: c,
                    fillOpacity: 0.5, opacity: 0.7, weight: 1,
                    renderer: this.canvasRenderer
                }).bindPopup(() => this._navaidPopup(nav), { className: 'ref-popup-container', maxWidth: 280 })
                  .addTo(this.layers.navaids);
            }

            if (showLabels && domBounds.contains(pos)) {
                this.L.marker(pos, {
                    icon: this.L.divIcon({
                        html: `<div class="navaid-label">${nav.ident}</div>`,
                        className: 'navaid-label-container',
                        iconSize: [36, 14], iconAnchor: [18, -6]
                    }),
                    interactive: false
                }).addTo(this.layers.navaids);
            }
        }
    }

    _navaidPopup(nav) {
        const freqDisplay = nav.frequency_khz ? `${(nav.frequency_khz / 1000).toFixed(nav.frequency_khz >= 100000 ? 2 : 1)} ${nav.frequency_khz >= 100000 ? 'MHz' : 'kHz'}` : '';
        const popupSvg = this._navaidSVG(nav.type, 24);
        return `<div class="ref-popup">
            <div class="ref-popup-title" style="display:flex;align-items:center;gap:6px">${popupSvg} ${nav.ident} - ${nav.name}</div>
            <div class="ref-popup-subtitle">${nav.type}${freqDisplay ? ' &middot; ' + freqDisplay : ''}</div>
            ${nav.elevation_ft ? `<div class="ref-popup-detail">Elev: ${nav.elevation_ft} ft</div>` : ''}
            ${nav.associated_airport ? `<div class="ref-popup-detail">Airport: ${nav.associated_airport}</div>` : ''}
            ${nav.usage_type ? `<div class="ref-popup-detail">Usage: ${nav.usage_type}${nav.power ? ' &middot; ' + nav.power : ''}</div>` : ''}
        </div>`;
    }

    drawAllRunways(runways) {
        if (!this.map) return;
        this.allRunwaysData = runways;
        this._renderAllRunways();
        this._ensureRefDataListeners();
    }

    _renderAllRunways() {
        if (!this.map || !this.allRunwaysData) return;
        this.layers.allRunways.clearLayers();

        const zoom = this.map.getZoom();
        if (zoom < 10) return;

        const showLabels = zoom >= 12;
        const labelBounds = showLabels ? this._getViewBounds() : null;
        const weight = Math.max(2, Math.min(5, zoom - 8));

        for (let i = 0, len = this.allRunwaysData.length; i < len; i++) {
            const rwy = this.allRunwaysData[i];
            if (!rwy.le_latitude || !rwy.le_longitude || !rwy.he_latitude || !rwy.he_longitude) continue;

            // Canvas polylines: no viewport culling — renderer clips internally
            this.L.polyline(
                [[rwy.le_latitude, rwy.le_longitude], [rwy.he_latitude, rwy.he_longitude]],
                { color: '#9CA3AF', weight, opacity: 0.6, renderer: this.canvasRenderer }
            ).addTo(this.layers.allRunways);

            if (showLabels) {
                const leInView = labelBounds.contains([rwy.le_latitude, rwy.le_longitude]);
                const heInView = labelBounds.contains([rwy.he_latitude, rwy.he_longitude]);
                if (rwy.le_ident && leInView) {
                    this.L.marker([rwy.le_latitude, rwy.le_longitude], {
                        icon: this.L.divIcon({
                            html: `<div class="runway-label" style="font-size:8px;opacity:0.7">${rwy.le_ident}</div>`,
                            className: 'runway-label-container', iconSize: [24, 14]
                        }),
                        interactive: false
                    }).addTo(this.layers.allRunways);
                }
                if (rwy.he_ident && heInView) {
                    this.L.marker([rwy.he_latitude, rwy.he_longitude], {
                        icon: this.L.divIcon({
                            html: `<div class="runway-label" style="font-size:8px;opacity:0.7">${rwy.he_ident}</div>`,
                            className: 'runway-label-container', iconSize: [24, 14]
                        }),
                        interactive: false
                    }).addTo(this.layers.allRunways);
                }
            }
        }
    }

    // Toggle layer visibility
    toggleLayerVisibility(layerName, visible) {
        const layer = this.layers[layerName];
        if (!layer || !this.map) return;
        if (visible) {
            if (!this.map.hasLayer(layer)) layer.addTo(this.map);
        } else {
            if (this.map.hasLayer(layer)) this.map.removeLayer(layer);
        }
    }

    // Clean up runway rendering resources
    cleanupRunwayRendering() {
        this.runwayData = null;
        if (this.layers.runways) this.layers.runways.clearLayers();
    }

    // Release timers, listeners, and map references to avoid leaks on teardown/reload
    cleanup() {
        if (this.labelRefreshTimer) {
            clearInterval(this.labelRefreshTimer);
            this.labelRefreshTimer = null;
        }
        if (this._visibleListUpdateTimeout) {
            clearTimeout(this._visibleListUpdateTimeout);
            this._visibleListUpdateTimeout = null;
        }
        if (this._refTimeout) {
            clearTimeout(this._refTimeout);
            this._refTimeout = null;
        }
        if (this._refreshThrottleTimer) {
            clearTimeout(this._refreshThrottleTimer);
            this._refreshThrottleTimer = null;
        }

        this.cleanupTracksMiniMap();

        if (this.map) {
            try {
                this.map.off();
                this.map.remove();
            } catch (error) {
                console.warn('MapManager cleanup error:', error);
            }
            this.map = null;
        }

        Object.keys(this.markers).forEach(hex => {
            this.removeAircraft(hex);
        });

        this.layers.aircraft.clearLayers();
        this.layers.trails.clearLayers();
        this.layers.rangeRings.clearLayers();
        this.layers.runways.clearLayers();
        this.layers.airports.clearLayers();
        this.layers.heliports.clearLayers();
        this.layers.navaids.clearLayers();
        this.layers.allRunways.clearLayers();

        this.markers = {};
        this.trails = {};
        this.trailVersions = {};
        this.trailLayersByHex = {};
        this._markerVisibilityState = {};
        this._visualStateVersionByHex = {};
        this._flightCardHoverStateByHex = {};
        this.proximityHexSet = null;
        this.proximityRefHex = null;
        this.proximityDistanceNM = null;
    }

    // Return lightweight map performance stats for a recent time window.
    getPerformanceStats() {
        const now = performance.now();
        const elapsedSeconds = Math.max(0.001, (now - this._perfSnapshot.timestamp) / 1000);

        const delta = {};
        const keys = Object.keys(this._perfCounters);
        for (let i = 0; i < keys.length; i++) {
            const key = keys[i];
            delta[key] = this._perfCounters[key] - (this._perfSnapshot.counters[key] || 0);
        }

        const refreshExecuted = delta.refreshExecuted || 0;
        const avgRefreshMs = refreshExecuted > 0 ? (delta.refreshDurationMs / refreshExecuted) : 0;
        const visualTotal = (delta.visualStateApplied || 0) + (delta.visualStateSkipped || 0);
        const visualSkipPct = visualTotal > 0 ? ((delta.visualStateSkipped / visualTotal) * 100) : 0;
        const markerOps = (delta.markerAdds || 0) + (delta.markerRemoves || 0) + (delta.labelAdds || 0) + (delta.labelRemoves || 0);
        const coalescedRatio = (delta.refreshRequested || 0) > 0 ? (delta.refreshCoalesced / delta.refreshRequested) : 0;

        const stats = {
            windowSec: Number(elapsedSeconds.toFixed(1)),
            refreshPerSec: Number((refreshExecuted / elapsedSeconds).toFixed(2)),
            avgRefreshMs: Number(avgRefreshMs.toFixed(2)),
            coalescedPct: Number((coalescedRatio * 100).toFixed(1)),
            markerOpsPerSec: Number((markerOps / elapsedSeconds).toFixed(2)),
            singleUpdatesPerSec: Number(((delta.singleAircraftUpdates || 0) / elapsedSeconds).toFixed(2)),
            visualSkipPct: Number(visualSkipPct.toFixed(1)),
            trailLayerOpsPerSec: Number((((delta.trailLayersAdded || 0) + (delta.trailLayersRemoved || 0)) / elapsedSeconds).toFixed(2)),
            fullVisibilityPasses: delta.fullVisibilityPasses || 0
        };

        this._perfSnapshot = {
            timestamp: now,
            counters: { ...this._perfCounters }
        };

        return stats;
    }
    
    // Center the map on a specific aircraft
    centerOnAircraft(aircraft) {
        if (!aircraft || !aircraft.adsb || !aircraft.adsb.lat || !aircraft.adsb.lon) return;
        
        // Get the aircraft position
        const position = [aircraft.adsb.lat, aircraft.adsb.lon];
        
        // Center the map on the aircraft position
        this.map.setView(position, this.map.getZoom());
    }

    // Show a highlighted position on the map when hovering over history rows
    showPositionHighlight(lat, lon, positionData) {
        if (!this.map || !lat || !lon) return;

        // Clear any existing highlight
        this.clearPositionHighlight();

        // Create a temporary marker for the highlighted position
        const highlightIcon = L.divIcon({
            className: 'position-highlight-marker',
            html: `<div style="
                width: 12px;
                height: 12px;
                background: #4CAF50;
                border: 2px solid #ffffff;
                border-radius: 50%;
                box-shadow: 0 0 10px rgba(76,175,80,0.8);
                animation: pulse 1.5s infinite;
            "></div>`,
            iconSize: [16, 16],
            iconAnchor: [8, 8]
        });

        this.positionHighlightMarker = L.marker([lat, lon], {
            icon: highlightIcon,
            zIndexOffset: 2000 // Ensure it appears above other markers
        }).addTo(this.map);

        // Create a popup with position details
        if (positionData) {
            const popupContent = `
                <div style="font-size: 11px; line-height: 1.3;">
                    <strong>Historical Position</strong><br>
                    <strong>Time:</strong> ${new Date(positionData.timestamp).toLocaleString()}<br>
                    <strong>Altitude:</strong> ${Math.round(positionData.altitude)} ft<br>
                    <strong>True Heading:</strong> ${Math.round(positionData.true_heading)}°<br>
                    <strong>Ground Speed:</strong> ${Math.round(positionData.speed_gs)} kts<br>
                    <strong>True Speed:</strong> ${Math.round(positionData.speed_true)} kts<br>
                    <strong>Coordinates:</strong> ${lat.toFixed(6)}, ${lon.toFixed(6)}
                </div>
            `;
            
            this.positionHighlightMarker.bindPopup(popupContent, {
                offset: [0, -10],
                closeButton: false,
                autoClose: false,
                closeOnClick: false
            }).openPopup();
        }

        // Add CSS animation if not already added
        if (!document.getElementById('position-highlight-styles')) {
            const style = document.createElement('style');
            style.id = 'position-highlight-styles';
            style.textContent = `
                @keyframes pulse {
                    0% { transform: scale(1); opacity: 1; }
                    50% { transform: scale(1.3); opacity: 0.7; }
                    100% { transform: scale(1); opacity: 1; }
                }
            `;
            document.head.appendChild(style);
        }
    }

    // Clear the position highlight from the map
    clearPositionHighlight() {
        if (this.positionHighlightMarker) {
            this.map.removeLayer(this.positionHighlightMarker);
            this.positionHighlightMarker = null;
        }
    }

    // Show takeoff/landing visual effect
    showTakeoffLandingEffect(hex, eventType, phase) {
        const aircraft = this.store.aircraft[hex];
        if (!aircraft || !aircraft.adsb || !aircraft.adsb.lat || !aircraft.adsb.lon) {
            console.warn(`Cannot show ${eventType} effect: aircraft ${hex} not found or missing position`);
            return;
        }

        const position = [aircraft.adsb.lat, aircraft.adsb.lon];

        // Use the phase from the event data to determine color
        const color = this.getPhaseColor(phase);

        // Create multiple expanding circles for a pulse effect
        const pulseCount = 3;
        const maxRadius = 2000; // meters
        const animationDuration = 2000; // milliseconds
        
        for (let i = 0; i < pulseCount; i++) {
            setTimeout(() => {
                // Create an expanding circle
                const circle = this.L.circle(position, {
                    radius: 1,
                    color: color,
                    fillColor: color,
                    fillOpacity: 0.3,
                    weight: 2,
                    opacity: 0.8,
                    renderer: this.canvasRenderer  // Canvas for performance
                }).addTo(this.map);
                
                // Animate the circle expansion and fade
                let startTime = Date.now();
                const animate = () => {
                    const elapsed = Date.now() - startTime;
                    const progress = Math.min(elapsed / animationDuration, 1);
                    
                    // Exponential easing for smooth animation
                    const easeOut = 1 - Math.pow(1 - progress, 3);
                    
                    // Update radius
                    const currentRadius = maxRadius * easeOut;
                    circle.setRadius(currentRadius);
                    
                    // Update opacity (fade out)
                    const opacity = 0.8 * (1 - progress);
                    const fillOpacity = 0.3 * (1 - progress);
                    circle.setStyle({
                        opacity: opacity,
                        fillOpacity: fillOpacity
                    });
                    
                    if (progress < 1) {
                        requestAnimationFrame(animate);
                    } else {
                        // Remove the circle when animation is complete
                        this.map.removeLayer(circle);
                    }
                };
                
                requestAnimationFrame(animate);
            }, i * 300); // Stagger each pulse by 300ms
        }
        
        // Also add a temporary highlight to the aircraft marker
        if (this.markers[hex]) {
            const marker = this.markers[hex].aircraft;
            const label = this.markers[hex].label;
            
            // Store original classes
            const originalMarkerClass = marker._icon ? marker._icon.className : '';
            const originalLabelClass = label._icon ? label._icon.className : '';
            
            // Add highlight class
            if (marker._icon) {
                marker._icon.classList.add('takeoff-landing-highlight');
            }
            if (label._icon) {
                label._icon.classList.add('takeoff-landing-label-highlight');
            }
            
            // Remove highlight after animation
            setTimeout(() => {
                if (marker._icon) {
                    marker._icon.classList.remove('takeoff-landing-highlight');
                }
                if (label._icon) {
                    label._icon.classList.remove('takeoff-landing-label-highlight');
                }
            }, animationDuration + 1000); // Keep highlighted a bit longer than the pulse
        }
    }

    // Get phase color as hex value for animations
    getPhaseColor(phase) {
        const phaseColorMap = {
            'NEW': '#9CA3AF',    // gray-400
            'TAX': '#A78BFA',    // purple-400
            'T/O': '#FB923C',    // orange-400
            'CLB': '#A3E635',    // lime-400
            'DEP': '#4ADE80',    // green-400
            'CRZ': '#60A5FA',    // blue-400
            'ARR': '#F9A8D4',    // pink-300
            'APP': '#FACC15',    // yellow-400
            'T/D': '#2DD4BF',    // teal-400
            'UNK': '#94A3B8'     // slate-400
        };
        return phaseColorMap[phase] || '#9CA3AF'; // Default to gray
    }
}

// Export the MapManager class if using modules, or attach to window for global access
window.MapManager = MapManager;