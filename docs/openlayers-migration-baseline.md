# OpenLayers Migration Baseline

**Status:** Active baseline capture template
**Date:** 2026-02-18
**Related RFC:** `docs/openlayers-full-migration-rfc.md`

---

## 1) Purpose

This document captures the pre-migration baseline for map behavior and performance.
It is the reference used to decide whether OpenLayers migration is acceptable.

---

## 2) Environment Snapshot

Fill this out before each baseline run.

- Machine:
- OS:
- Browser + version:
- GPU (if available):
- Build/branch:
- Config profile (`configs/config.toml` notes):
- Data source (`local` / `external`):
- Approx aircraft volume during run:
- Run date/time:

---

## 3) Baseline Scenarios

Run each scenario for at least 5 minutes unless noted.

### Scenario A — Normal Traffic
- Zoom: operational default
- Aircraft count: normal expected range
- Actions: pan/zoom, select/deselect, toggle labels/trails/rings

### Scenario B — High Traffic
- Aircraft count: peak expected + stress margin
- Actions: frequent map movement, search/filter changes, selected aircraft tracking

### Scenario C — Overlay Stress
- Enable all current overlays/features available in Leaflet stack
- Actions: pan/zoom continuously, switch selected aircraft repeatedly

### Scenario D — Long Session Stability
- Duration: 24 hours
- Actions: normal operator workflow
- Goal: watch for memory growth / stutter / stale UI state

---

## 4) Quantitative Metrics

Use current map perf stats and browser perf tools.

### 4.1 Map Manager Stats (per 60s windows)
Capture at least 5 windows per scenario.

| Scenario | refreshPerSec | avgRefreshMs | coalescedPct | markerOpsPerSec | singleUpdatesPerSec | visualSkipPct | trailLayerOpsPerSec | fullVisibilityPasses |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| A-1 |  |  |  |  |  |  |  |  |
| A-2 |  |  |  |  |  |  |  |  |
| B-1 |  |  |  |  |  |  |  |  |
| B-2 |  |  |  |  |  |  |  |  |
| C-1 |  |  |  |  |  |  |  |  |

### 4.2 Browser Metrics

| Scenario | Avg FPS | Main-thread busy % (est.) | JS heap start (MB) | JS heap end (MB) | Notable GC pauses |
|---|---:|---:|---:|---:|---|
| A |  |  |  |  |  |
| B |  |  |  |  |  |
| C |  |  |  |  |  |
| D |  |  |  |  |  |

### 4.3 Network (optional but useful)

| Scenario | WS msg rate | WS throughput | Tile/cache behavior notes |
|---|---:|---:|---|
| A |  |  |  |
| B |  |  |  |
| C |  |  |  |

---

## 5) Functional Parity Baseline Checklist

Mark current Leaflet behavior before migration.

### Interactions
- [ ] Map click deselects selected aircraft
- [ ] Double-click clears search (when populated)
- [ ] Station override map-click mode sets coordinates correctly
- [ ] Selected aircraft remains visible when filters would hide others

### Rendering
- [ ] Aircraft position/heading updates are visually consistent
- [ ] Labels show/hide correctly via toggle
- [ ] Label stale timer updates last-seen text
- [ ] Trail length slider changes rendered trail depth correctly
- [ ] History/hindcast/future paths render for selected aircraft

### Map Features
- [ ] Distance rings toggle works
- [ ] Airports toggle works
- [ ] Heliports toggle works
- [ ] Navaids toggle works
- [ ] All runways toggle works
- [ ] Proximity circle + inclusion behavior works
- [ ] Mini-map loads, updates, and cleans up correctly

### Stability
- [ ] No unbounded memory growth in long session
- [ ] No progressive lag after repeated selection/filter/map movement
- [ ] No recurring console errors affecting map behavior

---

## 6) Baseline Thresholds for Migration Acceptance

These are the minimum requirements post-migration (OpenLayers):

1. **Functional parity:** all checklist items above must pass.
2. **Performance floor:**
   - `avgRefreshMs` must be <= baseline + 10% in normal traffic.
   - Under high traffic, responsiveness must be baseline-equivalent or better.
3. **Stability floor:**
   - No critical regressions in 30–60 min run.
   - No map-breaking errors from optional overlays.
4. **Operator usability:**
   - Selection, labels, and trails behavior must be operationally equivalent.

---

## 7) Regression Log During Migration

Use this table throughout migration phases.

| Date | Phase | Scenario | Regression | Severity | Status | Owner | Notes |
|---|---|---|---|---|---|---|---|
|  |  |  |  |  |  |  |  |

Severity guide:
- **P0:** blocks operations / map unusable
- **P1:** major workflow break
- **P2:** noticeable degradation with workaround
- **P3:** minor issue

---

## 8) Go / No-Go Summary

Complete after OpenLayers implementation reaches Phase 7.

- Functional parity: **PASS / FAIL**
- Performance gate: **PASS / FAIL**
- Stability gate: **PASS / FAIL**
- Overlay isolation gate: **PASS / FAIL**
- Final decision: **GO / NO-GO**

Decision notes:

---

## 9) Sign-Off

- Migration lead:
- QA/perf owner:
- Product/operator representative:
- Sign-off date:
