// Colour arithmetic shared by the design tools: parsing, WCAG contrast, CIE Lab
// distance, hue family, and colour-vision-deficiency simulation.

const clamp = (v) => Math.max(0, Math.min(255, Math.round(v)));
const hex2 = (v) => clamp(v).toString(16).padStart(2, '0');
export function toHex(r, g, b, a = 1) {
    const h = `#${hex2(r)}${hex2(g)}${hex2(b)}`;
    return a >= 1 ? h : `${h}@${Math.round(a * 100) / 100}`;
}
const NAMED = { white: [255, 255, 255], black: [0, 0, 0] };
// Parses a CSS colour value; returns '#rrggbb' or '#rrggbb@alpha', or null.
export function parseColor(v, opacityVars = {}) {
    if (!v) return null;
    v = v.trim().replace(/\s*!important$/, '');
    let m = v.match(/^#([0-9a-f]{3,8})$/i);
    if (m) {
        let h = m[1];
        if (h.length === 3 || h.length === 4) h = [...h].map((c) => c + c).join('');
        const a = h.length === 8 ? parseInt(h.slice(6), 16) / 255 : 1;
        return toHex(parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16), a);
    }
    m = v.match(/^rgba?\(\s*([\d.]+)[\s,]+([\d.]+)[\s,]+([\d.]+)\s*(?:[,/]\s*(.+?))?\s*\)$/i);
    if (m) {
        let a = 1;
        if (m[4] !== undefined) {
            const s = m[4].trim();
            const vm = s.match(/^var\((--[\w-]+)(?:,\s*([\d.]+))?\)$/);
            if (vm) a = opacityVars[vm[1]] ?? (vm[2] !== undefined ? +vm[2] : 1);
            else if (s.endsWith('%')) a = parseFloat(s) / 100;
            else a = parseFloat(s);
            if (Number.isNaN(a)) return null; // dynamic alpha (template literal)
        }
        return toHex(+m[1], +m[2], +m[3], a);
    }
    if (NAMED[v.toLowerCase()]) return toHex(...NAMED[v.toLowerCase()]);
    return null;
}
export const rgbOf = (h) => [1, 3, 5].map((i) => parseInt(h.slice(i, i + 2), 16));
export const alphaOf = (h) => (h.includes('@') ? parseFloat(h.split('@')[1]) : 1);
export function over(fg, bg) { // composite fg (may carry alpha) on an opaque bg
    const a = alphaOf(fg), f = rgbOf(fg), b = rgbOf(bg);
    return toHex(...f.map((c, i) => c * a + b[i] * (1 - a)));
}
export function luminance(h) {
    const [r, g, b] = rgbOf(h).map((c) => {
        c /= 255;
        return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
    });
    return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}
export function contrast(fg, bg) {
    const f = over(fg, bg);
    const [l1, l2] = [luminance(f), luminance(bg)].sort((a, b) => b - a);
    return (l1 + 0.05) / (l2 + 0.05);
}
export function lab(h) { // sRGB -> CIE Lab (D65), for the ΔE clustering
    const lin = rgbOf(h).map((c) => {
        c /= 255;
        return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
    });
    const X = (lin[0] * 0.4124 + lin[1] * 0.3576 + lin[2] * 0.1805) / 0.95047;
    const Y = lin[0] * 0.2126 + lin[1] * 0.7152 + lin[2] * 0.0722;
    const Z = (lin[0] * 0.0193 + lin[1] * 0.1192 + lin[2] * 0.9505) / 1.08883;
    const f = (t) => (t > 0.008856 ? Math.cbrt(t) : 7.787 * t + 16 / 116);
    return [116 * f(Y) - 16, 500 * (f(X) - f(Y)), 200 * (f(Y) - f(Z))];
}
export const deltaE = (a, b) => Math.hypot(...lab(a).map((v, i) => v - lab(b)[i]));
// Hue family, for grouping colours by what they say (red, amber, green…).
export function family(h) {
    const [r, g, b] = rgbOf(h).map((c) => c / 255);
    const max = Math.max(r, g, b), min = Math.min(r, g, b), d = max - min;
    const l = (max + min) / 2;
    const s = d === 0 ? 0 : d / (1 - Math.abs(2 * l - 1));
    if (s < 0.15 || d < 0.08) return 'grey';
    let hue = max === r ? ((g - b) / d) % 6 : max === g ? (b - r) / d + 2 : (r - g) / d + 4;
    hue = (hue * 60 + 360) % 360;
    if (hue < 15 || hue >= 345) return 'red';
    if (hue < 40) return 'orange';
    if (hue < 65) return 'amber/yellow';
    if (hue < 90) return 'lime';
    if (hue < 165) return 'green';
    if (hue < 195) return 'teal/cyan';
    if (hue < 250) return 'blue';
    if (hue < 290) return 'violet';
    return 'pink';
}

// Dichromacies, Machado, Oliveira & Fernandes 2009, severity 1, applied in
// linear RGB.
export const CVD = {
    protanopia: [[0.152286, 1.052583, -0.204868], [0.114503, 0.786281, 0.099216], [-0.003882, -0.048116, 1.051998]],
    deuteranopia: [[0.367322, 0.860646, -0.227968], [0.280085, 0.672501, 0.047413], [-0.011820, 0.042940, 0.968881]],
    tritanopia: [[1.255528, -0.076749, -0.178779], [-0.078411, 0.930809, 0.147602], [0.004733, 0.691367, 0.303900]],
};
export function simulate(h, m) {
    const lin = rgbOf(h).map((c) => { c /= 255; return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4; });
    const o = m.map((row) => Math.max(0, Math.min(1, row[0] * lin[0] + row[1] * lin[1] + row[2] * lin[2])));
    return toHex(...o.map((c) => 255 * (c <= 0.0031308 ? 12.92 * c : 1.055 * c ** (1 / 2.4) - 0.055)));
}
