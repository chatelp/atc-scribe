// Aircraft Smooth Animation Engine
// Provides vector extrapolation and smooth interpolation for aircraft movement
// Uses requestAnimationFrame for 60 FPS visual smoothness

class AircraftAnimationEngine {
    constructor(mapManager, store) {
        this.mapManager = mapManager;
        this.store = store;

        // Animation configuration
        this.config = {
            enabled: true,
            interpolationFps: 30,                    // Visual update rate (30fps is smooth enough)
            physicsUpdateRate: 10,                   // Physics/velocity calculations per second
            maxExtrapolationSeconds: 2.4,            // 20% beyond 2s update interval
            confidenceDecayRate: 0.5,                // Exponential decay factor
            minConfidenceThreshold: 0.3,             // Stop animating below this
            viewportCulling: true,                   // Only animate visible aircraft
            adaptivePerformance: true,               // Reduce quality under load
            maxProcessingTimeMs: 30,                 // 30ms frame budget for 30fps
            maxHistoryPoints: 5,                     // Position history per aircraft
            enableCurvedInterpolation: true,         // Use track rate for turning aircraft
            enableAltitudeInterpolation: true,       // Interpolate altitude changes
            distanceQualityReduction: 50,            // Reduce quality beyond this distance (NM)
            cleanupIntervalMs: 30000,                // Cleanup stale states every 30s
            useCssTransforms: false                  // Use Leaflet setLatLng (more reliable)
        };

        // Animation state
        this.rafId = null;                           // requestAnimationFrame ID
        this.aircraftStates = new Map();             // hex -> AircraftState
        this.lastAnimationTime = 0;
        this.lastPhysicsTime = 0;
        this.lastCleanupTime = 0;
        this.frameTimeHistory = [];
        this.qualityLevel = 1.0;                     // 1.0 = full quality, 0.3 = minimum
        this.isRunning = false;

        // Pending marker updates for batching
        this.pendingMarkerUpdates = new Map();
        this.markerUpdateScheduled = false;

        // Performance monitoring
        this.performanceMonitor = new PerformanceMonitor();

        // Bind methods
        this.animationFrame = this.animationFrame.bind(this);
        this.rafLoop = this.rafLoop.bind(this);
    }
    
    // Initialize the animation engine
    initialize() {
        console.log('Aircraft Animation Engine: Initializing...');
        
        // Load configuration from store if available
        if (this.store.settings && this.store.settings.aircraftAnimation) {
            Object.assign(this.config, this.store.settings.aircraftAnimation);
        }
        
        // Start animation if enabled
        if (this.config.enabled) {
            this.start();
        }
        
        console.log('Aircraft Animation Engine: Initialized with config:', this.config);
    }
    
    // Start the animation engine
    start() {
        if (this.isRunning) {
            console.warn('Aircraft Animation Engine: Already running');
            return;
        }

        console.log('Aircraft Animation Engine: Starting with requestAnimationFrame...');
        this.isRunning = true;
        this.lastAnimationTime = performance.now();
        this.lastPhysicsTime = performance.now();
        this.lastCleanupTime = Date.now();

        // Start RAF loop
        this.rafLoop();
    }

    // Stop the animation engine
    stop() {
        if (!this.isRunning) {
            return;
        }

        console.log('Aircraft Animation Engine: Stopping...');
        this.isRunning = false;

        if (this.rafId) {
            cancelAnimationFrame(this.rafId);
            this.rafId = null;
        }

        // Reset CSS transforms on all markers before clearing states
        this.resetAllMarkerTransforms();

        // Clear all aircraft states
        this.aircraftStates.clear();
        this.pendingMarkerUpdates.clear();
    }

    // Reset CSS transforms on all markers
    resetAllMarkerTransforms() {
        if (!this.mapManager || !this.mapManager.markers) return;

        Object.keys(this.mapManager.markers).forEach(hex => {
            const markers = this.mapManager.markers[hex];
            if (markers?.aircraft) {
                const element = markers.aircraft.getElement();
                if (element) {
                    element.style.transform = '';
                    element.style.transition = '';
                }
            }
            if (markers?.label) {
                const element = markers.label.getElement();
                if (element) {
                    element.style.transform = '';
                    element.style.transition = '';
                }
            }
        });
    }

