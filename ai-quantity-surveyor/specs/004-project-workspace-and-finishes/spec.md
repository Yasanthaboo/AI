# Project Workspace, Finishes Estimating, and Model Insight Specification

**Status:** Approved under constitution 1.4.0. Implemented; see `tasks.md` for traceability to code and tests.

## Goal

Turn the five-step proof-of-concept wizard into a small product: surveyors can manage many projects, see a project at a glance, trust what the AI and the defaults contributed, estimate the common interior finishes, compare estimate versions, and inspect the model interactively.

## User Stories

- US1 As a surveyor, I see all my projects with their status and latest total, and can search, open, duplicate, and archive them.
- US2 As a surveyor, I open a project overview showing its details, drawing, analysis, confirmation history, estimates, and activity.
- US3 As a surveyor, I select a room in the table, the drawing, or the 3D model and see it highlighted in all three.
- US4 As a surveyor, I see every assumption behind an estimate (defaults, manual exchange rate, rates, waste, approximate placement) so I can defend the number.
- US5 As a surveyor, I compare two estimate versions and see what changed.
- US6 As a surveyor, I estimate flooring, ceiling, skirting, and wall tiling as well as plaster and paint, from the same confirmed geometry.
- US7 As a surveyor, I save my usual rates, currency, unit, default wall height, and waste allowance once.
- US8 As a surveyor, I choose which finishes apply to each room (for example tiles in the bathroom, no flooring in the garage).
- US9 As a surveyor, I see cost by trade and by room as charts.
- US10 As a surveyor, I see which engine and model analysed the drawing, how long it took, the confidence per room, and which values were missing; I can re-run with another engine.
- US11 As a surveyor, I see every confirmation revision and what changed between them.
- US12 As a surveyor, I see doors and windows in the 3D model, hover rooms for details, switch to a top view, and save a screenshot.

## Functional Requirements

### Priority 1 — Workspace and trust

- **FR-001 Projects dashboard.** The app opens on a list of projects showing name, client, location, stage (Draft, Uploaded, Analyzed, Confirmed, Estimated), latest estimate total with currency, and last-updated time, newest first. Users can search by name, client, or location; show or hide archived projects; open; duplicate; archive; and unarchive. Duplicating copies project details and the drawing, not analyses or estimates.
- **FR-002 Project overview.** An Overview view for the open project shows details, a drawing thumbnail, analysis details (FR-013), room and opening counts, the latest estimate total, confirmation history (FR-014), and an activity timeline (created, drawing uploaded, analysis started/completed, each confirmation, each estimate). The drawing is served by the API so it is available in any browser.
- **FR-003 Linked room selection.** On the model step, selecting a room in the table, in the drawing overlay, or in the 3D model highlights the same room in all three views. Selecting it again clears the selection.
- **FR-004 Assumptions panel.** Each estimate lists its assumptions: rooms whose wall height is the configured default, rooms placed approximately in the model, the exchange rate as a manual entry with its timestamp, all rates, the waste allowance, and per-room finish exclusions or tiling. The panel is included in print output.
- **FR-005 Version comparison.** With two or more estimate versions, the user picks two and sees quantities, rates, currency, per-trade costs, per-room costs, and grand total side by side with the difference and percent change.

### Priority 2 — Finishes estimating

- **FR-006 Finishes quantities.** For each room, in addition to plaster and paint: perimeter, flooring area, ceiling area, skirting length, and wall tile area are calculated deterministically in Go (formulas in constitution principle 5).
- **FR-007 Settings and rate library.** A Settings view stores, in the browser, default rates for plaster, paint, flooring, ceiling, tiling (per m²) and skirting (per m), currency, exchange rate, preferred unit, default wall height with unit, and waste percentage. New estimates start from these values; new rooms use the preferred unit; missing wall heights use the default wall height converted to the room unit (fallback 6 ft).
- **FR-008 Per-room finishes.** Before calculating, the user can include or exclude plaster, paint, flooring, ceiling, and skirting per room and set a wall tiling percentage (0–100). Excluded items have zero quantity and cost for that room. The selection is stored with the estimate version.
- **FR-009 Cost charts.** The estimate shows cost by trade and cost by room as bar charts, with values in the estimate currency and accessible text labels.

### Priority 3 — Analysis and model insight

- **FR-013 Analysis details.** Show the engine, model, start and completion time, duration, per-room confidence, and a count of missing length, width, and wall-height values. Offer re-running the same drawing with another available engine; a successful re-run replaces the current analysis and clears its confirmation and estimates in the workspace (earlier records stay on the server).
- **FR-014 Confirmation history.** List every confirmation revision with its time and room/opening counts, and describe changes from the previous revision (rooms added, removed, renamed, or re-dimensioned; opening count changes).
- **FR-015 Model improvements.** The 3D model draws doors and windows as openings in walls at indicative positions (evenly spaced along the room's walls, windows with a 0.9 m sill), labelled as indicative. Hovering a room shows its name, dimensions, and floor area. A top-view toggle and a screenshot (PNG download) are available.

## Non-Functional Requirements

- **NFR-001** No new runtime dependencies. Charts are SVG/CSS.
- **NFR-002** API input stays validated: archive body, finishes (tile 0–100), rates (finite, ≥ 0; plaster and paint > 0), waste (0–50 %). The drawing download returns only stored, previously validated file types with `nosniff`.
- **NFR-003** Existing APIs remain backward compatible: estimates without the new fields calculate as before.
- **NFR-004** Desktop and mobile layouts must not overflow horizontally.

## Out of Scope

Multiple drawings or floors per project, users and roles, server-side settings, branded reports, engine health monitoring, opening positions extracted from the drawing, and any quantity derived from model geometry.

## Acceptance Criteria

1. AC-01 (FR-001) A new user lands on the dashboard; created projects appear with stage and total; search filters them; archive hides a project until "Show archived" is on; duplicate creates "<name> (copy)" with the drawing.
2. AC-02 (FR-002) Opening a project shows the overview with details, drawing, analysis engine, counts, latest total, and a timeline entry for each confirmation and estimate.
3. AC-03 (FR-003) Clicking a room row highlights the drawing outline and the model room; clicking the drawing outline selects the row.
4. AC-04 (FR-004) An estimate built with a defaulted wall height lists that room under assumptions, together with rates and waste.
5. AC-05 (FR-005) Two versions with different rates show a non-zero difference in cost and total.
6. AC-06 (FR-006) Go tests prove each new formula, including door-width skirting deduction, tile/paint split, waste, and exclusions.
7. AC-07 (FR-007) Settings saved once pre-fill the estimate rates and the default wall height after reload.
8. AC-08 (FR-008) Excluding flooring in a room zeroes its flooring cost; setting tiling reduces its paint area by the tiled area.
9. AC-09 (FR-009) Charts render one bar per priced trade and per room.
10. AC-10 (FR-013) Review shows engine and model; re-running with another engine returns a new analysis for review.
11. AC-11 (FR-014) Confirming twice lists two revisions and describes the change.
12. AC-12 (FR-015) The model shows opening geometry, a hover tooltip, a working top view, and a screenshot download.
