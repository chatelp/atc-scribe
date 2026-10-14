// The thread (design system, ThreadHead and Exchange; migration step 6): every
// exchange of every frequency in one list, oldest at the top, growing at the
// bottom, filtered by "Show" and a search. It replaces the per-frequency
// transcript windows of the bottom radio bar, which stays until step 7 to be
// compared over an evening.
//
// One domain of the Alpine store 'atc' (docs-fr/32, palier 5.1); see
// store/station.js for how the domains are assembled. The exchanges are the
// ones transcripts.js already keeps per frequency (originalTranscriptions,
// unfiltered); nothing new is fetched.
(function () {
    // As many as the per-frequency windows kept together (99 each).
    const MAX_EXCHANGES = 200;
    // "N exchanges in the last 2 min".
    const PACE_WINDOW_MS = 2 * 60 * 1000;

    window.AtcStore = window.AtcStore || [];
    window.AtcStore.push(() => ({
        // 'all' or a frequency id: which frequencies the thread shows.
        threadShow: 'all',
        threadSearch: '',
        threadSearchOpen: false,
        // The frequency whose menu (volume, only this one) is open, or null.
        threadMenuFor: null,
        // Per callsign heard: the aircraft's hex and phase, refreshed every
        // 5 s in one pass over the aircraft. Looked up live, every chip
        // searched every aircraft at every aircraft update (1 179 searches in
        // 10 s for 117 exchanges, measured 13/10).
        threadChipInfo: {},

        startThread() {
            if (this._threadStarted) return;
            this._threadStarted = true;
            this.refreshThreadChips();
            setInterval(() => this.refreshThreadChips(), 5000);
        },

        refreshThreadChips() {
            const byFlight = new Map();
            for (const a of Object.values(this.aircraft || {})) {
                const f = (a.flight || '').trim().toUpperCase();
                if (f) byFlight.set(f, a);
            }
            const next = {};
            for (const list of Object.values(this.originalTranscriptions || {})) {
                for (const t of list || []) {
                    const cs = (t.callsign || '').trim().toUpperCase();
                    if (!cs || next[cs]) continue;
                    const a = byFlight.get(cs);
                    next[cs] = a ? { hex: a.hex, phase: this.getCurrentPhase(a) } : { hex: null, phase: null };
                }
            }
            const before = this.threadChipInfo;
            const same = Object.keys(next).length === Object.keys(before).length &&
                Object.entries(next).every(([k, v]) => before[k] && before[k].hex === v.hex && before[k].phase === v.phase);
            if (!same) this.threadChipInfo = next;
        },

        // Every exchange kept, oldest first, as the thread shows them.
        get threadExchanges() {
            const show = this.threadShow;
            const term = (this.threadSearch || '').trim().toLowerCase();
            const all = [];
            for (const [freqId, list] of Object.entries(this.originalTranscriptions || {})) {
                if (show !== 'all' && show !== freqId) continue;
                for (const t of list || []) all.push(t);
            }
            const at = (t) => Date.parse(t.timestamp || t.created_at || '') || 0;
            all.sort((a, b) => at(a) - at(b) || ((a.id || 0) - (b.id || 0)));
            const found = term ? all.filter((t) => this.exchangeMatches(t, term)) : all;
            return found.slice(-MAX_EXCHANGES);
        },

        exchangeMatches(t, term) {
            return [t.text, t.content_processed, t.content_second, t.callsign, t.speaker_type,
                ...(Array.isArray(t.plain) ? t.plain.map((p) => p.text) : [])]
                .some((s) => s && String(s).toLowerCase().includes(term));
        },

        // "5 exchanges in the last 2 min", following the clock.
        get threadPace() {
            const now = (this.currentTimeForPhases || new Date()).getTime();
            let n = 0;
            for (const list of Object.values(this.originalTranscriptions || {})) {
                for (const t of list || []) {
                    if (now - (Date.parse(t.timestamp || t.created_at || '') || 0) <= PACE_WINDOW_MS) n++;
                }
            }
            return n === 0 ? 'nothing in the last 2 min' : `${n} exchange${n === 1 ? '' : 's'} in the last 2 min`;
        },

        // The frequencies on which someone is speaking now: sound above the
        // threshold within the last second (audio-client.js measures it on
        // every frequency the page has a stream for, muted or not).
        get threadLive() {
            const secs = this.secondsSinceLastAudio || {};
            return (this.audioFrequencies || []).filter((f) => {
                const v = parseInt(secs[f.id], 10);
                return Number.isFinite(v) && v <= 1 && !this.isFrequencyFailed(f.id);
            });
        },

        // "124.350", or the name for a mixed stream with no single frequency.
        freqLabel(freqId) {
            const f = (this.audioFrequencies || []).find((x) => x.id === freqId);
            if (!f) return String(freqId || '');
            return Number(f.frequency_mhz) > 0 ? Number(f.frequency_mhz).toFixed(3) : 'mixed';
        },

        // The frequency's name without the MHz the station often puts first
        // ("124.350 Approche De Gaulle" -> "Approche De Gaulle"): the button
        // shows the MHz beside it already.
        freqShortName(freq) {
            const name = String((freq && freq.name) || '').trim();
            return name.replace(/^\d{3}[.,]\d{1,3}\s*[-·:]?\s*/, '') || name;
        },

        exchangeTime(t) {
            const d = new Date(t.timestamp || t.created_at);
            if (isNaN(d.getTime())) return '';
            if (this.showLocalDates) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
            return d.toISOString().substring(11, 19) + 'Z';
        },

        // The aircraft's chip: its phase family and whether it is the selected
        // one; dashed when it is out of sight, warned when the match is weak.
        exchangeChip(t) {
            const cs = (t.callsign || '').trim().toUpperCase();
            const info = this.threadChipInfo[cs] || {};
            // A callsign matched since the last refresh gets its chip soon after.
            if (cs && !this.threadChipInfo[cs] && !this._chipRefreshPending) {
                this._chipRefreshPending = true;
                setTimeout(() => { this._chipRefreshPending = false; this.refreshThreadChips(); }, 300);
            }
            const phase = info.phase || null;
            const weak = this.matchStrength(t) === 'weak';
            return {
                text: weak ? `probably ${t.callsign}` : t.callsign,
                phase,
                phaseClass: phase && window.PhaseFamilies ? `cl-phase--${PhaseFamilies.modifierOf(phase)}` : '',
                classes: [
                    this.selectedAircraft && info.hex && this.selectedAircraft.hex === info.hex ? 'is-selected' : '',
                    info.hex ? '' : 'is-away',
                    weak ? 'is-guess' : '',
                ].filter(Boolean).join(' '),
            };
        },

        // The chip's help, as transcriptAircraftHelp says it, from the same table.
        threadChipHelp(t) {
            const info = this.threadChipInfo[(t.callsign || '').trim().toUpperCase()] || {};
            const how = t.callsign_source === 'fr'
                ? 'It was recognised in the French reading of the transmission'
                : 'It was recognised in the transmission';
            const where = info.hex ? 'Click to show it on the map and open its panel.' : 'It is no longer in range of the receiver.';
            return `${t.callsign}: the aircraft this transmission was matched to. ${how}; the words in bold are the ones that matched its callsign. ${this.matchProofText(t)} ${where}`;
        },

        // 3, 2 or 1 bars for strong, medium, weak; 0 when there is no proof.
        strengthBars(t) {
            return { strong: 3, medium: 2, weak: 1 }[this.matchStrength(t)] || 0;
        },

        // Why no aircraft was named, in a newcomer's words (Exchange sheet, 14/10):
        // the server keeps the matcher's terms ("best X at 0.30, below the
        // floor", "ambiguous between X and Y"), the page translates them.
        // { guess: the callsign shown dashed with "?", or null; text; help }.
        unmatchedView(t) {
            const r = (t.match_reason || '').replace(/^unmatched:\s*/, '').trim();
            let m = /^best (\S+) at ([\d.]+), below the floor$/.exec(r);
            if (m) {
                return { guess: m[1], text: 'too uncertain to name',
                    help: `The closest callsign heard was ${m[1]}, with a score of ${m[2]}: under 0.60, the least the tool accepts, so it names no aircraft. A guess, not a match.` };
            }
            m = /^ambiguous between (.+)$/.exec(r);
            if (m) {
                const names = m[1].split(/\s*(?:,|\band\b)\s*/).filter(Boolean);
                const list = names.length > 1 ? names.slice(0, -1).join(', ') + ' or ' + names[names.length - 1] : names[0];
                return { guess: null, text: `could be ${list}`,
                    help: 'Two or more aircraft in range fit what was heard equally well: the tool names none rather than guess.' };
            }
            if (r === 'no aircraft in range') {
                return { guess: null, text: 'no aircraft was in range', help: 'No aircraft was in sight of the receiver when this was heard.' };
            }
            return { guess: null, text: 'no aircraft in range was named',
                help: 'No callsign of an aircraft in sight of the receiver was heard in this transmission.' };
        },

        // "being matched…" for the few seconds the matcher takes; an exchange
        // left unmatched past two minutes (the server stopped before matching
        // it) says so instead of waiting forever. Follows nowTick (30 s).
        pendingText(t) {
            this.nowTick;
            const at = Date.parse(t.timestamp || t.created_at || '') || 0;
            return Date.now() - at > 2 * 60 * 1000 ? 'not matched' : 'being matched…';
        },

        exchangeIsFrench(t) {
            return (t.language || '').toLowerCase().startsWith('fr');
        },

        setThreadShow(freqId) {
            this.threadShow = freqId && freqId !== 'all' ? freqId : 'all';
        },

        // The volume the open menu shows: audioClient keeps the volumes outside
        // the store, so the menu reads this copy, set when it opens.
        threadMenuVolume: 1,

        toggleThreadMenu(freqId) {
            this.threadMenuFor = this.threadMenuFor === freqId ? null : freqId;
            if (this.threadMenuFor) this.threadMenuVolume = Number(this.volumeOf(freqId));
        },

        setThreadVolume(freqId, value) {
            this.threadMenuVolume = Number(value);
            this.setVolume(freqId, this.threadMenuVolume);
        },

        // The channel button's help in the thread (the tile's spoke of the bar).
        channelHelp(freq) {
            const state = this.isFrequencyFailed(freq.id) ? `Connection failed: ${this.getFrequencyError(freq.id) || 'unknown error'}; the page tries again by itself.`
                : this.isFrequencyConnecting(freq.id) ? 'Connecting…'
                : this.isUnmuted(freq.id) ? 'Listening in this browser. Click to mute.' : 'Muted in this browser. Click to listen.';
            const mhz = freq.frequency_mhz > 0 ? `${Number(freq.frequency_mhz).toFixed(3)} MHz` : 'a mixed stream of several channels';
            return `${freq.name} (${mhz}). ${state} The arrow beside it opens its volume and "only this frequency in the thread".`;
        },

        // In the frequency's menu: its state in this browser, in a word.
        channelState(freq) {
            if (this.isFrequencyFailed(freq.id)) return { level: 'bad', text: 'Failed: ' + (this.getFrequencyError(freq.id) || 'no stream') };
            if (this.isFrequencyConnecting(freq.id)) return { level: 'switching', text: 'Connecting' };
            if (this.isUnmuted(freq.id)) return { level: 'ok', text: 'Listening in this browser' };
            return { level: 'off', text: 'Muted in this browser' };
        },

        // "Last sound 8 s ago · 41 today."
        channelNote(freq) {
            const secs = (this.secondsSinceLastAudio || {})[freq.id];
            const last = secs && secs !== '--s' ? `Last sound ${secs.replace('s', ' s')} ago` : 'No sound yet';
            const today = new Date().toDateString();
            const n = (this.originalTranscriptions[freq.id] || [])
                .filter((t) => new Date(t.timestamp || t.created_at).toDateString() === today).length;
            return `${last} · ${n} transcribed today.`;
        },
    }));
})();

