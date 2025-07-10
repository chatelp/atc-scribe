// Audio Client for Co-ATC
class AudioClient {
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
    }

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

    prepareFrequency(frequency) {
        if (this.audioElements[frequency.id]?.element) {
            console.log(`Frequency ${frequency.id} already prepared.`);
            return;
        }
        console.log(`Preparing frequency: ${frequency.id}`);
        this.initAudioContext();

        const audioElement = document.createElement('audio');
        audioElement.crossOrigin = "anonymous";
        audioElement.preload = "metadata";

        if (this.userSetVolumes === undefined) this.userSetVolumes = {};
        this.userSetVolumes[frequency.id] = this.userSetVolumes[frequency.id] || 1.0;

        let streamUrl = frequency.stream_url;
        if (streamUrl.includes('CLIENT_ID')) {
            streamUrl = streamUrl.replace('CLIENT_ID', this.store.clientID);
        } else {
            streamUrl = `${streamUrl}?id=${this.store.clientID}`;
        }

        audioElement.addEventListener('error', (e) => {
            console.error(`Audio error for ${frequency.id}:`, e.target.error ? e.target.error.message : 'Unknown error');
        });
        audioElement.addEventListener('playing', () => {
            console.log(`Audio playing for ${frequency.id}`);
            if (!this.visualizationFrameIds[frequency.id]) {
                this.startVisualization(frequency.id);
            }
        });
        audioElement.addEventListener('pause', () => {
            console.log(`Audio paused for ${frequency.id}`);
        });

        this.audioElements[frequency.id] = {
            element: audioElement,
            intendedSrc: streamUrl,
            isPrepared: true
        };
        
        this.setupVisualization(frequency.id, audioElement);
    }

    connectToFrequency(frequency) {
        if (!this.audioElements[frequency.id]?.element) {
            console.warn(`Audio element for ${frequency.id} not found during connect. Preparing now.`);
            this.prepareFrequency(frequency);
        }
        
        const audioInfo = this.audioElements[frequency.id];
        if (!audioInfo || !audioInfo.element || !audioInfo.intendedSrc) {
            console.error(`Audio info incomplete for frequency: ${frequency.id}. Cannot connect.`);
            return;
        }

        const audioElement = audioInfo.element;
        const intendedSrc = audioInfo.intendedSrc;

        // Set volume first
        audioElement.volume = this.store.unmutedFrequencies.has(frequency.id) ? (this.userSetVolumes[frequency.id] || 1.0) : 0.01;

        if (audioElement.currentSrc !== intendedSrc) {
            console.log(`Setting src for ${frequency.id} to ${intendedSrc}`);
            audioElement.src = intendedSrc;
        }

        if (!this.visualizationFrameIds[frequency.id] && this.audioAnalysers[frequency.id]) {
            this.startVisualization(frequency.id);
        }
        
        console.log(`ConnectToFrequency: Attempting to load and play ${frequency.id}`);
        
        // Avoid calling load() if the element is already loading or has loaded the correct source
        if (audioElement.readyState === 0 || audioElement.currentSrc !== intendedSrc) {
            audioElement.load();
        }
        
        // Only attempt to play if not already playing or attempting to play
        if (audioElement.paused && audioElement.readyState !== 1) { // readyState 1 = HAVE_METADATA (loading)
            const playPromise = audioElement.play();
            
            if (playPromise !== undefined) {
                playPromise.then(() => {
                    // 'playing' event in prepareFrequency handles the main playing log
                }).catch(error => {
                    // Only log if it's not an AbortError (which is expected in some cases)
                    if (error.name !== 'AbortError') {
                        console.error(`Error invoking play for ${frequency.id}:`, error);
                    }
                });
            }
        }
    }

    startAllRadios() {
        if (this.store.radiosStarted) {
            console.log("Radios play command already issued.");
            return;
        }
        console.log("Issuing play command for all prepared radio frequencies...");
        this.store.radiosStarted = true; 
        this.initAudioContext();

        const resumeContextAndPlay = () => {
            this.store.audioFrequencies.forEach(freq => {
                this.connectToFrequency(freq);
            });
            console.log("Finished issuing play commands for all radio frequencies.");
        };

        if (this.audioContext.state === 'suspended') {
            this.audioContext.resume().then(() => {
                console.log("AudioContext resumed successfully");
                resumeContextAndPlay();
            }).catch(e => {
                console.error("Error resuming audio context:", e);
                resumeContextAndPlay(); 
            });
        } else {
            resumeContextAndPlay();
        }
    }

    setupVisualization(frequencyId, audioElement) {
        if (!this.audioContext) {
            console.warn("AudioContext not initialized. Skipping visualization setup for", frequencyId);
            return;
        }

        if (this.sourceNodes[frequencyId]) {
            try { this.sourceNodes[frequencyId].disconnect(); } catch (e) { /* ignore */ }
            delete this.sourceNodes[frequencyId];
        }
        if (this.audioAnalysers[frequencyId]) {
            try { this.audioAnalysers[frequencyId].disconnect(); } catch (e) { /* ignore */ }
            delete this.audioAnalysers[frequencyId];
        }

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
            
            console.log(`Visualization (analyser direct to destination) set up for frequency: ${frequencyId}`);
        } catch (e) {
            console.error(`Error setting up visualization for ${frequencyId}:`, e);
            if (this.sourceNodes[frequencyId]) { try {this.sourceNodes[frequencyId].disconnect();} catch(err){} delete this.sourceNodes[frequencyId];}
            if (this.audioAnalysers[frequencyId]) { try {this.audioAnalysers[frequencyId].disconnect();} catch(err){} delete this.audioAnalysers[frequencyId];}
        }
    }

    startVisualization(frequencyId) {
        if (this.visualizationFrameIds[frequencyId]) return;
        
        console.log(`Starting visualization for frequency: ${frequencyId}`);
        
        let lastFrameTime = 0;
        const targetFPS = 30; // Limit to 30 FPS instead of 60
        const frameInterval = 1000 / targetFPS;
        
        const generateDummyData = () => {
            const result = new Uint8Array(128);
            for (let i = 0; i < result.length; i++) {
                if (i > 5 && i < 40) {
                    result[i] = Math.random() * 50;
                } else {
                    result[i] = Math.random() * 20;
                }
            }
            return result;
        };
        
        const renderFrame = (currentTime) => {
            // Throttle frame rate
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
                const dummyData = generateDummyData();
                for (let i = 0; i < Math.min(dataArray.length, dummyData.length); i++) {
                    dataArray[i] = dummyData[i];
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

            let significantAudioThreshold = 0.10; // Default threshold
            
            const unmutedMultiplier = 150; 
            let visualizerMultiplier = unmutedMultiplier;
            
            if (!this.store.unmutedFrequencies.has(frequencyId)) { 
                visualizerMultiplier = unmutedMultiplier * 5;
                significantAudioThreshold = 0.02; // Lower threshold for muted frequencies
            }

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
                
                if (this.store.unmutedFrequencies.has(frequencyId)) {
                    barElement.style.backgroundColor = '#4CAF50';
                } else {
                    barElement.style.backgroundColor = '#888888';
                }
                
                barElement.style.opacity = '1';
            }
            
            this.visualizationFrameIds[frequencyId] = requestAnimationFrame(renderFrame);
        };
        
        this.visualizationFrameIds[frequencyId] = requestAnimationFrame(renderFrame);
    }

    cleanupVisualization(frequencyId) {
        if (this.visualizationFrameIds[frequencyId]) {
            cancelAnimationFrame(this.visualizationFrameIds[frequencyId]);
            delete this.visualizationFrameIds[frequencyId];
        }
        if (this.secondsSinceLastAudio[frequencyId] !== '--s') {
            this.secondsSinceLastAudio[frequencyId] = '--s'; 
        }
    }

    toggleMute(frequency) {
        if (!this.store.radiosStarted) {
            this.startAllRadios();
        }

        const audioInfo = this.audioElements[frequency.id];
        if (!audioInfo || !audioInfo.element) {
            console.error(`No audio element found for frequency: ${frequency.id} in toggleMute.`);
            this.prepareFrequency(frequency);
            setTimeout(() => this.toggleMute(frequency), 250); 
            return;
        }
        const audioElement = audioInfo.element;

        const isCurrentlyUnmuted = this.store.unmutedFrequencies.has(frequency.id);

        if (isCurrentlyUnmuted) {
            this.userSetVolumes[frequency.id] = audioElement.volume > 0.01 ? audioElement.volume : 1.0;
            audioElement.volume = 0.01;
            this.store.unmutedFrequencies.delete(frequency.id);
            console.log(`Muted frequency: ${frequency.id} (audioElement.volume: 0.01)`);
        } else {
            audioElement.volume = this.userSetVolumes[frequency.id] || 1.0;
            this.store.unmutedFrequencies.add(frequency.id);
            console.log(`Unmuted frequency: ${frequency.id} (audioElement.volume: ${audioElement.volume.toFixed(2)})`);
        }

        if (audioElement.paused && this.store.unmutedFrequencies.has(frequency.id) && this.store.radiosStarted) {
            console.log(`Audio for ${frequency.id} was paused, attempting to play after unmute.`);
            setTimeout(() => {
                if (audioElement.paused && this.store.unmutedFrequencies.has(frequency.id)) {
                    audioElement.play().catch(err => {
                        console.error(`Error playing audio for ${frequency.id} after unmute:`, err);
                    });
                }
            }, 100);
        }
    }

    cleanupFrequency(frequencyId) {
        if (this.visualizationFrameIds[frequencyId]) {
            cancelAnimationFrame(this.visualizationFrameIds[frequencyId]);
            delete this.visualizationFrameIds[frequencyId];
        }
        if (this.audioAnalysers[frequencyId]) {
            try { this.audioAnalysers[frequencyId].disconnect(); } catch(e) { /* ignore */ }
            delete this.audioAnalysers[frequencyId];
        }
        if (this.sourceNodes[frequencyId]) {
            try { this.sourceNodes[frequencyId].disconnect(); } catch(e) { /* ignore */ }
            delete this.sourceNodes[frequencyId];
        }
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

    updateSecondsSinceLastAudio(storeSecondsSinceLastAudio) {
        if (!storeSecondsSinceLastAudio) {
            console.warn("AudioClient: storeSecondsSinceLastAudio object not provided for update.");
            return;
        }
        Object.keys(this.audioElements).forEach(frequencyId => {
            if (this.audioElements[frequencyId]?.element && this.lastSignificantAudioTime[frequencyId]) {
                const seconds = Math.floor((Date.now() - this.lastSignificantAudioTime[frequencyId]) / 1000);
                // Update the store's object directly
                storeSecondsSinceLastAudio[frequencyId] = `${seconds}s`;
            } else if (this.audioElements[frequencyId]?.element && !this.lastSignificantAudioTime[frequencyId]) {
                // Update the store's object directly
                storeSecondsSinceLastAudio[frequencyId] = '--s'; 
            }
        });
    }

    playRetardSound() {
        if (!this.audioContext) {
            this.initAudioContext();
        }
        // Check if AudioContext is successfully initialized
        if (!this.audioContext) {
            console.error("AudioContext could not be initialized. Cannot play retard sound.");
            return;
        }
        // Resume AudioContext if it's suspended (e.g., due to browser autoplay policies)
        if (this.audioContext.state === 'suspended') {
            this.audioContext.resume().then(() => {
                console.log("AudioContext resumed for retard sound.");
                this._playRetardSoundInternal();
            }).catch(e => {
                console.error("Error resuming AudioContext for retard sound:", e);
            });
        } else {
            this._playRetardSoundInternal();
        }
    }

    _playRetardSoundInternal() {
        const retardSound = new Audio('/sounds/airbus_retard.mp3');
        retardSound.play()
            .then(() => {
                console.log("Playing airbus_retard.mp3");
            })
            .catch(error => {
                console.error("Error playing airbus_retard.mp3:", error);
            });
    }
}

// Export the AudioClient class
window.AudioClient = AudioClient; 