    // Main RAF loop - throttled to configured FPS
    rafLoop() {
        if (!this.isRunning) return;

        const now = performance.now();
        const deltaTime = now - this.lastAnimationTime;
        const targetFrameTime = 1000 / this.config.interpolationFps;

        // Throttle to configured FPS to reduce CPU usage
        if (deltaTime >= targetFrameTime) {
            // Run animation frame logic
            this.animationFrame(now, deltaTime);
            this.lastAnimationTime = now;
        }

        // Schedule next frame
        this.rafId = requestAnimationFrame(this.rafLoop);
    }
    
    // Main animation frame processing (called by RAF loop)
    animationFrame(now, deltaTime) {
        if (!this.isRunning || !this.config.enabled) {
            return;
        }

        const startTime = performance.now();
        const currentTime = Date.now();

        try {
            // Periodic cleanup of stale states (every 30s)
            if (currentTime - this.lastCleanupTime > this.config.cleanupIntervalMs) {
                this.cleanupStaleStates();
                this.lastCleanupTime = currentTime;
            }

            // Get visible aircraft for viewport culling
            const visibleAircraft = this.config.viewportCulling ?
                this.getVisibleAircraft() :
                Object.values(this.store.aircraft || {});

            // Update interpolated positions for all visible aircraft
            let updatedCount = 0;
            for (const aircraft of visibleAircraft) {
                if (this.updateAircraftPosition(aircraft, currentTime, deltaTime)) {
                    updatedCount++;
                }

                // Check processing time limit
                if (performance.now() - startTime > this.config.maxProcessingTimeMs) {
                    break;
                }
            }

            // Process batched marker updates
            this.processPendingMarkerUpdates();

            // Performance monitoring
            const processingTime = performance.now() - startTime;
            this.performanceMonitor.recordFrameTime(processingTime);

            // Adaptive performance adjustment
            if (this.config.adaptivePerformance) {
                this.qualityLevel = this.performanceMonitor.getQualityLevel();
            }

        } catch (error) {
            console.error('Aircraft Animation: Error in animation frame:', error);
        }
    }

    // Cleanup animation states for aircraft no longer in the store
    cleanupStaleStates() {
        const storeHexes = new Set(Object.keys(this.store.aircraft || {}));
        let removedCount = 0;

        for (const hex of this.aircraftStates.keys()) {
            if (!storeHexes.has(hex)) {
                this.aircraftStates.delete(hex);
                removedCount++;
            }
        }

        if (removedCount > 0) {
            console.log(`Aircraft Animation: Cleaned up ${removedCount} stale states`);
        }
    }

    // Queue marker update for batch processing
    queueMarkerUpdate(hex, position) {
        this.pendingMarkerUpdates.set(hex, position);
    }

    // Process all pending marker updates in a batch
    processPendingMarkerUpdates() {
        if (this.pendingMarkerUpdates.size === 0) return;

        const updates = Array.from(this.pendingMarkerUpdates.entries());
        this.pendingMarkerUpdates.clear();

        // Batch DOM updates
        for (const [hex, position] of updates) {
            this.applyMarkerTransform(hex, position);
        }
    }

    // Apply CSS transform to marker for smooth interpolated movement
    applyMarkerTransform(hex, position) {
        if (!this.mapManager || !this.mapManager.markers || !this.mapManager.markers[hex]) {
            return;
        }

        const markers = this.mapManager.markers[hex];
        if (!markers.aircraft) return;

        // Get the base Leaflet position (from last real data update)
        const baseLatLng = markers.aircraft.getLatLng();
        if (!baseLatLng) return;

        // Convert positions to container pixels for transform calculation
        const map = this.mapManager.map;
        if (!map) return;

        const basePoint = map.latLngToContainerPoint(baseLatLng);
        const targetPoint = map.latLngToContainerPoint([position.lat, position.lon]);

        // Calculate pixel offset
        const dx = targetPoint.x - basePoint.x;
        const dy = targetPoint.y - basePoint.y;

        // Skip if movement is negligible
        if (Math.abs(dx) < 0.5 && Math.abs(dy) < 0.5) return;

        // Apply CSS transform to aircraft marker
        const aircraftElement = markers.aircraft.getElement();
        if (aircraftElement) {
            // Preserve existing rotation transform, add translation
            const iconContainer = aircraftElement.querySelector('.aircraft-icon-container');
            if (iconContainer) {
                const currentRotation = iconContainer.style.transform.match(/rotate\([^)]+\)/)?.[0] || '';
                iconContainer.style.transform = `translate(${dx}px, ${dy}px) ${currentRotation}`;
            } else {
                aircraftElement.style.transform = `translate(${dx}px, ${dy}px)`;
            }
        }

