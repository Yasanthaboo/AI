# Initial Tasks

## Foundation

- [x] Define shared JSON contracts and common error format.
- [x] Add Go project, configuration, health endpoint, and test command.
- [x] Add project, floor-plan, and file validation APIs for PDF/PNG/JPG/JPEG files up to 10 MB.
- [x] Add retryable errors for unreadable drawings and failed uploads.
- [x] Add durable project and uploaded-file persistence for local development.

## AI

- [x] Define analyzer interface and structured extraction model.
- [x] Implement response validation and uncertainty handling.
- [x] Add analysis execution and status APIs using a deterministic development analyzer.
- [x] Add configured Ollama-compatible vision provider adapter.
- [x] Persist analysis candidates and confirmations durably in PostgreSQL.

## Frontend

- [x] Replace starter page with project workflow shell.
- [x] Connect upload and analysis status to the Go API.
- [x] Add visible multi-room and editable door/window controls to the review UI; confirmation API integration is connected.
- [x] Add estimate summary and export action.

## QS and Estimate

- [x] Implement deterministic unit conversion and calculations in Go.
- [x] Add USD rate validation and target-currency conversion with captured exchange rate metadata.
- [x] Add immutable estimate version APIs and preserve prior versions in the estimate service.
- [x] Persist estimate versions durably in PostgreSQL.
- [x] Generate calculated-values-only Excel exports with Summary, Rooms, and Openings worksheets.
- [x] Add formula, validation, currency, versioning, export, API, and end-to-end tests.
