/**
 * Module: aircraft-animation
 * Why it exists:
 * - Smooths visual aircraft movement between sparse/non-uniform realtime updates.
 * - Reduces perceptual jitter by blending prediction, heading smoothing, and correction easing.
 *
 * Key responsibilities:
 * - Maintain per-aircraft animation state and lifecycle.
 * - Predict short-horizon pose and merge toward real observations.
 * - Emit pose updates to the map manager at the configured interpolation cadence.
 * - Track runtime animation telemetry for debug/performance overlays.
 *
 * Quirks / contracts:
 * - Prediction is intentionally conservative and bounded by deadbands/thresholds to
 *   avoid oscillation from noisy deltas.
 * - Designed to coexist with map upsert logic that preserves pose while smoothing
 *   is active; bypassing that contract causes visible snap-back artifacts.
 */
class AircraftAnimationEngine {
    constructor(mapManager, store) {
        this.mapManager = mapManager;
        this.store = store;

        this.config = {
            enabled: true,
            interpolationFps: 30,
            // How long a sample is carried forward before the aircraft
            // stops, in seconds. The receiver decodes ~0.6 positions a second
            // (13/10/2026), so one interval in six is longer than 3 s (the
            // value before) and the aircraft stopped there until the next
            // fix; one in forty is longer than 6 s.
            maxPredictionSeconds: 6.0,
            // At the aircraft's own speed: below one (0.72 before) the aircraft
            // flew at 72 % of its speed between two samples and caught up the
            // rest on each one, a sawtooth of speed once a second. An overshoot
            // is paid back by the fading correction, gently.
            predictionGain: 1.0,
            predictedJitterDeadbandNm: 0.0035,
            predictedHeadingDeadbandDeg: 1.4,
            mergeDurationMs: 450,
            minCorrectionDurationMs: 180,
            maxCorrectionDurationMs: 2500,
            realCorrectionThresholdNm: 0.016,
            predictedCorrectionThresholdNm: 0.004,
            headingSmoothingAlpha: 0.22,
            viewportCulling: true,
            adaptivePerformance: true,
            maxFrameBudgetMs: 14,
            cleanupIntervalMs: 30000,
            maxStates: 600
        };

        this.aircraftStates = new Map();
        this.isRunning = false;
        this.rafId = null;
        this.lastFrameAt = 0;
        this.lastCleanupAt = 0;

        this.qualityLevel = 1.0;
        this.frameTimes = [];
        this.maxFrameSamples = 60;

        this.stats = {
            measuredFps: 0,
            averageFrameTime: 0,
            activeStates: 0,
            visibleTargets: 0,
            animatedThisFrame: 0,
            predictedPerSec: 0,
            correctedPerSec: 0,
            markerUpdatesPerSec: 0,
            qualityLevel: 1.0,
            droppedFrames: 0,
            config: this.config
        };

        this._secWindowStart = performance.now();
        this._secFrameCount = 0;
        this._secPredictedCount = 0;
        this._secCorrectedCount = 0;
        this._secMarkerUpdates = 0;

        this.rafLoop = this.rafLoop.bind(this);
    }

    initialize() {
        if (this.store?.settings?.aircraftAnimation) {
            this.updateConfig(this.store.settings.aircraftAnimation);
        }
        if (this.config.enabled) {
            this.start();
        }
    }

    start() {
        if (this.isRunning) return;
        this.isRunning = true;
        this.lastFrameAt = performance.now();
        this.lastCleanupAt = Date.now();
        this._resetSecondWindow(performance.now());
        this.rafId = requestAnimationFrame(this.rafLoop);
    }

    stop() {
        if (!this.isRunning) return;
        this.isRunning = false;
        if (this.rafId) {
            cancelAnimationFrame(this.rafId);
            this.rafId = null;
        }
        this.aircraftStates.clear();
        this.frameTimes.length = 0;
        this.stats.activeStates = 0;
        this.stats.visibleTargets = 0;
        this.stats.animatedThisFrame = 0;
        this.stats.predictedPerSec = 0;
        this.stats.correctedPerSec = 0;
        this.stats.markerUpdatesPerSec = 0;
        this.stats.measuredFps = 0;
        this.stats.averageFrameTime = 0;
    }

