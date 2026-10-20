# Planned Development and Verification: Feature 005

**Status:** Sample implementation in progress. Selected-page review, material budgets, fixture comparison and separate export are integrated and verified. Holdout accuracy, operator catalog import, live suppliers, hosted/private-access approval and release gates remain pending.

## Setup

1. Continue on the current feature branch; do not create another branch or commit/push without approval.
2. Read constitution 1.6.0, `spec.md`, `plan.md`, `research.md`, `data-model.md` and `contracts.md`. Continue implementing accepted scope and track external gates separately.
3. Install existing frontend dependencies with `npm ci`. Configure PostgreSQL/provider secrets locally or in hosted environment settings; never paste keys/connection credentials into chat or commit them.
4. Implement the API runtime Poppler dependency and verify its selected-page preparation tests. A raw Ollama PDF is not supported until this preprocessing slice is delivered. Use existing Compose for the target runtime; host binaries must be explicitly available if running Go outside Docker.
5. After quota/lease implementation, configure nonsecret `PROVIDER_QUOTA_SCOPE`, `GEMINI_DAILY_CALL_LIMIT` and `OLLAMA_DAILY_CALL_LIMIT` consistently across API replicas, below your account allowances. Missing/zero limits disable dispatch; PostgreSQL quota availability is required. Do not assume these planned settings are implemented today. Call quotas limit attempts, not guaranteed monetary spend.

## Existing Commands and Required Gates

