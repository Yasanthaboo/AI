# Implementation Plan: Project Workspace, Finishes Estimating, and Model Insight

**Spec:** `spec.md` · **Constitution:** 1.4.0 · **Status:** Implemented

## Constitution Check

- AI still produces candidates only; all new quantities are deterministic Go formulas (principle 5, amended).
- Defaults (wall height) are marked in review and recorded on the confirmed room as `assumptions`, then listed on estimates (principle 4, amended).
- Openings are drawn at indicative positions and labeled; model geometry never feeds calculations (principle 13, amended).
- New trades are interior finishes only; full BOQ, multi-floor, and roles stay excluded (principle 12).

## Backend Changes (Go)

| Area | Change | Requirement |
|---|---|---|
| `domain` | `Rates` gains flooring, ceiling, skirting (per m), tiling, `wastePercent`; `RoomFinish` (include flags + `tilePercent`); `RoomCalculation`/`CalculationResult` gain perimeter, flooring, ceiling, skirting, tile quantities and costs; `CalculateWithFinishes` | FR-006, FR-008 |
| `estimate` | `Request` accepts new rates, waste, `finishes`; `Version` stores `rates`, `finishes`; Excel adds finishes columns, summary rows, and an Assumptions sheet | FR-004, FR-006, FR-008 |
| `analysis` | `Candidate` gains `engine`, `model`, `startedAt`, `completedAt`; `Room` gains `assumptions`; `Confirmation` gains `confirmedAt`; `LatestForFloorPlan`, `Confirmations` | FR-002, FR-013, FR-014 |
| `project` | `Project.archived`; `ListProjects`, `SetArchived`, `FloorPlanForProject`; `Duplicate` | FR-001 |
| `portfolio` (new) | Builds project summaries (stage, floor plan, latest analysis, revision, estimate count, latest total, updatedAt) | FR-001, FR-002 |

Persistence is additive: `ALTER TABLE projects ADD COLUMN IF NOT EXISTS archived`, `ALTER TABLE analysis_confirmations ADD COLUMN IF NOT EXISTS confirmed_at`. New candidate and version fields live in existing JSONB payloads.

### API Contracts

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/projects` | Project summaries (dashboard) |
| GET | `/api/projects/{id}/summary` | One project summary (overview, restore) |
| PATCH | `/api/projects/{id}` | `{"archived": bool}` |
| POST | `/api/projects/{id}/duplicate` | Copy details and drawing; 201 with new project |
| GET | `/api/floor-plans/{id}/file` | Stored drawing, original content type, `nosniff` |
| GET | `/api/analyses/{id}/confirmations` | All confirmation revisions, oldest first |
| POST | `/api/projects/{id}/estimates` | Adds optional `flooringUsdPerSquareMeter`, `ceilingUsdPerSquareMeter`, `skirtingUsdPerMeter`, `tilingUsdPerSquareMeter`, `wastePercent`, `finishes` |

Stage derivation: Estimated (≥1 estimate) > Confirmed (≥1 revision) > Analyzed (ready candidate) > Uploaded (floor plan) > Draft.

## Frontend Changes (Next.js)

| File | Change | Requirement |
|---|---|---|
| `page.tsx` | Views: dashboard, project (overview + 5 steps), settings; restore from summary; fetch drawing from API when not cached | FR-001, FR-002 |
| `dashboard.tsx` | List, search, archived toggle, open/duplicate/archive | FR-001 |
| `overview.tsx` | Details, thumbnail, counts, totals, timeline, analysis details, confirmation history | FR-002, FR-013, FR-014 |
| `analysis-details.tsx` | Engine/model/timing/confidence/missing values, re-run | FR-013 |
| `confirmation-history.tsx` | Revisions and diffs | FR-014 |
| `settings.ts`, `settings-view.tsx` | Browser-stored defaults | FR-007 |
| `step-review.tsx` | Default wall height from settings, `assumptions` on confirmed rooms, analysis details panel | FR-007, FR-013 |
| `step-model.tsx`, `drawing-view.tsx`, `room-scene.tsx`, `preview.ts` | Linked selection, openings, hover, top view, screenshot | FR-003, FR-015 |
| `step-estimate.tsx`, `estimate-parts.tsx` | All rates, waste, per-room finishes, assumptions, compare, charts, CSV | FR-004–FR-009 |

## Test Strategy

- Go: domain formula tests (AC-06, AC-08), estimate request/version storage, analysis metadata and confirmation list, project list/archive/duplicate, portfolio stage derivation, HTTP handlers.
- Playwright: dashboard (AC-01), overview (AC-02), linked selection (AC-03), assumptions (AC-04), compare (AC-05), settings (AC-07), finishes (AC-08), charts (AC-09), analysis details and re-run (AC-10), history (AC-11), model tools (AC-12), mobile overflow (NFR-004).

## Decisions

- Settings are browser-local for the POC (no users). Estimates store the rates they used, so versions stay reproducible.
- Waste is applied to cost, not reported quantities, so measured quantities remain auditable.
- Duplicate copies the drawing so the copy can be re-analyzed with another engine; analyses and estimates are not copied.
- Opening placement is indicative only; openings are distributed on the room's walls, longest first.
