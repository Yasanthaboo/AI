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
  await page.goto("/");
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
  await page.route("**/auth/v1/recover**", async (route) => route.fulfill({ status: 200, json: {} }));
  await page.goto("/");
  await page.getByRole("button", { name: "Forgot password?" }).click();
  await page.getByLabel("Email").fill("designer@example.test");
  await page.getByRole("button", { name: "Send reset link" }).click();
  await expect(page.getByRole("status")).toContainText("If an account exists for that email");
});