# OpenLayers Migration — Phase 3 Runbook (Aircraft Rendering, WebGL-First)

**Status:** Ready to execute
**Phase window:** 7–10 days
**Pre-req:** Phase 2 complete (`docs/openlayers-phase-2-foundation.md`)
**Parent RFC:** `docs/openlayers-full-migration-rfc.md`

---

## 1) Phase Objective

Implement aircraft rendering on OpenLayers using a WebGL-first path, while preserving behavior expected by existing map/store flows.

This phase focuses on aircraft positions, heading/visual state, filtering visibility, selected-aircraft behavior, and update throughput under high traffic.

---

## 2) Required Outcomes

By end of Phase 3, all must be true:

1. Aircraft rendering is active on OpenLayers and stable.
2. Single-aircraft update path is fast and parity-equivalent.
3. Filtering/visibility logic is functionally equivalent to baseline.
4. Selected-aircraft visibility and styling behavior is preserved.
5. High-load performance is baseline-equivalent or better.

---

## 3) Scope Boundaries

### In scope
- Aircraft feature source + renderer implementation
- Heading/rotation/state style mapping
- Visibility/filter integration
- Incremental updates and stale aircraft removal
- Performance instrumentation and high-load validation

### Out of scope
- Full trails/hindcast/future parity (Phase 4)
- Full label parity (Phase 4)
- Full overlay migration (Phase 5)
- Leaflet deletion (Phase 6)

---

## 4) Files to Touch

Primary:
- `www/map/renderers/aircraft-webgl.js`
- `www/map/core/visibility-rules.js`
- `www/map/perf/telemetry.js`
- `www/map/openlayers-map-manager.js`

Potential supporting:
- `www/app.js` (only if integration hooks require minimal updates)
- `www/map/core/map-engine.js` (source/layer lifecycle helpers)
- `www/map-manager.js` (temporary compatibility forwarding, if still present)

---

## 5) Data & Rendering Model (Target)

Use an OpenLayers vector source for aircraft features with WebGL points rendering.

Per-aircraft feature properties should include at least:
- `hex`
- `lat`, `lon`
- `heading` (or fallback track)
- `on_ground`
- `status`
- `phase`
- `selected`
- `visibilityFlags` (optional packed model)
- style keys needed by renderer

Rendering requirements:
- Rotation by heading/track with stable fallback
- Color/state styling equivalent to existing visual cues
- Selected aircraft prominence behavior preserved
- Fast incremental updates (avoid full source rebuild each tick)

---

## 6) Work Plan (Execution Sequence)

## Step 1 — Aircraft Source + Layer Scaffolding (Day 1)

### Tasks
- Create aircraft vector source and WebGL points layer in `aircraft-webgl.js`.
- Register layer in central layer registry with proper z-index.
- Expose init/dispose hooks through map manager.

### Checkpoint
- Layer initializes with no runtime errors.
- Empty map state remains stable.

---

## Step 2 — Feature Lifecycle API (Day 1–2)

### Tasks
- Implement explicit lifecycle methods:
  - `upsertAircraftFeature(aircraft)`
  - `removeAircraftFeature(hex)`
  - `bulkSyncAircraft(aircraftMap)`
- Maintain `hex -> feature` index for O(1) lookup.

### Checkpoint
- Insert/update/remove works deterministically.
- No orphan features after removals.

---

## Step 3 — Visual State Mapping (Day 2–3)

### Tasks
- Port current visual logic into style-property mapping:
  - heading/track fallback
  - status/phase color decisions
  - selected vs non-selected emphasis
  - signal-lost/stale treatment
- Ensure rotation units and projection handling are consistent.

### Checkpoint
- Visual behavior matches baseline intent for representative aircraft states.

---

## Step 4 — Filter & Visibility Parity (Day 3–4)

### Tasks
- Integrate existing filter predicates from `visibility-rules.js`:
  - search
  - altitude
  - phase
  - air/ground toggles
  - last-seen cutoff
- Preserve selected-aircraft override (selected remains visible even if filtered).
- Keep viewport-related logic equivalent where applicable.

### Checkpoint
- Filter outcomes match baseline for test dataset.

---

## Step 5 — Fast Update Path (Day 4–6)

