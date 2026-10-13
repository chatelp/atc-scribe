/**
 * A METAR read in plain English, for someone who has never decoded one
 * (design system, Weather; migration step 5): wind, visibility, weather,
 * clouds, temperature and QNH, in words.
 *
 * The weather source sends its own English reading ("Wind 270° 10kt.
 * Visibility 10km or more. Clouds broken 4300ft"), still in the METAR's
 * terms; this one says "from the west" and "broken clouds at 4 300 ft".
 * Unknown groups are left out rather than guessed; the raw METAR stays
 * shown above the reading.
 */
(function (root, factory) {
    const api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;
    } else {
        root.MetarPlain = api;
    }
})(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    const NBSP = ' ';

    const POINTS = ['north', 'north-east', 'east', 'south-east', 'south', 'south-west', 'west', 'north-west'];

    const COVER = {
        FEW: 'a few clouds',
        SCT: 'scattered clouds',
        BKN: 'broken clouds',
        OVC: 'overcast',
    };

    const DESCRIPTOR = {
        MI: 'shallow', BC: 'patches of', PR: 'partial', DR: 'drifting', BL: 'blowing',
        SH: 'showers', TS: 'thunderstorm', FZ: 'freezing',
    };

    const PHENOMENON = {
        DZ: 'drizzle', RA: 'rain', SN: 'snow', SG: 'snow grains', IC: 'ice crystals',
        PL: 'ice pellets', GR: 'hail', GS: 'small hail', UP: 'precipitation',
        BR: 'mist', FG: 'fog', FU: 'smoke', VA: 'volcanic ash', DU: 'dust', SA: 'sand',
        HZ: 'haze', PO: 'dust whirls', SQ: 'squalls', FC: 'funnel cloud', SS: 'sandstorm',
        DS: 'duststorm',
    };

    // 3000 -> "3 000", with a non-breaking space so a number never splits.
    function thousands(n) {
        return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, NBSP);
    }

    function point(deg) {
        return POINTS[Math.round(((deg % 360) + 360) % 360 / 45) % 8];
    }

    function wind(token, variation) {
        const m = /^(\d{3}|VRB)(\d{2,3})(?:G(\d{2,3}))?(KT|MPS)$/.exec(token);
        if (!m) return null;
        const unit = m[4] === 'MPS' ? 'm/s' : 'kt';
        const speed = Number(m[2]);
        if (speed === 0) return 'Calm wind';
        let from = m[1] === 'VRB' ? 'Variable wind' : `Wind from the ${point(Number(m[1]))}`;
        if (variation) from += `, varying ${variation[1]}° to ${variation[2]}°`;
        const gust = m[3] ? `, gusting ${Number(m[3])}${NBSP}${unit}` : '';
        return `${from}, ${speed}${NBSP}${unit}${gust}`;
    }

    function visibility(token) {
        if (token === '9999') return 'visibility over 10 km'.replace(' km', `${NBSP}km`);
        const m = /^(\d{4})(NDV)?$/.exec(token);
        if (!m) return null;
        const metres = Number(m[1]);
        if (metres >= 5000) return `visibility ${Math.round(metres / 1000)}${NBSP}km`;
        return `visibility ${thousands(metres)}${NBSP}m`;
    }

    function weather(token) {
        const m = /^(\+|-|VC)?(MI|BC|PR|DR|BL|SH|TS|FZ)?((?:DZ|RA|SN|SG|IC|PL|GR|GS|UP|BR|FG|FU|VA|DU|SA|HZ|PO|SQ|FC|SS|DS)+)?$/.exec(token);
        if (!m || (!m[2] && !m[3])) return null;
        const kinds = (m[3] || '').match(/../g) || [];
        let words = kinds.map((k) => PHENOMENON[k]).join(' and ');
        const desc = m[2] ? DESCRIPTOR[m[2]] : '';
        if (m[2] === 'SH') words = `${words || 'rain'} showers`;
        else if (m[2] === 'TS') words = words ? `thunderstorm with ${words}` : 'thunderstorm';
        else if (desc) words = `${desc} ${words}`.trim();
        if (m[1] === '-') words = `light ${words}`;
        else if (m[1] === '+') words = `heavy ${words}`;
        else if (m[1] === 'VC') words = `${words} nearby`;
        return words;
    }

    function cloud(token) {
        if (/^(NSC|NCD|SKC|CLR)$/.test(token)) return 'no significant cloud';
        const vv = /^VV(\d{3}|\/\/\/)$/.exec(token);
        if (vv) return vv[1] === '///' ? 'sky hidden' : `sky hidden, vertical visibility ${thousands(Number(vv[1]) * 100)}${NBSP}ft`;
        const m = /^(FEW|SCT|BKN|OVC)(\d{3}|\/\/\/)(CB|TCU|\/\/\/)?$/.exec(token);
        if (!m) return null;
        const at = m[2] === '///' ? '' : ` at ${thousands(Number(m[2]) * 100)}${NBSP}ft`;
        const type = m[3] === 'CB' ? ' (storm clouds)' : m[3] === 'TCU' ? ' (towering clouds)' : '';
        return `${COVER[m[1]]}${at}${type}`;
    }

    function temperature(token) {
        const m = /^(M?\d{2})\/(M?\d{2})?$/.exec(token);
        if (!m) return null;
        const t = Number(m[1].replace('M', '-'));
        return `${t}${NBSP}°C`;
    }

    function pressure(token) {
        let m = /^Q(\d{4})$/.exec(token);
        if (m) return `QNH ${Number(m[1])}`;
        m = /^A(\d{4})$/.exec(token);
        if (m) return `altimeter ${(Number(m[1]) / 100).toFixed(2)}${NBSP}inHg`;
        return null;
    }

    // read returns the reading, one sentence ending with a full stop, or ''
    // when the text is not a METAR it can read.
    function read(raw) {
        if (!raw || typeof raw !== 'string') return '';
        const tokens = raw.trim().replace(/=$/, '').split(/\s+/);
        let i = 0;
        if (tokens[i] === 'METAR' || tokens[i] === 'SPECI') i++;
        if (!/^[A-Z]{4}$/.test(tokens[i] || '')) return '';
        i++;
        if (/^\d{6}Z$/.test(tokens[i] || '')) i++;
        while (tokens[i] === 'AUTO' || tokens[i] === 'COR' || tokens[i] === 'NIL') {
            if (tokens[i] === 'NIL') return '';
            i++;
        }

        const parts = [];
        const clouds = [];
        const wx = [];
        let windText = null;
        let changes = null;
        for (; i < tokens.length; i++) {
            const t = tokens[i];
            if (t === 'RMK') break;
            if (t === 'NOSIG') { changes = 'no significant change expected'; break; }
            if (t === 'TEMPO' || t === 'BECMG') { changes = 'changes expected'; break; }
            if (!windText) {
                const variation = /^(\d{3})V(\d{3})$/.exec(tokens[i + 1] || '');
                const w = wind(t, variation);
                if (w) { windText = w; if (variation) i++; continue; }
            }
            if (t === 'CAVOK') {
                parts.push(`visibility over 10${NBSP}km`, 'no cloud below 5' + NBSP + '000' + NBSP + 'ft');
                continue;
            }
            const v = visibility(t);
            if (v) { parts.push(v); continue; }
            const c = cloud(t);
            if (c) { clouds.push(c); continue; }
            const tp = temperature(t);
            if (tp) { parts.push(tp); continue; }
            const p = pressure(t);
            if (p) { parts.push(p); continue; }
            const w = weather(t);
            if (w) { wx.push(w); continue; }
            // RVR, wind shear and the rest: left out, not guessed.
        }

        // Wind, visibility, weather, clouds, temperature, pressure, in that order.
        const vis = parts.filter((p) => p.startsWith('visibility') || p.startsWith('no cloud'));
        const rest = parts.filter((p) => !vis.includes(p));
        const sentence = [windText, ...vis, wx.join(', '), clouds.join(', '), ...rest, changes]
            .filter(Boolean)
            .join('; ');
        if (!sentence) return '';
        return sentence.charAt(0).toUpperCase() + sentence.slice(1) + '.';
    }

    return { read, point };
});