        // Apply same transform to label
        if (markers.label) {
            const labelElement = markers.label.getElement();
            if (labelElement) {
                labelElement.style.transform = `translate(${dx}px, ${dy}px)`;
            }
        }
    }

    // Reset transform for a single marker (called when real data arrives)
    resetMarkerTransform(hex) {
        if (!this.mapManager || !this.mapManager.markers || !this.mapManager.markers[hex]) {
            return;
        }

        const markers = this.mapManager.markers[hex];

        if (markers.aircraft) {
            const element = markers.aircraft.getElement();
            if (element) {
                const iconContainer = element.querySelector('.aircraft-icon-container');
                if (iconContainer) {
                    // Preserve rotation, remove translation
                    const rotation = iconContainer.style.transform.match(/rotate\([^)]+\)/)?.[0] || '';
                    iconContainer.style.transform = rotation;
                } else {
                    element.style.transform = '';
                }
            }
        }

        if (markers.label) {
            const element = markers.label.getElement();
            if (element) {
                element.style.transform = '';
            }
        }
    }
    
    // Update aircraft when new data arrives
    updateAircraft(aircraft) {
        if (!this.config.enabled || !aircraft || !aircraft.hex) {
            return;
        }

        const hex = aircraft.hex;

        // Remove signal_lost aircraft from animation tracking
        if (aircraft.status === 'signal_lost') {
            this.aircraftStates.delete(hex);
            this.resetMarkerTransform(hex);
            return;
        }

        let state = this.aircraftStates.get(hex);

        if (!state) {
            state = new AircraftState(hex);
            this.aircraftStates.set(hex, state);
        }

        // Add new position to history
        if (aircraft.adsb && aircraft.adsb.lat && aircraft.adsb.lon) {
            const position = {
                lat: aircraft.adsb.lat,
                lon: aircraft.adsb.lon,
                alt: aircraft.adsb.alt_baro || 0,
                groundSpeed: aircraft.adsb.gs || 0,
                track: aircraft.adsb.track || 0,
                verticalRate: aircraft.adsb.baro_rate || 0,
                trackRate: aircraft.adsb.track_rate || 0
            };

            state.addPosition(position, Date.now());

            // Update aircraft reference
            state.aircraft = aircraft;

            // Reset CSS transform since Leaflet will update the actual position
            if (this.config.useCssTransforms) {
                this.resetMarkerTransform(hex);
            }
        }
    }

    // Update aircraft with delta (only changed fields) - more efficient than full update
    updateAircraftDelta(hex, delta) {
        if (!this.config.enabled || !hex || !delta) {
            return;
        }

        // Handle status change to signal_lost
        if (delta.status === 'signal_lost') {
            this.aircraftStates.delete(hex);
            this.resetMarkerTransform(hex);
            return;
        }

        const state = this.aircraftStates.get(hex);
        if (!state) {
            // No existing state - need full aircraft data first
            return;
        }

        // Check if position changed
        const hasPositionChange = delta.lat !== undefined || delta.lon !== undefined;

        if (hasPositionChange) {
            // Build position object from delta and existing state
            const currentAircraft = state.aircraft;
            const currentAdsb = currentAircraft?.adsb || {};

            const position = {
                lat: delta.lat !== undefined ? delta.lat : currentAdsb.lat,
                lon: delta.lon !== undefined ? delta.lon : currentAdsb.lon,
                alt: delta.alt_baro !== undefined ? delta.alt_baro : (currentAdsb.alt_baro || 0),
                groundSpeed: delta.gs !== undefined ? delta.gs : (currentAdsb.gs || 0),
                track: delta.track !== undefined ? delta.track : (currentAdsb.track || 0),
                verticalRate: delta.baro_rate !== undefined ? delta.baro_rate : (currentAdsb.baro_rate || 0),
                trackRate: currentAdsb.track_rate || 0
            };

            state.addPosition(position, Date.now());

            // Reset CSS transform since Leaflet will update the actual position
            if (this.config.useCssTransforms) {
                this.resetMarkerTransform(hex);
            }
        }

        // Update the aircraft reference with delta values
        if (state.aircraft) {
            if (!state.aircraft.adsb) state.aircraft.adsb = {};

            if (delta.lat !== undefined) state.aircraft.adsb.lat = delta.lat;
            if (delta.lon !== undefined) state.aircraft.adsb.lon = delta.lon;
            if (delta.alt_baro !== undefined) state.aircraft.adsb.alt_baro = delta.alt_baro;
            if (delta.track !== undefined) state.aircraft.adsb.track = delta.track;
            if (delta.gs !== undefined) state.aircraft.adsb.gs = delta.gs;
            if (delta.baro_rate !== undefined) state.aircraft.adsb.baro_rate = delta.baro_rate;
            if (delta.status !== undefined) state.aircraft.status = delta.status;
            if (delta.on_ground !== undefined) state.aircraft.on_ground = delta.on_ground;
        }
    }

    // Remove aircraft from animation
    removeAircraft(hex) {
        this.aircraftStates.delete(hex);
    }
    
    // Update single aircraft position with interpolation
    updateAircraftPosition(aircraft, currentTime, deltaTime) {
        if (!aircraft || !aircraft.hex) {
            return false;
        }

        // Don't animate signal_lost aircraft - keep them static
        if (aircraft.status === 'signal_lost') {
            return false;
        }

        const state = this.aircraftStates.get(aircraft.hex);
        if (!state || !state.hasValidVelocity()) {
            return false;
        }

        // Calculate elapsed time since last server update
        const elapsedTime = (currentTime - state.lastUpdateTime) / 1000; // seconds

        // Don't extrapolate beyond configured limit
        if (elapsedTime > this.config.maxExtrapolationSeconds) {
            return false;
        }

        // Calculate interpolated position
        const interpolatedPosition = this.interpolatePosition(state, elapsedTime);

        if (!interpolatedPosition || interpolatedPosition.confidence < this.config.minConfidenceThreshold) {
            return false;
        }

        // Queue marker update for batch processing (uses CSS transforms)
        if (this.config.useCssTransforms) {
            this.queueMarkerUpdate(aircraft.hex, interpolatedPosition);
        } else {
            // Fallback: direct Leaflet update
            this.updateMapMarker(aircraft.hex, interpolatedPosition);
        }

        return true;
    }
    
    // Calculate interpolated position based on velocity vector
    interpolatePosition(state, elapsedTime) {
        const vector = state.velocityVector;
        if (!vector) {
            return null;
        }
        
        // Apply quality level adjustment
        const adjustedElapsedTime = elapsedTime * this.qualityLevel;
        
        // Calculate time factor with extrapolation limit
        const timeFactor = Math.min(adjustedElapsedTime, this.config.maxExtrapolationSeconds);
        
        // Apply confidence decay for older predictions
        const confidence = vector.confidence * Math.exp(-timeFactor * this.config.confidenceDecayRate);
        
        if (confidence < this.config.minConfidenceThreshold) {
            return null;
        }
        
        // Calculate position delta using velocity vector
        const deltaLat = (vector.vy * timeFactor) / 111320; // meters to degrees
        const deltaLon = (vector.vx * timeFactor) / (111320 * Math.cos(state.lastKnownPosition.lat * Math.PI / 180));
        
        let adjustedDeltaLat = deltaLat;
        let adjustedDeltaLon = deltaLon;
        
        // Apply curved interpolation for turning aircraft
        if (this.config.enableCurvedInterpolation && state.lastKnownPosition.trackRate && Math.abs(state.lastKnownPosition.trackRate) > 1) {
            const turnAdjustment = this.calculateTurnAdjustment(vector, state.lastKnownPosition.trackRate, timeFactor);
            adjustedDeltaLat += turnAdjustment.lat;
            adjustedDeltaLon += turnAdjustment.lon;
        }
        
        // Calculate altitude interpolation
        let interpolatedAlt = state.lastKnownPosition.alt;
        if (this.config.enableAltitudeInterpolation && vector.vz) {
            interpolatedAlt += vector.vz * timeFactor;
        }
        
        const newLat = state.lastKnownPosition.lat + adjustedDeltaLat;
        const newLon = state.lastKnownPosition.lon + adjustedDeltaLon;

        // Guard against NaN from incomplete ADSB data (e.g. missing lon or velocity)
        if (!Number.isFinite(newLat) || !Number.isFinite(newLon)) {
            return null;
        }

        return {
            lat: newLat,
            lon: newLon,
            alt: interpolatedAlt,
            confidence: confidence,
            interpolated: true,
            elapsedTime: elapsedTime
        };
    }
    
    // Calculate turn adjustment for curved interpolation
    calculateTurnAdjustment(vector, trackRate, timeFactor) {
        // Simple turn radius calculation
        const speed = Math.sqrt(vector.vx * vector.vx + vector.vy * vector.vy); // m/s
        if (speed < 1) return { lat: 0, lon: 0 }; // Too slow to calculate meaningful turn
        
        const turnRateRad = trackRate * Math.PI / 180; // degrees/s to radians/s
        const turnRadius = speed / Math.abs(turnRateRad); // meters
        
        // Calculate arc displacement
        const arcAngle = turnRateRad * timeFactor;
        const arcDisplacement = turnRadius * Math.sin(Math.abs(arcAngle));
        
        // Apply turn direction
        const turnDirection = trackRate > 0 ? 1 : -1;
        const perpAngle = Math.atan2(vector.vy, vector.vx) + (Math.PI / 2) * turnDirection;
        
        const turnDeltaX = arcDisplacement * Math.cos(perpAngle) * 0.1; // Reduced factor for subtle effect
        const turnDeltaY = arcDisplacement * Math.sin(perpAngle) * 0.1;
        
        return {
            lat: turnDeltaY / 111320,
            lon: turnDeltaX / (111320 * Math.cos(vector.lat || 0))
        };
    }
    
    // Update map marker with interpolated position
    updateMapMarker(hex, position) {
        if (!this.mapManager || !this.mapManager.markers || !this.mapManager.markers[hex]) {
            return;
        }
        
        const markers = this.mapManager.markers[hex];
        const newLatLng = [position.lat, position.lon];
        
        // Update aircraft marker position
        if (markers.aircraft) {
            markers.aircraft.setLatLng(newLatLng);
        }
        
        // Update label position
        if (markers.label) {
            markers.label.setLatLng(newLatLng);
        }
        
        // Store interpolated position for debugging
        if (markers.aircraft) {
            markers.aircraft._interpolatedPosition = position;
        }
    }
    
    // Get aircraft visible in current viewport
    getVisibleAircraft() {
        if (!this.mapManager || !this.mapManager.map) {
            return Object.values(this.store.aircraft || {});
        }
        
        const bounds = this.mapManager.map.getBounds();
        const currentZoom = this.mapManager.map.getZoom();
        const visibleAircraft = [];
        
        // Only use viewport culling when zoomed in (zoom level > 11)
        const useViewportCulling = currentZoom > 11;
        
        for (const aircraft of Object.values(this.store.aircraft || {})) {
            // Always include selected aircraft regardless of viewport
            if (this.store.selectedAircraft && this.store.selectedAircraft.hex === aircraft.hex) {
                visibleAircraft.push(aircraft);
                continue;
            }
            
            // Skip signal_lost aircraft for animation (they should remain static)
            if (aircraft.status === 'signal_lost') {
                continue;
            }
            
            if (aircraft.adsb && aircraft.adsb.lat && aircraft.adsb.lon) {
                const position = [aircraft.adsb.lat, aircraft.adsb.lon];
                
                if (!useViewportCulling || bounds.contains(position)) {
                    visibleAircraft.push(aircraft);
                }
            }
        }
        
        return visibleAircraft;
    }
    
    // Get animation statistics
    getStats() {
        return {
            isRunning: this.isRunning,
            aircraftCount: this.aircraftStates.size,
            qualityLevel: this.qualityLevel,
            averageFrameTime: this.performanceMonitor.getAverageFrameTime(),
            config: this.config
        };
    }
    
    // Update configuration
    updateConfig(newConfig) {
        Object.assign(this.config, newConfig);

        // Restart if enabled state changed
        if (newConfig.enabled !== undefined) {
            if (newConfig.enabled && !this.isRunning) {
                this.start();
            } else if (!newConfig.enabled && this.isRunning) {
                this.stop();
            }
        }

        console.log('Aircraft Animation: Configuration updated:', this.config);
    }
}

