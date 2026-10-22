# Requirements: Accounts and Home Design Studio

**Status:** Supabase auth, account plans, room concepts and conceptual exterior/roof options are implemented. NVIDIA FLUX generated two distinct 1024x1024 concepts in a live local UI check on 2026-10-21. Exterior options are revision-bound and owner-scoped, with flat/gable/hip/shed controls and an editable non-WebGL fallback. Live OpenAI generation, hosted verification, real Google login and reset-email delivery are not claimed verified.

## Goal

Add private user workspaces, then let users explore room-level interior concepts and conceptual exterior roof options from explicitly confirmed project data. Design choices must never silently alter confirmed dimensions, quantity takeoffs, material budgets, or estimates.

## User Stories

### US1: Account access

As a user, I can create an account and sign in with email/password or an approved third-party identity provider, so my projects and drawings are private to me.

Requirements:

- Email/password signup, sign-in, sign-out, email verification, and password reset.
- OAuth/OIDC sign-in for explicitly selected providers; link an identity to one account without duplicate accounts.
- Secure, revocable sessions; rate-limit authentication attempts and avoid account-enumeration responses.
- Require authentication for project, drawing, analysis, confirmation, estimate, material-budget, comparison, and export operations.
- Derive ownership from the authenticated principal on the server. Never trust a client-supplied owner ID.
- Migrate existing ownerless projects through an explicit deployment decision; do not make them visible to every new account.
- Keep provider secrets and token verification on the server. Prefer a maintained identity provider over custom password/session cryptography.

Acceptance criteria:

- Verified signup and each configured third-party sign-in create or resolve one account.
- Unverified, invalid, expired, revoked, or missing sessions cannot access protected APIs.
- A user cannot list, read, mutate, duplicate, export, or infer another user's project resources by changing IDs.
- Sign-out revokes the session; password reset invalidates prior sessions according to the chosen policy.
- Existing projects are either assigned to an explicitly designated owner or retained in a clearly isolated migration/demo state.

### US2: Room-by-room interior concepts

As a user with a confirmed project, I can open a separate Design Studio menu, select one room, request AI-generated interior concepts, compare them, and save a preferred concept.

Requirements:

- Show the Design Studio entry only when the project has a confirmed dimensions/openings revision; otherwise explain the unmet prerequisite.
- Select one confirmed room at a time and show its confirmed dimensions and associated openings as read-only inputs.
- Let the user select design intent such as style, palette, and finish preferences before generation.
- Generate clearly labeled visual concepts, show generation status/errors, and let the user compare, select, save, revisit, or remove concepts.
- Store the exact confirmation revision, room ID, generation provider/model, user choices, and generated asset reference with each saved concept.
- Generated images are illustrative, not measured or construction-ready. They must not mutate the source drawing, dimensions, finish quantities, material requirements, or estimate.
- Respect account ownership, provider quotas, asset-size limits, retention/deletion policy, and user consent for sending drawing/context to an external provider.

Acceptance criteria:

- No generation request is possible before a valid confirmation revision exists.
- The selected room and its confirmed revision are visible in the result and remain traceable after a later confirmation.
- Users can compare at least two concepts and save one without changing any QS or material totals.
- Provider failure, timeout, or quota exhaustion leaves existing designs and estimates unchanged.
- A user can only access design assets belonging to their account.

### US3: Conceptual exterior and roof options

As a user, I can view a conceptual exterior and switch between supported roof styles to compare the house's appearance.

Requirements:

- Provide a separate exterior view based on available confirmed layout and explicit exterior assumptions.
- Support a reviewed initial set of roof forms (flat, gable, hip, and shed); expose pitch, overhang, and material only when the model can represent them honestly.
- Clearly identify inferred or missing exterior geometry. Do not invent verified elevations, structural properties, load capacity, or code compliance.
- Let users switch roof styles and save/revisit a preferred visual option without changing measurements or estimates.
- Keep the view usable on desktop and mobile, with a non-WebGL fallback that preserves the selected style and project details.

Acceptance criteria:

- Roof-style changes update only the conceptual exterior view.
- Unknown dimensions and unmodeled building levels are labeled; unsupported geometry is not presented as measured fact.
- Saved options retain their source confirmation revision and can be reopened after refresh.
- Existing floor-plan, estimate, and material exports remain byte-for-byte behaviorally unchanged.

