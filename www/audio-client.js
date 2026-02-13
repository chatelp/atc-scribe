/**
 * Audio streaming client for Co-ATC.
 *
 * Responsibilities:
 * - Prepare and manage per-frequency HTMLAudioElement streams.
 * - Maintain Web Audio analyser pipelines for UI visualization.
 * - Provide mute/unmute and playback orchestration helpers.
 * - Track time since last significant audio per frequency.
 */
class AudioClient {
    /**
     * @param {Object} store Alpine store instance.
     */
    constructor(store) {
        this.store = store;
        this.audioContext = null;
        this.audioElements = {};
        this.audioAnalysers = {};
        this.audioDataArrays = {};
        this.visualizationFrameIds = {};
        this.sourceNodes = {};
        this.userSetVolumes = {};
        this.lastSignificantAudioTime = {};
        this.secondsSinceLastAudio = {};

        this.mutedVolume = 0.01;
        this.defaultVolume = 1.0;
        this.visualizationTargetFps = 30;
        this.significantAudioThresholdUnmuted = 0.10;
        this.significantAudioThresholdMuted = 0.02;
        this.visualizerMultiplierUnmuted = 150;
        this.visualizerMultiplierMutedFactor = 5;
    }

    /**
     * Initialize and cache the AudioContext.
     * @returns {AudioContext|null}
     */
    initAudioContext() {
        if (!this.audioContext) {
            try {
                this.audioContext = new (window.AudioContext || window.webkitAudioContext)();
                console.log("AudioContext initialized.");
            } catch (e) {
                console.error("Web Audio API is not supported.", e);
            }
        }
        return this.audioContext;
    }

    /**
     * Ensure a frequency stream is prepared with element, source URL, and analyser pipeline.
     * @param {{id: string|number, stream_port?: number|string, stream_url?: string}} frequency
     */
    prepareFrequency(frequency) {
        const frequencyId = this._normalizeFrequencyId(frequency?.id);
        if (!frequencyId) {
            console.error('prepareFrequency: missing frequency.id');
            return;
        }

        if (this.audioElements[frequencyId]?.element) {
            return;
        }

        console.log(`Preparing frequency: ${frequencyId}`);
        this.initAudioContext();

        const audioElement = document.createElement('audio');
        audioElement.crossOrigin = 'anonymous';
        audioElement.preload = 'metadata';
        audioElement.playsInline = true;

        this.userSetVolumes[frequencyId] = this.userSetVolumes[frequencyId] || this.defaultVolume;

        const streamUrl = this._buildStreamUrl(frequency);
        if (!streamUrl) {
            console.error(`prepareFrequency: invalid stream URL for ${frequencyId}`);
            return;
        }

        audioElement.addEventListener('error', (e) => {
            const message = e.target && e.target.error ? e.target.error.message : 'Unknown error';
            console.error(`Audio error for ${frequencyId}:`, message);
        });
        audioElement.addEventListener('playing', () => {
            if (!this.visualizationFrameIds[frequencyId]) {
                this.startVisualization(frequencyId);
            }
        });
        audioElement.addEventListener('pause', () => {
            this.cleanupVisualization(frequencyId);
        });

        this.audioElements[frequencyId] = {
            element: audioElement,
            intendedSrc: streamUrl,
            isPrepared: true
        };
        
        this.setupVisualization(frequencyId, audioElement);
    }

