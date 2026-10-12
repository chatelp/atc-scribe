/**
 * Module: map/features/voice-bubbles-feature
 *
 * A speech bubble over each aircraft the radio has named recently, with its
 * last transmission (owner's request, 12/10). Bubbles are HTML overlays of
 * OpenLayers so they can be clicked: a click opens the frequency's transcript
 * at that transmission. They follow the animated aircraft (the manager calls
 * move() from updateAnimatedAircraftPose) and are rebuilt every two seconds
 * from the store: an aircraft heard in the last ten minutes, shown on the map,
 * with a position. The full text and who said it are in the help bubble.
 */
(function () {
    const MAX_CHARS = 64;
    const WINDOW_MS = 10 * 60 * 1000;
    const REFRESH_MS = 2000;

    function create(manager, store) {
        const map = manager._olMap;
        if (!map || !window.ol || !window.ol.Overlay) return null;
        const bubbles = new Map(); // hex -> { overlay, el, text }

        function shortText(text) {
            const t = String(text || '').replace(/\s+/g, ' ').trim();
            return t.length > MAX_CHARS ? t.slice(0, MAX_CHARS - 1).trimEnd() + '…' : t;
        }

        function helpOf(a) {
            const v = a.voice;
            const when = v.last_heard ? new Date(v.last_heard) : null;
            const ago = when ? Math.max(0, Math.round((Date.now() - when.getTime()) / 1000)) : null;
            const agoText = ago === null ? '' : ago < 60 ? `${ago} s ago` : `${Math.round(ago / 60)} min ago`;
            return `${(a.flight || a.hex).trim()}, last transmission matched to it${agoText ? ', ' + agoText : ''}: "${String(v.last_text || '').trim()}". Click to open the transcript at it.`;
        }

        function build(hex) {
            const el = document.createElement('div');
            el.className = 'voice-bubble';
            el.setAttribute('data-help-title', 'Last transmission');
            el.addEventListener('click', (ev) => {
                ev.stopPropagation();
                const a = store.aircraft[hex];
                if (a && a.voice && a.voice.last_id && typeof store.openTranscriptionAt === 'function') {
                    store.openTranscriptionAt(a.voice.last_frequency_id, a.voice.last_id);
                }
            });
            const overlay = new window.ol.Overlay({
                element: el,
                positioning: 'bottom-center',
                offset: [0, -22],
                stopEvent: true,
                insertFirst: false,
            });
            map.addOverlay(overlay);
            const b = { overlay, el, text: '' };
            bubbles.set(hex, b);
            return b;
        }

        function remove(hex) {
            const b = bubbles.get(hex);
            if (!b) return;
            map.removeOverlay(b.overlay);
            bubbles.delete(hex);
        }

        function positionOf(hex, a) {
            const feature = typeof manager._getAircraftFeature === 'function' ? manager._getAircraftFeature(hex) : null;
            const geometry = feature && feature.getGeometry && feature.getGeometry();
            if (geometry && typeof geometry.getCoordinates === 'function') return geometry.getCoordinates();
            const lat = a.adsb && a.adsb.lat, lon = a.adsb && a.adsb.lon;
            if (Number.isFinite(lat) && Number.isFinite(lon) && (lat !== 0 || lon !== 0)) return manager._toMapCoordinate(lat, lon);
            return null;
        }

        function refresh() {
            const wanted = new Set();
            if (store.settings && store.settings.voiceBubbles !== false && store.mainView !== 'par') {
                const now = Date.now();
                const rules = window.MapVisibilityRules;
                for (const a of Object.values(store.aircraft || {})) {
                    const v = a && a.voice;
                    if (!v || !v.last_text || !v.last_heard) continue;
                    if (now - Date.parse(v.last_heard) > WINDOW_MS) continue;
                    if (rules && !rules.shouldShowAircraftOnMap(a, store, { searchTerm: store.searchTerm })) continue;
                    const pos = positionOf(a.hex, a);
                    if (!pos) continue;
                    wanted.add(a.hex);
                    const b = bubbles.get(a.hex) || build(a.hex);
                    const text = shortText(v.last_text);
                    if (text !== b.text) {
                        b.text = text;
                        b.el.textContent = text;
                    }
                    b.el.setAttribute('data-help', helpOf(a));
                    b.overlay.setPosition(pos);
                }
            }
            for (const hex of [...bubbles.keys()]) {
                if (!wanted.has(hex)) remove(hex);
            }
        }

        // The animated position, from the manager, between two refreshes.
        function move(hex, coordinate) {
            const b = bubbles.get(hex);
            if (b) b.overlay.setPosition(coordinate);
        }

        const timer = setInterval(refresh, REFRESH_MS);
        function destroy() {
            clearInterval(timer);
            for (const hex of [...bubbles.keys()]) remove(hex);
        }

        return { refresh, move, destroy };
    }

    window.MapVoiceBubbles = { create };
})();
