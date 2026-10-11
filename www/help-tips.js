// Help bubbles for the whole page: any element with a data-help attribute
// (and an optional data-help-title) explains itself when the pointer rests on
// it, in the same bubble as the PAR view's (.par-tip in style.css). Meant for
// what a newcomer to aviation would not guess: QNH and flight levels, the
// transition altitude, what a count or a colour means.
//
// The PAR view keeps its own bubble, wired to the parts it redraws every
// second; this one leaves it alone.

(function () {
    const DELAY_MS = 350;
    let tip = null;
    let key = null;
    let timer = null;
    let x = 0;
    let y = 0;

    function esc(s) {
        return String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
    }

    function place() {
        const pad = 14;
        let left = x + pad;
        let top = y + pad;
        if (left + tip.offsetWidth > window.innerWidth - 8) left = x - pad - tip.offsetWidth;
        if (top + tip.offsetHeight > window.innerHeight - 8) top = y - pad - tip.offsetHeight;
        tip.style.left = Math.max(8, left) + 'px';
        tip.style.top = Math.max(8, top) + 'px';
    }

    function hide() {
        clearTimeout(timer);
        timer = null;
        key = null;
        if (tip) tip.style.display = 'none';
    }

    document.addEventListener('mousemove', (ev) => {
        x = ev.clientX;
        y = ev.clientY;
        const el = ev.target.closest && ev.target.closest('[data-help]');
        if (!el || el.closest('#par-view')) {
            if (key) hide();
            return;
        }
        const title = el.getAttribute('data-help-title') || '';
        const body = el.getAttribute('data-help') || '';
        if (!body) { hide(); return; }
        const k = title + '\n' + body;
        if (k === key) {
            if (tip && tip.style.display !== 'none') place();
            return;
        }
        hide();
        key = k;
        timer = setTimeout(() => {
            if (!tip) {
                tip = document.createElement('div');
                tip.className = 'par-tip';
                document.body.appendChild(tip);
            }
            tip.innerHTML = (title ? `<div class="par-tip-title">${esc(title)}</div>` : '') + `<div>${esc(body)}</div>`;
            tip.style.display = 'block';
            place();
        }, DELAY_MS);
    });
    document.addEventListener('mousedown', hide, true);
    document.addEventListener('scroll', hide, true);
    window.addEventListener('blur', hide);
})();
