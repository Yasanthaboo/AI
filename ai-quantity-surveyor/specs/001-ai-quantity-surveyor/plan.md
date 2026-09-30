# Implementation Plan

## Architecture

`Next.js web UI -> Go HTTP API -> application services -> domain calculation engine -> infrastructure (PostgreSQL, file storage, AI provider)`

The AI provider is isolated behind an analyzer interface. The calculation engine accepts only confirmed, validated input and has no dependency on the AI or HTTP layers.

## Workstreams

### Foundation and Project Management

Own Go module setup, HTTP conventions, PostgreSQL migrations/repositories, project and floor-plan records, file validation/storage, and project/drawing APIs. File validation accepts PDF/PNG/JPG/JPEG up to 10 MB and returns retryable errors for unreadable drawings.

### AI Floor-Plan Analysis

Own analyzer interface, provider adapter, structured response schema, parsing/validation, confidence handling, analysis persistence, and analysis APIs. It must not calculate QS quantities.

### Human Verification and Frontend

Own Next.js screens for project creation, upload, preview, analysis state, editable room/opening tables, uncertainty messages, confirmation, and estimate display. Use mock API contracts while backend work is parallel.

### QS Calculation and Estimate

Own unit conversion, validation, deterministic formulas, configurable USD rates, target-currency conversion, immutable estimate version persistence/API, and Excel export. Use fixture confirmed data for independent development. Capture the exchange rate and timestamp with each estimate; fail clearly when a current rate is unavailable.

## Shared Contracts

Agree before parallel implementation: Project, FloorPlan, Room, Door, Window, FloorPlanAnalysis, ConfirmedFloorPlan, CalculationInput, CalculationResult, Rate, ExchangeRate, EstimateVersion, and common API error response. JSON field names, original dimensions, converted meters, currencies, exchange-rate metadata, and units are stable integration boundaries.

## API Surface

- `POST /api/projects`, `GET /api/projects/{id}`
- `POST /api/projects/{id}/floor-plan`, `GET /api/floor-plans/{id}`
- `POST /api/floor-plans/{id}/analysis`, `GET /api/analyses/{id}`
- `PUT /api/analyses/{id}/confirmed-data`
- `POST /api/projects/{id}/estimates`, `GET /api/estimates/{id}`
- `GET /api/projects/{id}/estimates` (list immutable versions)
- `GET /api/estimates/{id}/export` (Summary, Rooms, Openings worksheets)

## Dependency Graph

Contracts -> Foundation plus AI plus Calculation plus Frontend mocks -> API integration -> end-to-end workflow.

## Delivery Sequence

1. Establish contracts, repository, Go module, and Next.js scaffold.
2. Implement independent deterministic calculation and validation tests.
3. Implement upload/foundation and AI parsing against contracts.
4. Implement verification UI against mocks, then connect APIs.
5. Integrate confirmed data through estimate versioning, USD-to-target currency conversion, and Excel export.
6. Run unit, integration, API, frontend, currency, versioning, export, and end-to-end tests.
