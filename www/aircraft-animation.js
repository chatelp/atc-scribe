// Aircraft Smooth Animation Engine
// Provides vector extrapolation and smooth interpolation for aircraft movement

class AircraftAnimationEngine {
    constructor(mapManager, store) {
        this.mapManager = mapManager;
        this.store = store;
        
        // Animation configuration
        this.config = {
            enabled: true,
            interpolationFps: 10,                    // 100ms updates
            maxExtrapolationSeconds: 2.4,            // 20% beyond 2s update interval
            confidenceDecayRate: 0.5,                // Exponential decay factor
            minConfidenceThreshold: 0.3,             // Stop animating below this
            viewportCulling: true,                   // Only animate visible aircraft
            adaptivePerformance: true,               // Reduce quality under load
            maxProcessingTimeMs: 50,                 // Max time per animation frame
            maxHistoryPoints: 5,                     // Position history per aircraft
            enableCurvedInterpolation: true,         // Use track rate for turning aircraft
            enableAltitudeInterpolation: true,       // Interpolate altitude changes
            distanceQualityReduction: 50             // Reduce quality beyond this distance (NM)
        };
        
        // Animation state
        this.animationTimer = null;
        this.aircraftStates = new Map();             // hex -> AircraftState
        this.lastAnimationTime = 0;
        this.frameTimeHistory = [];
        this.qualityLevel = 1.0;                     // 1.0 = full quality, 0.3 = minimum
        this.isRunning = false;
        
        // Performance monitoring
        this.performanceMonitor = new PerformanceMonitor();
        
        // Bind methods
        this.animationFrame = this.animationFrame.bind(this);
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
        
        console.log('Aircraft Animation Engine: Starting...');
        this.isRunning = true;
        this.lastAnimationTime = Date.now();
        
        // Start animation timer
        this.animationTimer = setInterval(this.animationFrame, 1000 / this.config.interpolationFps);
    }
    
    // Stop the animation engine
    stop() {
        if (!this.isRunning) {
            return;
        }
        
        console.log('Aircraft Animation Engine: Stopping...');
        this.isRunning = false;
        
        if (this.animationTimer) {
            clearInterval(this.animationTimer);
            this.animationTimer = null;
        }
        
        // Clear all aircraft states
        this.aircraftStates.clear();
    }
    
    // Main animation frame processing
    animationFrame() {
        if (!this.isRunning || !this.config.enabled) {
            return;
        }
        
        const startTime = performance.now();
        const currentTime = Date.now();
        
        try {
            // Get visible aircraft for viewport culling
            const visibleAircraft = this.config.viewportCulling ? 
                this.getVisibleAircraft() : 
                Object.values(this.store.aircraft || {});
            
            // Update positions for all visible aircraft
            let updatedCount = 0;
            for (const aircraft of visibleAircraft) {
                if (this.updateAircraftPosition(aircraft, currentTime)) {
                    updatedCount++;
                }
                
                // Check processing time limit
                if (performance.now() - startTime > this.config.maxProcessingTimeMs) {
                    console.warn('Aircraft Animation: Frame time limit exceeded, stopping early');
                    break;
                }
            }
            
            this.lastAnimationTime = currentTime;
            
            // Performance monitoring
            const processingTime = performance.now() - startTime;
            this.performanceMonitor.recordFrameTime(processingTime);
            
            // Adaptive performance adjustment
            if (this.config.adaptivePerformance) {
                this.qualityLevel = this.performanceMonitor.getQualityLevel();
            }
            
            if (updatedCount > 0) {
                //console.log(`Animation frame: ${updatedCount} aircraft updated in ${processingTime.toFixed(1)}ms`);
            }
            
        } catch (error) {
            console.error('Aircraft Animation: Error in animation frame:', error);
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
        }
    }
    
    // Remove aircraft from animation
    removeAircraft(hex) {
        this.aircraftStates.delete(hex);
    }
    
    // Update single aircraft position with interpolation
    updateAircraftPosition(aircraft, currentTime) {
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
        
        // Update map marker with interpolated position
        this.updateMapMarker(aircraft.hex, interpolatedPosition);
        
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
        
        return {
            lat: state.lastKnownPosition.lat + adjustedDeltaLat,
            lon: state.lastKnownPosition.lon + adjustedDeltaLon,
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
        
        // Restart if FPS changed
        if (this.isRunning && newConfig.interpolationFps) {
            this.stop();
            this.start();
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