    updateConfig(newConfig = {}) {
        Object.assign(this.config, newConfig);
        this.stats.config = this.config;

        if (newConfig.enabled === false && this.isRunning) {
            this.stop();
        } else if (newConfig.enabled === true && !this.isRunning) {
            this.start();
        }
    }

    removeAircraft(hex) {
        if (!hex) return;
        this.aircraftStates.delete(hex);
    }

    updateAircraft(aircraft) {
        if (!this.config.enabled || !aircraft?.hex) return;

        const snapshot = this._extractSnapshotFromAircraft(aircraft);
        if (!snapshot) return;

        if (this.aircraftStates.size >= this.config.maxStates && !this.aircraftStates.has(aircraft.hex)) {
            this._evictOldestState();
        }

        const now = performance.now();
        let state = this.aircraftStates.get(aircraft.hex);
        if (!state) {
            state = this._createState(aircraft.hex);
            this.aircraftStates.set(aircraft.hex, state);
        }

        const previousReal = state.lastReal;
        // Where the aircraft is drawn now: the last frame drawn carried
        // forward over the few milliseconds since, at the velocity it was
        // drawn with. The gap measured from the last frame as it stood made
        // the first frame after each sample advance only the time left to
        // it (13 ms of flight instead of 33 at 30 fps, measured on the replay
        // bench): a nearly stopped frame on every sample.
        // An aircraft not moved for longer than two frames was not moving on
        // the map (stopped, or not animated): it is taken where it stands.
        const frameMs = 1000 / Math.max(1, this.config.interpolationFps || 30);
        const previousRendered = state.lastRendered && (now - state.lastRendered.timestamp) <= 2 * frameMs
            ? this._carriedForward(state.lastRendered, state.velocity, now)
            : (state.lastRendered || previousReal);
        let effectiveSnapshot = snapshot;

        // A real update whose position is the one already known is not a new
        // fix: the receiver decoded no position since the last sample, only the
        // altitude, the speed or a heading moved. Taking the old fix for a fresh
        // observation (timestamp now) drags the aircraft back to it, then the
        // prediction runs ahead again: measured on 13/10/2026 against a receiver
        // giving a position every ~1.7 s, polled every second, 30 % of the
        // rendered steps went backwards. Keep extrapolating from the fix as it
        // was dated; the new figures only refresh the velocity and the heading.
        if (snapshot.source !== 'predicted' && state.lastRealFix &&
            Math.abs(state.lastRealFix.lat - snapshot.lat) < 1e-7 &&
            Math.abs(state.lastRealFix.lon - snapshot.lon) < 1e-7) {
            state.lastSeenAt = Date.now();
            state.status = aircraft.status || 'active';
            this._refreshFromRepeatedFix(state, snapshot);
            return;
        }

        if (previousReal && snapshot.source === 'predicted') {
            effectiveSnapshot = this._blendPredictedSnapshot(previousReal, snapshot);
        }

        state.lastSeenAt = Date.now();
        state.status = aircraft.status || 'active';
        state.lastSampleSource = snapshot.source || 'real';
        if (Number.isFinite(effectiveSnapshot.heading)) {
            const baseAlpha = Number.isFinite(this.config.headingSmoothingAlpha)
                ? this.config.headingSmoothingAlpha
                : 0.22;
            const alpha = snapshot.source === 'predicted'
                ? baseAlpha
                : Math.min(0.45, baseAlpha * 1.35);
            const currentHeading = Number.isFinite(state.displayHeading) ? state.displayHeading : null;
            const headingDelta = Number.isFinite(currentHeading)
                ? Math.abs(this._angleDiffDeg(effectiveSnapshot.heading, currentHeading))
                : Infinity;
            const headingDeadband = Number.isFinite(this.config.predictedHeadingDeadbandDeg)
                ? this.config.predictedHeadingDeadbandDeg
                : 1.4;
            if (!(snapshot.source === 'predicted' && Number.isFinite(headingDelta) && headingDelta < headingDeadband)) {
                state.displayHeading = this._lerpAngleDeg(state.displayHeading, effectiveSnapshot.heading, alpha);
            }
        }

        if (previousReal && snapshot.source !== 'predicted') {
            const derivedVelocity = this._deriveVelocity(previousReal, effectiveSnapshot);
            if (derivedVelocity) {
                state.velocity = this._blendVelocity(state.velocity, derivedVelocity);
            }
        }

        if (previousReal) {
            if (previousRendered) {
                const distanceNm = this._distanceNm(previousRendered.lat, previousRendered.lon, effectiveSnapshot.lat, effectiveSnapshot.lon);
                const correctionThresholdNm = snapshot.source === 'predicted'
                    ? this.config.predictedCorrectionThresholdNm
                    : this.config.realCorrectionThresholdNm;
                if (distanceNm > correctionThresholdNm) {
                    // The gap closes at no more than half the aircraft's own
                    // speed at the peak of the ease (1.5 times the mean rate):
                    // closing 150 m in 450 ms made a 205 m/s aircraft sprint at
                    // three times its speed for a few frames. A stationary
                    // aircraft takes the base duration.
                    const speedMps = state.velocity
                        ? Math.hypot(state.velocity.eastMps, state.velocity.northMps)
                        : 0;
                    const gapM = distanceNm * 1852;
                    const baseDuration = Math.max(this.config.minCorrectionDurationMs, this.config.mergeDurationMs);
                    const gentleMs = speedMps > 1 ? (3 * gapM / speedMps) * 1000 : baseDuration;
                    const correctionDurationMs = Math.min(
                        this.config.maxCorrectionDurationMs,
                        Math.max(this.config.minCorrectionDurationMs, Math.round(gentleMs))
                    );
                    // The correction is an offset that fades, not a glide to a
                    // fixed point: the aircraft keeps flying its prediction from
                    // the new sample, and the gap between where it was drawn and
                    // where the sample puts it closes over the duration. Gliding
                    // to the sample's position, as before, stopped the aircraft
                    // there while the prediction moved on, then jumped to catch up
                    // (measured 13/10/2026: 7 m, 5 m, 2 m, 0.2 m, then 76 m in one
                    // frame, once a second, on every aircraft).
                    state.correction = {
                        dLat: previousRendered.lat - effectiveSnapshot.lat,
                        dLon: previousRendered.lon - effectiveSnapshot.lon,
                        dAlt: previousRendered.alt - effectiveSnapshot.alt,
                        startAt: now,
                        endAt: now + correctionDurationMs
                    };
                } else {
                    state.correction = null;
                }
            }
        } else {
            state.correction = null;
        }

        state.lastReal = effectiveSnapshot;
        if (snapshot.source !== 'predicted') {
            state.lastRealFix = { lat: snapshot.lat, lon: snapshot.lon };
        }

        if (!state.lastRendered) {
            state.lastRendered = { ...effectiveSnapshot };
        }
    }

