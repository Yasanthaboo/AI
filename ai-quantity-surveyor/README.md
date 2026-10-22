# AI Quantity Surveyor

An AI-assisted quantity surveying MVP for residential house floor plans.

## Stack

- Web: Next.js, React, TypeScript, Tailwind CSS
- API: Go `net/http`
- PostgreSQL is the configured production repository layer; local JSON remains an explicit development fallback when `DATABASE_URL` is absent.
- AI: provider behind an analyzer interface

## Structure

- The repository root contains the Next.js UI scaffold.
- `api` contains the Go backend foundation.
- `.specify/memory/constitution.md` is the governing project constitution.
- `specs/001-ai-quantity-surveyor/` contains the specification, implementation plan, and initial tasks.
- `specs/004-project-workspace-and-finishes/` covers the projects dashboard, overview, finishes estimating, settings, and model tools; `tasks.md` traces each requirement to code and tests.
- `specs/005-next-demo-recognition-and-material-prices/` defines the next demo: improved windows/stairs/balconies review, dimension confirmation, paint/tile/plaster material budgets and local-price comparisons. Constitution 1.6.0 ratifies the bounded plaster-material addition. Selected-page review, informational-feature acknowledgement, versioned material budgets, fixture comparisons and separate material export are implemented. Recognition holdout accuracy, supplier approvals/import, hosted verification and private-data release remain pending. The comparison pilot is Colombo/Sri Lanka/LKR and uses fictional paint/tile offers only; plaster price coverage is absent.

## Next Demo

Deliver recognition quality and source-drawing evidence first, then deterministic paint/tile/plaster material requirements, then an allowlisted local-price pilot. Plaster uses confirmed net plaster area and explicit reviewed recipes only; it is not construction or structural mix advice. Preserve human confirmation and existing estimate/export results. RAG and multi-agent orchestration are deferred until measured evaluation shows a need. Hosted Render/Neon deployment was reported successful during the first demo follow-up; access protection, regression verification, price freshness, and reproducible saved comparisons remain release gates. Use sample drawings until both web and API are protected.

Feature 005 has managed job/source persistence, page/source-image APIs, dimension review and correction history; immutable paint/tile/plaster budgets and fictional-price workflows remain separate from QS totals. Plaster recipes require explicit thickness, consumption/mix, density and pack inputs; the UI provides no defaults. Unknown/excluded charges remain incomplete in previews and separate exports. Constitution 1.6.0 ratifies plaster-material calculation but does not approve structural mix design, plaster suppliers or plaster price coverage. `SURVEYOR_ENABLE_PLASTER_MATERIALS` defaults off; Compose enables it only for the isolated test stack. None of the sample prices are live quotations or stock reservations. Current Go, vet, TypeScript, focused lint and all 26 browser tests pass against disposable sample infrastructure; recognition accuracy, live supplier approval, hosted verification and private-release checks remain open.

### Provider Quotas

External analysis providers require `DATABASE_URL`, their usual provider settings, a shared nonsecret `PROVIDER_QUOTA_SCOPE`, and positive daily limits (`GEMINI_DAILY_CALL_LIMIT`, `OLLAMA_DAILY_CALL_LIMIT`, or `NVIDIA_DAILY_CALL_LIMIT`) below the account allowance. NVIDIA Nemotron analysis uses `NVIDIA_API_KEY`, `NVIDIA_NEMOTRON_MODEL` (default `nvidia/nemotron-3.5-super-vl-120b-a12b`), and `NVIDIA_NIM_BASE_URL` (default `https://integrate.api.nvidia.com/v1`). The API sends the prepared page PNG to the NVIDIA chat-completions endpoint and expects structured JSON output. External engines are hidden unless PostgreSQL, a shared quota scope, and a positive per-provider limit are configured. Set values only on the API service. Call quotas limit attempts, not guaranteed monetary spend.

### Authentication

The browser supports Supabase email/password signup and sign-in, Google OAuth, email verification, password reset/recovery, and sign-out. It reads `SUPABASE_URL` and `SUPABASE_ANON_KEY` at server runtime and forwards session access tokens to the Go API. Configure both values on the web and API services. In Supabase Auth, enable Google and set the Site URL and allowed redirect URLs to the deployed web origin (and local development origin where needed).