// The thread's scroll: it follows the newest exchange while the reader is at
// the bottom; scrolled up, the new ones gather in "↓ N new" (decision 2).
window.threadView = function () {
    // The reader's intent decides, not the scroll position: an exchange grows
    // after it arrives (matched, chips, explanation) and the cap removes one
    // at the top, both of which move scrollTop without the reader (design
    // review of step 6: 290 px off the bottom in 12 s, untouched). Only a
    // wheel, a touch, a key or a drag of the scrollbar lets go of the bottom.
    return {
        atBottom: true,
        newCount: 0,
        lastLength: 0,
        _userUntil: 0,
        init() {
            const el = this.$refs.scroller;
            this.lastLength = this.$store.atc.threadExchanges.length;
            const byReader = () => { this._userUntil = Date.now() + 800; };
            for (const ev of ['wheel', 'touchstart', 'touchmove', 'keydown', 'pointerdown']) {
                el.addEventListener(ev, byReader, { passive: true });
            }
            // Whatever changes inside the list, the bottom is kept while the
            // reader wants it.
            new MutationObserver(() => { if (this.atBottom) this.toBottom(false); })
                .observe(el, { childList: true, subtree: true, characterData: true });
            this.$nextTick(() => this.toBottom(false));
            // A line of a followed aircraft's story leads to its exchange
            // (step 8): the filters let it through, the bottom is let go of.
            window.addEventListener('thread-show', (e) => this.showExchange(e.detail && e.detail.id));
            this.$watch(() => this.$store.atc.threadExchanges.length, (n) => {
                const added = Math.max(0, n - this.lastLength);
                this.lastLength = n;
                if (!this.atBottom) this.newCount += added;
            });
            this.$watch(() => this.$store.atc.threadShow + '|' + this.$store.atc.threadSearch, () => {
                this.newCount = 0;
                this.$nextTick(() => this.toBottom(false));
            });
        },
        onScroll() {
            if (Date.now() > this._userUntil) return;
            const el = this.$refs.scroller;
            this.atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 24;
            if (this.atBottom) this.newCount = 0;
        },
        showExchange(id) {
            const atc = this.$store.atc;
            const filtered = atc.threadShow !== 'all' || atc.threadSearch;
            if (filtered) { atc.threadShow = 'all'; atc.threadSearch = ''; }
            // After the filters' own return to the bottom.
            setTimeout(() => {
                const el = this.$refs.scroller && this.$refs.scroller.querySelector(`[data-exchange="${CSS.escape(String(id))}"]`);
                if (!el) return;
                this.atBottom = false;
                this._userUntil = Date.now() + 800;
                el.scrollIntoView({ block: 'center', behavior: 'smooth' });
                if (el.animate) el.animate([{ background: 'var(--color-surface-hover)' }, { background: 'transparent' }], { duration: 1800 });
            }, filtered ? 80 : 0);
        },
        toBottom(smooth) {
            const el = this.$refs.scroller;
            if (!el) return;
            el.scrollTo({ top: el.scrollHeight, behavior: smooth ? 'smooth' : 'auto' });
            this.atBottom = true;
            this.newCount = 0;
        },
    };
};