    // A drawn point moved on to time `now` at a velocity.
    _carriedForward(point, velocity, now) {
        if (!point || !velocity || !Number.isFinite(point.timestamp)) return point;
        const sec = Math.max(0, (now - point.timestamp) / 1000);
        if (sec === 0) return point;
        return {
            ...point,
            lat: point.lat + (velocity.northMps * sec) / 111320,
            lon: point.lon + (velocity.eastMps * sec) / (111320 * Math.cos((point.lat * Math.PI) / 180)),
            alt: point.alt + velocity.verticalMps * sec * 3.28084
        };
    }

    // The sample repeats the known fix: its speed, track and vertical rate are
    // current even though the position is not, so the velocity follows them
    // (an aircraft that has stopped must stop) and the heading is smoothed as
    // for any real sample.
    _refreshFromRepeatedFix(state, snapshot) {
        if (Number.isFinite(snapshot.speedKts) && Number.isFinite(snapshot.track)) {
            const speedMps = snapshot.speedKts * 0.514444;
            const trackRad = (snapshot.track * Math.PI) / 180;
            state.velocity = this._blendVelocity(state.velocity, {
                eastMps: speedMps * Math.sin(trackRad),
                northMps: speedMps * Math.cos(trackRad),
                verticalMps: Number.isFinite(snapshot.baroRate) ? snapshot.baroRate * 0.00508 : 0,
                headingDeg: snapshot.track
            });
        }
        if (Number.isFinite(snapshot.heading)) {
            const baseAlpha = Number.isFinite(this.config.headingSmoothingAlpha)
                ? this.config.headingSmoothingAlpha
                : 0.22;
            state.displayHeading = this._lerpAngleDeg(state.displayHeading, snapshot.heading, Math.min(0.45, baseAlpha * 1.35));
        }
    }

    updateAircraftDelta(hex, _delta) {
        if (!hex) return;
        const aircraft = this.store?.aircraft?.[hex];
        if (!aircraft) return;
        this.updateAircraft(aircraft);
    }