The Go API validates access tokens and scopes project, drawing, analysis, estimate, material-budget, comparison, and export routes to the authenticated user. PostgreSQL deployments also require `INITIAL_ADMIN_USER_ID`; startup assigns existing ownerless/local projects to that initial admin and refuses to start if migration fails. Keep deployment values out of source control. Local Compose and the isolated test Compose file explicitly allow anonymous development access; never enable this on a public deployment.

### Plans and Feature Access

The authenticated **Plans** view compares the packages and shows the current account plan. Paid prices are marked **Coming soon**; there is no payment collection, checkout, subscription renewal, or billing-provider integration.

| Feature | Free | Pro | Studio |
| --- | --- | --- | --- |
| Projects, demo analysis, dimension review, basic finish estimates | Yes | Yes | Yes |
| Configured AI drawing analyzers | No | Yes | Yes |
| Conceptual 3D workspace and exterior/roof options | No | Yes | Yes |
| Material budgets and fixture price comparisons | No | Yes | Yes |
| Excel, CSV, print/PDF export controls | No | Yes | Yes |
| New room-image generation | No | No | Yes |

`GET /api/plans` returns the catalog; `GET /api/account/plan` returns the caller's effective tier and demo-switch capability. New accounts default to Free. PostgreSQL persists assignments in `account_plans`; without PostgreSQL the explicit development store is memory-only. Owner identity always comes from the verified API request, never a submitted user ID or browser storage. The API checks AI analysis, material routes (including project-scoped budgets), Excel export and room-generation requests independently of UI locks. Because 3D, CSV and print are derived from already-readable project/estimate data, those are UI access controls, not DRM or restrictions on copying one's own data. Existing saved room images remain readable after downgrade; switching plans does not delete projects or reset provider quotas.

Self-service switching is disabled by default. For an isolated demo, configure both API settings:

```dotenv
APP_ENV=demo
PLAN_DEMO_MODE=true
```

`APP_ENV=development` is also accepted. Startup refuses `PLAN_DEMO_MODE=true` for production or an unspecified environment. In demo mode, `PUT /api/account/plan` accepts only `{"planId":"free"}`, `pro`, or `studio`; the UI labels every change as a demo and states that no payment is taken. Demo upgrades are recorded with `assignment_source='demo'` and resolve to Free whenever demo mode is turned off. Every protected request rereads the effective plan and fails closed if plan storage is unavailable. Existing provider configuration and daily limits remain additional requirements: a plan never configures keys, enables a missing provider, or increases usage limits.

For production, leave demo mode off. Until a trusted billing/admin flow is added, only a database operator can provision a paid assignment using an authenticated user's verified ID, a valid `plan_id`, and `assignment_source='manual'`; there is deliberately no public production upgrade endpoint. Demo switches overwrite the current assignment, so test them with test accounts rather than manually provisioned production accounts. Account plan lookup failures expose a retry and keep paid features locked. The new database schema is additive; existing projects and financial snapshots are not migrated or altered.

Validation: `go test ./api/internal/plans ./api/cmd/server -run TestPlan` covers account isolation and server gates. `e2e/plans.spec.ts` covers demo tier changes, reload persistence, locked features, non-demo restrictions and light/dark mobile layout with mocked services. To run the guarded real database test, set `SURVEYOR_PLAN_TEST_DATABASE_URL` to a disposable database whose name starts with `surveyor_plan_test`, then run `go test ./api/internal/plans -run TestPlanPostgresPersistence -count=1`; customer database names are rejected.

