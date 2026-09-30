# Tasks: Single-Floor Conceptual 3D Preview

**Status:** Proposed. T001 must be approved before implementing T003 onward. Complete contract decisions in T002 before parallel backend and frontend work.

## Scope and Contracts

- [ ] T001 Approve the single-floor conceptual visualization as an explicit extension of `.specify/memory/constitution.md`; keep no-fabrication, human verification, deterministic QS, and multi-floor exclusions intact.
- [ ] T002 Define the confirmation revision contract: response fields, latest and historical read endpoints, 404 behavior, matching analysis IDs, and migration/fallback for previously confirmed data without invented revision history. Record the decision in `plan.md` before coding.

## Confirmed Data Backend

- [ ] T003 Add immutable per-analysis confirmation revisions in `api/internal/analysis/` and additive PostgreSQL persistence. Keep `Confirmed(analysisId)` returning the latest validated data for existing estimate callers; handle old persisted confirmations explicitly. (T002; acceptance 6)
- [ ] T004 Add read-only latest and revision-specific confirmed-data API handlers in `api/cmd/server/main.go`; do not expose a candidate as confirmed or change the existing analysis status API. (T003; acceptance 1, 6)
- [ ] T005 Add service, repository, and HTTP tests for first and repeated confirmations, stable revision retrieval, restart persistence, old records, missing/invalid IDs, 404, and unchanged estimate input. (T003-T004; acceptance 1, 6, 8)

## Read-Only Scene Data

- [ ] T006 Define a typed client projection from an identified confirmed snapshot to eligible rectangular rooms. Convert supported units to meters for scene dimensions while retaining original units for labels; reject missing, nonfinite, nonpositive, and unsupported measurements rather than substituting values. (T002; acceptance 2, 5)
- [ ] T007 Keep openings without verified positions in the text-only uncertainty list and exclude cutouts, shared walls, and inferred adjacency from the scene. Mark all layout positions and appearance as conceptual display choices. (T006; acceptance 3-4)
- [ ] T008 Test projection with one and multiple rooms, mixed units, malformed dimensions, absent rooms, unknown opening locations, and different confirmed revisions. Assert that it cannot write to estimate or confirmation inputs. (T006-T007; acceptance 2-6, 8)

## Review and Visualization

- [ ] T009 Add `Review | 3D preview` navigation to the existing Next.js analysis workspace. Before confirmation show verification required and a text list of unverified inputs; never render candidate dimensions as verified geometry. (T002; acceptance 1)
- [ ] T010 Integrate the confirmed snapshot API with the view. Label its analysis ID and revision; keep edits tied to the older confirmed preview until reconfirmation; clear or explicitly distinguish a previous analysis when a new run begins. Handle confirmation success even if subsequent estimate creation fails. (T004, T009; acceptance 6)
- [ ] T011 Install Three.js and implement a lazily loaded, client-only, responsive, unframed scene with separated room volumes, orbit/zoom/reset controls, readable room labels, and the persistent conceptual-arrangement warning. No geometry derived from the scene may be submitted to backend APIs. (T006-T007, T009; acceptance 2-4, 9)
- [ ] T012 Provide an equivalent room/uncertainty list and localized loading, empty, error, retry, and WebGL-unavailable states. Keep review, confirmation, estimate, and Excel actions usable without 3D. (T009-T011; acceptance 5, 7, 9)

## Verification

- [ ] T013 Add focused browser tests for pre-confirmation gating, confirmed room labels and units, conceptual layout, unknown opening positions, missing dimensions, edits/re-analysis, revision changes, and rendering failure fallback. (T010-T012; acceptance 1-7)
- [ ] T014 Compare quantities, costs, and Excel output for the same confirmed input before and after opening or manipulating the preview; verify no new estimate or confirmation is created by viewing it. (T010-T012; acceptance 8)
- [ ] T015 Use Playwright desktop and mobile screenshots plus canvas-pixel checks to confirm the scene is nonblank, correctly framed, interactive, and free from overlapping labels or controls; test keyboard-accessible text fallback. (T011-T012; acceptance 9)
- [ ] T016 Run Go API tests, client typecheck and lint, focused end-to-end tests, and the existing estimate/export workflow; document preview limitations and revision semantics. (T005, T008, T013-T015)

## Dependency Order

T001 -> T002 -> backend T003-T005 and client projection T006-T008 -> UI T009-T012 -> verification T013-T016. After T002, T003 and T006 can proceed independently; UI may be developed against the agreed contract with mock responses.