    rafLoop(now) {
        if (!this.isRunning || !this.config.enabled) return;

        const targetFrameMs = 1000 / Math.max(1, this.config.interpolationFps || 30);
        const elapsed = now - this.lastFrameAt;

        if (elapsed >= targetFrameMs) {
            this._processFrame(now);
            this.lastFrameAt = now;
        }

        this.rafId = requestAnimationFrame(this.rafLoop);
    }

    _processFrame(now) {
        const frameStart = performance.now();
        this._secFrameCount++;

        const targets = this._getAnimationTargetHexes();
        this.stats.visibleTargets = targets.length;

        let animatedThisFrame = 0;
        let processed = 0;
        const maxPerFrame = this._getFrameAircraftLimit(targets.length);

        for (let i = 0; i < targets.length; i++) {
            if (processed >= maxPerFrame) {
                this.stats.droppedFrames++;
                break;
            }

            const hex = targets[i];
            const state = this.aircraftStates.get(hex);
            if (!state || state.status === 'signal_lost') continue;

            const pose = this._computePose(state, now);
            if (!pose) continue;

            if (this._applyPose(hex, pose, state)) {
                animatedThisFrame++;
                this._secMarkerUpdates++;
            }

            processed++;

            if (this.config.adaptivePerformance && (performance.now() - frameStart) > this.config.maxFrameBudgetMs) {
                this.stats.droppedFrames++;
                break;
            }
        }

        this.stats.animatedThisFrame = animatedThisFrame;
        this.stats.activeStates = this.aircraftStates.size;

        const frameMs = performance.now() - frameStart;
        this.frameTimes.push(frameMs);
        if (this.frameTimes.length > this.maxFrameSamples) {
            this.frameTimes.shift();
        }

        this.stats.averageFrameTime = this._average(this.frameTimes);

        if (this.config.adaptivePerformance) {
            this._updateQualityLevel();
        }

        this._updateSecondWindow(now);

        if (Date.now() - this.lastCleanupAt > this.config.cleanupIntervalMs) {
            this._cleanupStaleStates();
            this.lastCleanupAt = Date.now();
        }
    }

    _computePose(state, now) {
        const real = state.lastReal;
        if (!real) return null;

        const heading = Number.isFinite(state.displayHeading)
            ? state.displayHeading
            : (Number.isFinite(real.heading)
                ? real.heading
                : (Number.isFinite(real.track)
                    ? real.track
                    : (Number.isFinite(state.velocity?.headingDeg)
                        ? state.velocity.headingDeg
                        : null)));

        const pose = this._predictedPose(state, real, now, heading);

        // A correction in progress: the aircraft flies its prediction, and the
        // gap between where it was drawn and where the sample put it fades out.
        if (state.correction) {
            const duration = Math.max(1, state.correction.endAt - state.correction.startAt);
            const t = Math.min(1, Math.max(0, (now - state.correction.startAt) / duration));
            const remaining = 1 - (t * t * (3 - (2 * t)));

            pose.lat += state.correction.dLat * remaining;
            pose.lon += state.correction.dLon * remaining;
            pose.alt += state.correction.dAlt * remaining;

            if (t >= 1) {
                state.correction = null;
            }
            this._secCorrectedCount++;
        }

        return pose;
    }

    // Where the last sample says the aircraft is now: the sample carried forward
    // by the velocity over a bounded horizon, whether it was observed or
    // predicted by the server (a predicted sample left where it stood, as
    // before, stopped the aircraft for a second, then the next real sample
    // made it jump); the sample itself when there is no velocity.
    _predictedPose(state, real, now, heading) {
        if (!state.velocity) {
            return { lat: real.lat, lon: real.lon, alt: real.alt, heading };
        }

        // The horizon does not shrink with the quality level: a changing
        // horizon moved every aircraft back and forth as the frame budget came
        // and went. The quality level only limits how many aircraft a frame
        // animates.
        const elapsedSec = Math.max(0, (now - real.timestamp) / 1000);
        const baseHorizon = Math.min(this.config.maxPredictionSeconds, elapsedSec);
        const horizon = baseHorizon * (Number.isFinite(this.config.predictionGain) ? this.config.predictionGain : 0.72);

        const dNorth = state.velocity.northMps * horizon;
        const dEast = state.velocity.eastMps * horizon;

        const lat = real.lat + (dNorth / 111320);
        const lon = real.lon + (dEast / (111320 * Math.cos((real.lat * Math.PI) / 180)));

        if (!Number.isFinite(lat) || !Number.isFinite(lon)) {
            return { lat: real.lat, lon: real.lon, alt: real.alt, heading };
        }

        const alt = real.alt + (state.velocity.verticalMps * horizon * 3.28084);

        this._secPredictedCount++;
        return { lat, lon, alt, heading };
    }

