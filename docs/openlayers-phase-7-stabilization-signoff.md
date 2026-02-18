# OpenLayers Migration — Phase 7 Runbook (Stabilization, Regression, Sign-Off)

**Status:** Ready to execute
**Phase window:** 3–5 days
**Pre-req:** Phase 6 complete (`docs/openlayers-phase-6-leaflet-removal-cleanup.md`)
**Parent RFC:** `docs/openlayers-full-migration-rfc.md`

---

## 1) Phase Objective

Validate final production readiness of the OpenLayers migration through focused regression, performance comparison, and operational sign-off.

This phase closes the migration project.

---

## 2) Required Outcomes

By end of Phase 7, all must be true:

1. Functional parity checklist is fully green.
2. Performance gates are met against baseline.
3. No unresolved P0/P1 map regressions remain.
4. Operational stakeholders sign off.
5. Migration RFC is marked complete.

---

## 3) Scope Boundaries

### In scope
- Full regression runs across map workflows
- Baseline comparison and performance analysis
- Defect triage and migration-related fixes
- Final go/no-go decision and sign-off
- Final documentation status updates

### Out of scope
- New non-migration features
- Broad refactors unrelated to parity/performance/stability

---

## 4) Inputs Required

- `docs/openlayers-migration-baseline.md` (captured baseline)
- `docs/openlayers-full-migration-rfc.md` (criteria and overall scope)
- Phase runbooks and closeout notes from Phases 1–6
- Current branch/release candidate build

---

## 5) Validation Plan

## 5.1 Functional Regression Sweep

Use the parity checklist from baseline + RFC. Confirm:
- map interactions (click/dblclick/station-click)
- aircraft rendering and selection behavior
- filters and visibility rules
- trails/history/hindcast/future
- labels + stale updates
- proximity behavior
- reference overlays + external overlays
- mini-map behavior

## 5.2 Performance Comparison

Run baseline scenarios (A/B/C/D) and compare:
- refresh/update metrics
- responsiveness under high traffic
- memory behavior in long session
- overlay impact when enabled

## 5.3 Reliability / Failure Isolation

Simulate/observe:
- overlay source failures
- network jitter or delayed updates
- rapid operator interactions

Confirm core aircraft rendering remains stable.

---

## 6) Work Plan (Execution Sequence)

## Step 1 — Freeze Window + Candidate Build (Day 1)

### Tasks
- Freeze non-migration map changes.
- Produce release candidate build for validation.

### Checkpoint
- Single candidate target for all validation passes.

---

## Step 2 — Functional Regression Execution (Day 1–2)

### Tasks
- Execute full map parity checklist.
- Record defects in regression log with severity.

### Checkpoint
- Complete defect list with owners and ETA.

---

## Step 3 — Performance Runs (Day 2–3)

### Tasks
- Re-run baseline scenarios and fill comparison tables.
- Identify any threshold misses.

### Checkpoint
- Performance verdict documented (pass/fail with rationale).

---

## Step 4 — Defect Burn-Down (Day 3–4)

### Tasks
- Fix migration regressions only (P0/P1 first).
- Re-test impacted areas immediately.

### Checkpoint
- No open P0/P1 issues.

---

## Step 5 — Final Sign-Off Review (Day 4–5)

### Tasks
- Present parity + performance + reliability summary.
- Capture go/no-go decision.
- Mark RFC complete if go.

### Checkpoint
- Formal decision logged with approvers.

---

## 7) PR Slicing Strategy (Recommended)

1. PR-7A: validation report scaffolding (no code changes)
2. PR-7B: focused regression fixes set 1
3. PR-7C: focused regression fixes set 2 (if needed)
4. PR-7D: final docs/status update and sign-off record

Keep changes narrowly scoped to migration readiness.

---

## 8) Exit Gates (Hard)

All gates must pass for GO:

1. **Functional parity gate:** complete pass.
2. **Performance gate:** meets baseline acceptance thresholds.
3. **Stability gate:** no critical degradation in long session.
4. **Isolation gate:** overlay failures do not break aircraft operations.
5. **Operations gate:** stakeholder sign-off complete.

If any gate fails, decision is NO-GO until corrected.

---

## 9) Regression & Sign-Off Artifacts

Create/fill these artifacts:

- Baseline comparison summary (append to `openlayers-migration-baseline.md`)
- Final migration decision note (append to RFC)
- Defect log closure table (open/closed counts by severity)

Suggested summary table:

| Gate | Result | Evidence | Owner |
|---|---|---|---|
| Functional parity |  |  |  |
| Performance |  |  |  |
| Stability |  |  |  |
| Isolation |  |  |  |
| Operations sign-off |  |  |  |

---

## 10) Known Risks in This Phase

1. **Late-discovered edge-case regressions**
   - Mitigation: prioritize operator-critical workflows first.

2. **Performance regressions hidden by short tests**
   - Mitigation: enforce long-session run before sign-off.

3. **Scope creep in stabilization**
   - Mitigation: migration-only fixes policy during Phase 7.

---

## 11) Deliverables

- Completed regression and performance validation
- Closed critical migration defects
- Formal go/no-go decision
- RFC completion status update

---

## 12) Ownership & Tracking

Fill before execution:

- Phase owner:
- QA lead:
- Perf lead:
- Operations approver:
- Start date:
- Target completion date:

Tracking table:

| Task | Owner | Status | PR | Notes |
|---|---|---|---|---|
| Candidate build freeze |  |  |  |  |
| Functional regression run |  |  |  |  |
| Performance comparison run |  |  |  |  |
| Defect burn-down |  |  |  |  |
| Final sign-off |  |  |  |  |

---

## 13) Project Closure Step

After GO decision:

- Mark migration complete in `docs/openlayers-full-migration-rfc.md`.
- Update `docs/technical_docs.md` and README to final OpenLayers architecture language.
- Archive this phase doc as executed with date and approvers.
