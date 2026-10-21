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

The room Design Studio defaults to the API-only OpenAI image generator when `OPENAI_API_KEY` is configured; `OPENAI_IMAGE_MODEL` defaults to `gpt-image-1`. To use NVIDIA FLUX instead, set `ROOM_DESIGN_PROVIDER=nvidia`, `NVIDIA_API_KEY`, `NVIDIA_FLUX_MODEL`, and the exact `NVIDIA_NIM_IMAGE_ENDPOINT` shown for the enabled image model in your NVIDIA API Catalog. The Nemotron chat endpoint is separate and cannot generate images. `ROOM_DESIGN_DAILY_IMAGE_LIMIT` defaults to four generated images per user per UTC day. Each request generates two concepts from server-loaded confirmed room dimensions/openings plus user-selected style, palette, and preferences. Floor-plan files and source-image URLs are never sent to either image provider. Generated concept PNGs and their exact confirmation revision are stored with the project in PostgreSQL. Without a configured image provider the Design Studio remains visible after confirmation but generation is disabled.

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