    _applyPose(hex, pose, state) {
        const previous = state.lastRendered;
        const positionChanged = !previous ||
            Math.abs(previous.lat - pose.lat) >= 1e-8 ||
            Math.abs(previous.lon - pose.lon) >= 1e-8;
        const headingChanged = Number.isFinite(pose.heading) && (!previous || Math.abs(this._angleDiffDeg(previous.track, pose.heading)) > 0.25);

        if (!positionChanged && !headingChanged) {
            return false;
        }

        if (!this.mapManager || typeof this.mapManager.updateAnimatedAircraftPose !== 'function') {
            return false;
        }

        const applied = this.mapManager.updateAnimatedAircraftPose(hex, {
            lat: pose.lat,
            lon: pose.lon,
            heading: pose.heading,
        });
        if (!applied) {
            return false;
        }

        state.lastRendered = {
            lat: pose.lat,
            lon: pose.lon,
            alt: pose.alt,
            track: Number.isFinite(pose.heading) ? pose.heading : (previous?.track ?? 0),
            timestamp: performance.now()
        };

        return true;
    }

    _angleDiffDeg(a, b) {
        if (!Number.isFinite(a) || !Number.isFinite(b)) return 0;
        let d = ((a - b) % 360 + 360) % 360;
        if (d > 180) d -= 360;
        return d;
    }

    _extractSnapshotFromAircraft(aircraft) {
        const adsb = aircraft?.adsb;
        if (!adsb) return null;

        const lat = adsb.lat;
        const lon = adsb.lon;
        if (!Number.isFinite(lat) || !Number.isFinite(lon) || (lat === 0 && lon === 0)) {
            return null;
        }

        const gs = Number.isFinite(adsb.gs) ? adsb.gs : null;
        const tas = Number.isFinite(adsb.tas) ? adsb.tas : null;
        const speedKts = gs ?? tas;
        const track = Number.isFinite(adsb.track) ? adsb.track : null;
        const trueHeading = Number.isFinite(adsb.true_heading) ? adsb.true_heading : null;
        const magHeading = Number.isFinite(adsb.mag_heading) ? adsb.mag_heading : null;
        const heading = trueHeading ?? magHeading ?? track;
        const alt = Number.isFinite(adsb.alt_baro) ? adsb.alt_baro : 0;
        const baroRate = Number.isFinite(adsb.baro_rate) ? adsb.baro_rate : 0;

        return {
            lat,
            lon,
            alt,
            heading,
            track,
            speedKts,
            baroRate,
            source: adsb.source === 'predicted' ? 'predicted' : 'real',
            timestamp: performance.now()
        };
    }

    _deriveVelocity(previous, current) {
        const dt = Math.max(0.001, (current.timestamp - previous.timestamp) / 1000);

        let eastMps;
        let northMps;
        let headingDeg;

        if (Number.isFinite(current.speedKts) && Number.isFinite(current.track)) {
            const speedMps = current.speedKts * 0.514444;
            const trackRad = (current.track * Math.PI) / 180;
            eastMps = speedMps * Math.sin(trackRad);
            northMps = speedMps * Math.cos(trackRad);
            headingDeg = current.track;
        } else {
            const avgLatRad = ((previous.lat + current.lat) / 2) * (Math.PI / 180);
            const dNorth = (current.lat - previous.lat) * 111320;
            const dEast = (current.lon - previous.lon) * 111320 * Math.cos(avgLatRad);
            northMps = dNorth / dt;
            eastMps = dEast / dt;
            headingDeg = this._bearingDeg(previous.lat, previous.lon, current.lat, current.lon);
        }

        if (!Number.isFinite(eastMps) || !Number.isFinite(northMps)) {
            return null;
        }

        const verticalMps = Number.isFinite(current.baroRate)
            ? current.baroRate * 0.00508
            : ((current.alt - previous.alt) * 0.3048) / dt;

        return {
            eastMps,
            northMps,
            verticalMps: Number.isFinite(verticalMps) ? verticalMps : 0,
            headingDeg: Number.isFinite(headingDeg) ? headingDeg : 0
        };
    }

