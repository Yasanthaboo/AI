# Research Decisions: Feature 005

**Date:** 2026-10-13 | **Status:** Code-grounded planning decisions; no implementation, benchmark or supplier permission claimed.

## R1 - One Canonical Selected-Page Image

**Evidence:** `src/app/drawing-view.tsx` calls `document.getPage(1)`. `api/internal/analysis/drawing.go` uses PDF regex matches for an aspect estimate, not a complete PDF page parser. Existing analysis registration accepts engine selection, and Ollama rejects raw PDFs.

**Decision:** Add server page metadata and selected-page rendering using Poppler tools; both providers and the dimension-review preview consume the same stored PNG, with original page/digest metadata. New UI explicitly selects a page; legacy API omission defaults to page 1 and records that origin. Selection changes start new analyses.

**Alternatives rejected:** Prompting a provider to ignore other PDF pages does not isolate input. Browser-only rasterization makes source metadata caller-dependent and duplicates trusted preparation. Full multi-page extraction exceeds scope. Maintaining a handwritten PDF parser is not justified.

**Risks/checks:** Verify Alpine tool packaging, license obligations, rotation/crop output, time/pixel limits and 512 MB resource behavior. Page count extraction is isolated behind the metadata adapter; never reuse regex aspect heuristics as selected-page authority. OCR/vision accuracy is measured separately, not assumed from rendering resolution.

## R2 - Explicit Numeric Confirmation, Not Verified Geometry

**Evidence:** `analysis.Service.Confirm` preserves immutable snapshots; existing `Room.Bounds` and indicative openings are visualization inputs. The current drawing component supports room selection but lacks a pre-confirmation field-evidence dimension panel.

**Decision:** Extend the existing review UI with original/pending/confirmed state, field-level evidence, manual corrections, zoom/pan and readable list fallback. Source marks show evidence locations, not pixel-derived dimensions or verified wall anchors. Informational features are absent from calculation conversion.

**Alternative rejected:** Separate layout/dimension-confirmation workflow would revive historical spec 003 rather than the clarified POC. Automatic confirmation on preview/open/edit violates the constitution.

## R3 - Exact Revision Provenance

**Evidence:** `estimate.Service.Create` currently reads `analysis.Confirmed` and stores `SourceAnalysisID`, but `Version` lacks an explicit confirmation revision. Repository JSONB permits additive fields.

**Decision:** Compute new estimates from one fetched immutable confirmation snapshot and persist its revision/source identity. Budget from that frozen estimate result; reject a legacy estimate without exact provenance. Validate project ownership rather than trusting the fallback ID lookup.

**Alternative rejected:** Attaching the latest confirmation to an old estimate fabricates lineage; reading latest revision after calculation races concurrent confirmations. Existing QS/export compatibility must remain unchanged.

## R4 - Exact Material Arithmetic and One Application Boundary

**Evidence:** Existing QS uses float64 and supported metre conversion factors; Excelize and PostgreSQL are already dependencies. Material pack rounding and consumption rules do not yet exist.

**Decision:** Leave existing QS formulas untouched. Material-only inputs are normalized to six decimal places under `finish-materials-v1`, then computed using standard-library rational arithmetic, with exact ceiling and recorded waste/precision. Pure domain formulas plus one materials service/repository own budgets/catalog observations/comparisons; reuse Excelize for separate XLSX export.

**Alternative rejected:** Repricing measured finish totals with product prices double-counts costs; a new decimal dependency or separate pricing microservice is unnecessary. Epsilon-based ceiling without a versioned precision rule can hide real additional pack requirements.

**Risk/check:** Quantization below one-millionth of the material unit is explicit and must have boundary tests. Exact rational arithmetic cannot repair inaccurate source dimensions or product specifications; human-reviewed assumptions remain mandatory.

**A3 remediation:** Validate again after normalization. Zero-normalized coverage/pack amounts yield named errors; positive source areas rounded to zero are unsupported precision, not zero packs. Test immediately below/at/above the half-unit boundary without clamping inputs.

**A2 remediation:** Allow one allocation per `(roomId, trade)` within a budget, with trade `paint|wallTile|floorTile`. Reject duplicates server-side before quantity aggregation, naming the room/trade. Source amounts come from the frozen confirmed estimate, never client-submitted areas. Aggregate compatible products across different rooms only after uniqueness validation; a client compatibility-group label alone cannot establish identical recipes/products. Split-area or multiple-product allocations within one room/trade are deferred. No extra human confirmation screen is needed.

