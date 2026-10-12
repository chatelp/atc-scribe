// The transcripts: what was said on each frequency, who said it and to which
// aircraft, the proof of the match, the words in plain language.
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
        transcriptions: [], // Array of transcription messages
        transcriptionSearchTerm: '', // For searching transcriptions
        originalTranscriptions: {}, // Store original transcriptions before filtering
        frequencyTranscriptions: {}, // Stores transcriptions per frequency_id
        transcriptionViewerVisible: {}, // Stores visibility state for each frequency's viewer
        unreadTranscriptions: {}, // Unread count per frequency (only live WS messages while viewer is closed)
        _readDividerId: {}, // ID of the first already-read message when viewer opens with unread
        
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

            // Attach the match to the aircraft straight away. The server carries
            // the same fact on the next bulk refresh, but waiting for it would mean
            // an aircraft the controller has just addressed stays hidden behind the
            // voice filter for as long as that refresh takes.
            if (data.callsign) {
                const target = Object.values(this.aircraft).find(
                    a => (a.flight || '').trim() === data.callsign);
                if (target) {
                    const previous = target.voice ? target.voice.transmissions : 0;
                    target.voice = {
                        transmissions: previous + 1,
                        last_heard: data.timestamp,
                        last_text: data.text
                    };
                    this.applyFilters();
                }
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
                    callsign_source: data.callsign_source,
                    callsign_evidence: data.callsign_evidence,
                    content_second: data.content_second,
                    plain: data.plain,
                    match_score: data.match_score,
                    match_reason: data.match_reason,
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
                        callsign_source: data.callsign_source,
                        callsign_evidence: data.callsign_evidence,
                        content_second: data.content_second,
                        plain: data.plain,
                        match_score: data.match_score,
                        match_reason: data.match_reason,
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
                            callsign_source: data.callsign_source,
                            callsign_evidence: data.callsign_evidence,
                            content_second: data.content_second,
                            plain: data.plain,
                            match_score: data.match_score,
                            match_reason: data.match_reason,
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

        // Handle clearance issued messages
        handleClearanceIssued(data) {
            console.log('Clearance issued:', data);
            
            // Update aircraft clearances if currently selected
            const selectedAircraft = this.selectedAircraft;
            if (selectedAircraft && selectedAircraft.flight === data.callsign) {
                // Refresh aircraft details to show new clearance
                this.refreshSelectedAircraftDetails();
            }
            
            // Log to console for debugging
            console.log(`Clearance issued: ${data.callsign} → ${data.clearance_type.toUpperCase()} CLEARANCE`);
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

        // A transcript as HTML: the words that named its aircraft in bold, kept
        // when the match was made (callsign_evidence, UTF-16 offsets into this
        // very text), and the search term underlined. Everything is escaped here:
        // the text comes from a speech model and must never reach x-html raw.
        transcriptHtml(text, spans) {
            if (!text) return '';
            const esc = str => str.replace(/[&<>"']/g, c =>
                ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
            const term = (this.transcriptionSearchTerm || '').trim();
            const re = term ? new RegExp('(' + term.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + ')', 'gi') : null;
            const plain = str => re
                ? str.split(re).map((piece, i) => i % 2
                    ? '<span class="border-b border-red-400">' + esc(piece) + '</span>'
                    : esc(piece)).join('')
                : esc(str);
            // Offsets that do not fit this text name words in another one: shown
            // without highlight rather than with a wrong one.
            const fits = Array.isArray(spans) && spans.length > 0 && spans.every(sp =>
                Array.isArray(sp) && sp[0] >= 0 && sp[0] < sp[1] && sp[1] <= text.length);
            if (!fits) return plain(text);
            let out = '', at = 0;
            for (const [a, b] of [...spans].sort((x, y) => x[0] - y[0])) {
                if (a < at) continue;
                out += plain(text.slice(at, a)) +
                    '<b class="font-bold text-amber-300">' + plain(text.slice(a, b)) + '</b>';
                at = b;
            }
            return out + plain(text.slice(at));
        },

        // Before a transmission's plain words: who gives them.
        plainLead(t) {
            if (t.speaker_type === 'ATC') return 'Controller:';
            if (t.speaker_type === 'PILOT') return 'Pilot:';
            return '';
        },

        plainHelp(t) {
            const who = t.speaker_type === 'ATC' ? 'What the controller says'
                : t.speaker_type === 'PILOT' ? 'What the pilot reads back or reports'
                : 'What the transmission says';
            return `${who}, in plain words, as the phraseology grammar reads it from the transcript: green for a clearance, amber for a level, heading, speed, frequency or setting. A misheard word can make it wrong: listen again to check.`;
        },

        // The aircraft in sight with this callsign, or null.
        aircraftByCallsign(callsign) {
            const cs = (callsign || '').trim().toUpperCase();
            if (!cs) return null;
            for (const a of Object.values(this.aircraft)) {
                if ((a.flight || '').trim().toUpperCase() === cs) return a;
            }
            return null;
        },

        speakerHelp(type) {
            if (type === 'ATC') return 'The controller is speaking. The aircraft named after the arrow is the one spoken to.';
            if (type === 'PILOT') return 'A pilot is speaking, from the aircraft named next to it.';
            return 'Who is speaking could not be told from this transmission.';
        },

        transcriptAircraftHelp(t) {
            const how = t.callsign_source === 'fr'
                ? 'It was recognised in the French reading of the transmission'
                : 'It was recognised in the transmission';
            const where = this.aircraftByCallsign(t.callsign)
                ? 'Click to show it on the map and open its panel.'
                : 'It is no longer in range of the receiver.';
            return `${t.callsign}: the aircraft this transmission was matched to. ${how}; the words in bold are the ones that matched its callsign. ${this.matchProofText(t)} ${where}`;
        },

        // The proof of a match (D72), in plain words: why the matcher chose
        // this aircraft, and how strong the clues are -- not a probability,
        // none has been measured.
        matchReasons(t) {
            const said = {
                'digits exact': 'its flight number was heard in full',
                'digits suffix': 'the end of its flight number was heard',
                'digits off by one': 'a number one digit away from its flight number was heard',
                'first digits': 'the first digits of its flight number followed its airline\'s name',
                'flight part exact': 'the number and letters of its callsign were heard',
                'flight part, last letter missing': 'its callsign was heard, the last letter missing',
                'last letters of an aircraft just heard': 'the last letters of an aircraft just spoken to on this frequency',
                'only aircraft of the named airline just heard': 'the only aircraft of the named airline just spoken to here',
                'last two digits of an aircraft just heard': 'the last two digits of an aircraft just spoken to here',
                'digits repeated': 'the number was said more than once',
                'letters': 'the letters of its callsign were heard',
                'operator named': 'its airline was named',
                'altitude agrees': 'the level said matches its ADS-B altitude, within 1,500 ft',
                'altitude contradicts': 'the level said is far from its ADS-B altitude (counts against it)',
                'cruising, not landing': 'a landing or approach clearance though it is cruising (counts against it)',
                'airborne, not departing': 'a takeoff clearance though it is airborne (counts against it)',
                'in sector': 'it was inside the area this frequency covers',
            };
            return (t.match_reason || '').split(' + ').map((r) => r.trim()).filter(Boolean).map((r) => said[r] || r);
        },

        matchStrength(t) {
            if (!t || !t.match_reason) return null;
            const r = new Set(t.match_reason.split(' + ').map((x) => x.trim()));
            const exact = r.has('digits exact') || r.has('flight part exact');
            const named = exact || ['digits suffix', 'digits off by one', 'first digits', 'flight part, last letter missing',
                'last letters of an aircraft just heard', 'only aircraft of the named airline just heard',
                'last two digits of an aircraft just heard'].some((x) => r.has(x));
            // "in sector" holds for every match on a sectored frequency: listed, not counted.
            const support = ['operator named', 'altitude agrees', 'digits repeated', 'letters'].filter((x) => r.has(x)).length;
            const against = ['altitude contradicts', 'cruising, not landing', 'airborne, not departing'].some((x) => r.has(x));
            if (exact && support > 0 && !against) return 'strong';
            if ((exact || (named && support > 0)) && !against) return 'medium';
            return 'weak';
        },

        matchProofText(t) {
            const strength = this.matchStrength(t);
            if (!strength) return 'Why it was chosen was not kept: matched before 11/10.';
            const reasons = this.matchReasons(t);
            const what = { strong: 'strong: an exact callsign and something else that agrees',
                medium: 'medium: one clue, or a partial callsign with support',
                weak: 'weak: a partial clue, or something against it' }[strength];
            const score = typeof t.match_score === 'number' ? ` Score ${t.match_score.toFixed(2)}, 0.60 being the least accepted.` : '';
            return `Chosen because ${reasons.join('; ')}. Clues ${what} -- a count of clues, not a measured probability.${score}`;
        },

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
            
            // Every open transcript is laid out again: side by side, since the
            // tiles under them are narrower than a readable transcript.
            this.layoutTranscriptionViewers();
        },

        // The open transcripts, left to right in the order of their tiles, each
        // 320 px wide and never over its neighbour: a transcript starts at its
        // tile, or right after the previous transcript when that is further.
        // The last ones are pushed back onto the screen if they would leave it
        // (audit 4.1: tiles are 160 px, transcripts 320).
        layoutTranscriptionViewers() {
            const width = 320;
            const gap = 8;
            let nextLeft = 4;
            const placed = [];
            for (const freq of this.audioFrequencies) {
                if (!this.transcriptionViewerVisible[freq.id]) continue;
                const tile = document.querySelector(`[data-freq-id="${freq.id}"]`);
                const viewer = document.querySelector(`[data-viewer-id="${freq.id}"]`);
                if (!tile || !viewer) continue;
                const rect = tile.getBoundingClientRect();
                const left = Math.max(nextLeft, rect.left);
                placed.push({ viewer, left, top: rect.top });
                nextLeft = left + width + gap;
            }
            // Past the right edge: shift the row back, keeping the order.
            const overflow = nextLeft - gap + 4 - window.innerWidth;
            const shift = overflow > 0 ? overflow : 0;
            for (const p of placed) {
                p.viewer.style.left = `${Math.max(4, p.left - shift)}px`;
                p.viewer.style.width = `${width}px`;
                p.viewer.style.bottom = `${window.innerHeight - p.top + 8}px`;
                p.viewer.style.transition = 'none';
                p.viewer.style.transform = 'none';
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
    }));
})();
