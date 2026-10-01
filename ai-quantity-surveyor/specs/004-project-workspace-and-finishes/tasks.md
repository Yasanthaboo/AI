# Tasks: Project Workspace, Finishes Estimating, and Model Insight

**Status:** Complete. Each task lists the requirement it implements (spec.md), where it lives, and the tests that verify it.

## Governance

- [x] T001 Amend constitution to 1.4.0: finishes quantities and formulas (principle 5), visible defaults recorded as assumptions (principle 4), indicative openings (principle 13), scope list (principle 12). — `.specify/memory/constitution.md`
- [x] T002 Write spec, plan, and API contracts. — `specs/004-project-workspace-and-finishes/spec.md`, `plan.md`

## Backend (Go)

- [x] T010 Finishes formulas, per-room exclusions, tiling split, waste on cost, input validation. **FR-006, FR-008, NFR-002** — `api/internal/domain/quantity.go` · tests `TestCalculateFinishesQuantitiesWasteAndExclusions`, `TestCalculateRejectsInvalidFinishInputs`, `TestCalculateUsesDeterministicWallFormulas` (regression) · **AC-06, AC-08**
- [x] T011 Estimate request accepts finish rates, waste, finishes; versions store `rates` and non-default `finishes`. **FR-004, FR-007, FR-008, NFR-003** — `api/internal/estimate/service.go` · test `TestCreateRequiresConfirmationAndPreservesVersions`
- [x] T012 Excel export adds finishes columns, summary rows, and an Assumptions sheet. **FR-004, FR-006** — `api/internal/estimate/export.go`
- [x] T013 Analysis records `engine`, `model`, `startedAt`, `completedAt`; analyzers report `ModelName`; engines cannot set `assumptions`; confirmation keeps metadata and only known assumptions. **FR-013, FR-004** — `api/internal/analysis/{contract,service,gemini,ollama}.go` · test `TestAnalysisRecordsEngineModelTimingAndHistory` · **AC-10**
- [x] T014 Confirmation timestamps and history listing; latest ready analysis per floor plan (memory + PostgreSQL, additive `confirmed_at`). **FR-014, FR-002** — `api/internal/analysis/{service,postgres}.go` · tests `TestAnalysisRecordsEngineModelTimingAndHistory`, `TestConfirmationRevisionsSurviveServiceRestart` · **AC-11**
- [x] T015 Project `archived`, `ListProjects`, `SetArchived`, `FloorPlanForProject`, `Duplicate` (memory + PostgreSQL, additive `archived`). **FR-001** — `api/internal/project/{service,postgres}.go` · test `TestListArchiveAndDuplicate` · **AC-01**
- [x] T016 Project summaries with stage derivation, counts, latest total, updatedAt. **FR-001, FR-002** — `api/internal/portfolio/portfolio.go` · test `TestSummarizeDerivesStageAndLatestTotal`
- [x] T017 HTTP: `GET /api/projects`, `GET /api/projects/{id}/summary`, `PATCH /api/projects/{id}`, `POST /api/projects/{id}/duplicate`, `GET /api/floor-plans/{id}/file` (extension-derived type, `nosniff`), `GET /api/analyses/{id}/confirmations`; CORS allows PATCH. **FR-001, FR-002, FR-014, NFR-002** — `api/cmd/server/main.go` · tests `TestPortfolioHandlers`, `TestConfirmationReadHandlers`

## Frontend (Next.js)

- [x] T020 Shared types, trade table, `runAnalysis`, `fetchDrawing`. — `src/app/api.ts`
- [x] T021 Views (dashboard, project, settings), Overview tab, restore from summary, drawing fallback from API. **FR-001, FR-002** — `src/app/page.tsx`
- [x] T022 Dashboard: search, archived toggle, open, duplicate, archive. **FR-001** — `src/app/dashboard.tsx` · e2e `dashboard lists, searches, duplicates, archives and opens projects` · **AC-01, AC-02**
- [x] T023 Overview: details, drawing, metrics, analysis details, activity timeline, confirmation history. **FR-002, FR-014** — `src/app/overview.tsx`, `src/app/confirmation-history.tsx` · e2e `dashboard …`, `confirmation history records each revision and its changes` · **AC-02, AC-11**
- [x] T024 Analysis details and re-run with another engine. **FR-013** — `src/app/analysis-details.tsx`, `src/app/step-review.tsx` · e2e `analysis details show the engine and a re-run replaces the analysis` · **AC-10**
- [x] T025 Settings and rate library (validated browser storage); default wall height and preferred unit in review; `assumptions` sent on confirmation. **FR-007, FR-004** — `src/app/settings.ts`, `src/app/settings-view.tsx`, `src/app/step-review.tsx` · e2e `settings pre-fill estimate rates and the default wall height`, `missing wall height defaults to 6 ft or its unit equivalent` · **AC-07**
- [x] T026 Estimate: all rates, waste, per-room finishes, trade table, CSV columns. **FR-006, FR-008** — `src/app/step-estimate.tsx`, `src/app/estimate-parts.tsx` · e2e `finishes, assumptions, charts and version comparison`, `guided workflow …` · **AC-08**
- [x] T027 Assumptions panel (also printed). **FR-004** — `src/app/estimate-parts.tsx` · e2e `finishes, assumptions, …` · **AC-04**
- [x] T028 Version comparison. **FR-005** — `src/app/estimate-parts.tsx` · e2e `finishes, assumptions, …` · **AC-05**
- [x] T029 Cost charts by trade and room. **FR-009, NFR-001** — `src/app/estimate-parts.tsx` · e2e `finishes, assumptions, …` · **AC-09**
- [x] T030 Linked room selection across table, drawing, and model. **FR-003** — `src/app/step-model.tsx`, `src/app/drawing-view.tsx`, `src/app/room-scene.tsx` · e2e `selecting a room highlights it in the table, drawing and model` · **AC-03**
- [x] T031 Indicative openings, hover tooltip, top view, screenshot. **FR-015** — `src/app/preview.ts` (`buildWalls`), `src/app/room-scene.tsx` · e2e `indicative openings are spaced on room walls without exceeding them`, `model offers top view, hover details and a screenshot` · **AC-12**
- [x] T032 Styles, responsive and print rules. **NFR-004** — `src/app/globals.css` · e2e `workflow fits a mobile screen`

## Verification

- [x] T040 `go test ./api/...`, `go vet`, `gofmt` clean.
- [x] T041 TypeScript `--noEmit` and ESLint clean.
- [x] T042 Full Playwright suite against rebuilt containers (19 tests).
- [x] T043 Desktop screenshots of dashboard, overview, review, model, estimate, and settings reviewed.

## Known Limits

- Settings are per browser; there are no users or server-side preferences.
- Opening positions are indicative, not extracted from the drawing.
- One drawing and one floor per project; duplicate copies the drawing only.