    /**
     * Connect and start playback for a prepared frequency stream.
     * @param {{id: string|number}} frequency
     */
    connectToFrequency(frequency) {
        const frequencyId = this._normalizeFrequencyId(frequency?.id);
        if (!frequencyId) {
            console.error('connectToFrequency: missing frequency.id');
            return;
        }

        if (!this.audioElements[frequencyId]?.element) {
            console.warn(`Audio element for ${frequencyId} not found during connect. Preparing now.`);
            this.prepareFrequency(frequency);
        }
        
        const audioInfo = this.audioElements[frequencyId];
        if (!audioInfo || !audioInfo.element || !audioInfo.intendedSrc) {
            console.error(`Audio info incomplete for frequency: ${frequencyId}. Cannot connect.`);
            return;
        }

        const audioElement = audioInfo.element;
        const intendedSrc = audioInfo.intendedSrc;

        audioElement.volume = this.store.unmutedFrequencies.has(frequencyId)
            ? (this.userSetVolumes[frequencyId] || this.defaultVolume)
            : this.mutedVolume;

        if (audioElement.currentSrc !== intendedSrc) {
            audioElement.src = intendedSrc;
        }

        if (!this.visualizationFrameIds[frequencyId] && this.audioAnalysers[frequencyId]) {
            this.startVisualization(frequencyId);
        }

        if (audioElement.readyState === 0 || audioElement.currentSrc !== intendedSrc) {
            audioElement.load();
        }

        if (audioElement.paused) {
            const playPromise = audioElement.play();
            
            if (playPromise !== undefined) {
                playPromise.catch(error => {
                    if (error.name !== 'AbortError') {
                        console.error(`Error invoking play for ${frequencyId}:`, error);
                    }
                });
            }
        }
    }

    /**
     * Start playback attempts across all known radio frequencies.
     */
    startAllRadios() {
        if (this.store.radiosStarted) {
            return;
        }

        this.store.radiosStarted = true; 
        const context = this.initAudioContext();
        if (!context) {
            console.error('startAllRadios: AudioContext unavailable');
            return;
        }

        const resumeContextAndPlay = () => {
            this.store.audioFrequencies.forEach(freq => {
                this.connectToFrequency(freq);
            });
        };

        if (context.state === 'suspended') {
            context.resume().then(() => {
                resumeContextAndPlay();
            }).catch(e => {
                console.error("Error resuming audio context:", e);
                resumeContextAndPlay(); 
            });
        } else {
            resumeContextAndPlay();
        }
    }

    /**
     * Create analyser pipeline for a given frequency.
     * @param {string} frequencyId
     * @param {HTMLAudioElement} audioElement
     */
    setupVisualization(frequencyId, audioElement) {
        if (!this.audioContext) {
            return;
        }

        this._safeDisconnectNode(this.sourceNodes[frequencyId]);
        this._safeDisconnectNode(this.audioAnalysers[frequencyId]);
        delete this.sourceNodes[frequencyId];
        delete this.audioAnalysers[frequencyId];

        try {
            const sourceNode = this.audioContext.createMediaElementSource(audioElement);
            this.sourceNodes[frequencyId] = sourceNode;

            const analyserNode = this.audioContext.createAnalyser();
            analyserNode.fftSize = 256;
            analyserNode.smoothingTimeConstant = 0.5;
            this.audioAnalysers[frequencyId] = analyserNode;

            this.audioDataArrays[frequencyId] = new Uint8Array(analyserNode.frequencyBinCount);

            sourceNode.connect(analyserNode);
            analyserNode.connect(this.audioContext.destination);
        } catch (e) {
            console.error(`Error setting up visualization for ${frequencyId}:`, e);
            this._safeDisconnectNode(this.sourceNodes[frequencyId]);
            this._safeDisconnectNode(this.audioAnalysers[frequencyId]);
            delete this.sourceNodes[frequencyId];
            delete this.audioAnalysers[frequencyId];
        }
    }