// Aircraft state management class
class AircraftState {
    constructor(hex) {
        this.hex = hex;
        this.positionHistory = [];
        this.velocityVector = null;
        this.lastUpdateTime = 0;
        this.lastKnownPosition = null;
        this.aircraft = null;
        this.confidence = 1.0;
    }
    
    addPosition(position, timestamp) {
        // Add to history
        this.positionHistory.push({ position, timestamp });
        
        // Keep only last N positions for memory efficiency
        const maxHistory = 5;
        if (this.positionHistory.length > maxHistory) {
            this.positionHistory.shift();
        }
        
        // Update velocity vector if we have enough history
        if (this.positionHistory.length >= 2) {
            this.updateVelocityVector();
        }
        
        this.lastKnownPosition = position;
        this.lastUpdateTime = timestamp;
    }
    
    updateVelocityVector() {
        const current = this.positionHistory[this.positionHistory.length - 1];
        const previous = this.positionHistory[this.positionHistory.length - 2];
        
        if (!current || !previous) {
            return;
        }
        
        const deltaTime = (current.timestamp - previous.timestamp) / 1000; // seconds
        if (deltaTime <= 0) {
            return;
        }
        
        this.velocityVector = this.calculateVelocityVector(
            current.position,
            previous.position,
            deltaTime
        );
    }
    
