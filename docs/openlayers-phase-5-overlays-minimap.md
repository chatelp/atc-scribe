# OpenLayers Migration — Phase 5 Runbook (Reference Overlays, Aviation/Weather Layers, Mini-Map)

**Status:** Ready to execute
**Phase window:** 7–10 days
**Pre-req:** Phase 4 complete (`docs/openlayers-phase-4-trails-labels-proximity.md`)
**Parent RFC:** `docs/openlayers-full-migration-rfc.md`

---

## 1) Phase Objective

Port remaining map feature layers and deliver aviation-focused overlay capabilities:

- airports, heliports, navaids, runways, all-runways, rings
- aviation chart overlays/basemaps (as approved by source policy)
- weather radar/cloud overlays
- airspace polygons/rings overlays
- mini-map parity for selected aircraft details

This phase must maintain stability of aircraft rendering while overlays are enabled and toggled.

---

## 2) Required Outcomes

By end of Phase 5, all must be true:

1. Existing reference layers are parity-equivalent.
2. Overlay framework supports aviation/weather/airspace layers with isolated failure handling.
3. Mini-map functionality is parity-equivalent.
4. Overlay toggles, opacity, visibility and refresh behavior are stable.
5. Overlay failures do not impact primary aircraft rendering.

---

## 3) Scope Boundaries

### In scope
- Layer registry expansion for all reference + external overlays
- Porting current reference renderers to OpenLayers
- Overlay source adapters (XYZ/WMS/GeoJSON/Vector)
- Per-layer controls (enabled, opacity, zoom gates, refresh cadence)
- Mini-map implementation parity
- Source policy metadata + attribution/license notes

### Out of scope
- Full Leaflet deletion (Phase 6)
- Final signoff stabilization (Phase 7)

---

## 4) Files to Touch

Primary:
- `www/map/core/layer-registry.js`
- `www/map/renderers/reference.js`
- `www/map/features/minimap.js`
- `www/map/openlayers-map-manager.js`
- `www/map/core/map-engine.js`

Potential supporting:
- `www/app.js` (UI control wiring for overlay toggles/opacities)
- `www/style.css` (overlay legend/control polish only as needed)
- `docs/openlayers-full-migration-rfc.md` (optional source decisions update)
- `docs/technical_docs.md` (architecture updates)

---

## 5) Overlay Design Requirements

## 5.1 Central Overlay Registry
Every overlay entry should declare:
- `id`
- `type` (base/overlay/reference)
- `sourceType` (XYZ/WMS/GeoJSON/Vector)
- `zIndex`
- `defaultVisible`
- `minZoom` / `maxZoom`
- `opacity`
- `refreshPolicy` (none / interval / on-demand)
- `attribution`
- `licenseNotes`
- `failureIsolation` settings

## 5.2 Failure Isolation
- If an overlay source fails, only that layer is degraded.
- Aircraft rendering remains unaffected.
- Retry policy per layer with bounded backoff.
- Surface non-blocking status in logs/UI diagnostics.

## 5.3 Source Policy Guardrails
For each external source, document:
- usage limits / API key needs
- attribution obligations
- caching allowances/restrictions
- commercial-use constraints

No source should be enabled by default if policy is unclear.

---

## 6) Work Plan (Execution Sequence)

## Step 1 — Reference Layer Port (Day 1–2)

### Tasks
- Port airports/heliports/navaids/runways/all-runways/rings into OpenLayers reference renderer.
- Preserve existing toggle semantics from settings/store.
- Ensure draw order and style intent parity.

### Checkpoint
- All current reference toggles work with expected visuals.

---

## Step 2 — Overlay Registry + Adapter Layer (Day 2–3)

### Tasks
- Implement registry-backed layer factory/adapter for XYZ/WMS/GeoJSON/vector sources.
- Add lifecycle controls for create/show/hide/dispose.
- Add per-layer opacity and zoom gating support.

### Checkpoint
- Multiple overlay types can be declared and loaded consistently.

---

## Step 3 — Aviation Overlay Integration (Day 3–4)

### Tasks
- Integrate approved aviation chart/basemap overlays.
- Add safe defaults (off by default unless explicitly approved).
- Ensure attribution rendering and source metadata are present.

### Checkpoint
- Aviation overlays toggle cleanly and do not break core map flow.

---

## Step 4 — Weather Overlay Integration (Day 4–5)

### Tasks
- Integrate weather radar/cloud overlays via approved sources.
- Add refresh policies (timed refresh/rebuild when required).
- Ensure map responsiveness under overlay refresh events.

