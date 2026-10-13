#!/usr/bin/env python3
"""Three directions for the sign-in page, for the owner to choose from.

    python3 tools/design/connexion.py

Writes docs-fr/design/connexion/{a-scope,b-echange,c-ciel}.html at 1440 x 900.
They use the design system (tokens, fields, button) and, while no direction is
chosen, a few rules of their own in a <style>; the chosen one moves into the
system (SignIn) and the final screens. Nothing here shows real data: the page
is seen before signing in.
"""
import math
import os
import re

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), '../..'))
OUT = os.path.join(ROOT, 'docs-fr/design/connexion')
SYS = '../systeme/project'
ID = os.path.join(ROOT, 'docs-fr/design/identite')
AV = os.path.join(ROOT, 'docs-fr/design/avions/famille')

SIGN_64 = ('<svg viewBox="0 0 64 64" aria-hidden="true"><circle cx="32" cy="32" r="26.5" fill="none" stroke="#e0e0e0" stroke-width="4"/>'
           '<circle cx="32" cy="32" r="13" fill="none" stroke="#e0e0e0" stroke-width="2" opacity="0.55"/>'
           '<path d="M32 32 L32 6 A26 26 0 0 1 50.385 13.615 Z" fill="#4caf50" fill-opacity="0.35"/>'
           '<path d="M32 32 L50.385 13.615" fill="none" stroke="#4caf50" stroke-width="4" stroke-linecap="round"/>'
           '<circle cx="39.18" cy="16.59" r="3.8" fill="#4caf50"/></svg>')


def one_ink(color):
    """The sign in one colour, the afterglow at 35 % (05-identite.md: engraving, menu bar)."""
    return (f'<svg viewBox="0 0 64 64" aria-hidden="true"><circle cx="32" cy="32" r="26.5" fill="none" stroke="{color}" stroke-width="4"/>'
            f'<circle cx="32" cy="32" r="13" fill="none" stroke="{color}" stroke-width="2" opacity="0.55"/>'
            f'<path d="M32 32 L32 6 A26 26 0 0 1 50.385 13.615 Z" fill="{color}" fill-opacity="0.35"/>'
            f'<path d="M32 32 L50.385 13.615" fill="none" stroke="{color}" stroke-width="4" stroke-linecap="round"/>'
            f'<circle cx="39.18" cy="16.59" r="3.8" fill="{color}"/></svg>')


def aircraft(key, x, y, rot, size, color):
    """One of our nine silhouettes (avions/famille), recoloured on the altitude ramp."""
    svg = open(os.path.join(AV, key + '.svg')).read()
    inner = re.search(r'<svg[^>]*>(.*)</svg>', svg, re.S).group(1)
    inner = re.sub(r'<title>.*?</title>', '', inner, flags=re.S).replace('#000000', color)
    return (f'<g transform="translate({x} {y}) rotate({rot}) translate({-size / 2} {-size / 2}) scale({size / 64})">{inner}</g>')


def form(first_title='Sign in'):
    return f'''<form class="lg-form" onsubmit="return false">
  <h1 class="lg-h">{first_title}</h1>
  <label class="cl-signin__field"><span class="cl-label">User</span><input class="cl-input" autocomplete="username" value="pierre"></label>
  <label class="cl-signin__field"><span class="cl-label">Password</span><input class="cl-input" type="password" autocomplete="current-password" value="••••••••••••"></label>
  <button class="cl-btn cl-btn--primary" type="submit">Sign in</button>
  <span class="cl-plain">After 8 failed tries, wait 15 minutes.</span>
</form>'''