The room Design Studio defaults to the API-only OpenAI image generator when `OPENAI_API_KEY` is configured; `OPENAI_IMAGE_MODEL` defaults to `gpt-image-1`. To use NVIDIA FLUX instead, set `ROOM_DESIGN_PROVIDER=nvidia`, `NVIDIA_API_KEY`, `NVIDIA_FLUX_MODEL`, and the exact `NVIDIA_NIM_IMAGE_ENDPOINT` shown for the enabled image model in your NVIDIA API Catalog. The Nemotron chat endpoint is separate and cannot generate images. `ROOM_DESIGN_DAILY_IMAGE_LIMIT` defaults to four generated images per user per UTC day. Each request generates two concepts from server-loaded confirmed room dimensions/openings plus user-selected style, palette, and preferences. Floor-plan files and source-image URLs are never sent to either image provider. Generated concept PNGs and their exact confirmation revision are stored with the project in PostgreSQL. Without a configured image provider the Design Studio remains visible after confirmation but generation is disabled.

### Seeded Plan Preview Accounts

An opt-in bootstrap creates three real Supabase email/password logins, one per plan, and isolated synthetic workspaces:

| Login | Permanent plan | Starting data |
| --- | --- | --- |
| `demo-free@quantity-surveyor.test` | Free | Synthetic drawing, four confirmed demo rooms and an estimate |
| `demo-pro@quantity-surveyor.test` | Pro | Its own copy of the same synthetic fixture |
| `demo-studio@quantity-surveyor.test` | Studio | Its own copy of the same synthetic fixture |

The reserved `.test` addresses cannot receive mail. Admin creation explicitly confirms these synthetic test identities, so they can sign in without verification email. Use real aliases instead if reset/verification-mail testing is needed. These accounts are never administrators. Accounts with the same login share projects, edits and per-account daily quotas; different plan accounts are isolated. Give testers only that plan's email/password through a private channel, never the Supabase admin key. Use sample data only in shared accounts.

**Required private API configuration:** keep the existing `DATABASE_URL` and `SUPABASE_URL`; set `SUPABASE_SERVICE_ROLE_KEY` to the Supabase server-side service-role/admin key. Set three distinct generated secrets in `SEED_PLAN_FREE_PASSWORD`, `SEED_PLAN_PRO_PASSWORD` and `SEED_PLAN_STUDIO_PASSWORD` (16-256 characters each). Do not paste these values into chat, source files, browser variables, logs or commands that print environment configuration. No default passwords or admin keys are provided.

Set these nonsecret API options to provision the accounts:

```dotenv
SEED_PLAN_USERS=true
SEED_PLAN_FREE_EMAIL=demo-free@quantity-surveyor.test
SEED_PLAN_PRO_EMAIL=demo-pro@quantity-surveyor.test
SEED_PLAN_STUDIO_EMAIL=demo-studio@quantity-surveyor.test
```

For local Compose, configure the private values in the ignored local environment file, then run from the workspace parent:

```powershell
docker compose -f ai-quantity-surveyor/compose.yaml up --build -d --no-deps api
```

For Render, configure these values **only on the API service** using its private environment settings, set `SEED_PLAN_USERS=true`, and redeploy. The blueprint includes API-only placeholders and defaults seeding off. The API image also includes `/app/seed-plan-users` for an operator-run one-shot bootstrap (`go run ./api/cmd/seed-plan-users` from the app root in a trusted host environment). No public seed endpoint is exposed. The normal startup hook supports deployments without a remote shell.

Startup validates all three emails/passwords before any admin call. It then acquires a PostgreSQL advisory lock, creates missing Supabase identities with a server-owned seed marker, atomically assigns their matching plans with `assignment_source='manual'`, and creates or resumes each synthetic drawing/confirmation/estimate using the demo analyzer only. The generated PNG is clearly marked synthetic; it has canonical source provenance for material budgeting. No customer project is copied, and neither cloud analysis nor image generation is called. A workspace registry prevents duplicate samples after renaming/archiving. Existing confirmation revisions, estimates, drawings and tester changes are left in place.

Reruns reuse already-confirmed identities carrying the correct seed marker and tier. They **never reset passwords**, delete identities or silently adopt an unrelated user whose email collides. If provisioning fails partway through, already-created seed identities can be reused on the next run; automatic deletion/rollback of Supabase users is deliberately avoided. A collision with an unmanaged identity stops seeding for manual admin review. To rotate a preview password, use Supabase's private admin interface, not a redeploy with a new password variable.