    _blendVelocity(existing, incoming) {
        if (!existing) return incoming;
        const alpha = 0.35;
        return {
            eastMps: this._lerp(existing.eastMps, incoming.eastMps, alpha),
            northMps: this._lerp(existing.northMps, incoming.northMps, alpha),
            verticalMps: this._lerp(existing.verticalMps, incoming.verticalMps, alpha),
            headingDeg: incoming.headingDeg
        };
    }

    _blendPredictedSnapshot(previousReal, predictedSnapshot) {
        const distanceNm = this._distanceNm(previousReal.lat, previousReal.lon, predictedSnapshot.lat, predictedSnapshot.lon);
        const headingDelta = Number.isFinite(previousReal.heading) && Number.isFinite(predictedSnapshot.heading)
            ? Math.abs(this._angleDiffDeg(predictedSnapshot.heading, previousReal.heading))
            : Infinity;
        const jitterDeadbandNm = Number.isFinite(this.config.predictedJitterDeadbandNm)
            ? this.config.predictedJitterDeadbandNm
            : 0.0035;
        const headingDeadbandDeg = Number.isFinite(this.config.predictedHeadingDeadbandDeg)
            ? this.config.predictedHeadingDeadbandDeg
            : 1.4;

        if (distanceNm < jitterDeadbandNm && headingDelta < headingDeadbandDeg) {
            const carryHeading = Number.isFinite(previousReal.heading)
                ? previousReal.heading
                : predictedSnapshot.heading;
            return {
                ...predictedSnapshot,
                lat: previousReal.lat,
                lon: previousReal.lon,
                alt: this._lerp(previousReal.alt, predictedSnapshot.alt, 0.35),
                heading: carryHeading,
            };
        }

        const gain = 0.78;
        const lat = this._lerp(previousReal.lat, predictedSnapshot.lat, gain);
        const lon = this._lerp(previousReal.lon, predictedSnapshot.lon, gain);
        const alt = this._lerp(previousReal.alt, predictedSnapshot.alt, gain);
        const heading = Number.isFinite(predictedSnapshot.heading)
            ? predictedSnapshot.heading
            : previousReal.heading;

        return {
            ...predictedSnapshot,
            lat,
            lon,
            alt,
            heading,
        };
    }

    _lerpAngleDeg(previous, target, alpha) {
        if (!Number.isFinite(target)) return Number.isFinite(previous) ? previous : 0;
        if (!Number.isFinite(previous)) return this._normalizeHeadingDeg(target);
        const diff = this._angleDiffDeg(target, previous);
        return this._normalizeHeadingDeg(previous + (diff * alpha));
    }

    _normalizeHeadingDeg(value) {
        if (!Number.isFinite(value)) return 0;
        return ((value % 360) + 360) % 360;
    }

    _getAnimationTargetHexes() {
        const selectedHex = this.store?.selectedAircraft?.hex;

        if (this.config.viewportCulling) {
            const visibleSet = this.store?.visibleAircraftOnMap;
            if (visibleSet && visibleSet.size > 0) {
                const targets = [];
                visibleSet.forEach((hex) => {
                    if (this.aircraftStates.has(hex)) {
                        targets.push(hex);
                    }
                });
                if (selectedHex && this.aircraftStates.has(selectedHex) && !targets.includes(selectedHex)) {
                    targets.push(selectedHex);
                }
                return targets;
            }
        }

        const all = Array.from(this.aircraftStates.keys());
        if (selectedHex && !all.includes(selectedHex) && this.aircraftStates.has(selectedHex)) {
            all.push(selectedHex);
        }
        return all;
    }

    _getFrameAircraftLimit(targetCount) {
        if (!this.config.adaptivePerformance) return targetCount;
        const baseLimit = 240;
        const scaled = Math.floor(baseLimit * this.qualityLevel);
        return Math.max(30, Math.min(targetCount, scaled));
    }