### Tasks
- Wire `updateSingleAircraft(hex, aircraft)` to update only changed feature properties.
- Keep refresh coalescing behavior compatible with existing app rhythm.
- Avoid full layer/source rebuild on routine websocket updates.

### Checkpoint
- Single-aircraft update path operational under websocket update load.
- No frame collapse during normal traffic.

---

## Step 6 — Stale Cleanup + Consistency (Day 6–7)

### Tasks
- Port stale/orphan cleanup behavior from existing manager semantics.
- Ensure removed aircraft also clear associated aircraft-specific caches.
- Keep selected-aircraft consistency if selected item disappears.

### Checkpoint
- No stale ghost aircraft after removals.
- No selected-aircraft desync issues.

---

## Step 7 — Performance Tuning & Validation (Day 7–10)

### Tasks
- Add/verify perf telemetry for aircraft rendering path:
  - updates/sec
  - avg update cost
  - feature count
  - dropped/coalesced refresh ratio
- Run baseline scenarios A/B from `openlayers-migration-baseline.md`.
- Compare against acceptance thresholds.

### Checkpoint
- High-load behavior baseline-equivalent or better.
- No P0/P1 regressions in aircraft rendering workflow.

---

## 7) PR Slicing Strategy (Recommended)

1. PR-3A: aircraft layer/source scaffolding
2. PR-3B: feature lifecycle APIs + indexing
3. PR-3C: visual state/rotation mapping
4. PR-3D: filter/visibility integration
5. PR-3E: single-aircraft fast update path
6. PR-3F: stale cleanup + perf instrumentation
7. PR-3G: high-load validation + fixes

Each PR must keep app runnable.

---

## 8) Regression Checklist (Phase 3)

After each PR:

- [ ] Aircraft appear at correct positions
- [ ] Heading/rotation behaves as expected
- [ ] Selected aircraft remains visually emphasized
- [ ] Selected aircraft remains visible through filters
- [ ] Search/altitude/phase/air-ground filters behave correctly
- [ ] Single-aircraft updates do not trigger full redraw regressions
- [ ] Aircraft removals cleanly remove rendered feature
- [ ] No duplicate aircraft rendering artifacts
- [ ] No new map-breaking console errors

---

## 9) Performance Gates (Hard)

Use baseline docs as source of truth.

Phase 3 passes only if:

1. Normal traffic: `avgRefreshMs` (or equivalent OpenLayers metric) <= baseline + 10%.
2. High traffic: responsiveness and interaction remain operationally equivalent or better.
3. No sustained FPS collapse during scenario B.
4. No unbounded memory growth in 30+ minute aircraft-heavy run.

If gate fails, do not proceed to Phase 4 until corrected or formally accepted as temporary debt.

---

## 10) Known Risks in This Phase

1. **Projection/rotation mismatch**
   - Mitigation: centralize coordinate conversion and rotation unit handling.

2. **Full-rebuild regressions under frequent websocket updates**
   - Mitigation: enforce incremental feature updates and indexing.

3. **Selection behavior drift**
   - Mitigation: explicit selection parity tests every PR.

4. **WebGL feature-style constraints**
   - Mitigation: encode style state as feature properties and minimize per-frame style recomputation.

---

## 11) Deliverables

- Working OpenLayers aircraft renderer module (`aircraft-webgl.js`)
- Stable integration in `openlayers-map-manager.js`
- Updated telemetry for aircraft rendering path
- Validation notes against baseline performance scenarios

---

## 12) Ownership & Tracking

Fill before execution:

- Phase owner:
- Reviewer(s):
- QA/perf owner:
- Start date:
- Target completion date:

Tracking table:

| Task | Owner | Status | PR | Notes |
|---|---|---|---|---|
| Layer/source scaffolding |  |  |  |  |
| Feature lifecycle API |  |  |  |  |
| Visual state mapping |  |  |  |  |
| Filter parity integration |  |  |  |  |
| Fast update path |  |  |  |  |
| Stale cleanup |  |  |  |  |
| Perf validation |  |  |  |  |

---

## 13) Handoff to Phase 4

At completion, provide:

- List of remaining trail/label/proximity gaps
- Known aircraft rendering caveats (if any)
- Performance summary vs baseline
- Explicit “ready for Phase 4” decision