These are durable plan assignments, so `PLAN_DEMO_MODE` can stay `false` and `APP_ENV` can stay `production`. They survive ordinary API restarts/deployments as long as the same Supabase project and application PostgreSQL database are retained. The bootstrap can restore assignments and samples on another application database using the same marked identities. After successful first setup, set `SEED_PLAN_USERS=false` and remove the service-role key and seed passwords from the long-running API environment if no further provisioning is required; the logins and plans continue to work. Changing only an email setting creates a different identity; it does not delete the old one. Disabling the seed does not disable or revoke existing preview logins: retire them explicitly in Supabase when testing ends.

Before handing out access, confirm that logs report each plan/email ready (passwords are never logged), sign in as each preview user and check the current plan and sample project. Pro/Studio provider features still depend on configured providers and existing quota limits. Live account creation remains blocked until the private admin key/password settings have been supplied. Offline tests mock Supabase; guarded `SURVEYOR_PLAN_SEED_TEST_DATABASE_URL` tests use only database names starting with `surveyor_plan_seed_test` and verify restart-safe/idempotent samples and production plan restoration.

### Exterior and Roof Options

After confirming dimensions, **Exterior** opens a separate Pro/Studio view with flat, gable, hip and shed roofs. Supported pitch is 5-60 degrees for pitched forms; flat uses a zero-pitch schematic surface and disables that control. Overhang is 0-2 m. Metal/tile/membrane choices change appearance only. Roof changes never write to confirmations, quantity calculations, material budgets or estimates.

The first envelope suggestion uses the conceptual arrangement of confirmed rooms and known wall heights, not verified building elevations. Users must review length/width (1-100 m), wall height (1-12 m) and the explicit assumptions before saving. The model is a rectangular single-storey envelope, not BIM: openings, setbacks, additional levels, drainage, material specifications, loads and code compliance are unmodeled. Non-WebGL browsers retain the same editable controls and save/reopen actions.

`GET /api/projects/{id}/exterior-options?analysisId=...&revision=...` and `POST /api/projects/{id}/exterior-options` are ownership- and plan-protected. Saves require a valid project-owned confirmed snapshot and acknowledgement of assumptions. Immutable options in PostgreSQL's additive `exterior_options` table retain their exact analysis/revision and server-written assumptions. Selecting historical confirmations shows only their saved options. No financial snapshots or existing exports are migrated.

Design Studio now rounds displayed room dimensions without altering confirmed measurements, shows explicit per-image fetch/decode failures with retry, and offers authenticated PNG downloads. Retry only fetches a saved asset; it never generates or consumes another concept pair. Concept images are displayed uncropped.

### Local Image Fallback (No API Fee)

