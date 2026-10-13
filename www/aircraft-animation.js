/**
 * Module: aircraft-animation
 *
 * Moves each aircraft's symbol smoothly between the samples the page
 * receives: about one a second, each dated by its source.
 *
 * Input (store/feed.js; the sample's fields are listed at observe()):
 * - observe(hex, sample): what the station said, dated when the receiver
 *   decoded the position;
 * - estimate(hex, sample): where the server expects the aircraft between two
 *   polls, dated at the time predicted;
 * - updateAircraft(aircraft): an aircraft from the store, dated on arrival
 *   (the bulk load).
 * Output: once a frame, mapManager.updateAnimatedAircraftPose(hex, {lat, lon,
 * heading}) for each aircraft whose pose moved.
 *
 * Between samples, the last one is carried forward at the velocity the
 * station reported (ground speed and track, smoothed), over at most
 * maxPredictionSeconds from its source time. A new sample does not move the
 * symbol: the gap between where it is drawn and where the sample puts it
 * fades over a duration bounded by the aircraft's speed (D79).
 *
 * The map's own updates keep the symbol's pose while smoothing is on
 * (map/renderers/aircraft-webgl.js, preservePose); bypassing that makes the
 * symbol snap back.
 *
 * Measured by www/aircraft-replay.test.js against the true paths of
 * internal/adsb/replay_test.go.
 */
