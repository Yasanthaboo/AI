# Tasks: Next-Demo Recognition and Local Material Prices

**Status:** Proposed. All implementation/release tasks are unchecked. Spec 004 completion does not imply completion here.
**Dependencies:** Governance -> P1 recognition -> P2 material budgeting -> gated P3 live prices -> release. P1 can be demoed independently; P2/P3 may use labeled curated fixtures.

## Phase 1 - Approval and Baseline

- [ ] T001 Ratify bounded constitution 1.5.0 and synchronize developer instructions; preserve excluded trades/frameworks and approve only lookup/comparison, not procurement. Owner approval required.
- [ ] T002 Finalize `contracts.md` schemas, reference/revision rules, source confidence and candidate/confirmation validation boundaries. **FR-001-007, FR-009-013**
- [ ] T003 Select consented/redistributable annotated plan fixtures; record current engine/model/prompt baseline and agree holdout precision/recall/dimension targets before tuning. **FR-001-005; AC-01-03, AC-10**
- [ ] T004 Choose pilot country/city/currency, paint/tile specs, freshness interval, and two or three candidate suppliers; document permission/terms/robots and approved collection method per source. Live integration blocked until approved. **FR-008-012; AC-06-08**
- [ ] T005 Choose web/API access boundary, sample-only fallback, provider-call limits, free-hosting quotas, secret rotation and deployment branch/check policy. **FR-014; AC-09**

## Phase 2 - Recognition and Review (P1)

- [ ] T010 Extend analysis candidate/evidence/feature types additively and provider prompts/schemas in `api/internal/analysis`; preserve missing values and original candidates. **FR-001-002; AC-01-02**
- [ ] T011 Validate IDs/types, finite confidence, page/region coordinates, room associations and source provenance; keep incomplete candidates reviewable and strict window confirmation. **FR-002-004; AC-02**
- [ ] T012 Persist immutable reviewed feature/evidence revisions, manual corrections/dispositions and legacy payload compatibility; extend API types in `src/app/api.ts`. **FR-002-004; AC-03, AC-05**
- [ ] T013 Add window/stair/balcony list and source-overlay selection/edit/add/remove to existing review/drawing views; uncertainty and manual status stay visible. **FR-003; AC-01-03**
- [ ] T014 Add evidence-linked informational model annotations/text fallback; no invented stair geometry, verified wall anchors or independent layout confirmation. **FR-005; AC-01, AC-03, AC-05**
- [ ] T015 Run holdout extraction evaluation, provider parsing/API tests, legacy workflow regressions, desktop/mobile review screenshots and golden QS/export comparisons; record results versus baseline. **FR-001-005; AC-01-03, AC-05, AC-10**

## Phase 3 - Finish Material Budget (P2)

- [ ] T020 Add deterministic paint/tiles recipe validation, exclusions, quantity conversion, single waste application, compatible aggregation, whole-pack rounding and surplus tests. **FR-006-007; AC-04**
- [ ] T021 Add exact source-confirmation/estimate provenance and frozen quantity inputs for material budgets; reject unresolved legacy provenance instead of selecting latest revision silently. **FR-007; AC-05**
- [ ] T022 Add additive budget persistence/read/create APIs and immutability/restart tests, without changing existing finish totals/exports. **FR-007; AC-05**
- [ ] T023 Extend estimate workspace with reviewed product specs, separate material-only budget, assumptions and missing-yield errors using curated sourced/fixture inputs. **FR-006-007; AC-04-05**

## Phase 4 - Approved Price Pilot (P3; Requires T004/T005)

- [ ] T030 Implement allowlisted authorized source adapters and recorded fixtures; structured parsers, redirects/SSRF controls, response bounds, request limits and no bypasses. **FR-009, FR-011; AC-07**
- [ ] T031 Persist observations/cache and refresh status; operator-only bounded refresh, failure handling, provenance/freshness and restart behavior. **FR-009, FR-011-012; AC-07**
- [ ] T032 Implement manual location and optional consented geolocation fallback, branch/product filtering and delivery coverage states. **FR-008-009; AC-06**
- [ ] T033 Normalize equivalent pack/spec offers, validate optional currency conversion, compute whole-pack totals and basket-level known charges, exclude incomparable offers from ranking. **FR-010; AC-06, AC-08**
- [ ] T034 Add UI comparison options with source, time, availability and tax/delivery unknowns; reviewed budget/standard/premium grouping only. **FR-009-012; AC-06-08**
- [ ] T035 Persist immutable selected comparisons and separate export; prove later catalog refresh cannot change saved values. **FR-013; AC-05, AC-08**

## Phase 5 - Demo Verification and Release

- [ ] T040 Run focused Go domain/provider/API/persistence tests; frontend typecheck/lint and existing/new Playwright workflows. **AC-01-10**
- [ ] T041 Verify deployed web/API URLs, CORS, provider configuration, PostgreSQL restart persistence, source-policy enforcement, bounded provider/collector calls, access protection or explicit sample-only restrictions, desktop/mobile usability and cold-start/error recovery. **FR-014; AC-09**
- [ ] T042 Perform approved live supplier smoke checks if permitted; otherwise clearly label fixture/imported prices and remove live/current-price claims. **FR-009-012; AC-06-08**
- [ ] T043 Record demo acceptance/evaluation results, known limits and unresolved gates; update README/spec status only after verification. **AC-01-10**

## Traceability

| Acceptance | Tasks |
| --- | --- |
| AC-01 | T003, T010, T013-T015 |
| AC-02 | T003, T010-T011, T015 |
| AC-03 | T003, T012-T015 |
| AC-04 | T020, T023 |
| AC-05 | T012, T014-T015, T021-T023, T035 |
| AC-06 | T004, T032-T034, T042 |
| AC-07 | T004, T030-T031, T034, T042 |
| AC-08 | T004, T033-T035, T042 |
| AC-09 | T005, T041 |
| AC-10 | T003, T015 |

Do not mark approval, source permissions, credentials, live collection or implementation complete merely because this documentation exists. No commits or pushes are performed by this package.