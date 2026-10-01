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

The current Next.js scaffold is at the repository root so it can run immediately. The Go API starts with `go run ./api/cmd/server` from the repository root after changing into `api`, or with `go run ./cmd/server` from `api`.

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

The API currently exposes `GET /health`, project creation/retrieval, validated floor-plan upload, analysis start/status, confirmed-data submission, estimate creation/listing/retrieval, and Excel export endpoints. Workspace endpoints (spec 004): `GET /api/projects` (summaries with stage and latest total), `GET /api/projects/{id}/summary`, `PATCH /api/projects/{id}` with `{"archived": bool}`, `POST /api/projects/{id}/duplicate`, `GET /api/floor-plans/{id}/file`, and `GET /api/analyses/{id}/confirmations`. Set `DATABASE_URL` to use PostgreSQL. The analysis engine is selected for each drawing in the UI, or with `POST /api/floor-plans/{id}/analysis` and JSON `{"engine":"gemini"}` (`demo`, `ollama`, or `gemini`). `GET /api/analysis-engines` lists configured choices. Omitting the engine keeps the prior default behavior: Ollama when configured, otherwise demo.

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