    /**
     * Start bar visualization animation loop for a frequency.
     * @param {string} frequencyId
     */
    startVisualization(frequencyId) {
        if (this.visualizationFrameIds[frequencyId]) return;

        let lastFrameTime = 0;
        const frameInterval = 1000 / this.visualizationTargetFps;
        
        const renderFrame = (currentTime) => {
            if (currentTime - lastFrameTime < frameInterval) {
                this.visualizationFrameIds[frequencyId] = requestAnimationFrame(renderFrame);
                return;
            }
            lastFrameTime = currentTime;
            
            const analyser = this.audioAnalysers[frequencyId];
            const dataArray = this.audioDataArrays[frequencyId];
            
            if (!analyser || !dataArray) {
                this.cleanupVisualization(frequencyId);
                return;
            }
            
            try {
                analyser.getByteFrequencyData(dataArray);
            } catch (e) {
                for (let i = 0; i < dataArray.length; i++) {
                    dataArray[i] = 0;
                }
            }
            
            let totalSum = 0;
            let totalPoints = 0;
            const maxBin = Math.min(dataArray.length, 40);
            for (let j = 1; j < maxBin; j++) { 
                const weight = 1 - (j / maxBin * 0.5);
                totalSum += dataArray[j] * weight;
                totalPoints += weight;
            }
            
            const audioLevel = totalPoints > 0 ? (totalSum / totalPoints) / 255 : 0;

            const isUnmuted = this.store.unmutedFrequencies.has(frequencyId);
            const significantAudioThreshold = isUnmuted
                ? this.significantAudioThresholdUnmuted
                : this.significantAudioThresholdMuted;
            const visualizerMultiplier = isUnmuted
                ? this.visualizerMultiplierUnmuted
                : (this.visualizerMultiplierUnmuted * this.visualizerMultiplierMutedFactor);

            if (audioLevel >= significantAudioThreshold) {
                this.lastSignificantAudioTime[frequencyId] = Date.now();
            } else if (!this.lastSignificantAudioTime[frequencyId]) {
                this.lastSignificantAudioTime[frequencyId] = Date.now();
            }

            const widthPercentage = Math.min(100, audioLevel * visualizerMultiplier);
            
            const barElement = document.getElementById(`vis-bar-${frequencyId}`);
            if (barElement) {
                const currentWidth = parseFloat(barElement.style.width) || 0;
                const smoothingFactor = 0.3;
                const newWidth = (currentWidth * smoothingFactor) + (widthPercentage * (1 - smoothingFactor));
                
                barElement.style.width = newWidth + '%';
                barElement.style.backgroundColor = isUnmuted ? '#4CAF50' : '#888888';
                
                barElement.style.opacity = '1';
            }
            
            this.visualizationFrameIds[frequencyId] = requestAnimationFrame(renderFrame);
        };
        
        this.visualizationFrameIds[frequencyId] = requestAnimationFrame(renderFrame);
    }

    /**
     * Stop visualization animation for a frequency.
     * @param {string} frequencyId
     */
    cleanupVisualization(frequencyId) {
        if (this.visualizationFrameIds[frequencyId]) {
            cancelAnimationFrame(this.visualizationFrameIds[frequencyId]);
            delete this.visualizationFrameIds[frequencyId];
        }
        if (this.secondsSinceLastAudio[frequencyId] !== '--s') {
            this.secondsSinceLastAudio[frequencyId] = '--s'; 
        }
    }

    /**
     * Toggle mute state for a frequency while preserving user-set volume.
     * @param {{id: string|number}} frequency
     */
    toggleMute(frequency) {
        if (!this.store.radiosStarted) {
            this.startAllRadios();
        }

        const frequencyId = this._normalizeFrequencyId(frequency?.id);
        if (!frequencyId) {
            console.error('toggleMute: missing frequency.id');
            return;
        }

        let audioInfo = this.audioElements[frequencyId];
        if (!audioInfo || !audioInfo.element) {
            this.prepareFrequency(frequency);
            audioInfo = this.audioElements[frequencyId];
            if (!audioInfo || !audioInfo.element) {
                console.error(`No audio element found for frequency: ${frequencyId} in toggleMute.`);
                return;
            }
        }
        const audioElement = audioInfo.element;

        const isCurrentlyUnmuted = this.store.unmutedFrequencies.has(frequencyId);

        if (isCurrentlyUnmuted) {
            this.userSetVolumes[frequencyId] = audioElement.volume > this.mutedVolume ? audioElement.volume : this.defaultVolume;
            audioElement.volume = this.mutedVolume;
            this.store.unmutedFrequencies.delete(frequencyId);
        } else {
            audioElement.volume = this.userSetVolumes[frequencyId] || this.defaultVolume;
            this.store.unmutedFrequencies.add(frequencyId);
        }

        if (audioElement.paused && this.store.unmutedFrequencies.has(frequencyId) && this.store.radiosStarted) {
            setTimeout(() => {
                if (audioElement.paused && this.store.unmutedFrequencies.has(frequencyId)) {
                    audioElement.play().catch(err => {
                        if (err.name !== 'AbortError') {
                            console.error(`Error playing audio for ${frequencyId} after unmute:`, err);
                        }
                    });
                }
            }, 100);
        }
    }