### Checkpoint
- Weather overlays update and render without impacting aircraft updates.

---

## Step 5 — Airspace Overlay Integration (Day 5–6)

### Tasks
- Integrate airspace polygons/rings overlays.
- Add optional label behavior and zoom-aware display rules.
- Validate declutter/readability at operational zoom ranges.

### Checkpoint
- Airspace overlays are usable and legible for operations.

---

## Step 6 — Mini-Map Port (Day 6–7)

### Tasks
- Port selected-aircraft detail mini-map lifecycle:
  - init with retries/visibility checks
  - render current/history/hindcast/future layers
  - fit bounds/set view behavior
  - cleanup on close/context change
- Preserve mini-map behavior parity from existing implementation.

### Checkpoint
- Mini-map fully functional with no leaked map instances.

---

## Step 7 — Reliability & Load Validation (Day 7–10)

### Tasks
- Run stress checks with overlays toggled on/off while traffic updates continue.
- Validate failure isolation by simulating overlay source failures.
- Tune layer refresh intervals and retry backoff.

### Checkpoint
- Core aircraft map remains stable regardless of overlay health.

---

## 7) PR Slicing Strategy (Recommended)

1. PR-5A: reference layers port
2. PR-5B: overlay registry + adapters
3. PR-5C: aviation overlays integration
4. PR-5D: weather overlays integration
5. PR-5E: airspace overlays integration
6. PR-5F: mini-map parity port
7. PR-5G: resilience/perf validation and fixes

Each PR must keep app runnable.

---

## 8) Regression Checklist (Phase 5)

### Reference parity
- [ ] Airports toggle works
- [ ] Heliports toggle works
- [ ] Navaids toggle works
- [ ] Runways toggle works
- [ ] All runways toggle works
- [ ] Distance rings toggle works

### Overlay behavior
- [ ] Overlay toggle on/off works per layer
- [ ] Overlay opacity controls apply correctly
- [ ] Zoom-gated visibility behaves correctly
- [ ] Overlay refresh policies execute as configured
- [ ] Attribution/license display requirements are met

### Mini-map parity
- [ ] Mini-map initializes reliably in details view
- [ ] Current/history/hindcast/future rendering works
- [ ] Fit bounds/set view behavior is correct
- [ ] Mini-map cleanup works (no leaks/orphans)

### Isolation/stability
- [ ] Overlay failure does not affect aircraft rendering
- [ ] No map-breaking console errors
- [ ] No severe UI stutter when toggling overlays rapidly

---

## 9) Performance & Reliability Gates (Hard)

Phase 5 passes only if:

1. Aircraft update responsiveness remains within accepted threshold when overlays are active.
2. Overlay refresh operations do not cause prolonged frame collapse.
3. Simulated source failures stay isolated to affected layers.
4. Long-session run with mixed overlays shows no unbounded memory growth.

---

## 10) Known Risks in This Phase

1. **External source instability / policy ambiguity**
   - Mitigation: source policy metadata, safe defaults, disable-by-default for uncertain sources.

2. **Overlay overdraw hurting responsiveness**
   - Mitigation: zoom gates, simplified styles, cautious z-index ordering.

3. **Mini-map lifecycle leaks**
   - Mitigation: strict create/dispose guards and container existence checks.

4. **Attribution/legal non-compliance**
   - Mitigation: per-layer attribution/license review before enablement.

---

## 11) Deliverables

- OpenLayers reference layer parity
- Overlay registry + adapters for aviation/weather/airspace sources
- Mini-map parity implementation
- Resilience validation results for overlay failure isolation

---

## 12) Ownership & Tracking

Fill before execution:

- Phase owner:
- Reviewer(s):
- QA/perf owner:
- Source policy owner:
- Start date:
- Target completion date:

Tracking table:

| Task | Owner | Status | PR | Notes |
|---|---|---|---|---|
| Reference layer parity |  |  |  |  |
| Overlay registry/adapters |  |  |  |  |
| Aviation overlays |  |  |  |  |
| Weather overlays |  |  |  |  |
| Airspace overlays |  |  |  |  |
| Mini-map parity |  |  |  |  |
| Reliability/perf closeout |  |  |  |  |

---

## 13) Handoff to Phase 6

At completion, provide:

- Confirmation that only Leaflet removal/cleanup remains for migration stack
- List of any temporary compatibility shims still in code
- Source policy summary for enabled overlays
- Explicit “ready for Phase 6” decision