    calculateVelocityVector(currentPos, previousPos, deltaTime) {
        // Primary: Use ADS-B ground speed and track if available
        const groundSpeed = currentPos.groundSpeed; // knots
        const track = currentPos.track; // degrees
        const verticalRate = currentPos.verticalRate; // ft/min
        
        let vx, vy, vz;
        let confidence = 1.0;
        
        if (groundSpeed && track !== undefined && groundSpeed > 0) {
            // Use ADS-B velocity data (preferred)
            const speedMs = groundSpeed * 0.514444; // knots to m/s
            vx = speedMs * Math.sin(track * Math.PI / 180);
            vy = speedMs * Math.cos(track * Math.PI / 180);
            confidence = 0.9; // High confidence for ADS-B data
        } else {
            // Fallback: Calculate from position changes
            const deltaLat = currentPos.lat - previousPos.lat;
            const deltaLon = currentPos.lon - previousPos.lon;
            
            vx = (deltaLon * 111320 * Math.cos(currentPos.lat * Math.PI / 180)) / deltaTime;
            vy = (deltaLat * 111320) / deltaTime;
            confidence = 0.6; // Lower confidence for calculated velocity
        }
        
        // Vertical velocity
        if (verticalRate) {
            vz = verticalRate * 0.00508; // ft/min to m/s
        } else {
            const deltaAlt = currentPos.alt - previousPos.alt;
            vz = deltaAlt / deltaTime;
        }
        
        // Adjust confidence based on data quality
        if (deltaTime > 5) confidence *= 0.8; // Reduce confidence for old data
        if (Math.abs(vx) > 200 || Math.abs(vy) > 200) confidence *= 0.5; // Reduce for unrealistic speeds
        
        return {
            vx: vx,
            vy: vy,
            vz: vz,
            confidence: Math.max(0.1, confidence),
            timestamp: Date.now()
        };
    }
    