## Delivery Order

1. Implement Supabase email/password + Google auth and verify its access tokens at the Go API.
2. Add project ownership and scope every project, drawing, analysis, estimate, material budget, comparison, and export API; bootstrap existing projects to an initial admin user.
3. Add account-aware browser navigation and project isolation; test cross-account access denial and migration behavior before exposing signup.
4. Add OpenAI Images generation from confirmed room dimensions/openings and user style choices only; do not send source drawings. Add saved, revision-bound concepts and provider limits.
5. Implement the conceptual exterior model and roof-style controls from confirmed layout data, with assumptions and fallback behavior.
6. Run API, database migration, browser, mobile, privacy, and regression checks before release.

## Recorded Decisions

- Identity: Supabase Auth with email/password and Google OAuth.
- Verification/reset mail: Supabase Auth email delivery initially; production custom SMTP/domain configuration remains an environment/deployment prerequisite.
- Interior concepts: OpenAI Images using confirmed room data only; raw floor-plan/image uploads are prohibited.
- Existing project migration: assign ownerless projects to an explicitly configured initial admin account, never to all users.
- Exterior: Pro/Studio conceptual single-storey rectangular envelope with reviewed, editable visualization assumptions. Flat/gable/hip/shed forms, 5-60 degree supported pitched roofs (flat uses schematic zero), 0-2 m overhang and appearance-only finishes; not a structural or code-compliance model.
- Secrets such as Supabase keys and OpenAI API keys must be configured outside source control and must never be requested in chat.

## Current Architecture Notes

- The browser now gates the workspace on a Supabase session when configured and forwards bearer tokens with API calls. Anonymous mode requires explicit `AUTH_ALLOW_ANONYMOUS=true`.
- PostgreSQL projects have an owner column, owner-filtered operations, and an initial-admin adoption migration.
- Confirmed rooms/openings and source revisions already exist and can anchor the design snapshot.
- The room model remains conceptual. The separate Three.js exterior view visualizes only a reviewed rectangular envelope and roof; actual elevations, openings, extra levels and structural properties are not inferred as facts.
- `api/internal/auth` verifies bearer tokens against Supabase Auth and adds the verified user to request context. The server scopes project-linked API resources to the verified owner and migrates legacy `local` projects to `INITIAL_ADMIN_USER_ID` before serving requests.
- Local/isolated Compose explicitly opts into anonymous development mode. The web client supports email/password, Google OAuth, verification, password reset/recovery, sign-out, and bearer forwarding. Real Supabase credentials, Google provider setup, SMTP configuration, and live provider smoke tests remain deployment prerequisites.
- The room Design Studio uses the latest confirmed room snapshot or a selected prior confirmation revision. It generates two concepts using confirmed text dimensions/openings only, stores PNGs with model/style/palette and revision provenance, and exposes a per-user daily limit. NVIDIA generation is live-verified locally; the OpenAI and optional local SDXL adapters remain separate provider options. Per-image load/decode errors now have retry controls and authenticated PNG download links. Room dimensions are rounded for display only.
- Exterior save/read endpoints verify project ownership, source analysis and confirmation revision; paid 3D access is enforced server-side. PostgreSQL stores immutable exterior options separately from financial snapshots, and a disposable API restart test verifies reopening the saved source-bound option.

## Release Verification Boundary

- Local browser coverage includes authenticated legacy workflows, plan gates, all four exterior forms, desktop/mobile screenshots, nonblank canvas pixels/orbit interaction, image retry/download, OAuth request construction and reset redirects. These mocks do not prove external OAuth completion or mail delivery.
- Real Poppler preparation passes under a 512 MiB, 128-process, one-CPU container cap with non-root execution, dropped capabilities and no-new-privileges. This is a fixture workload check, not certification of hosted cold starts or all real-image/concurrent workloads.
- Production dependency audit is clean. The development ESLint tree still reports five high findings from the unpatched braces stack-exhaustion advisory GHSA-vfj7-8cjw-p6xm; Next.js/ESLint config versions are aligned at 16.4.2. No forced framework downgrade or advisory suppression is used.
- At the user's request, hosted checks and live Google/reset-mail checks remain pending. A deployed web/API URL, designated test identity and interactive mailbox/Google access are required; no account credentials are requested in chat.