class AircraftAnimationEngine {
    constructor(mapManager, store) {
        this.mapManager = mapManager;
        this.store = store;

        // Every setting, its unit and its reason. enabled, interpolationFps
        // and viewportCulling come from the settings panel (app.js, stored);
        // the others are fixed here.
        this.config = {
            // Smoothing on (true) or off: off, the map draws each sample.
            enabled: true,
            // Frames per second the poses are computed at.
            interpolationFps: 30,
            // How long a sample is carried forward before the aircraft stops,
            // in seconds from the sample's source time (about a second before
            // it reaches the page): 7 s from the source is 6 s from arrival.
            // The receiver decodes ~0.6 positions a second (13/10/2026): one
            // interval between two fixes in six is longer than 3 s (the value
            // before, from arrival), one in forty longer than 6 s.
            maxPredictionSeconds: 7.0,
            // Fraction of the elapsed time the sample is carried over. At 1 the
            // aircraft flies at its own speed; at 0.72 (before) it flew at 72 %
            // between two samples and caught up the rest on each one, a
            // sawtooth of speed once a second (D79).
            predictionGain: 1.0,
            // Duration of the fading correction of an aircraft at rest (below
            // 1 m/s), in ms; a moving one takes three times the gap over its
            // speed, so it catches up at no more than half its own speed (D79).
            mergeDurationMs: 450,
            // Bounds of every correction's duration, in ms.
            minCorrectionDurationMs: 180,
            maxCorrectionDurationMs: 2500,
            // Share of the way the drawn heading moves toward a sample's (an
            // observation's: 1.35 times this, at most 0.45), per sample.
            headingSmoothingAlpha: 0.22,
            // Animate only the aircraft the map shows (and the selected one).
            viewportCulling: true,
            // When on, a frame stops animating once it has taken
            // maxFrameBudgetMs, and qualityLevel (0.4 to 1, down 0.05 a frame
            // over budget, up 0.02 a frame under 70 % of it) caps the aircraft
            // a frame animates at 240 times it, never fewer than 30. It changes
            // nothing else: the horizon no longer shrinks with it (D79).
            adaptivePerformance: true,
            maxFrameBudgetMs: 14,
            // How often states unheard of for two minutes are dropped, in ms,
            // and how many states are kept at most (the oldest go first).
            cleanupIntervalMs: 30000,
            maxStates: 600,
            // An estimate below this confidence is left out; the server applies
            // the same floor (internal/adsb/live_prediction.go).
            minEstimateConfidence: 0.65,
            // Two estimates of one aircraft at least this far apart, in ms: the
            // server sends one a second.
            minEstimateIntervalMs: 1000,
            // Seconds of messages the server clock's offset is taken over
            // (_pageTime).
            clockWindowSec: 30
        };

        this.aircraftStates = new Map();
        // Server clock less page clock, ms (_pageTime), and its evidence by
        // second.
        this.serverClockOffsetMs = null;
        this._clockBuckets = [];
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

    // ---------------------------------------------------------------- input
    //
    // Two entries, one per kind of sample, each dated by its source:
    //   observe(hex, sample)   what the station said: an aircraft_added or
    //                          aircraft_update (store/feed.js);
    //   estimate(hex, sample)  where the server expects the aircraft between
    //                          two polls: an aircraft_predicted_state.
    // sample: { lat, lon (deg), alt (ft), track (deg), gs (kt), baroRate
    // (ft/min), heading (deg, observe only: the station's, with the wind's
    // drift), at (server wall clock, ms: when the position was decoded, or
    // the time predicted), sentAt (server wall clock, ms, when the message
    // left), confidence and basedOn (estimate only), status (observe only) }.
    // A sample without `at` is dated on arrival.
    //
    // One output: the pose of each aircraft, once a frame, to
    // mapManager.updateAnimatedAircraftPose(hex, { lat, lon, heading }).

    observe(hex, sample) {
        if (!this.config.enabled || !hex || !sample) return;
        if (!this._hasPosition(sample)) return;
        const snapshot = this._snapshot(sample, 'real');
        const state = this._ingest(hex, snapshot, sample.status);
        if (state && Number.isFinite(sample.sentAt)) {
            state.lastObservedAt = sample.sentAt;
        }
    }

    // Left out below the confidence floor, when based on an older poll than
    // the last observation received (an observation always wins), and within
    // minEstimateIntervalMs of the last one applied.
    estimate(hex, sample) {
        if (!this.config.enabled || !hex || !sample) return;
        if (!this._hasPosition(sample)) return;
        if (Number.isFinite(sample.confidence) && sample.confidence < this.config.minEstimateConfidence) return;

        const state = this.aircraftStates.get(hex);
        if (state && Number.isFinite(sample.basedOn) && Number.isFinite(state.lastObservedAt) && sample.basedOn < state.lastObservedAt) {
            return;
        }
        const at = Number.isFinite(sample.at) ? sample.at : (Number.isFinite(sample.basedOn) ? sample.basedOn : Date.now());
        if (state && Number.isFinite(state.lastEstimateAt) && (at - state.lastEstimateAt) < this.config.minEstimateIntervalMs) {
            return;
        }

        // The heading and the vertical rate are the station's; an estimate
        // carries only a track.
        const observed = state?.lastObserved;
        const snapshot = this._snapshot({
            ...sample,
            alt: Number.isFinite(sample.alt) ? sample.alt : (observed?.alt ?? 0),
            track: Number.isFinite(sample.track) ? sample.track : (observed?.track ?? null),
            gs: Number.isFinite(sample.gs) ? sample.gs : (observed?.speedKts ?? null),
            heading: observed?.heading ?? sample.track,
            baroRate: observed?.baroRate ?? 0,
            sentAt: Number.isFinite(sample.sentAt) ? sample.sentAt : sample.at,
        }, 'predicted');
        const updated = this._ingest(hex, snapshot, state?.status);
        if (updated) {
            updated.lastEstimateAt = at;
        }
    }

    // An aircraft as the store holds it, for the callers that have no
    // message to date it from (the bulk load): dated on arrival.
    updateAircraft(aircraft) {
        if (!aircraft?.hex) return;
        this.observe(aircraft.hex, this.sampleFromAircraft(aircraft));
    }

    // The observe() sample of an aircraft's adsb block. observedAtMs is the
    // message's observed_at (the server's poll less the whole seconds of
    // seen); with seen and seen_pos it dates the position as the server
    // does (internal/adsb, TrajectorySnapshot.PositionTime).
    // timing is the adsb block that came with the message, when the
    // aircraft's own is a delta applied to an older one (aircraft_update).
    sampleFromAircraft(aircraft, observedAtMs = null, timing = null) {
        const adsb = aircraft?.adsb;
        if (!adsb) return null;
        const clock = timing || adsb;
        const num = (v) => (Number.isFinite(v) ? v : null);
        const sample = {
            lat: adsb.lat,
            lon: adsb.lon,
            alt: num(adsb.alt_baro) ?? 0,
            track: num(adsb.track),
            gs: num(adsb.gs) ?? num(adsb.tas),
            baroRate: num(adsb.baro_rate) ?? 0,
            heading: num(adsb.true_heading) ?? num(adsb.mag_heading) ?? num(adsb.track),
            status: aircraft.status,
        };
        if (!this._hasPosition(sample)) return null;
        if (Number.isFinite(observedAtMs)) {
            const pollAt = observedAtMs + Math.trunc(num(clock.seen) ?? 0) * 1000;
            sample.sentAt = pollAt;
            sample.at = Number.isFinite(clock.seen_pos) ? pollAt - clock.seen_pos * 1000 : pollAt;
        }
        return sample;
    }

    _hasPosition(s) {
        return Number.isFinite(s.lat) && Number.isFinite(s.lon) && !(s.lat === 0 && s.lon === 0);
    }

    // A sample in the engine's terms, its time on the page's clock
    // (performance.now()): the source's time, moved from the server's clock
    // to the page's by the offset between them.
    _snapshot(s, source) {
        return {
            lat: s.lat,
            lon: s.lon,
            alt: Number.isFinite(s.alt) ? s.alt : 0,
            heading: Number.isFinite(s.heading) ? s.heading : (Number.isFinite(s.track) ? s.track : null),
            track: Number.isFinite(s.track) ? s.track : null,
            speedKts: Number.isFinite(s.gs) ? s.gs : null,
            baroRate: Number.isFinite(s.baroRate) ? s.baroRate : 0,
            source,
            timestamp: this._pageTime(s.at, s.sentAt)
        };
    }

    // The page time of a server time. The server's clock and the page's may
    // differ (a phone, a VPN); every message tells when it left the server,
    // and the largest lead of that time over the page's at arrival, over the
    // last clockWindowSec seconds, is the offset less the shortest delivery
    // time. A time ahead of the page's now is taken as now.
    _pageTime(serverAt, sentAt) {
        const pageNow = performance.now();
        const wallNow = Date.now();
        if (Number.isFinite(sentAt)) this._noteServerClock(sentAt - wallNow, wallNow);
        if (!Number.isFinite(serverAt) || !Number.isFinite(this.serverClockOffsetMs)) return pageNow;
        const ageMs = (wallNow + this.serverClockOffsetMs) - serverAt;
        return pageNow - Math.max(0, ageMs);
    }

    _noteServerClock(leadMs, wallNow) {
        const sec = Math.floor(wallNow / 1000);
        const buckets = this._clockBuckets;
        const last = buckets[buckets.length - 1];
        if (last && last.sec === sec) {
            if (leadMs <= last.max) return;
            last.max = leadMs;
        } else {
            buckets.push({ sec, max: leadMs });
            while (buckets.length && buckets[0].sec <= sec - this.config.clockWindowSec) buckets.shift();
        }
        let max = -Infinity;
        for (const b of buckets) if (b.max > max) max = b.max;
        this.serverClockOffsetMs = max;
    }

    _ingest(hex, snapshot, status) {
        if (this.aircraftStates.size >= this.config.maxStates && !this.aircraftStates.has(hex)) {
            this._evictOldestState();
        }

        const now = performance.now();
        let state = this.aircraftStates.get(hex);
        if (!state) {
            state = this._createState(hex);
            this.aircraftStates.set(hex, state);
        }
        if (snapshot.source !== 'predicted') {
            state.lastObserved = snapshot;
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
            state.status = status || 'active';
            this._refreshFromRepeatedFix(state, snapshot);
            return state;
        }

        state.lastSeenAt = Date.now();
        state.status = status || 'active';
        state.lastSampleSource = snapshot.source || 'real';
        if (Number.isFinite(snapshot.heading)) {
            const baseAlpha = this.config.headingSmoothingAlpha;
            const alpha = snapshot.source === 'predicted' ? baseAlpha : Math.min(0.45, baseAlpha * 1.35);
            state.displayHeading = this._lerpAngleDeg(state.displayHeading, snapshot.heading, alpha);
        }

        if (previousReal && snapshot.source !== 'predicted') {
            const derivedVelocity = this._deriveVelocity(previousReal, snapshot);
            if (derivedVelocity) {
                state.velocity = this._blendVelocity(state.velocity, derivedVelocity);
            }
        }

        state.lastReal = snapshot;
        if (snapshot.source !== 'predicted') {
            state.lastRealFix = { lat: snapshot.lat, lon: snapshot.lon };
        }

        // The correction: the gap between where the aircraft is drawn now and
        // where the new sample, carried to now, puts it.
        state.correction = null;
        if (previousReal && previousRendered) {
            const fresh = this._predictedPose(state, snapshot, now, null);
            // Every gap fades, however small: below a threshold (30 m for
            // a real sample, 7 m for a predicted one, before) the aircraft
            // jumped to the sample in one frame, backwards as often as
            // forwards (replay bench: 0.83 % of the steps backwards, 0.03 %
            // without the thresholds).
            const distanceNm = this._distanceNm(previousRendered.lat, previousRendered.lon, fresh.lat, fresh.lon);
            if (distanceNm > 0) {
                // The gap closes at no more than half the aircraft's own
                // speed at the peak of the ease (1.5 times the mean rate):
                // closing 150 m in 450 ms made a 205 m/s aircraft sprint at
                // three times its speed for a few frames. A stationary
                // aircraft takes the base duration.
                const speedMps = state.velocity
                    ? Math.hypot(state.velocity.eastMps, state.velocity.northMps)
                    : 0;
                const gapM = distanceNm * 1852;
                const gentleMs = speedMps > 1 ? (3 * gapM / speedMps) * 1000 : this.config.mergeDurationMs;
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
                    dLat: previousRendered.lat - fresh.lat,
                    dLon: previousRendered.lon - fresh.lon,
                    dAlt: previousRendered.alt - fresh.alt,
                    startAt: now,
                    endAt: now + correctionDurationMs
                };
            }
        }

        if (!state.lastRendered) {
            state.lastRendered = { ...snapshot, timestamp: now };
        }
        return state;
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
            state.displayHeading = this._lerpAngleDeg(state.displayHeading, snapshot.heading, Math.min(0.45, this.config.headingSmoothingAlpha * 1.35));
        }
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
        if (state.velocity) this._secPredictedCount++;

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
        const horizon = baseHorizon * (Number.isFinite(this.config.predictionGain) ? this.config.predictionGain : 1);

        const dNorth = state.velocity.northMps * horizon;
        const dEast = state.velocity.eastMps * horizon;

        const lat = real.lat + (dNorth / 111320);
        const lon = real.lon + (dEast / (111320 * Math.cos((real.lat * Math.PI) / 180)));

        if (!Number.isFinite(lat) || !Number.isFinite(lon)) {
            return { lat: real.lat, lon: real.lon, alt: real.alt, heading };
        }

        const alt = real.alt + (state.velocity.verticalMps * horizon * 3.28084);

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
            // The sample the pose is carried from, observed or estimated
            // (upstream's name); lastSampleSource says which ('real' or
            // 'predicted').
            lastReal: null,
            lastSampleSource: 'real',
            // The last observed fix, to tell a repeated one (D79).
            lastRealFix: null,
            lastRendered: null,
            displayHeading: null,
            // { eastMps, northMps, verticalMps, headingDeg }: the station's
            // ground speed and track, smoothed.
            velocity: null,
            // The fading offset: { dLat, dLon, dAlt, startAt, endAt }.
            correction: null,
            // The last observation, its server send time (ms), and the time
            // of the last estimate applied (ms).
            lastObserved: null,
            lastObservedAt: null,
            lastEstimateAt: null
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
