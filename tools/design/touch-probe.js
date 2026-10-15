// Touch targets under 44 px on the screen as it stands (migration step 12):
//   node tools/design/capture.mjs URL OUT.png --size 375x812 --after "$(cat tools/design/touch-probe.js)"
// Every visible control counts, its hit area included (an absolute ::after
// reaching past its box, as a callsign chip's). Prints the total and the
// small ones, by kind and size.
(() => {
  const SEL = 'button, a[href], input:not([type="hidden"]), select, [role="button"], [role="tab"], [role="checkbox"], [role="switch"], summary';
  const small = {};
    let total = 0;
    for (const el of document.querySelectorAll(SEL)) {
      let r = el.getBoundingClientRect();
      if (!r.width || !r.height) continue;
      // The hit area: an absolute ::after that reaches past the box counts.
      const after = getComputedStyle(el, '::after');
      if (after.content && after.content !== 'none' && after.position === 'absolute') {
        const px = (v) => parseFloat(v) || 0;
        r = { left: r.left + Math.min(0, px(after.left)), right: r.right - Math.min(0, px(after.right)),
              top: r.top + Math.min(0, px(after.top)), bottom: r.bottom - Math.min(0, px(after.bottom)) };
        r.width = r.right - r.left; r.height = r.bottom - r.top;
      }
      if (!r.width || !r.height || r.bottom < 0 || r.top > innerHeight || r.right < 0 || r.left > innerWidth) continue;
      if (getComputedStyle(el).visibility === 'hidden' || el.closest('[inert]')) continue;
      // An invisible range over a drawn slider is measured by its own box.
      total++;
      if (r.width < 44 || r.height < 44) {
        const name = (el.getAttribute('aria-label') || el.className || el.tagName).toString().replace(/\s+/g, ' ').slice(0, 40);
        const key = el.tagName.toLowerCase() + ' ' + name + ' ' + Math.round(r.width) + 'x' + Math.round(r.height);
        small[key] = (small[key] || 0) + 1;
      }
    }
  return JSON.stringify({ total, small: Object.entries(small).sort((a, b) => b[1] - a[1]) });
})()