def page(title, style, body):
    return f'''<!doctype html>
<html lang="en" data-theme="dark">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{title}</title>
<link rel="stylesheet" href="{SYS}/tokens.css">
<link rel="stylesheet" href="{SYS}/components/bundle.css">
<style>
html, body {{ margin: 0; height: 100%; background: var(--color-surface-page); }}
.lg-form {{ display: flex; flex-direction: column; gap: var(--space-4); width: 320px; }}
.lg-h {{ margin: 0; font: var(--font-weight-strong) var(--font-size-title) / var(--font-line-height-title) var(--font-family-text); color: var(--color-text-primary); }}
.lg-form .cl-btn--primary {{ height: var(--size-control-large); justify-content: center; }}
.lg-name {{ font: var(--font-weight-strong) 26px / 1 var(--font-family-mono); color: var(--color-text-primary); display: flex; align-items: center; gap: 10px; }}
.lg-name svg {{ width: 40px; height: 40px; }}
.lg-foot {{ font: var(--font-weight-regular) var(--font-size-caption) / var(--font-line-height-caption) var(--font-family-mono); color: var(--color-text-tertiary); }}
{VITALS_STYLE}
{style}
</style>
</head>
<body>
<div class="cl-root" style="height: 100%;">
{body}
</div>
</body>
</html>
'''


PROMISE = 'Hear the airband you receive, read it in English and French, and see which aircraft is speaking.'

VITALS_STYLE = '''
.vt { display: grid; grid-template-columns: max-content 1fr; gap: 6px var(--space-4); margin: 0; font: var(--font-weight-regular) var(--font-size-small) / var(--font-line-height-small) var(--font-family-mono); }
.vt dt { color: var(--color-text-tertiary); text-transform: lowercase; }
.vt dd { margin: 0; color: var(--color-text-primary); display: flex; gap: var(--space-2); flex-wrap: wrap; align-items: center; }
.vt dd i { width: 8px; height: 8px; border-radius: 50%; background: var(--color-status-ok); display: inline-block; }
.vt dd span { color: var(--color-text-tertiary); }
.vt-head { display: flex; justify-content: space-between; align-items: baseline; font: var(--font-weight-regular) var(--font-size-caption) / var(--font-line-height-caption) var(--font-family-mono); color: var(--color-text-tertiary); text-transform: uppercase; letter-spacing: var(--font-tracking-label); }
'''


def vitals():
    """The station's vital signs, aggregated only: no position, no callsign, no named frequency."""
    return '''<div class="vt-wrap" style="display: flex; flex-direction: column; gap: var(--space-2);">
<div class="vt-head"><span>This station, live</span><span>refreshed every 10 s</span></div>
<dl class="vt">
  <dt>ads-b</dt><dd><i></i>107 aircraft <span>·</span> 702 msg/s <span>·</span> farthest 182 NM</dd>
  <dt>airband</dt><dd><i></i>2 channels on air <span>·</span> 412 transmissions today</dd>
  <dt>speech</dt><dd><i></i>whisper large-v3 <span>·</span> GPU <span>·</span> 1.4 s a transmission</dd>
  <dt>matched</dt><dd>31 % of transmissions to an aircraft today</dd>
  <dt>runways</dt><dd>3 airports followed <span>·</span> 4 runways in use</dd>
  <dt>up</dt><dd>6 d 14 h <span>·</span> Clairance 2026.10 <span>·</span> a7489d5</dd>
</dl>
</div>'''
LOCAL = 'Runs on your own machine: nothing leaves it. Made for the listener; it judges no one.'

# ---------------------------------------------------------------- A · the scope
A_STYLE = '''
.a { height: 100%; display: grid; grid-template-columns: 1fr 460px; }
.a-mark { position: relative; overflow: hidden; display: flex; flex-direction: column; justify-content: center; gap: var(--space-8); padding: 0 96px; border-right: var(--border-width-default) solid var(--color-border-default); }
.a-big { display: flex; align-items: center; gap: var(--space-8); }
.a-big svg { width: 240px; height: 240px; flex: none; }
.a-big b { font: var(--font-weight-strong) 88px / 0.95 var(--font-family-mono); color: var(--color-text-primary); letter-spacing: -0.02em; }
.a-big p { margin: var(--space-4) 0 0; max-width: 480px; font: var(--font-weight-regular) var(--font-size-title) / 26px var(--font-family-text); color: var(--color-text-secondary); }
.a-mark .vt-wrap { max-width: 640px; padding-top: var(--space-6); border-top: var(--border-width-default) solid var(--color-border-default); }
.a-side { display: flex; flex-direction: column; justify-content: center; gap: var(--space-8); padding: 0 var(--space-8) 0 64px; background: var(--color-surface-panel); }
'''


