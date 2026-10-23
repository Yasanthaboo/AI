import { expect, test } from "@playwright/test";

const authUser = {
  id: "auth-user-1",
  aud: "authenticated",
  role: "authenticated",
  email: "designer@example.test",
  app_metadata: { provider: "email", providers: ["email"] },
  user_metadata: {},
  identities: [],
  created_at: "2026-10-20T00:00:00.000Z",
};

const authSession = {
  access_token: "access-token-fixture",
  token_type: "bearer",
  expires_in: 3600,
  refresh_token: "refresh-token-fixture",
  user: authUser,
};

test("email signup presents verification guidance", async ({ page }) => {
  await page.route("**/auth/v1/signup**", async (route) => route.fulfill({ status: 200, json: authUser }));
  const response = await page.goto("/");
  expect(response?.headers()["x-content-type-options"]).toBe("nosniff");
  expect(response?.headers()["x-frame-options"]).toBe("DENY");
  expect(response?.headers()["content-security-policy"]).toContain("frame-ancestors 'none'");
  await page.getByRole("tab", { name: "Create account" }).click();
  await page.getByLabel("Email").fill("designer@example.test");
  await page.getByLabel("Password", { exact: true }).fill("secure-password-1");
  await page.getByLabel("Confirm password").fill("secure-password-1");
  await page.getByRole("button", { name: "Create account", exact: true }).last().click();
  await expect(page.getByRole("status")).toContainText("Check your email for a verification link");
});

test("signed-in requests carry the access token and sign-out clears the workspace", async ({ page }) => {
  let forwardedAuthorization = "";
  await page.route("**/auth/v1/token**", async (route) => route.fulfill({ status: 200, json: authSession }));
  await page.route("**/auth/v1/logout**", async (route) => route.fulfill({ status: 204, body: "" }));
  await page.route("**/api/**", async (route) => {
    forwardedAuthorization = route.request().headers().authorization ?? "";
    const path = new URL(route.request().url()).pathname;
    if (path === "/api/analysis-engines") return route.fulfill({ json: ["demo"] });
    if (path === "/api/projects") return route.fulfill({ json: [] });
    return route.fulfill({ status: 404, json: { errorCode: "NotFound", message: "Not found" } });
  });

  await page.goto("/");
  await page.getByLabel("Email").fill(authUser.email);
  await page.getByLabel("Password", { exact: true }).fill("secure-password-1");
  await page.getByRole("button", { name: "Sign in", exact: true }).last().click();
  await expect(page.getByRole("heading", { name: "Projects" })).toBeVisible();
  await expect(page.getByText(authUser.email)).toBeVisible();
  expect(forwardedAuthorization).toBe("Bearer access-token-fixture");

  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Projects" })).toHaveCount(0);
});

test("password reset uses non-enumerating confirmation copy", async ({ page }) => {
  let redirect = "";
  await page.route("**/auth/v1/recover**", async (route) => { redirect = new URL(route.request().url()).searchParams.get("redirect_to") ?? ""; await route.fulfill({ status: 200, json: {} }); });
  await page.goto("/");
  await page.getByRole("button", { name: "Forgot password?" }).click();
  await page.getByLabel("Email").fill("designer@example.test");
  await page.getByRole("button", { name: "Send reset link" }).click();
  await expect(page.getByRole("status")).toContainText("If an account exists for that email");
  expect(redirect).toBe(new URL(page.url()).origin);
});

test("Google sign-in hands off with the approved web redirect", async ({ page }) => {
  await page.route("**/auth/v1/authorize**", (route) => route.fulfill({ status: 200, contentType: "text/plain", body: "OAuth handoff fixture" }));
  await page.goto("/");
  const origin = new URL(page.url()).origin;
  const authorization = page.waitForRequest((request) => new URL(request.url()).pathname === "/auth/v1/authorize");
  await page.getByRole("button", { name: "Continue with Google", exact: true }).click();
  const target = new URL((await authorization).url());
  expect(target.searchParams.get("provider")).toBe("google");
  expect(target.searchParams.get("redirect_to")).toBe(origin);
});

test("production web requires complete runtime authentication configuration", async ({ page }) => {
  const missing = process.env.PLAYWRIGHT_AUTH_MISSING_URL;
  const partial = process.env.PLAYWRIGHT_AUTH_PARTIAL_URL;
  const configured = process.env.PLAYWRIGHT_AUTH_CONFIGURED_URL;
  test.skip(!missing || !partial || !configured, "Requires isolated production web containers for auth configuration checks.");
  for (const target of [missing!, partial!, configured!]) {
    const url = new URL(target);
    expect(url.hostname).toBe("127.0.0.1");
    expect(Number(url.port)).toBeGreaterThanOrEqual(18000);
  }
  for (const target of [missing!, partial!]) {
    await page.goto(target);
    await expect(page.getByRole("heading", { name: "Sign-in is not configured", exact: true })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Projects", exact: true })).toHaveCount(0);
    await expect(page.getByRole("button", { name: "New project" })).toHaveCount(0);
  }
  await page.goto(configured!);
  await expect(page.getByRole("heading", { name: "Sign in", exact: true })).toBeVisible();
  await expect(page.getByLabel("Email", { exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Projects", exact: true })).toHaveCount(0);
});