From `C:\AI\ai-quantity-surveyor` (or the corresponding clone's app root):

```powershell
go test ./api/...
go vet ./api/...
npx --no-install tsc --noEmit
npm run lint
```

Start the existing local stack only after required image/env changes are implemented:

```powershell
docker compose up --build -d
```

The current Playwright config starts `npm run dev -- --port 3001` and `go run ./api/cmd/server`, reusing existing servers on web 3001 and API 8080. If Compose is running on those ports, it reuses that stack; otherwise host Go must have the planned Poppler executables available. Use a disposable test PostgreSQL configuration in either mode and verify the server on each port is the intended test instance before running:

```powershell
npm run test:e2e
```

Avoid duplicate servers on occupied ports. Never point new integration tests at customer/production data, delete local volumes or reset user data as part of validation.

## Slice Acceptance Sequence

1. **Selected page:** upload a small multi-page/rotated fixture, select a non-first page and start analysis. Verify the prepared PNG/provider input/evidence/review all share the same digest/original page. Change selection and confirm a new analysis ID. Test invalid page and preparation failure without provider invocation.
2. **Review preview:** select room/opening, inspect source evidence and units/metre equivalents, edit pending dimensions, observe original/pending/confirmed distinctions. Invalid values prevent confirmation; explicit valid confirmation creates a revision. Zoom/pan and a renderer failure never derive measurements or mutate old revisions.
3. **Recognition evaluation:** freeze >=10 consented pages/>=20 instances per feature and raw identity/association rules. Report detection, dimensions and localization separately by provider/model; use IoU >=0.5 only for localization. Mark legacy unsupported location as notSupported rather than missing all detections. Count duplicates/ambiguity, apply unchanged new-output thresholds and compare jointly supported raw baseline metrics. Human correction/confirmation tests remain separate.
4. **Material budget:** run both AC-04 examples from a traceable estimate. Verify exact packs/surplus, single waste, excludes and compatible cross-room aggregation. Reject duplicate room/trade allocations even with different recipes/groups, without saving a version; client areas cannot replace frozen quantities. Verify legacy/foreign-source rejection and zero-normalization boundary errors without substitution. Existing QS golden values remain identical.
5. **Local fixture prices:** operator-load fictional paint/tile fixtures in Colombo/LKR. Preview alternative baskets without creating any saved version; verify authoritative packs/charges/warnings and no supplier/provider calls. Explicit Save recomputes against selected observation IDs and rejects newly expired selections. Verify unknown charges, stale/out-of-stock/quote-only and currency states; never label fixtures live.
6. **Snapshots/export:** save a comparison, load a newer price observation, reopen/export the old selection and verify identical values. Restart API/DB and verify durable records; failed create/refresh must not publish partial data.
7. **Hosted sample demo:** verify Render contexts, web HOSTNAME/PORT/API URL, origin, Neon, providers and workload bounds. Simulate overlap deployment, lost heartbeat/expiry and stale completion: live jobs survive rollout and old owners cannot publish. Test shared quota concurrency, failure/retry accounting, restart persistence, UTC rollover and config/store rejection without dispatch; verify clear 429/reset messages. Capture desktop/mobile screens; confidential plans stay blocked until both web/API protection is approved/tested.

## Still Blocked or Deferred

Supplier shortlist/permission, live collection, real-source freshness/equivalence, private-access selection, consented holdout acquisition and measured recognition remain gated. Poppler packaging/single-page runtime is verified, but redistribution review and full concurrent workload/resource benchmarks remain pending. Source permissions/credentials are not implied by a passing schema or helper test.

## Recorded Foundation Verification (2026-10-13)

- `go test ./api/...`, `go vet ./api/...`, installed TypeScript typecheck and lint for `src/app/api.ts` pass for the current slices. Browser workflow/E2E and new PostgreSQL migrations are not claimed verified.
- API image packages Alpine `poppler-utils-24.02.0-r2`; pdfinfo/pdftoppm 24.02.0 run as UID 10001. Package metadata declares GPL-2.0-or-later. Retain upstream notices and satisfy license/corresponding-source obligations if redistributing the image/binaries; review redistribution requirements before release. The image test does not certify legal compliance for a distribution.
- Canonical resampling uses `golang.org/x/image v0.38.0`, already required transitively by the repository's existing Excelize 2.11.0; the export library was not downgraded.
- Real selected-page integration uses a generated technical two-page PDF, not a recognition holdout, and runs with a 512 MB container limit. Page 2's rotation/crop becomes 3000x2000 PNG (22208 bytes). Docker reports memory limiting without swap; no full concurrent workload benchmark is claimed.

```powershell
docker build --target page-tests -f Dockerfile.api -t surveyor-page-tests .
docker run --rm --memory 512m surveyor-page-tests
```

The normal Docker build still ends with the production API stage. The page-tests target contains only a test binary; it does not start the API or touch customer data. Host Go tests skip the real Poppler case when binaries are absent; the disposable-container case above must be run to verify it.

## Async Error Smoke Check

Test pre-acceptance invalid page/capacity/quota/configuration failures as HTTP errors without provider calls. Then accept an analysis with 202 and force a quota race or provider/preparation failure: polling must return HTTP 200 with AnalysisFailed and structured code/retryable/retryAt plus legacy errorMessage. Verify the UI stops polling, displays reset information and retains saved inputs. A quota-store outage must never dispatch; if terminal persistence is temporarily impossible, status reads may return 503 until lease-fenced failure/recovery succeeds. No later error changes the already accepted POST response.

## Material Sample Verification

The estimate workflow includes explicit paint/wall-tile/floor-tile allocation, reviewed recipe inputs, material waste, immutable budget versions and frozen source assumptions. Budgets require a verified confirmation revision and exact canonical drawing page/image digest. They do not alter QS quantities, finish costs or the existing estimate workbook. Price alternatives use fictional persisted Colombo observations only; preview is read-only and explicit Save revalidates selected observations. Tax and delivery are distinct nullable source states; unknown or excluded charges never become a complete delivered total. The separate material workbook exports frozen requirements, offers, sources and assumptions.

The isolated sample stack passed `go test ./api/... -count=1`, `go vet ./api/...`, TypeScript, focused ESLint and all 26 Playwright tests. A guarded `surveyor_job_test` PostgreSQL test verifies concurrent material versions, failed-insert rollback, connection reopen and snapshot immutability after a newer observation. The browser and pricing tests call no suppliers or AI providers. These fictional fixtures imply no source approval or live price/stock claim.

### Plaster Materials

Constitution 1.6.0 ratifies traditional cement/sand and premix consumption budgets using confirmed net plaster area and explicitly reviewed inputs; no default recipe or construction recommendation is provided. The API capability defaults off and can be enabled with `SURVEYOR_ENABLE_PLASTER_MATERIALS=true`. The isolated `compose.test.yaml` stack opts in for browser tests. Price fixtures remain paint/tile only; real plaster offers require a separately approved source and equivalence policy. Formula tests are deterministic arithmetic checks, not engineering recommendations.

## Recorded Managed Integration (2026-10-14)

- Managed runtime is wired in the API; page metadata/start/status/canonical-image routes and pre/post-202 errors have focused server tests. External engines require PostgreSQL and configured quota limits; memory-only server mode exposes demo only. Provider redirect replay is rejected by both adapters without altering prompts.
- Guarded live PostgreSQL tests pass for canonical-source durability/immutability, concurrent quota limits and late completion rejection. Runtime/memory tests cover canonical provider input, quota no-dispatch, missing configuration and UTC rollover. All test databases are disposable and the live job test refuses database names outside the surveyor_job_test prefix.
- All 26 Playwright tests pass against the edited web app and disposable API/PostgreSQL stack, without real AI keys. Cases cover selected PDF page, pending/original/confirmed values, canonical-image failure, mobile overflow, transient polling 503 and quota reset/identity, plus material-budget restoration, no-write preview, incomplete charges, explicit save/export and informational-feature acknowledgement. Screenshots were inspected; the confirmed 3D canvas has a nonblank pixel check. These tests do not measure extraction accuracy.
- `compose.test.yaml` isolates test infrastructure, with IPv4 API mapping default 18081 and origin `http://localhost:3002`. Ports 8081/localhost may be intercepted by IIS on this machine; never modify those unrelated services to run tests. Use a unique `docker compose -p` project and temporary `SURVEYOR_TEST_PASSWORD`; no customer volumes are attached. `PLAYWRIGHT_WEB_PORT=3002` selects the web port.
- Launch the test web process with `NEXT_PUBLIC_API_URL=http://127.0.0.1:18081` set in that SAME process so a reused server cannot silently point to an older API. The API supports `PORT`; default remains 8080. Set `SURVEYOR_TEST_API_PORT` if another isolated port is needed.
- Source evidence field filtering and correction/feature audit have focused coverage; full provider/evidence evaluation and privacy/access rollout remain pending. Catalog import/refresh, real supplier permission, holdout acquisition and hosted verification remain unchecked. Fictional material workflows are sample-only and not a confidential customer deployment.