    _updateQualityLevel() {
        const avg = this.stats.averageFrameTime;
        if (!Number.isFinite(avg) || avg <= 0) return;

        if (avg > this.config.maxFrameBudgetMs) {
            this.qualityLevel = Math.max(0.4, this.qualityLevel - 0.05);
        } else if (avg < this.config.maxFrameBudgetMs * 0.7) {
            this.qualityLevel = Math.min(1.0, this.qualityLevel + 0.02);
        }
        this.stats.qualityLevel = Number(this.qualityLevel.toFixed(2));
    }

    _cleanupStaleStates() {
        const cutoff = Date.now() - 120000;
        for (const [hex, state] of this.aircraftStates.entries()) {
            if (state.lastSeenAt < cutoff || state.status === 'signal_lost') {
                this.aircraftStates.delete(hex);
            }
        }
    }

    _evictOldestState() {
        let oldestHex = null;
        let oldestSeen = Infinity;
        for (const [hex, state] of this.aircraftStates.entries()) {
            if (state.lastSeenAt < oldestSeen) {
                oldestSeen = state.lastSeenAt;
                oldestHex = hex;
            }
        }
        if (oldestHex) {
            this.aircraftStates.delete(oldestHex);
        }
    }

    _createState(hex) {
        return {
            hex,
            status: 'active',
            lastSeenAt: Date.now(),
            lastReal: null,
            lastRealFix: null,
            lastRendered: null,
            displayHeading: null,
            velocity: null,
            correction: null,
            lastSampleSource: 'real'
        };
    }

    _updateSecondWindow(now) {
        const elapsed = now - this._secWindowStart;
        if (elapsed < 1000) return;

        const seconds = elapsed / 1000;
        this.stats.measuredFps = Number((this._secFrameCount / seconds).toFixed(1));
        this.stats.predictedPerSec = Number((this._secPredictedCount / seconds).toFixed(1));
        this.stats.correctedPerSec = Number((this._secCorrectedCount / seconds).toFixed(1));
        this.stats.markerUpdatesPerSec = Number((this._secMarkerUpdates / seconds).toFixed(1));

        this._resetSecondWindow(now);
    }

    _resetSecondWindow(now) {
        this._secWindowStart = now;
        this._secFrameCount = 0;
        this._secPredictedCount = 0;
        this._secCorrectedCount = 0;
        this._secMarkerUpdates = 0;
    }

    _distanceNm(lat1, lon1, lat2, lon2) {
        const r = 6371000;
        const toRad = Math.PI / 180;
        const dLat = (lat2 - lat1) * toRad;
        const dLon = (lon2 - lon1) * toRad;
        const a = Math.sin(dLat / 2) ** 2 + Math.cos(lat1 * toRad) * Math.cos(lat2 * toRad) * Math.sin(dLon / 2) ** 2;
        const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
        return (r * c) / 1852;
    }

    _bearingDeg(lat1, lon1, lat2, lon2) {
        const toRad = Math.PI / 180;
        const toDeg = 180 / Math.PI;
        const phi1 = lat1 * toRad;
        const phi2 = lat2 * toRad;
        const dLon = (lon2 - lon1) * toRad;
        const y = Math.sin(dLon) * Math.cos(phi2);
        const x = Math.cos(phi1) * Math.sin(phi2) - Math.sin(phi1) * Math.cos(phi2) * Math.cos(dLon);
        const brng = Math.atan2(y, x) * toDeg;
        return (brng + 360) % 360;
    }

    _lerp(a, b, t) {
        return a + (b - a) * t;
    }

    _average(values) {
        if (!values.length) return 0;
        let sum = 0;
        for (let i = 0; i < values.length; i++) sum += values[i];
        return Number((sum / values.length).toFixed(2));
    }

    getStats() {
        return {
            isRunning: this.isRunning,
            aircraftCount: this.aircraftStates.size,
            qualityLevel: this.stats.qualityLevel,
            measuredFps: this.stats.measuredFps,
            averageFrameTime: this.stats.averageFrameTime,
            visibleTargets: this.stats.visibleTargets,
            animatedThisFrame: this.stats.animatedThisFrame,
            predictedPerSec: this.stats.predictedPerSec,
            correctedPerSec: this.stats.correctedPerSec,
            markerUpdatesPerSec: this.stats.markerUpdatesPerSec,
            droppedFrames: this.stats.droppedFrames,
            config: this.config
        };
    }
}

if (typeof window !== 'undefined') {
    window.AircraftAnimationEngine = AircraftAnimationEngine;
}
if (typeof module === 'object' && module.exports) {
    module.exports = AircraftAnimationEngine;
}