Room concepts can optionally fall back to a locally hosted image model, such as **SDXL base 1.0**, using [stable-diffusion.cpp's OpenAI-compatible image API](https://github.com/leejet/stable-diffusion.cpp/blob/master/examples/server/api.md). This works independently of Ollama text/vision models. Local inference has no cloud API fee, but requires suitable hardware, model weights downloaded separately, and compliance with the model licence; GPU, electricity, and hosting costs still apply.

1. Install a compatible Windows `sd-server` build from stable-diffusion.cpp and obtain compatible SDXL weights. Start it with a model directory, for example `sd-server --model-dir C:\models --listen-port 1234`.
2. Open its local UI at `http://localhost:1234`, select the SDXL recipe and matching model files, and load the model. Keep this unauthenticated server private; if Docker needs a host-interface binding, restrict access with the host firewall rather than exposing it publicly.
3. Keep the existing NVIDIA or OpenAI primary provider and set these API environment variables:

```dotenv
ROOM_DESIGN_FALLBACK=local
LOCAL_IMAGE_ENDPOINT=http://host.docker.internal:1234/v1/images/generations
LOCAL_IMAGE_MODEL=sdxl-base-1.0
```

For a host-run Go API use `localhost` instead of `host.docker.internal`. The endpoint must use localhost, Docker's host alias, or a private IP address; redirects are rejected and cloud credentials are never forwarded. `LOCAL_IMAGE_MODEL` is a provenance label for the model loaded on that server, not a command to download or switch models. Use a label matching the actual loaded weights. The adapter requests two 1024x1024 PNGs with a 180-second local request timeout and validates image count, dimensions, decoding, and size before storage. Only confirmed room facts and preferences are sent, not floor-plan files.

Fallback is opt-in and only runs on connection failures, timeouts, provider HTTP 429, or HTTP 5xx. It does not run after moderation blocks, authorization errors, TLS certificate errors, malformed images, or request cancellation. The existing application daily quota is checked once before either provider; fallback does not bypass that quota, refund reserved slots, or undo any primary-provider charges. A successful fallback replaces the whole concept pair, with `local/<configured model>` recorded on each concept. Local requests cannot embed the server's `sd_cpp_extra_args` control tags in preferences.

For entirely local generation, set `ROOM_DESIGN_PROVIDER=local`, the same endpoint/model settings, and leave `ROOM_DESIGN_FALLBACK` empty. No OpenAI or NVIDIA key is required for that image provider. Leave both new options unset to preserve current behavior. No local server, model downloads, or live configuration changes are performed automatically. After configuring the local server, rebuild/recreate the API from the workspace parent with `docker compose -f ai-quantity-surveyor/compose.yaml up --build -d --no-deps api`.

Canonical review images are served by `/api/analyses/{id}/source-image`; `/api/floor-plans/{id}/pages` returns page metadata. Analysis POST accepts optional `pageNumber`; the new PDF UI requires explicit selection, while omitted legacy requests use page 1. Once a start returns 202, stored failures are read through HTTP 200 status polling with error/retry metadata.

The current Next.js scaffold is at the repository root so it can run immediately. The Go API starts with `go run ./api/cmd/server` from the repository root after changing into `api`, or with `go run ./cmd/server` from `api`.

For host-only anonymous API development, explicitly set `AUTH_ALLOW_ANONYMOUS=true`; the normal local Compose file already sets it. Without Supabase credentials or this opt-in, the API refuses to start.

## Development

### Release Checks and Remaining Gates

Local verification on 2026-10-21: all **40 Playwright tests**, full Go API tests/vet, ESLint and the Next.js production build pass. Browser tests run with three workers against a uniquely named disposable PostgreSQL/API stack, never the customer database. Roof tests include all four forms on desktop/mobile, nonblank canvas pixels, orbit interaction, source-revision reopening and forced WebGL failure. A saved exterior option also survives a real disposable API restart. The constrained API runs as UID 10001 with 512 MiB memory, 128 process and one-CPU limits, dropped capabilities and no-new-privileges; post-restart kernel measurements report 16,535,552 peak bytes and 12 processes, with no OOM kill. Real Poppler preparation produces the selected 3000x2000 page PNG under the same constraints. These small/mock-provider fixture checks do not establish real hosted concurrency or image-provider peak memory.

Legacy browser integration tests now require both `SURVEYOR_E2E_ISOLATED=true` and `PLAYWRIGHT_API_URL=http://127.0.0.1:<port>` with a port >=18000. Use a uniquely named `compose.test.yaml` project and a generated temporary `SURVEYOR_TEST_PASSWORD`; this stack has no real provider keys and never shares customer volumes. Set `PLAYWRIGHT_WEB_PORT=3002` and the same `NEXT_PUBLIC_API_URL`. The tests mock browser Supabase sessions and explicitly provision Studio access only on the disposable API. Without the isolation opt-in, legacy integration cases skip instead of using the normal application API. Do not run them against customer infrastructure.

The API has header/read/idle timeouts, a 64 KiB header bound and a seven-minute response deadline covering configured image-provider fallback. Container defaults cap API memory at 512 MiB and processes at 128; `API_MEMORY_LIMIT` can explicitly override memory for another workload. The container runs non-root, drops Linux capabilities and disallows privilege escalation. Production (`APP_ENV=production`) refuses anonymous startup; Render explicitly disables anonymous access and demo plan switching. CORS is restricted to `ALLOWED_ORIGIN` and includes DELETE preflight. API data is private/no-store by default; web responses add anti-framing, nosniff, referrer/permission restrictions and baseline CSP directives. These are baseline controls, not a complete hosted security certification.

`npm audit --omit=dev` reports no production vulnerabilities. `eslint-config-next` is aligned with Next.js 16.4.2, but the development ESLint chain still reports five high-severity findings from **GHSA-vfj7-8cjw-p6xm** (`braces` <=3.0.3); no patched version is published in the registry checked for this release. No `audit fix --force`, incompatible framework downgrade, or advisory suppression is applied. Lint patterns must remain trusted developer input; reassess when an upstream patch is available.

Live production **Google login, reset-email delivery, hosted TLS/redirects, SMTP/domain setup, cold starts, hosted concurrency and deployed resource limits remain pending**, per the user's selected verification scope. Local mocked redirect tests cannot certify delivery or provider-dashboard configuration. Before private-data release: provide the deployed web/API URLs; validate Supabase Site URL/redirect allowlists and Google callback/origins; use a designated test account to complete Google login and a reset link/password update, confirm expired-link/session behavior, inspect SMTP delivery, verify HTTPS/security headers and missing/cross-account token denial, and benchmark concurrent consented workloads without bypassing quotas. Enter credentials only in the provider/browser UI, never in chat. Keep this gate open until evidence is recorded.

### Docker Compose

The local Compose stack runs Next.js, the Go API, and PostgreSQL in separate containers. From the repository root in PowerShell:

```powershell
Copy-Item .env.example .env
# Edit .env locally: replace POSTGRES_PASSWORD and optionally set GEMINI_API_KEY.
docker compose up --build -d
docker compose ps
```

Open `http://localhost:3001`; the API is at `http://localhost:8080`. Stop any separately running Next.js or Go API processes on those ports first. Uploaded drawings, projects, and confirmed analyses live in the named PostgreSQL volume and survive `docker compose down`; `docker compose down -v` deletes that data. Changes to `.env` require `docker compose up -d --force-recreate api`. Gemini keys remain on the API container; use `OLLAMA_ENDPOINT=http://host.docker.internal:11434` to reach a local Ollama service on Windows. Do not commit `.env`. The placeholder password in `.env.example` must be replaced with a strong URL-safe value before starting.

### Host Development

```text
npm run dev -- --port 3001
cd api
go run ./cmd/server
```

Open the app at `http://localhost:3001`. The repository also includes a Go workspace, so `go test ./...` can be run from the repository root.

The API currently exposes `GET /health`, project creation/retrieval, validated floor-plan upload, analysis start/status, confirmed-data submission, estimate creation/listing/retrieval, and Excel export endpoints. Workspace endpoints (spec 004): `GET /api/projects` (summaries with stage and latest total), `GET /api/projects/{id}/summary`, `PATCH /api/projects/{id}` with `{"archived": bool}`, `POST /api/projects/{id}/duplicate`, `GET /api/floor-plans/{id}/file`, and `GET /api/analyses/{id}/confirmations`. Feature 005 adds project-scoped material budgets, fixture offer reads, non-saving comparison preview, explicit saved comparisons and a separate XLSX download under `/api/material-budgets`, `/api/material-comparisons` and `/api/material-offers`. Budget requests cannot supply authoritative areas; exact confirmation revision, page and image digest are verified server-side. `GET /api/material-capabilities` advertises whether plaster budgeting is enabled; the feature defaults off and `SURVEYOR_ENABLE_PLASTER_MATERIALS=true` opts it in. Current fictional Colombo price offers cover paint/tile only. Set `DATABASE_URL` to use PostgreSQL. The analysis engine is selected for each drawing in the UI, or with `POST /api/floor-plans/{id}/analysis` and JSON `{"engine":"gemini"}` (`demo`, `ollama`, or `gemini`). `GET /api/analysis-engines` lists configured choices. Omitting the engine keeps the prior default behavior: Ollama when configured, otherwise demo.

The web app opens on a **Projects** dashboard (search, duplicate, archive, open). Each project has an **Overview** (details, drawing, analysis engine and model, activity timeline, confirmation history) and a five-step workflow: project details, floor-plan upload and AI analysis (with live progress), review and confirmation of rooms and openings (with analysis details and re-run on another engine), a 3D model, and an estimate. Estimates cover plaster, paint, wall tiling, flooring, ceiling, and skirting, with per-room finish choices, a waste allowance, an assumptions list, cost charts, version comparison, and Excel, CSV, and print exports. **Settings** stores default rates, currency, preferred unit, and default wall height in the browser; a defaulted wall height is marked in review and listed as an assumption. The AI also returns each room's boundary on the drawing; the 3D model places rooms at those positions, scales them from the confirmed dimensions, draws doors and windows at indicative positions, and offers linked room selection, hover details, a top view, and a screenshot. It is labeled an AI-estimated layout; rooms without a detected boundary are arranged approximately. Estimates always use the confirmed dimensions, never model geometry. The open project is restored after a browser reload. `GET /api/analyses/{id}/confirmed-data` returns the latest confirmed dimensions and room boundaries.

Set `OLLAMA_ENDPOINT` to enable Ollama and optionally `OLLAMA_MODEL` and `OLLAMA_API_KEY` (for Ollama Cloud, use `OLLAMA_ENDPOINT=https://ollama.com`). Set `GEMINI_API_KEY` on the Go server to enable Google Gemini, and optionally `GEMINI_MODEL` (defaults to the free-tier `gemini-3.8-flash`; choose an available model for your account). New Google API projects may not have access to the older Gemini 2.5 models. Restart the Go server after changing these settings. The UI shows unconfigured engines as disabled options. Never expose provider keys in browser code or commit them to source control. Gemini supports the uploaded PDF, PNG, JPG, and JPEG floor plans up to the app's 10 MB upload limit; Ollama still requires rasterization for PDFs. AI extraction remains subject to human confirmation before quantities and costs are calculated. Feature work must follow the constitution and confirmed-data calculation boundary.

If Gemini returns HTTP 404, check the model name in the same PowerShell session that has `GEMINI_API_KEY` set. After stopping the API, run `(Invoke-RestMethod -Uri 'https://generativelanguage.googleapis.com/v1beta/models' -Headers @{ 'x-goog-api-key' = $env:GEMINI_API_KEY }).models | Where-Object { $_.supportedGenerationMethods -contains 'generateContent' } | Select-Object -ExpandProperty name`. Set `GEMINI_MODEL` to one of those names without the `models/` prefix and restart the API.

If Gemini returns HTTP 401 from `generateContent`, check `GEMINI_API_KEY` in the ignored `.env` file and run `docker compose up -d --force-recreate api` after updating it. Model metadata access is not required for generation and is not used as an authorization check. Rotate any previously shared key; never paste keys into chat or commit them.
This is a [Next.js](https://nextjs.org) project bootstrapped with [`create-next-app`](https://nextjs.org/docs/app/api-reference/cli/create-next-app).

## Getting Started

First, run the development server:

```bash
npm run dev
# or
yarn dev
# or
pnpm dev
# or
bun dev
```

Open [http://localhost:3000](http://localhost:3000) with your browser to see the result.

You can start editing the page by modifying `app/page.tsx`. The page auto-updates as you edit the file.

This project uses [`next/font`](https://nextjs.org/docs/app/building-your-application/optimizing/fonts) to automatically optimize and load [Geist](https://vercel.com/font), a new font family for Vercel.

## Learn More

To learn more about Next.js, take a look at the following resources:

- [Next.js Documentation](https://nextjs.org/docs) - learn about Next.js features and API.
- [Learn Next.js](https://nextjs.org/learn) - an interactive Next.js tutorial.

You can check out [the Next.js GitHub repository](https://github.com/vercel/next.js) - your feedback and contributions are welcome!

## Deploy on Vercel

The easiest way to deploy your Next.js app is to use the [Vercel Platform](https://vercel.com/new?utm_medium=default-template&filter=next.js&utm_source=create-next-app&utm_campaign=create-next-app-readme) from the creators of Next.js.

Check out our [Next.js deployment documentation](https://nextjs.org/docs/app/building-your-application/deploying) for more details.
