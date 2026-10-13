// Rendered-text probe for the visual inventory (docs-fr/design/01-inventaire.md).
//
// Paste into the console of a running page (or run it through a browser
// automation tool); it returns a JSON-able summary of what is really on screen:
// every visible text run with its computed colour, the colour of what lies
// behind it, its size and weight, weighted by character count. tools/design/
// inventory.mjs says which pairs are possible; this says which are rendered.
//
// Rules:
// - a text run is a non-empty text node whose element has a non-zero box
//   inside the viewport and is not hidden (display, visibility, opacity 0);
// - and is the topmost element at the centre of its box (not under a panel);
// - the effective colour folds the alpha of the colour and the opacity of every
//   ancestor; the background is found by walking up to the first ancestor with a
//   non-transparent background, compositing semi-transparent layers on the way;
//   where none is found (text drawn over the map or the PAR canvas) the page
//   background is assumed, #000000 by default (the "None (black)" map);
// - SVG <text> counts (the PAR), with its fill as colour;
// - canvas text (map labels) is invisible to the DOM and is not measured here.
(function probe(opts = {}) {
    const fallbackBg = opts.fallbackBg || [0, 0, 0];
    const parse = (s) => {
        const m = s && s.match(/rgba?\(([^)]+)\)/);
        if (!m) return null;
        const p = m[1].split(/[\s,/]+/).filter(Boolean).map(Number);
        return [p[0], p[1], p[2], p[3] === undefined ? 1 : p[3]];
    };
    const over = (top, under) => top.slice(0, 3).map((c, i) => c * top[3] + under[i] * (1 - top[3]));
    const lum = (rgb) => {
        const [r, g, b] = rgb.map((c) => { c /= 255; return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4; });
        return 0.2126 * r + 0.7152 * g + 0.0722 * b;
    };
    const ratio = (a, b) => { const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p); return (x + 0.05) / (y + 0.05); };
    const hex = (rgb) => '#' + rgb.map((c) => Math.round(c).toString(16).padStart(2, '0')).join('');

    function background(el) {
        const layers = [];
        for (let e = el; e && e.nodeType === 1; e = e.parentElement) {
            const cs = getComputedStyle(e);
            const c = parse(cs.backgroundColor);
            if (c && c[3] > 0) { layers.push(c); if (c[3] >= 0.999) break; }
        }
        let bg = fallbackBg;
        for (let i = layers.length - 1; i >= 0; i--) bg = over(layers[i], bg);
        return bg;
    }
    function opacityChain(el) {
        let o = 1;
        for (let e = el; e && e.nodeType === 1; e = e.parentElement) o *= parseFloat(getComputedStyle(e).opacity);
        return o;
    }
    const vw = innerWidth, vh = innerHeight;
    const runs = new Map(); // key -> {chars, ...}
    const sizes = new Map();
    const families = new Map();
    let totalChars = 0;
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
    for (let n = walker.nextNode(); n; n = walker.nextNode()) {
        const text = n.nodeValue.replace(/\s+/g, ' ').trim();
        if (!text) continue;
        const el = n.parentElement;
        if (!el || el.closest('script,style,template,noscript')) continue;
        const cs = getComputedStyle(el);
        if (cs.display === 'none' || cs.visibility !== 'visible') continue;
        const range = document.createRange(); range.selectNodeContents(n);
        const r = range.getBoundingClientRect();
        if (r.width < 1 || r.height < 1 || r.bottom < 0 || r.right < 0 || r.top > vh || r.left > vw) continue;
        // Covered by another layer (a page, a panel, a popover): not seen.
        const hit = document.elementFromPoint(Math.min(vw - 1, Math.max(0, r.left + r.width / 2)), Math.min(vh - 1, Math.max(0, r.top + r.height / 2)));
        if (!hit || !(hit === el || el.contains(hit) || hit.contains(el))) continue;
        const op = opacityChain(el);
        if (op < 0.01) continue;
        const isSvg = el instanceof SVGElement;
        const fg = parse(isSvg ? (cs.fill && cs.fill !== 'none' ? cs.fill : cs.color) : cs.color) || [0, 0, 0, 1];
        const bg = background(el);
        const eff = over([fg[0], fg[1], fg[2], fg[3] * op], bg);
        const px = Math.round(parseFloat(cs.fontSize) * 10) / 10;
        const w = parseInt(cs.fontWeight, 10);
        const key = `${hex(eff)}|${hex(bg)}|${px}|${w >= 600 ? 'bold' : 'normal'}`;
        const e = runs.get(key) || { fg: hex(eff), bg: hex(bg), px, bold: w >= 600, chars: 0, ratio: ratio(eff, bg), sample: text.slice(0, 40), where: el.id || el.className?.baseVal || el.className || el.tagName };
        e.chars += text.length;
        runs.set(key, e);
        sizes.set(px, (sizes.get(px) || 0) + text.length);
        const fam = cs.fontFamily.split(',')[0].replace(/["']/g, '').trim();
        families.set(fam, (families.get(fam) || 0) + text.length);
        totalChars += text.length;
    }
    const all = [...runs.values()].sort((a, b) => b.chars - a.chars);
    const large = (r) => r.px >= 24 || (r.bold && r.px >= 18.66);
    const fails = all.filter((r) => r.ratio < (large(r) ? 3 : 4.5));
    const sumChars = (l) => l.reduce((s, r) => s + r.chars, 0);
    // Interactive targets: size of everything one can click or tap.
    const targets = [...document.querySelectorAll('button, a[href], input, select, textarea, [role=button], [\\@click], [x-on\\:click]')]
        .filter((e) => { const r = e.getBoundingClientRect(); const s = getComputedStyle(e); return r.width > 0 && r.height > 0 && s.visibility === 'visible' && r.bottom > 0 && r.top < vh && r.right > 0 && r.left < vw; })
        .map((e) => { const r = e.getBoundingClientRect(); return { w: r.width, h: r.height, label: (e.getAttribute('title') || e.textContent || e.getAttribute('aria-label') || e.tagName).trim().replace(/\s+/g, ' ').slice(0, 30) }; });
    const minSide = (t) => Math.min(t.w, t.h);
    return {
        viewport: `${vw}x${vh}`,
        totalChars,
        runs: all.length,
        failChars: sumChars(fails),
        failShare: totalChars ? Math.round(1000 * sumChars(fails) / totalChars) / 10 : 0,
        sizes: [...sizes.entries()].sort((a, b) => a[0] - b[0]).map(([px, c]) => ({ px, chars: c, share: Math.round(1000 * c / totalChars) / 10 })),
        families: Object.fromEntries(families),
        top: all.slice(0, 30).map((r) => ({ ...r, ratio: Math.round(r.ratio * 10) / 10 })),
        worst: fails.sort((a, b) => b.chars - a.chars).slice(0, 25).map((r) => ({ ...r, ratio: Math.round(r.ratio * 10) / 10 })),
        targets: {
            count: targets.length,
            under24: targets.filter((t) => minSide(t) < 24).length,
            under44: targets.filter((t) => minSide(t) < 44).length,
            smallest: targets.sort((a, b) => minSide(a) - minSide(b)).slice(0, 12).map((t) => `${Math.round(t.w)}x${Math.round(t.h)} ${t.label}`),
        },
        dataHelp: document.querySelectorAll('[data-help]').length,
    };
})();