    hasValidVelocity() {
        return this.velocityVector && 
               this.velocityVector.confidence > 0.1 && 
               this.lastKnownPosition;
    }
}

// Performance monitoring class
class PerformanceMonitor {
    constructor() {
        this.frameTimeHistory = [];
        this.maxFrameTime = 50; // ms
        this.qualityLevel = 1.0; // 1.0 = full quality, 0.3 = minimum
        this.maxHistorySize = 10;
    }
    
    recordFrameTime(time) {
        this.frameTimeHistory.push(time);
        
        if (this.frameTimeHistory.length > this.maxHistorySize) {
            this.frameTimeHistory.shift();
        }
        
        // Adjust quality based on performance
        const avgFrameTime = this.getAverageFrameTime();
        
        if (avgFrameTime > this.maxFrameTime) {
            // Performance is poor, reduce quality
            this.qualityLevel = Math.max(0.3, this.qualityLevel * 0.95);
        } else if (avgFrameTime < this.maxFrameTime * 0.7) {
            // Performance is good, increase quality
            this.qualityLevel = Math.min(1.0, this.qualityLevel * 1.02);
        }
    }
    
    getAverageFrameTime() {
        if (this.frameTimeHistory.length === 0) {
            return 0;
        }
        
        const sum = this.frameTimeHistory.reduce((a, b) => a + b, 0);
        return sum / this.frameTimeHistory.length;
    }
    
    getQualityLevel() {
        return this.qualityLevel;
    }
    
    getStats() {
        return {
            averageFrameTime: this.getAverageFrameTime(),
            qualityLevel: this.qualityLevel,
            frameCount: this.frameTimeHistory.length
        };
    }
}

// Export for use in other modules
window.AircraftAnimationEngine = AircraftAnimationEngine;