def a_scope():
    body = f'''<div class="a">
  <div class="a-mark">
    <div class="a-big">{SIGN_64}<div><b>Clairance</b><p>{PROMISE}</p></div></div>
    {vitals()}
    <span class="lg-foot">{LOCAL}</span>
  </div>
  <div class="a-side">{form()}<span class="lg-foot">station.lan · version 2026.10</span></div>
</div>'''
    return page('Sign in · A · the scope', A_STYLE, body)


# ---------------------------------------------------------------- B · an exchange
B_STYLE = '''
.b { height: 100%; display: grid; grid-template-columns: 1fr 480px; }
.b-left { display: flex; flex-direction: column; justify-content: center; gap: var(--space-6); padding: 0 96px; }
.b-name { display: flex; align-items: center; gap: var(--space-6); font: var(--font-weight-strong) 88px / 0.95 var(--font-family-mono); color: var(--color-text-primary); letter-spacing: -0.02em; }
.b-name svg { width: 96px; height: 96px; }
.b-left > p { margin: 0; max-width: 560px; font: var(--font-weight-regular) var(--font-size-title) / 26px var(--font-family-text); color: var(--color-text-secondary); }
.b-example { max-width: 560px; border: var(--border-width-default) solid var(--color-border-default); border-radius: var(--radius-lg); overflow: hidden; }
.b-example .cl-label { display: block; padding: var(--space-2) var(--space-3) 0; background: var(--color-surface-panel); }
.b-right { display: flex; flex-direction: column; justify-content: center; gap: var(--space-8); padding: 0 var(--space-8) 0 64px; background: var(--color-surface-panel); border-left: var(--border-width-default) solid var(--color-border-default); }
.b-right .vt { font-size: var(--font-size-caption); line-height: var(--font-line-height-caption); }
'''


def b_echange():
    chip = '<span class="cl-chip cl-chip--callsign"><span class="cl-phase cl-phase--arr cl-phase--small">APP</span>AFR47KX</span>'
    body = f'''<div class="b">
  <div class="b-left">
    <div class="b-name">{SIGN_64}Clairance</div>
    <p>{PROMISE}</p>
    <div class="b-example"><span class="cl-label">An example of what you will read</span>
      <article class="cl-exchange" lang="fr">
        <div class="cl-exchange__head" lang="en"><span class="cl-exchange__when">124.350</span><span class="cl-chip cl-chip--atc">ATC</span><span class="cl-exchange__to">→</span>{chip}</div>
        <p class="cl-exchange__text">Air France quatre sept Kilo X-ray, autorisé ILS piste deux six gauche, contactez la tour un deux zéro décimal six cinq.</p>
        <div class="cl-exchange__instr" lang="en"><span class="cl-chip cl-chip--instr cl-chip--clearance">Cleared ILS runway 26L</span><span class="cl-chip cl-chip--instr">Contact 120.650</span></div>
        <div class="cl-exchange__explain cl-exchange__explain--boxed">Autorisé à l'approche ILS, piste 26L. Contactez 120.650.</div>
        <div class="cl-exchange__reply" lang="en"><span>↩</span><span>Pilot replied, 4 s later</span></div>
      </article>
    </div>
    <p class="lg-foot">{LOCAL}</p>
  </div>
  <div class="b-right">{form()}{vitals()}</div>
</div>'''
    return page('Sign in · B · an exchange', B_STYLE, body)


