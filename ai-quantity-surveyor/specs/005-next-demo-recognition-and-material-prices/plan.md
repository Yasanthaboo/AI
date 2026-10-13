# Implementation Plan: Next-Demo Recognition and Local Material Prices

**Status:** Proposed; scope-expanding implementation gated by constitution ratification.
**Specification:** `spec.md` | **Contracts:** `contracts.md` | **Tasks:** `tasks.md`

## Technical Context

Next.js/React/TypeScript frontend, Go HTTP/application/domain services, PostgreSQL JSONB confirmations/estimates and stored drawing bytes. Gemini/Ollama implement `Analyzer`; candidates currently contain rooms, doors/windows, room bounds, and engine/timing metadata. Openings have no evidence region/page. Estimates accept finish rates and manual exchange metadata, but do not store raw-material recipes, offers, or a direct confirmation-revision field. Do not assume those proposed fields/APIs already exist.

Repo app root is `ai-quantity-surveyor/` within the Git repository. Render Docker contexts target that folder. Web build uses `NEXT_PUBLIC_API_URL`; runtime uses `PORT` and `HOSTNAME=0.0.0.0`. API uses Neon `DATABASE_URL`, `ALLOWED_ORIGIN`, and provider environment variables. Hosted deployment was reported successful, not independently certified for private customer data or persistent background jobs.

## Constitution Check

- Preserve AI candidate -> human confirmation -> deterministic QS calculation. Model geometry cannot feed calculations.
- Proposed informational feature extraction and finish-material formulas require the documented 1.5.0 amendment. Supplier price lookup is a narrowly bounded exception to the supplier exclusion, not procurement approval.
- Current exclusions remain until ratification; RAG/agent frameworks, structural trades, external works and multi-floor remain deferred.
- Authentication/access protection is an explicit release decision, not permission to invent users/roles or expose all drawings publicly.

## Architecture and Delivery Order

1. Approve bounded scope, select representative annotated drawings, baseline current extraction, and agree error metrics/targets. Resolve legacy spec 003 as historical; do not restore separate layout confirmation.
2. Extend candidate/evidence contracts additively in `api/internal/analysis/contract.go`, both providers, confirmation handling and PostgreSQL payloads. Separate permissive candidate validation from strict calculation confirmation. Preserve missing values and attach review issues instead of rejecting useful incomplete extraction wholesale.
3. Add source evidence/list editing in `src/app/api.ts`, `step-review.tsx`, `drawing-view.tsx`, and revision history. Link selection in model views; render informational annotations only. Keep renderer failures local and retain existing approximate/indicative labels.
4. Add a small deterministic materials module, reusing domain unit/validation conventions and existing tests. First paint/coats/coverage and tiles/pack coverage only. Freeze confirmed quantity inputs and exact source confirmation for each new budget; extend estimate provenance additively only where needed. Legacy missing provenance is explicit and requires review before creating a new material budget.
5. Introduce product-spec inputs and versioned material-budget persistence/APIs. Keep budget totals separate from finish-rate estimates. Prove formula, rounding, exclusion, double-counting and restart behavior using curated inputs before connecting suppliers.
6. Select one region and approved sources. Implement supplier adapters with structured JSON/HTML parsers, no ad hoc HTML regex. Use an authorized API/feed first; add a mature HTML parsing library only if permitted scraping is necessary. Domain calculation code stays independent of parsers/network calls.
7. Ingest and cache normalized offers in PostgreSQL. Refresh via a bounded manual job initially; evaluate an external scheduled trigger only if free hosting supports the required reliability. Do not rely on an always-running in-container timer or fire-and-forget job surviving Render sleep/redeploy. Persist refresh state and bound retries/concurrency.
8. Build location/product filters, equivalent-offer comparisons, selection snapshots, separate export, and known/unknown tax/delivery presentation. Optional browser location is consent-based with manual fallback.
9. Release on existing Render/Neon deployment after focused Go/provider/API tests, TypeScript/lint, existing E2E regressions and new customer workflows. Record desktop/mobile screenshots, provider availability, cold-start errors, persistence/restart, and access-protection checks.

## Persistence and Security

Use additive optional candidate fields and dedicated versioned product/offer/budget records; avoid overwriting historical JSONB payloads. Persist original candidate plus user corrections and server-owned source provenance. Store price observations separately from current refresh status, and copy selected offer values into snapshots.

Keep credentials in hosted environment settings only; rotate any previously shared keys. Allowlist supplier domains and redirects, reject private/link-local targets, bound response size/time, and prevent SSRF. Treat supplier content and drawing text as untrusted data, not instructions. Require explicit source-policy approval; stop a source that becomes disallowed.

Location data is minimized. Protect web AND API (including drawing/export routes and provider calls); decide a managed gateway/app-level mechanism before private-data release. Until then use isolated sample data, provider-call limits and an explicit demo restriction. Free hosting/data quotas and third-party AI usage are not unlimited or guaranteed zero-cost.

## Verification Strategy

- Annotated fixtures: missed/extra windows, repeated symbols, uncertain dimensions, stairs/balconies, rotated page mapping, missing bounds, provider omissions, and room/dimension regression. Report precision/recall separately from self-reported AI confidence.
- Domain fixtures: AC-04, finite/positive coverage validation, coat count, pack boundary rounding, excludes, consistent units, single waste application, and unknown charges/currencies.
- API/persistence: cross-project/revision/page rejection, legacy records, immutable snapshots, source approval enforcement, failed refresh, restart recovery and export parity.
- Browser: manual feature correction, incomplete data, no geolocation permission, stale/quote-only offers, incomparable products, text fallback and mobile overflow. Existing confirmed-input QS/Excel golden results remain identical.
- Supplier adapters use recorded permitted fixtures in automated tests, not flaky live sites; approved live smoke checks are a separate release gate.

## Open Gates and Demo Fallback

Do not start live supplier integration until geography, source permission, freshness rules and equivalence criteria are approved. No approved sources means labeled fixture/imported catalog data, not invented current prices. No private access mechanism means a sample-only demo. Extraction-quality targets must be agreed before claiming improved recognition. No new runtime framework is chosen merely to advertise RAG or multiple agents.