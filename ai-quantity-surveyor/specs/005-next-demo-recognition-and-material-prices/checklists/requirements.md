# Requirements Checklist: Next Demo

**Created:** 2026-10-13 | **Status:** Initial clarification pass complete; selected-page scope, recognition thresholds, dimension preview and Colombo/LKR pilot recorded. Suppliers explicitly deferred with labeled fixture/import fallback. Source approvals and release gates remain pending; this is document quality, not feature acceptance.

## Specification Quality

- [x] Prioritized user stories each have an independent test.
- [x] Recognition issues, local material prices and hosted-demo needs from customer discussions are represented.
- [x] Current behavior is distinguished from proposed contracts/endpoints and historical layout design.
- [x] Human confirmation and deterministic QS boundaries are preserved.
- [x] Informational stairs/balconies are excluded from pricing and multi-floor reconstruction.
- [x] Paint/tile material budgeting is separate from existing finish rates, with explicit waste, rounding, surplus and provenance.
- [x] Offers specify units/specification, location, currency, time, availability and unknown tax/delivery states.
- [x] Permitted sources, caching, access controls, consent, failure behavior and fixture fallback are defined.
- [x] Acceptance criteria map to tasks; unfinished implementation is not marked delivered.
- [x] RAG/multi-agent orchestration, procurement and structural trades are explicitly deferred.
- [x] User stories include Given/When/Then acceptance scenarios.
- [x] Edge cases and key entities describe observable behavior without requiring specific implementation technology.
- [x] Technical deployment/collector choices are left to the plan; the spec states user/security outcomes.
- [x] Success criteria cover material correctness, offer provenance, immutable results, unchanged QS outputs and desktop/mobile usability.
- [x] Q1 resolved: exactly one user-selected PDF page per analysis; changing page requires a new analysis.
- [x] Q2 thresholds agreed: 10 holdout pages/20 instances per feature; windows 90% precision/recall, stairs/balconies 85%, legible dimensions 95% with correct units/within 2%. Fixture acquisition and measured results are still pending.
- [x] Owner-requested pre-confirmation dimension preview is specified and mapped to FR-015, AC-11 and T013/T015; no implementation completion claimed.
- [x] Q3 recorded: Colombo, Sri Lanka; LKR comparisons. Supplier shortlist deliberately deferred; labeled fixtures/imports are the fallback and live-source permission is not claimed.

## Blocking Decisions and Release Gates

- [x] Ratify constitution amendment and align developer instructions before expanded implementation. Owner approved bounded P1-P3 scope on 2026-10-13; constitution 1.5.0 and development rules are synchronized.
- [x] Approve pilot geography/currency: Colombo, Sri Lanka; LKR local-price comparisons, without changing existing USD-base QS estimates.
- [ ] Identify/approve two or three sources and per-source automated access policy before live collection; use labeled fixtures/imports until then.
- [ ] Approve product/spec equivalence, consumption inputs, price freshness and refresh limits.
- [ ] Acquire/annotate holdout fixtures and finalize scoring rules before tuning; validate the agreed AC-10 recognition thresholds.
- [ ] Decide hosted web/API protection; keep sample-only restrictions until private-data release is safe.
- [ ] Verify formulas, provenance, saved snapshots, exports, legacy behavior and restart persistence.
- [ ] Record customer-demo acceptance results and disclose fixture/stale/unknown prices honestly.

## Planning Artifact Review

- [x] Code-grounded decisions cover selected-page rendering, explicit dimension review, exact revision provenance and separate material/comparison snapshots.
- [x] Research, additive data model, technical contracts and verification guide are recorded; planned dependencies and tests are not claimed installed/run.
- [x] Colombo/LKR uses labeled fixtures/imports while suppliers are deferred; no scraper or live-price claim is authorized by planning.
- [x] Source-image consistency, material precision/pack rounding, holdout scoring, concurrency/orphan recovery and legacy provenance behavior have explicit designs.
- [x] Refine tasks into stable-ID, story-mapped slices with file targets, dependencies and focused checks; validate unique IDs, acyclic dependencies and fixture-release independence from live/private approvals.
- [x] Record passed cross-artifact analysis and finalized bounded request/catalog limits; T002 is complete. External source/access, holdout and feature delivery gates remain pending.
- [ ] Verify Poppler packaging/license/resource limits, migrations, fixture scoring implementation and hosted workload/access behavior during delivery.

## Analysis Remediation (2026-10-13)

- [x] A1 design recorded: server-backed non-saving comparison preview and explicit Save recomputation; T049/T036/T034 cover implementation/testing. No preview API is implemented yet.
- [x] A3 design recorded: validation before/after normalization, named zero-divisor/precision errors and below/at/above boundary tests; T020/T030 cover delivery.
- [x] A4 design recorded: unique owner/attempt, expiring lease/heartbeat, expired-only recovery and conditional source/terminal writes; T044 covers delivery.
- [x] A6 design recorded: persistent daily quota/reservations, atomic pre-call accounting, UTC rollover and fail-closed config/storage behavior; T055 covers delivery. Call quotas are not spending guarantees.
- [x] A2 design recorded: one allocation per room/trade, server rejection before aggregation and frozen source amounts; T022/T025 cover implementation checks. Split-area/multiple-product allocations remain deferred, no extra confirmation screen.
- [x] A5 design recorded: raw detection/dimensions/localization measured separately, unsupported baseline localization disclosed and human correction tests separated; T003/T046/T015 cover delivery without changing approved thresholds.
- [x] Async response clarification recorded: immediate errors before acceptance; after 202, persisted structured AnalysisFailed read via HTTP 200 polling, preserving legacy errorMessage and retry/reset metadata. Failed quota reservation never dispatches; DB outage does not imply successful failure persistence. T044/T055/T009/T016 cover delivery.
- [x] Full design consistency review passed after A1-A6/async response remediation; T002 readiness is recorded. This does not certify runtime feature completion.

## Implementation Progress

- [x] Additive Go/TypeScript analysis contracts and compatibility tests pass (T006).
- [x] Poppler packaging/non-root runtime and canonical selected-page helper checks pass, including a real rotation/crop test under 512 MB (T045/T007).
- [x] Pure paint/tile formulas, exact packs/surplus, duplicate-allocation and precision boundary tests pass (T020).
- [ ] Complete source/job persistence, quota/lease orchestration, public routes and dimension-review UI.
- [ ] Complete evidence/confirmation validation, provenance ownership integration and material budget/comparison persistence/UI/export workflows.
- [ ] Acquire consented annotated recognition holdouts and record actual baseline/accuracy; generated rendering fixtures are not recognition evidence.