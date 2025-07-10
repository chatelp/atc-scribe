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
        if (historyData.length > 0) {
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
            runways: this.L.layerGroup(), // New layer for runways
        };
        this.markers = {}; // Stores Leaflet marker objects { hex: { aircraft: marker, label: labelMarker } }
        this.trails = {}; // Trails managed by MapManager
        this.proximityCircle = null; // For proximity visualization
        this.proximityHexSet = null; // Set of aircraft hex codes in proximity
        this.proximityRefHex = null; // Reference aircraft hex for proximity
        this.proximityDistanceNM = null; // Distance in NM for proximity circle
        
        // Runway rendering state
        this.runwayData = null;
        this.runwayZoomListener = null;
        this.runwayUpdateTimeout = null;
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
        
        this.map = this.L.map('map', {
            center: [centerLat, centerLon],
            zoom: this.CONFIG.defaultZoom,
            zoomControl: false,
            attributionControl: false,
            keyboard: false  // Disable Leaflet's default keyboard navigation to prevent conflicts with custom hotkeys
        });
        
        // Add event listeners for viewport changes to update visible aircraft list
        this.map.on('moveend zoomend', () => {
            this.updateVisibleAircraftList();
        });

        this.L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
            attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> &copy; <a href="https://carto.com/attributions">CARTO</a>',
            subdomains: 'abcd',
            maxZoom: 19
        }).addTo(this.map);

        this.layers.aircraft.addTo(this.map);
        this.layers.trails.addTo(this.map);
        this.layers.runways.addTo(this.map); // Add the runways layer
        
        // Only add range rings layer and rings if the setting is enabled
        if (this.store.settings.showRings) {
            this.layers.rangeRings.addTo(this.map);
            this.addRangeRings();
        }
        
        this.map.on('click', (e) => {
            const isMapClick = e.originalEvent.target.classList.contains('leaflet-container') ||
                              e.originalEvent.target.classList.contains('leaflet-tile') ||
                              e.originalEvent.target.classList.contains('leaflet-pane');
            
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
            const isMapClick = e.originalEvent.target.classList.contains('leaflet-container') ||
                              e.originalEvent.target.classList.contains('leaflet-tile') ||
                              e.originalEvent.target.classList.contains('leaflet-pane');
            
            if (isMapClick && this.store.searchTerm) {
                this.store.searchTerm = '';
                this.store.applyFilters();
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
                className: 'stroke-neutral-600/50 stroke-1 fill-none'
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
        if (!aircraft.adsb || !aircraft.adsb.lat || !aircraft.adsb.lon || !aircraft.adsb.gs || aircraft.adsb.gs < 5) {
            return [];
        }

        const currentLat = aircraft.adsb.lat;
        const currentLon = aircraft.adsb.lon;
        const groundSpeed = aircraft.adsb.gs; // knots
        const track = aircraft.adsb.track || 0; // degrees

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
        const now = new Date();

        if (!this.trails[aircraft.hex]) {
            this.trails[aircraft.hex] = [];
        }
        this.trails[aircraft.hex].push({
            lat: aircraft.adsb ? aircraft.adsb.lat : 0,
            lon: aircraft.adsb ? aircraft.adsb.lon : 0,
            alt_baro: aircraft.adsb ? aircraft.adsb.alt_baro : 0,
            time: now,
            isHistorical: false
        });

        // Use trailLength from settings, default to 2 minutes if not set
        const trailLengthMinutes = this.store.settings.trailLength !== undefined ? this.store.settings.trailLength : 2;
        const cutoffTime = new Date(now.getTime() - (trailLengthMinutes * 60 * 1000));
        this.trails[aircraft.hex] = this.trails[aircraft.hex].filter(point => point.time >= cutoffTime);

        const position = aircraft.adsb ? [aircraft.adsb.lat, aircraft.adsb.lon] : [0, 0];
        const heading = this.store.getHeadingWithFallback(aircraft); // Use same fallback as label text
        const icon = this.createAircraftIcon(heading);
        const newLabelContent = this.store.createLabelContent(aircraft, (aircraft.flight || aircraft.hex).trim(), aircraft.adsb ? aircraft.adsb.alt_baro : 0, this.getVerticalTrend(aircraft));
        const labelContentIcon = this.L.divIcon({
            html: newLabelContent,
            className: this.getLabelClassName(aircraft),
            iconSize: [130, 40],
            iconAnchor: [-8, 2] // Aircraft arrow positioned just off the top-left corner of the label
        });

        if (!this.markers[aircraft.hex]) {
            // SAFETY CHECK: Ensure no existing markers are on the map for this aircraft
            this.forceCleanupAircraft(aircraft.hex);
            
            const marker = this.L.marker(position, { icon: icon, riseOnHover: true });
            const label = this.L.marker(position, { icon: labelContentIcon, interactive: true });
            
            // Store last position, heading, label content, and altitude for change detection
            this.markers[aircraft.hex] = {
                aircraft: marker,
                label: label,
                lastLat: position[0],
                lastLon: position[1],
                lastHeading: heading,
                lastLabelContent: newLabelContent,
                lastAltitude: aircraft.adsb ? aircraft.adsb.alt_baro : 0,
                created: Date.now() // Track creation time for debugging
            };
            
            // PERFORMANCE FIX: Apply initial CSS rotation instead of generating rotated SVG
            setTimeout(() => {
                const markerElement = marker.getElement();
                if (markerElement) {
                    const iconContainer = markerElement.querySelector('.aircraft-icon-container');
                    if (iconContainer) {
                        iconContainer.style.transform = `rotate(${heading}deg)`;
                        iconContainer.style.transformOrigin = 'center';
                        iconContainer.style.transition = 'transform 0.2s ease';
                    }
                }
            }, 0);
            
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
                Alpine.store('atc').sendFilterUpdate();
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
                Alpine.store('atc').sendFilterUpdate();
                this.updateVisualState(aircraft.hex, true);
            });
            
            //console.log(`[MAP] Created new markers for ${aircraft.hex}`);
        } else {
            // UPDATE EXISTING MARKER WITH COMPREHENSIVE CHANGE DETECTION
            const existing = this.markers[aircraft.hex];
            
            // Only update position if moved more than ~25 meters (0.0002 degrees ≈ 25m)
            const positionChanged = Math.abs(existing.lastLat - position[0]) > 0.0002 || 
                                   Math.abs(existing.lastLon - position[1]) > 0.0002;
            
            if (positionChanged) {
                existing.aircraft.setLatLng(position);
                existing.label.setLatLng(position); // Fixed: Remove offset causing overlapping labels
                existing.lastLat = position[0];
                existing.lastLon = position[1];
            }
            
            // PERFORMANCE FIX: Use CSS rotation instead of expensive setIcon() calls
            const headingChanged = Math.abs((existing.lastHeading || 0) - heading) > 5;
            
            if (headingChanged) {
                // Apply CSS rotation directly to marker element instead of regenerating icon
                const markerElement = existing.aircraft.getElement();
                if (markerElement) {
                    const iconContainer = markerElement.querySelector('.aircraft-icon-container');
                    if (iconContainer) {
                        iconContainer.style.transform = `rotate(${heading}deg)`;
                        iconContainer.style.transformOrigin = 'center';
                    }
                }
                existing.lastHeading = heading;
            }
            
            // Check for altitude changes (more sensitive detection)
            const currentAltitude = aircraft.adsb ? aircraft.adsb.alt_baro : 0;
            const altitudeChanged = Math.abs((existing.lastAltitude || 0) - currentAltitude) > 50; // 50 ft threshold
            
            // Update label if content changed OR altitude changed significantly
            if (!existing.lastLabelContent || existing.lastLabelContent !== newLabelContent || altitudeChanged) {
                existing.label.setIcon(labelContentIcon);
                existing.lastLabelContent = newLabelContent;
                existing.lastAltitude = currentAltitude;
            }
        }
    }

    getVerticalTrend(aircraft) {
        const verticalRate = aircraft.adsb ? aircraft.adsb.baro_rate : 0;
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

    // PERFORMANCE FIX: Cache static aircraft icon and use CSS rotation
    getStaticAircraftIcon() {
        if (!this._staticAircraftIcon) {
            this._staticAircraftIcon = this.L.divIcon({
                html: `<div class="aircraft-icon-container">
                        <svg width="32" height="32" viewBox="0 0 32 32">
                            <path d="M16,4 L22,24 L16,20 L10,24 L16,4" fill="#FFFFFF" stroke="#000000" stroke-width="1.5" />
                        </svg>
                      </div>`,
                className: 'aircraft-marker',
                iconSize: [32, 32],
                iconAnchor: [16, 16]
            });
        }
        return this._staticAircraftIcon;
    }

    createAircraftIcon(heading = 0) {
        // CRITICAL FIX: Use static icon and CSS rotation instead of SVG regeneration
        return this.getStaticAircraftIcon();
    }

    updateFlightPaths() {
        // Don't clear all layers - only update changed trails
        if (!this.store.settings.showPaths) {
            this.layers.trails.clearLayers();
            return;
        }

        // Track which trails need updates
        const updatedTrails = new Set();
        
        Object.keys(this.store.aircraft).forEach(hex => {
            const aircraftData = this.store.aircraft[hex];
            if (!aircraftData) return;

            // Filter by air/ground state - both can be enabled/disabled independently
            const isVisibleByGroundState = (aircraftData.on_ground && this.store.settings.showGroundAircraft) ||
                                          (!aircraftData.on_ground && this.store.settings.showAirAircraft);
            
            // Only apply altitude filter to aircraft in the air
            const isVisibleByAltitude = aircraftData.on_ground ||
                (aircraftData.adsb && aircraftData.adsb.alt_baro >= this.store.settings.minAltitude &&
                aircraftData.adsb.alt_baro <= this.store.settings.maxAltitude);
            
            // Filter by flight phase - matching the table logic in app.js
            const currentPhase = this.getCurrentPhase(aircraftData);
            
            const isVisibleByPhase = !(this.store.settings.phaseFilters && this.store.settings.phaseFilters[currentPhase] === false);
            
            // Check if this aircraft is currently selected
            const isSelectedAircraft = this.store.selectedAircraft && this.store.selectedAircraft.hex === hex;
            
            // Determine if trail should be visible
            const shouldShowTrail = (isVisibleByGroundState && isVisibleByAltitude && isVisibleByPhase) || isSelectedAircraft;
            
            // If aircraft doesn't match filters and isn't selected, remove its trail
            if (!shouldShowTrail) {
                this.layers.trails.eachLayer(layer => {
                    if (layer.options.aircraftHex === hex) {
                        this.layers.trails.removeLayer(layer);
                    }
                });
                return;
            }

            const trail = this.trails[hex];
            if (!trail || trail.length < 2) return;
            
            // Only update trail if aircraft position changed significantly or if not yet updated
            const lastTrailPoint = trail[trail.length - 1];
            const currentPos = [aircraftData.adsb.lat, aircraftData.adsb.lon];
            
            // Check if position changed enough to warrant trail update
            const positionChanged = !lastTrailPoint || 
                Math.abs(lastTrailPoint.lat - currentPos[0]) > 0.0001 ||
                Math.abs(lastTrailPoint.lon - currentPos[1]) > 0.0001;
                
            if (!positionChanged && updatedTrails.has(hex)) return;
            
            // Remove old trail for this aircraft only
            this.layers.trails.eachLayer(layer => {
                if (layer.options.aircraftHex === hex) {
                    this.layers.trails.removeLayer(layer);
                }
            });
            
            let currentOpacity = 0.7;

            if (this.store.selectedAircraft && !isSelectedAircraft) {
                currentOpacity = this.CONFIG.selectedFadeOpacity;      
            } else if (isSelectedAircraft) {
                currentOpacity = 0.7;
            }

            const currentPoints = [];
            
            trail.forEach(point => {
                const latLng = [point.lat, point.lon];
                currentPoints.push(latLng);
            });
            
            if (currentPoints.length >= 2) {
                const polyline = this.L.polyline(currentPoints, {
                    className: 'stroke-2 fill-none',
                    color: this.getAircraftColor(hex),
                    opacity: currentOpacity,
                    aircraftHex: hex // Add identifier for cleanup
                });
                polyline.addTo(this.layers.trails);
            }

            // Use dedicated history data for selected aircraft from store.aircraftDetailsHistoryData
            if (isSelectedAircraft) {
                console.log(`Selected aircraft ${hex}, status: ${aircraftData.status}, historyData length: ${this.store.aircraftDetailsHistoryData ? this.store.aircraftDetailsHistoryData.length : 'undefined'}`);
            }
            
            if (isSelectedAircraft && this.store && this.store.aircraftDetailsHistoryData && this.store.aircraftDetailsHistoryData.length > 0) {
                // Filter valid positions and limit to max 100 points while maintaining time range
                const validPositions = this.store.aircraftDetailsHistoryData.filter(position => position.lat && position.lon);
                
                let historyPoints;
                const maxHistoryPoints = 50;
                
                if (validPositions.length <= maxHistoryPoints) {
                    // Use all points if we have 100 or fewer
                    historyPoints = validPositions.map(position => [position.lat, position.lon]);
                } else {
                    // Subsample to maintain time range with max 100 points
                    const step = Math.floor(validPositions.length / maxHistoryPoints);
                    historyPoints = [];
                    
                    for (let i = 0; i < validPositions.length; i += step) {
                        historyPoints.push([validPositions[i].lat, validPositions[i].lon]);
                    }
                    
                    // Always include the last point to maintain complete time range
                    const lastPosition = validPositions[validPositions.length - 1];
                    const lastPoint = [lastPosition.lat, lastPosition.lon];
                    if (historyPoints.length === 0 ||
                        historyPoints[historyPoints.length - 1][0] !== lastPoint[0] ||
                        historyPoints[historyPoints.length - 1][1] !== lastPoint[1]) {
                        historyPoints.push(lastPoint);
                    }
                }
                
                console.log(`History trail for ${hex}: ${historyPoints.length} points (from ${validPositions.length} total), aircraft status: ${aircraftData.status}`);
                
                // Create the main history trail polyline only if we have at least 2 points
                if (historyPoints.length >= 2) {
                    const historyPolyline = this.L.polyline(historyPoints, {
                        color: this.getAircraftColor(hex),
                        weight: 2,
                        opacity: 0.7,
                        lineJoin: 'round',
                        className: 'history-trail',
                        dashArray: '4, 4',
                        dashOffset: '0',
                        aircraftHex: hex
                    });
                    historyPolyline.addTo(this.layers.trails);
                }
                
                // Add markers at key points (balanced performance vs visibility)
                const pointsToShow = this.store.aircraftDetailsHistoryData.length <= 8 ?
                    this.store.aircraftDetailsHistoryData :
                    this.store.aircraftDetailsHistoryData.filter((_, i) =>
                        i === 0 || i === this.store.aircraftDetailsHistoryData.length - 1 || i % Math.ceil(this.store.aircraftDetailsHistoryData.length / 8) === 0
                    );
                
                pointsToShow.forEach((position, index) => {
                    const posLatLng = [position.lat, position.lon];
                    
                    // Format altitude for display
                    const altitude = position.altitude !== undefined ? position.altitude : 'N/A';
                    const altText = altitude === 0 ? 'GND' :
                                   altitude !== 'N/A' ? Math.round(altitude/100)*100 : 'N/A';
                    
                    // Calculate time ago in minutes
                    const now = new Date();
                    const positionTime = new Date(position.timestamp);
                    const minutesAgo = Math.round((now - positionTime) / (1000 * 60));
                    
                    // Create history marker in grey color without tooltip
                    const historyMarker = this.L.circleMarker(posLatLng, {
                        radius: 3,
                        color: '#888888',
                        fillColor: '#888888',
                        fillOpacity: 0.5,
                        opacity: 0.5,
                        weight: 1,
                        className: 'history-point-marker',
                        pane: 'shadowPane',
                        aircraftHex: hex
                    });
                    historyMarker.addTo(this.layers.trails);
                    
                    // Add time and altitude label next to the marker in grey
                    const historyLabel = this.L.marker(posLatLng, {
                        icon: this.L.divIcon({
                            html: `<div style="color: #888888; font-size: 10px; opacity: 0.7;">-${minutesAgo}m: ${altText}</div>`,
                            className: 'altitude-label-container',
                            iconSize: [50, 20],
                            iconAnchor: [-5, 0] // Position to the right of the point
                        }),
                        aircraftHex: hex
                    });
                    historyLabel.addTo(this.layers.trails);
                });
                
                // Add a special marker for the most recent position
                if (this.store.aircraftDetailsHistoryData.length > 0) {
                    const lastPos = this.store.aircraftDetailsHistoryData[this.store.aircraftDetailsHistoryData.length - 1];
                    const lastPosLatLng = [lastPos.lat, lastPos.lon];
                    
                    const lastHistoryMarker = this.L.circleMarker(lastPosLatLng, {
                        radius: 4,
                        color: '#888888',
                        fillColor: '#888888',
                        fillOpacity: 0.7,
                        opacity: 0.7,
                        weight: 1,
                        className: 'history-point-marker-last',
                        aircraftHex: hex
                    });
                    lastHistoryMarker.addTo(this.layers.trails);
                }
            }

            // Show future trajectories for all active aircraft (not signal_lost)
            const showFutureTrajectory = (isSelectedAircraft && this.store && this.store.aircraftDetailsFutureData && this.store.aircraftDetailsFutureData.length > 0) ||
                                       (aircraftData.status === 'active' && aircraftData.adsb && aircraftData.adsb.gs > 5); // Only for moving active aircraft
            
            if (showFutureTrajectory) {
                let futureDataPoints = [];
                
                if (isSelectedAircraft && this.store.aircraftDetailsFutureData) {
                    // Use detailed future data for selected aircraft
                    futureDataPoints = this.store.aircraftDetailsFutureData
                        .filter(position => position.lat && position.lon)
                        .map(position => [position.lat, position.lon]);
                } else if (aircraftData.status === 'active' && aircraftData.adsb) {
                    // Generate simple future trajectory for active aircraft based on current vector
                    futureDataPoints = this.generateFutureTrajectory(aircraftData);
                }
                
                if (futureDataPoints.length > 0) {
                    // Create a polyline for the future trajectory that starts from the current aircraft position
                    const currentPosition = [aircraftData.adsb.lat, aircraftData.adsb.lon];
                    const futurePoints = [currentPosition, ...futureDataPoints];
                    
                    //console.log(`Future trail for ${hex}: ${futurePoints.length} points, aircraft status: ${aircraftData.status}`);
                    
                    // Create the main future trajectory polyline only if we have at least 2 points
                    if (futurePoints.length >= 2) {
                        const futurePolyline = this.L.polyline(futurePoints, {
                            color: this.getAircraftColor(hex),
                            weight: 2,
                            opacity: isSelectedAircraft ? 0.5 : 0.3, // Lower opacity for non-selected aircraft
                            lineJoin: 'round',
                            className: 'future-trail',
                            dashArray: '3, 7', // Different dash pattern than history
                            dashOffset: '0',
                            aircraftHex: hex
                        });
                        futurePolyline.addTo(this.layers.trails);
                    }
                    
                    // Only add detailed markers for selected aircraft to avoid clutter
                    if (isSelectedAircraft && this.store.aircraftDetailsFutureData) {
                        // Add markers at each future position
                        this.store.aircraftDetailsFutureData.forEach((position, index) => {
                            const posLatLng = [position.lat, position.lon];
                            
                            // Format altitude for display
                            const altitude = position.altitude !== undefined ? position.altitude : 'N/A';
                            const minutesAhead = index + 1;
                            
                            // Create future position marker with altitude label
                            const altText = altitude === 0 ? 'GND' :
                                           altitude !== 'N/A' ? Math.round(altitude/100)*100 : 'N/A';
                            
                            const futureMarker = this.L.circleMarker(posLatLng, {
                                radius: 3,
                                color: '#888888',
                                fillColor: '#888888',
                                fillOpacity: 0.3,
                                opacity: 0.3,
                                weight: 1,
                                className: 'future-point-marker',
                                pane: 'shadowPane',
                                aircraftHex: hex
                            });
                            futureMarker.addTo(this.layers.trails);
                            
                            // Add altitude label next to the future marker in grey
                            const futureLabel = this.L.marker(posLatLng, {
                                icon: this.L.divIcon({
                                    html: `<div style="color: #888888; font-size: 10px; opacity: 0.6;">+${minutesAhead}m: ${altText}</div>`,
                                    className: 'altitude-label-container',
                                    iconSize: [60, 20],
                                    iconAnchor: [-5, 0] // Position to the right of the point
                                }),
                                aircraftHex: hex
                            });
                            futureLabel.addTo(this.layers.trails);
                        });
                        
                        // Add a special marker for the last future position
                        if (this.store.aircraftDetailsFutureData.length > 0) {
                            const lastPos = this.store.aircraftDetailsFutureData[this.store.aircraftDetailsFutureData.length - 1];
                            const lastPosLatLng = [lastPos.lat, lastPos.lon];
                            
                            const lastFutureMarker = this.L.circleMarker(lastPosLatLng, {
                                radius: 4,
                                color: '#888888',
                                fillColor: '#888888',
                                fillOpacity: 0.4,
                                opacity: 0.4,
                                weight: 1,
                                className: 'future-point-marker-last',
                                aircraftHex: hex
                            });
                            lastFutureMarker.addTo(this.layers.trails);
                        }
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

    updateVisualState(hex, forceImmediate = false) {
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

        if (isFadedDueToOtherSelection) {
            targetOpacity = this.CONFIG.selectedFadeOpacity;
        } else {
            if (isActuallySelected || isHovered || isInProximity) {
                targetOpacity = 1.0;
            } else {
                switch (aircraftData.status) {
                    case 'stale': targetOpacity = 0.7; break;
                    case 'signal_lost': targetOpacity = 0.5; break;
                    default: targetOpacity = 1.0;
                }
            }
        }

        // Only apply green highlighting to selected/hovered aircraft that are NOT in proximity
        const isHighlightedForStyle = (isActuallySelected || isHovered) && !isInProximity;

        const checkAircraftIcon = (apply = false) => {
            // Removed hover color changes - aircraft icons maintain their original colors
            return true;
        };
        this._updateElementStyle(markers.aircraft.getElement(), {}, {}, checkAircraftIcon, forceImmediate);
        
        const checkLabelStyle = (apply = false) => {
            const element = markers.label.getElement();
            const labelDiv = element?.querySelector('div') || element;
            
            if (labelDiv) {
                if (apply) {
                    if (this.proximityHexSet && this.proximityHexSet.has(hex)) {
                        // Proximity aircraft - use proximity styling
                        labelDiv.classList.remove('aircraft-label-table-hover');
                        if (!labelDiv.classList.contains('proximity-highlight')) {
                            labelDiv.classList.add('proximity-highlight');
                        }
                        // Remove inline styles when using proximity styling
                        labelDiv.style.backgroundColor = '';
                        labelDiv.style.borderColor = '';
                    } else if (isHighlightedForStyle) {
                        // Add subtle table hover effect - background and border
                        labelDiv.classList.add('aircraft-label-table-hover');
                        // Apply inline CSS for reliable visual effect
                        labelDiv.style.backgroundColor = 'rgba(76, 175, 80, 0.1)';
                        labelDiv.style.borderColor = 'rgba(76, 175, 80, 0.3)';
                        
                        // Also apply hover effect to corresponding flight card in table
                        this._updateFlightCardHover(hex, true);
                    } else {
                        // Remove table hover effect
                        labelDiv.classList.remove('aircraft-label-table-hover');
                        // Remove inline styles
                        labelDiv.style.backgroundColor = '';
                        labelDiv.style.borderColor = '';
                        
                        // Also remove hover effect from corresponding flight card in table
                        this._updateFlightCardHover(hex, false);
                    }
                    return true; // Changes applied
                } else {
                    // Just checking if changes are needed - return false to indicate changes are needed
                    return false;
                }
            }
            return true; // No element found, no changes needed
        };
        this._updateElementStyle(markers.label.getElement(), {}, {}, checkLabelStyle, forceImmediate);
        
        markers.aircraft.setOpacity(targetOpacity);
        if (markers.label && markers.label.setOpacity) {
            markers.label.setOpacity(targetOpacity);
        }
    }
    
    _updateElementStyle(element, highlightedStyle, normalStyle, checkFn, forceImmediate) {
        // Simplified style updater based on the original logic
        if (checkFn()) return true;
        if (forceImmediate) {
             if (checkFn(true)) return true;
        }
        // Remove delay for instant hover response
        checkFn(true);
    }

    // Helper function to update flight card hover effect in table
    _updateFlightCardHover(hex, isHovered) {
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
        const searchLower = this.store.searchTerm.toLowerCase();
        
        // Get current map bounds for viewport culling
        const mapBounds = this.map ? this.map.getBounds() : null;
        const currentZoom = this.map ? this.map.getZoom() : 10;
        
        // Enable viewport culling when zoomed in (zoom level > 11)
        const useViewportCulling = currentZoom > 11 && mapBounds;

        Object.keys(this.store.aircraft).forEach(hex => {
            const aircraft = this.store.aircraft[hex];
            const markerInfo = this.markers[hex];

            if (!markerInfo) return;

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
            
            // Aircraft should be visible if it matches filters AND is in viewport OR if it's selected
            const shouldBeVisibleOnMap = ((matchesSearch && isVisibleByGroundState && isVisibleByAltitude && isVisibleByPhase && isInViewport) || isSelectedAircraft);

            const markerIsOnMap = this.layers.aircraft.hasLayer(markerInfo.aircraft);
            const labelIsOnMap = markerInfo.label && this.layers.aircraft.hasLayer(markerInfo.label);

            if (shouldBeVisibleOnMap && !markerIsOnMap) {
                markerInfo.aircraft.addTo(this.layers.aircraft);
                
                // CRITICAL FIX: Apply heading rotation when aircraft becomes visible again
                const heading = this.store.getHeadingWithFallback(aircraft);
                setTimeout(() => {
                    const markerElement = markerInfo.aircraft.getElement();
                    if (markerElement) {
                        const iconContainer = markerElement.querySelector('.aircraft-icon-container');
                        if (iconContainer) {
                            iconContainer.style.transform = `rotate(${heading}deg)`;
                            iconContainer.style.transformOrigin = 'center';
                            iconContainer.style.transition = 'transform 0.2s ease';
                        }
                    }
                }, 0);
            } else if (!shouldBeVisibleOnMap && markerIsOnMap) {
                this.layers.aircraft.removeLayer(markerInfo.aircraft);
            }

            // Labels should be visible if labels are enabled AND (aircraft matches filters OR is selected)
            const shouldLabelBeVisible = this.store.settings.showLabels && shouldBeVisibleOnMap;
            if (markerInfo.label) {
                if (shouldLabelBeVisible && !labelIsOnMap) {
                    markerInfo.label.addTo(this.layers.aircraft);
                } else if (!shouldLabelBeVisible && labelIsOnMap) {
                    this.layers.aircraft.removeLayer(markerInfo.label);
                }
            }
            
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
        
        // Ensure leaflet objects exist
        this._ensureLeafletObjects(aircraft);
        
        // Update marker position and visual state
        const markerInfo = this.markers[hex];
        if (!markerInfo) return;
        
        // Update marker position if it exists
        if (aircraft.adsb && aircraft.adsb.lat && aircraft.adsb.lon) {
            const newPos = [aircraft.adsb.lat, aircraft.adsb.lon];
            markerInfo.aircraft.setLatLng(newPos);
        }
        
        // Update visual state (color, rotation, etc.)
        this.updateVisualState(hex);
        
        // Update label content with latest data
        if (markerInfo.label && this.store.settings.showLabels) {
            const callsign = aircraft.flight || aircraft.hex;
            const altitude = aircraft.adsb ? aircraft.adsb.alt_baro : 0;
            const verticalTrend = this.getVerticalTrend(aircraft);
            const labelContent = this.store.createLabelContent(aircraft, callsign, altitude, verticalTrend);
            markerInfo.label.setTooltipContent(labelContent);
        }
        
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
        
        const markerIsOnMap = this.layers.aircraft.hasLayer(markerInfo.aircraft);
        const labelIsOnMap = markerInfo.label && this.layers.aircraft.hasLayer(markerInfo.label);
        
        // Update marker visibility
        if (shouldBeVisibleOnMap && !markerIsOnMap) {
            markerInfo.aircraft.addTo(this.layers.aircraft);
        } else if (!shouldBeVisibleOnMap && markerIsOnMap) {
            this.layers.aircraft.removeLayer(markerInfo.aircraft);
        }
        
        // Update label visibility
        const shouldLabelBeVisible = this.store.settings.showLabels && shouldBeVisibleOnMap;
        if (markerInfo.label) {
            if (shouldLabelBeVisible && !labelIsOnMap) {
                markerInfo.label.addTo(this.layers.aircraft);
            } else if (!shouldLabelBeVisible && labelIsOnMap) {
                this.layers.aircraft.removeLayer(markerInfo.label);
            }
        }
    }

    // Centralized place for map-related updates when filters change
    applyFiltersAndRefreshView() {
        this.updateMapMarkerAndLabelVisibility();
        this.updateFlightPaths();
        this.updateVisibleAircraftList();
        
        // Update proximity circle if active
        this.updateProximityCircle();
        
        if (this.map) {
            this.map.invalidateSize(false); // Invalidate map size in case of layout changes
        }
    }
    
    // Update list of aircraft visible on map for UI indicators
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
        
        // Update store with visible aircraft list
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
            }
        });
    }

    // Helper to remove a single aircraft marker
    removeAircraft(hex) {
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
            className: 'proximity-circle-normal', // Always use normal style (no animation)
            pane: 'overlayPane', // Use the overlay pane which is below markers
            interactive: false // Make sure the circle doesn't interfere with clicks
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
                        
                        // Add hover event listeners
                        labelDiv.addEventListener('mouseenter', () => {
                            labelDiv.classList.add('proximity-highlight-hover');
                        });
                        
                        labelDiv.addEventListener('mouseleave', () => {
                            labelDiv.classList.remove('proximity-highlight-hover');
                        });
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
                        
                        // Remove event listeners (clone and replace the element)
                        const newLabelDiv = labelDiv.cloneNode(true);
                        labelDiv.parentNode.replaceChild(newLabelDiv, labelDiv);
                        
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
        // Safety check: ensure map is initialized before drawing runways
        if (!this.map) {
            console.warn('MapManager: Cannot draw runways - map not initialized yet');
            return;
        }
        
        if (!runwayData || !runwayData.runway_thresholds || !runwayData.runway_extensions) {
            console.warn("No runway data available");
            return;
        }

        // Store runway data for zoom-aware rendering
        this.runwayData = runwayData;
        
        // Initial render
        this.updateRunwayDisplay();
        
        // Add zoom event listener for dynamic detail adjustment (only once)
        if (!this.runwayZoomListener) {
            this.runwayZoomListener = () => {
                // Debounce runway updates to prevent excessive redraws during zoom
                clearTimeout(this.runwayUpdateTimeout);
                this.runwayUpdateTimeout = setTimeout(() => {
                    this.updateRunwayDisplay();
                }, 150);
            };
            this.map.on('zoomend', this.runwayZoomListener);
        }
    }

    updateRunwayDisplay() {
        if (!this.map || !this.runwayData) return;
        
        // Clear existing runway layers
        this.layers.runways.clearLayers();
        
        const currentZoom = this.map.getZoom();
        const showExtensions = currentZoom >= 11; // Only show extensions when zoomed in
        const showDistanceMarkers = currentZoom >= 13; // Only show distance markers when very zoomed in
        
        // Draw each runway with zoom-appropriate detail level
        for (const runwayId in this.runwayData.runway_thresholds) {
            const thresholds = this.runwayData.runway_thresholds[runwayId];
            const extensions = this.runwayData.runway_extensions[runwayId];
            
            // Get the two ends of the runway
            const ends = Object.keys(thresholds);
            if (ends.length !== 2) continue;
            
            const end1 = ends[0];
            const end2 = ends[1];
            
            // Always draw the runway itself (as a thick line)
            const runwayCoords = [
                [thresholds[end1].latitude, thresholds[end1].longitude],
                [thresholds[end2].latitude, thresholds[end2].longitude]
            ];
            
            this.L.polyline(runwayCoords, {
                color: '#FFFFFF',
                weight: Math.max(4, Math.min(8, currentZoom - 6)), // Scale line weight with zoom
                opacity: 0.8,
                className: 'runway-line'
            }).addTo(this.layers.runways);
            
            // Only show runway labels when zoomed in enough
            if (currentZoom >= 10) {
                this.L.marker([thresholds[end1].latitude, thresholds[end1].longitude], {
                    icon: this.L.divIcon({
                        html: `<div class="runway-label">${end1}</div>`,
                        className: 'runway-label-container',
                        iconSize: [30, 20]
                    })
                }).addTo(this.layers.runways);
                
                this.L.marker([thresholds[end2].latitude, thresholds[end2].longitude], {
                    icon: this.L.divIcon({
                        html: `<div class="runway-label">${end2}</div>`,
                        className: 'runway-label-container',
                        iconSize: [30, 20]
                    })
                }).addTo(this.layers.runways);
            }
            
            // Only draw extensions when zoomed in enough
            if (showExtensions && extensions) {
                // Draw extension for end1
                if (extensions[end1] && extensions[end1].length > 0) {
                    const extensionCoords = extensions[end1].map(point =>
                        [point.latitude, point.longitude]
                    );
                    
                    this.L.polyline(extensionCoords, {
                        color: '#76C76C',
                        weight: 1.5,
                        opacity: 0.4,
                        dashArray: '8, 12',
                        className: 'runway-extension-line'
                    }).addTo(this.layers.runways);
                    
                    // Only add distance markers when very zoomed in
                    if (showDistanceMarkers) {
                        extensions[end1].forEach(point => {
                            if (point.distance > 0 && point.distance % 2 === 0) { // Skip threshold and odd distances
                                this.L.circleMarker([point.latitude, point.longitude], {
                                    radius: 1.5,
                                    color: '#76C76C',
                                    fillColor: '#76C76C',
                                    fillOpacity: 0.6,
                                    opacity: 0.6,
                                    weight: 1,
                                    className: 'runway-distance-marker'
                                }).addTo(this.layers.runways);
                                
                                // Add distance label
                                this.L.marker([point.latitude, point.longitude], {
                                    icon: this.L.divIcon({
                                        html: `<div class="runway-distance-label">${point.distance}</div>`,
                                        className: 'runway-distance-label-container',
                                        iconSize: [20, 16],
                                        iconAnchor: [10, -5] // Position above the point
                                    })
                                }).addTo(this.layers.runways);
                            }
                        });
                    }
                }
                
                // Draw extension for end2
                if (extensions[end2] && extensions[end2].length > 0) {
                    const extensionCoords = extensions[end2].map(point =>
                        [point.latitude, point.longitude]
                    );
                    
                    this.L.polyline(extensionCoords, {
                        color: '#76C76C',
                        weight: 1.5,
                        opacity: 0.4,
                        dashArray: '8, 12',
                        className: 'runway-extension-line'
                    }).addTo(this.layers.runways);
                    
                    // Only add distance markers when very zoomed in
                    if (showDistanceMarkers) {
                        extensions[end2].forEach(point => {
                            if (point.distance > 0 && point.distance % 2 === 0) { // Skip threshold and odd distances
                                this.L.circleMarker([point.latitude, point.longitude], {
                                    radius: 1.5,
                                    color: '#76C76C',
                                    fillColor: '#76C76C',
                                    fillOpacity: 0.6,
                                    opacity: 0.6,
                                    weight: 1,
                                    className: 'runway-distance-marker'
                                }).addTo(this.layers.runways);
                                
                                // Add distance label
                                this.L.marker([point.latitude, point.longitude], {
                                    icon: this.L.divIcon({
                                        html: `<div class="runway-distance-label">${point.distance}</div>`,
                                        className: 'runway-distance-label-container',
                                        iconSize: [20, 16],
                                        iconAnchor: [10, -5] // Position above the point
                                    })
                                }).addTo(this.layers.runways);
                            }
                        });
                    }
                }
            }
        }
    }

    // Clean up runway rendering resources
    cleanupRunwayRendering() {
        // Clear any pending runway update timeout
        if (this.runwayUpdateTimeout) {
            clearTimeout(this.runwayUpdateTimeout);
            this.runwayUpdateTimeout = null;
        }
        
        // Remove zoom event listener
        if (this.map && this.runwayZoomListener) {
            this.map.off('zoomend', this.runwayZoomListener);
            this.runwayZoomListener = null;
        }
        
        // Clear runway data
        this.runwayData = null;
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
        
        console.log(`Shockwave animation for ${hex}: phase=${phase}, color=${color}`);
        
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
                    className: 'takeoff-landing-pulse'
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
            'DEP': '#4ADE80',    // green-400
            'CRZ': '#60A5FA',    // blue-400
            'ARR': '#F87171',    // red-400
            'APP': '#FACC15',    // yellow-400
            'T/D': '#2DD4BF'     // teal-400
        };
        return phaseColorMap[phase] || '#9CA3AF'; // Default to gray
    }
}

// Export the MapManager class if using modules, or attach to window for global access
window.MapManager = MapManager;