# ---------------------------------------------------------------- C · the sky
C_STYLE = '''
.c { position: relative; height: 100%; overflow: hidden; background: var(--color-surface-map); }
.c > svg { position: absolute; inset: 0; width: 100%; height: 100%; }
.c-card { position: absolute; left: 50%; top: 50%; transform: translate(-50%, -50%); width: 400px; padding: var(--space-8); display: flex; flex-direction: column; gap: var(--space-6);
  background: color-mix(in srgb, var(--color-surface-panel) 92%, transparent); border: var(--border-width-default) solid var(--color-border-default); border-radius: var(--radius-lg); box-shadow: var(--shadow-overlay); }
.c-card .lg-form { width: auto; }
.c-card p { margin: 0; font: var(--font-weight-regular) var(--font-size-small) / var(--font-line-height-small) var(--font-family-text); color: var(--color-text-secondary); }
.c-foot { position: absolute; left: 0; right: 0; bottom: var(--space-6); text-align: center; }
.c-brand { position: absolute; left: var(--space-8); top: var(--space-8); display: flex; align-items: center; gap: var(--space-4); font: var(--font-weight-strong) 56px / 1 var(--font-family-mono); color: var(--color-text-primary); letter-spacing: -0.02em; }
.c-brand svg { width: 64px; height: 64px; }
.c-console { position: absolute; right: var(--space-8); top: var(--space-8); width: 420px; padding: var(--space-4); background: color-mix(in srgb, var(--color-surface-page) 88%, transparent); border: var(--border-width-default) solid var(--color-border-default); border-radius: var(--radius-lg); }
.c-console .vt { font-size: var(--font-size-caption); line-height: var(--font-line-height-caption); }
'''


def scope_svg(cls='', w=1440, h=900):
    """The sign-in background: the sign's scope at the size of the page (rings, still sweep, afterglow,
    one blip) and a few silhouettes of our family on the altitude ramp. Decorative: no real data."""
    cx, cy = w // 2, h // 2
    rings = ''.join(f'<circle cx="{cx}" cy="{cy}" r="{r}" fill="none" stroke="#333333"/>' for r in (150, 300, 450, 600))
    # the background is the sign's own scope: the same sweep at 45 degrees and its afterglow, still
    R = 600
    ex, ey = cx + R * math.sin(math.radians(45)), cy - R * math.cos(math.radians(45))
    rings = (f'<path d="M{cx} {cy} L{cx} {cy - R} A{R} {R} 0 0 1 {ex:.1f} {ey:.1f} Z" fill="#4caf50" fill-opacity="0.10"/>' + rings +
             f'<line x1="{cx}" y1="{cy}" x2="{ex:.1f}" y2="{ey:.1f}" stroke="#4caf50" stroke-opacity="0.6" stroke-width="2"/>'
             f'<circle cx="{cx + 230 * math.sin(math.radians(25)):.1f}" cy="{cy - 230 * math.cos(math.radians(25)):.1f}" r="6" fill="#4caf50"/>')
    labels = ''.join(f'<text x="{cx}" y="{cy - r - 6}" text-anchor="middle" font-family="ui-monospace,Menlo,monospace" font-size="11" fill="#6b6b6b">{n} NM</text>'
                     for r, n in ((150, 5), (300, 10), (450, 15)))
    planes = ''.join([
        aircraft('bireacteur', 330, 210, 78, 26, '#c2a3ff'),
        aircraft('gros', 1150, 170, 250, 30, '#6e7b8f'),
        aircraft('leger', 560, 640, -30, 20, '#fff59a'),
        aircraft('turboprop', 1080, 690, 120, 24, '#e68cba'),
        aircraft('bireacteur', 1260, 470, 200, 26, '#61c9e3'),
        aircraft('affaires', 200, 560, 40, 22, '#61c9e3'),
        aircraft('helico', 880, 760, 0, 22, '#fff59a'),
    ])
    return f'<svg class="{cls}" viewBox="0 0 {w} {h}" preserveAspectRatio="xMidYMid slice" aria-hidden="true">{rings}{labels}{planes}</svg>'


def c_ciel():
    body = f'''<div class="c">
  {scope_svg()}
  <div class="c-brand">{SIGN_64}Clairance</div>
  <div class="c-console">{vitals()}</div>
  <div class="c-card"><p>{PROMISE}</p>{form()}</div>
  <div class="c-foot lg-foot">{LOCAL}</div>
</div>'''
    return page('Sign in · C · the sky', C_STYLE, body)


if __name__ == '__main__':
    os.makedirs(OUT, exist_ok=True)
    for name, html in (('a-scope.html', a_scope()), ('b-echange.html', b_echange()), ('c-ciel.html', c_ciel())):
        with open(os.path.join(OUT, name), 'w') as f:
            f.write(html)
        print(os.path.relpath(os.path.join(OUT, name), ROOT))