## R5 - Fixture-First Local Prices

**Evidence:** Owner selected Colombo/Sri Lanka/LKR and explicitly deferred supplier shortlist. Constitution permits bounded lookup but requires permission/provenance and unknown-charge handling.

**Decision:** Versioned fictional fixture catalog plus authorized structured imports, 24-hour observation freshness default, explicit status, reviewed equivalence keys and nullable delivered totals. Supplier refresh is operator-controlled; no live scraping implementation or new HTML dependency until approved sources exist.

**Alternative rejected:** Web-wide scraping, silent AI price estimates, generic budget/premium quality labels and distance-inferred delivery would produce unsupported comparisons. Real URLs must not be assigned to synthetic quotations.

**Deferred:** Supplier permission, real-source freshness/coverage, imported product equivalence and any recurring collection mechanism. These are operational gates, not an excuse to label fixture prices live.

**A1 remediation:** Define a transient budget-scoped server comparison preview before explicit Save. Preview and Save share pricing logic; Save recalculates against chosen immutable observation IDs/current eligibility and rejects expired selections. No preview insert/version allocation, collection or stock reservation.

## R6 - Holdout Scoring

**Decision / A5 remediation:** Freeze >=10 pages and >=20 instances per feature. Score raw AI output before any human edits in three independent categories: detection (correct feature identity/type), dimensions (correct entity/association/unit and <=2% relative error), and localization (same-type one-to-one region IoU >=0.5). Freeze identity/association matching and ambiguity rules before tuning; duplicates, unmatched and ambiguous predictions cannot be silently omitted. Missing region is a localization failure for new evidence-capable output, not automatically a raw-detection miss. Legacy unsupported localization is reported as `notSupported`, never as zero detection accuracy or invented coordinates. Unsupported legacy stair/balcony extraction is disclosed separately; do not claim comparative improvement for an unsupported metric.

Preserve approved thresholds: new detection and localization precision/recall use windows >=90% and stairs/balconies >=85%; legible dimension accuracy remains >=95% with correct units and <=2% error. Localization uses the same frozen set and counts missing regions rather than selecting an easy subset. Compare fewer window detection misses and room/dimension accuracy only on metrics supported by both raw baseline and new output. Record input representation/page and provider/model so preprocessing changes are visible. Human correction/confirmation tests are separate workflow evidence and cannot replace raw extraction scores. No threshold attainment is claimed.

**Deferred:** Actual licensed/consented fixture acquisition/annotation and measured results. Annotation matching/scoring definitions are recorded before tuning; no accuracy claim is made by planning.

## R7 - Free Hosting and Private Data

**Evidence:** Render services may restart/sleep; API processing is currently in-process. The current app has no user authorization, and local provider keys have previously been shared.

**Decision:** Sample-only release, bounded concurrency, owner/token/expiry-fenced recovery and persistent daily provider call reservations. A 60-second DB-time lease renews every 15 seconds; only expired jobs are reclaimed and stale owners cannot publish. Daily quota is shared across replicas, reserved atomically per outbound attempt/retry, starts a new bucket at UTC midnight and fails closed if config/storage is unavailable. Calls remain consumed conservatively after reservation; no refund/automatic replay or monetary guarantee. Persist controls/images/snapshots in Neon. Private access remains gated and credentials must be rotated outside chat. No user/role subsystem or persistent queue is introduced.

**Deferred:** Access-gateway selection, deploy runtime benchmarks, source permissions and confidential-data acceptance. Automatic Git deployments do not imply access security, guaranteed uptime or unlimited free AI calls.

## Remediation Boundaries

A1-A6 and async response designs passed the full consistency review; T002 is recorded complete. Optional analysis contracts, canonical page preparation/packaging and pure material formulas have focused verification. Job/quota persistence, public feature routes/UI, raw holdout evaluation and full acceptance remain pending. Readiness and foundation tests are not feature delivery.

## Async Failure Response Decision

Before acceptance, request/page/capacity/configuration/store failures use appropriate HTTP errors. Return 202 only after durable owned-job creation. After acceptance, preparation/quota/provider failures become ownership-checked `AnalysisFailed` records with structured code/retry/reset metadata and legacy errorMessage; successful polling reads use HTTP 200. Quota reservation remains atomic immediately before outbound calls, even when advisory preflight passed. No rejected call dispatches, and no later 429/503 rewrites the accepted response. DB outage cancels work and yields status-store unavailability until fenced failure/recovery can be recorded. T044/T055/T009/T016 must verify both paths.