    /**
     * Release all resources for a frequency.
     * @param {string} frequencyId
     */
    cleanupFrequency(frequencyId) {
        this.cleanupVisualization(frequencyId);

        this._safeDisconnectNode(this.audioAnalysers[frequencyId]);
        this._safeDisconnectNode(this.sourceNodes[frequencyId]);

        delete this.audioAnalysers[frequencyId];
        delete this.sourceNodes[frequencyId];

        if (this.audioDataArrays[frequencyId]) {
            delete this.audioDataArrays[frequencyId];
        }

        if (this.audioElements[frequencyId]) {
            const audioElement = this.audioElements[frequencyId].element;
            if (audioElement) {
                audioElement.pause();
                audioElement.src = '';
            }
            delete this.audioElements[frequencyId];
        }

        delete this.lastSignificantAudioTime[frequencyId];
        delete this.secondsSinceLastAudio[frequencyId];
    }

    /**
     * Update `seconds since last audio` values in the provided store-backed object.
     * @param {Object} storeSecondsSinceLastAudio
     */
    updateSecondsSinceLastAudio(storeSecondsSinceLastAudio) {
        if (!storeSecondsSinceLastAudio) {
            console.warn("AudioClient: storeSecondsSinceLastAudio object not provided for update.");
            return;
        }

        Object.keys(this.audioElements).forEach(frequencyId => {
            if (this.audioElements[frequencyId]?.element && this.lastSignificantAudioTime[frequencyId]) {
                const seconds = Math.floor((Date.now() - this.lastSignificantAudioTime[frequencyId]) / 1000);
                storeSecondsSinceLastAudio[frequencyId] = `${seconds}s`;
            } else if (this.audioElements[frequencyId]?.element && !this.lastSignificantAudioTime[frequencyId]) {
                storeSecondsSinceLastAudio[frequencyId] = '--s'; 
            }
        });
    }

    /**
     * Play the Airbus retard callout sound.
     */
    playRetardSound() {
        if (!this.audioContext) {
            this.initAudioContext();
        }
        if (!this.audioContext) {
            console.error("AudioContext could not be initialized. Cannot play retard sound.");
            return;
        }

        if (this.audioContext.state === 'suspended') {
            this.audioContext.resume().then(() => {
                this._playRetardSoundInternal();
            }).catch(e => {
                console.error("Error resuming AudioContext for retard sound:", e);
            });
        } else {
            this._playRetardSoundInternal();
        }
    }

    /**
     * Internal helper for playing the static retard sound file.
     */
    _playRetardSoundInternal() {
        const retardSound = new Audio('/sounds/airbus_retard.mp3');
        retardSound.play()
            .catch(error => {
                console.error("Error playing airbus_retard.mp3:", error);
            });
    }

    /**
     * Build a frequency stream URL and attach the client id.
     * @param {{stream_port?: number|string, stream_url?: string}} frequency
     * @returns {string|null}
     */
    _buildStreamUrl(frequency) {
        const streamPath = typeof frequency?.stream_url === 'string' ? frequency.stream_url : '';
        if (!streamPath) {
            return null;
        }

        const streamPort = frequency.stream_port || window.location.port;
        const base = `${window.location.protocol}//${window.location.hostname}:${streamPort}`;

        let url;
        try {
            url = new URL(streamPath, base);
        } catch {
            return null;
        }

        if (url.href.includes('CLIENT_ID')) {
            return url.href.replace('CLIENT_ID', encodeURIComponent(this.store.clientID));
        }

        if (!url.searchParams.has('id')) {
            url.searchParams.set('id', this.store.clientID);
        }

        return url.toString();
    }

    /**
     * Disconnect a Web Audio node safely.
     * @param {AudioNode|undefined|null} node
     */
    _safeDisconnectNode(node) {
        if (!node) {
            return;
        }
        try {
            node.disconnect();
        } catch {
            // no-op
        }
    }

    /**
     * Normalize frequency id values to string keys.
     * @param {string|number|undefined|null} frequencyId
     * @returns {string|null}
     */
    _normalizeFrequencyId(frequencyId) {
        if (frequencyId === undefined || frequencyId === null) {
            return null;
        }
        return String(frequencyId);
    }
}

// Export the AudioClient class
window.AudioClient = AudioClient; 