import { expect, test, type Page } from "@playwright/test";

const features = ["aiAnalysis", "model3d", "materials", "exports"];
const catalog = [
  { id: "free", name: "Free", priceLabel: "Free", description: "Explore sample estimates.", features: [] },
  { id: "pro", name: "Pro", priceLabel: "Coming soon", description: "Analyze drawings and prepare material budgets and exports.", features },
  { id: "studio", name: "Studio", priceLabel: "Coming soon", description: "Everything in Pro, plus room interior concepts.", features: [...features, "designStudio"] },
];
const project = { id: "plan-project", name: "Plan fixture", createdAt: "2026-10-21T00:00:00Z", status: "Confirmed" };
const room = { id: "room-1", name: "Living room", length: { value: 4, unit: "m" }, width: { value: 3, unit: "m" }, wallHeight: { value: 2.7, unit: "m" } };
const candidate = { id: "analysis-1", floorPlanId: "plan-1", analysisState: "AnalysisReady", rooms: [room], doors: [], windows: [] };
const confirmation = { analysisId: candidate.id, revision: 1, confirmedAt: "2026-10-21T00:00:00Z", candidate };
const summary = { project, stage: "Confirmed", floorPlan: { id: "plan-1", fileName: "plan.png", fileSize: 10 }, analysisId: candidate.id, confirmedRevision: 1, roomCount: 1, openingCount: 0, estimateCount: 0, updatedAt: project.createdAt };
const session = { access_token: "plan-fixture-token", token_type: "bearer", expires_in: 3600, refresh_token: "plan-fixture-refresh", user: { id: "plan-user", email: "plans@example.test", aud: "authenticated", role: "authenticated", app_metadata: { provider: "email", providers: ["email"] }, user_metadata: {}, identities: [], created_at: project.createdAt } };

async function setup(page: Page, demo = true, unavailable = false) {
  let plan = catalog[0];
  let changes = 0;
  await page.emulateMedia({ colorScheme: "light" });
  await page.route("**/auth/v1/token**", (route) => route.fulfill({ json: session }));
  await page.route("**/api/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (path === "/api/plans") return route.fulfill({ json: catalog });
    if (path === "/api/account/plan") {
      if (unavailable) return route.fulfill({ status: 503, json: { message: "Plan unavailable" } });
      if (request.method() === "PUT") {
        if (!demo) return route.fulfill({ status: 403, json: { message: "Plan switching disabled" } });
        const selected = catalog.find((entry) => entry.id === request.postDataJSON().planId);
        if (!selected) return route.fulfill({ status: 400, json: { message: "Invalid plan" } });
        plan = selected; changes++;
      }
      return route.fulfill({ json: { plan, assignmentSource: "demo", demoSwitchingEnabled: demo } });
    }
    if (path === "/api/analysis-engines") return route.fulfill({ json: ["demo", "nvidia"] });
    if (path === "/api/projects") return route.fulfill({ json: [summary] });
    if (path === `/api/projects/${project.id}/summary`) return route.fulfill({ json: summary });
    if (path === `/api/projects/${project.id}/estimates`) return route.fulfill({ json: [] });
    if (path === `/api/analyses/${candidate.id}`) return route.fulfill({ json: candidate });
    if (path.endsWith("/confirmed-data")) return route.fulfill({ json: request.method() === "PUT" ? { ...request.postDataJSON(), confirmationRevision: 2 } : confirmation });
    if (path.endsWith("/confirmations")) return route.fulfill({ json: [confirmation] });
    if (path === "/api/room-design-capabilities") return route.fulfill({ json: { enabled: true, dailyImageLimit: 4, styles: ["Contemporary"], palettes: ["Warm neutrals"] } });
    if (path.endsWith("/designs") && request.method() === "GET") return route.fulfill({ json: [] });
    return route.fulfill({ status: 404, json: { message: "Fixture endpoint not provided" } });
  });
  await page.goto("/");
  await page.getByLabel("Email").fill(session.user.email);
  await page.getByLabel("Password", { exact: true }).fill("fixture-password-1");
  await page.getByRole("button", { name: "Sign in", exact: true }).last().click();
  await expect(page.getByRole("heading", { name: "Projects", exact: true })).toBeVisible();
  return { changes: () => changes };
}

async function openPlans(page: Page) {
  await page.locator(".header-nav").getByRole("button", { name: /^Plans/ }).click();
  await expect(page.getByRole("heading", { name: "Plans", exact: true })).toBeVisible();
}

test("demo plans persist and unlock only their included features", async ({ page }) => {
  const fixture = await setup(page);
  await page.getByRole("button", { name: project.name, exact: true }).click();
  await page.locator(".stepper").getByRole("button", { name: /Floor plan/ }).click();
  await expect(page.getByLabel(/NVIDIA Nemotron/)).toBeDisabled();
  await expect(page.getByLabel(/Demo/)).toBeEnabled();
  await openPlans(page);
  await expect(page.locator(".plan-option--current")).toContainText("Free");
  await expect(page.getByText("Coming soon", { exact: true })).toHaveCount(2);
  await page.screenshot({ path: "test-results/plans-light-desktop.png", fullPage: true });
  await page.getByRole("button", { name: "Switch to Pro (demo)", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("No payment was taken");
  await expect(page.locator(".plan-option--current")).toContainText("Pro");
  await page.getByRole("button", { name: "Back", exact: true }).click();
  await expect(page.getByLabel(/NVIDIA Nemotron/)).toBeEnabled();
  await expect(page.locator(".stepper").getByRole("button", { name: /3D model/ })).toBeEnabled();
  await page.getByRole("button", { name: "Design Studio", exact: true }).click();
  await expect(page.getByRole("button", { name: "Generate two concepts" })).toHaveCount(0);
  await expect(page.getByRole("region", { name: "Room interior generation access" })).toBeVisible();
  await openPlans(page);
  await page.getByRole("button", { name: "Switch to Studio (demo)", exact: true }).click();
  await expect(page.locator(".plan-option--current")).toContainText("Studio");
  await page.getByRole("button", { name: "Back", exact: true }).click();
  await expect(page.getByRole("button", { name: "Generate two concepts" })).toBeEnabled();
  await page.reload();
  await openPlans(page);
  await expect(page.locator(".plan-option--current")).toContainText("Studio");
  await page.getByRole("button", { name: "Switch to dark theme" }).click();
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  expect(await page.locator(".app-header").evaluate((header) => {
    const boxes = Array.from(header.children).map((element) => element.getBoundingClientRect()).filter((box) => box.width && box.height);
    return boxes.every((box, index) => boxes.slice(index + 1).every((other) => box.right <= other.left || other.right <= box.left || box.bottom <= other.top || other.bottom <= box.top));
  })).toBe(true);
  await page.screenshot({ path: "test-results/plans-dark-mobile.png", fullPage: true, animations: "disabled" });
  await page.getByRole("button", { name: "Switch to Free (demo)", exact: true }).click();
  await expect(page.locator(".plan-option--current")).toContainText("Free");
  expect(fixture.changes()).toBe(3);
  await page.getByRole("button", { name: "Back", exact: true }).click();
  await page.locator(".stepper").getByRole("button", { name: /Floor plan/ }).click();
  await expect(page.getByLabel(/NVIDIA Nemotron/)).toBeDisabled();
});

test("non-demo plans cannot be self-upgraded", async ({ page }) => {
  const fixture = await setup(page, false);
  await openPlans(page);
  await expect(page.getByText("Paid pricing is coming soon. Payments and self-service upgrades are not enabled.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Not available yet" })).toHaveCount(2);
  for (const button of await page.getByRole("button", { name: "Not available yet" }).all()) await expect(button).toBeDisabled();
  expect(fixture.changes()).toBe(0);
});

test("unavailable plan lookup keeps paid analysis locked", async ({ page }) => {
  await setup(page, false, true);
  await expect(page.locator(".plan-status")).toContainText("Paid features are temporarily unavailable");
  await page.getByRole("button", { name: project.name, exact: true }).click();
  await page.locator(".stepper").getByRole("button", { name: /Floor plan/ }).click();
  await expect(page.getByLabel(/NVIDIA Nemotron/)).toBeDisabled();
});

test("Free confirmation continues to a basic estimate without paid 3D", async ({ page }) => {
  await setup(page);
  await page.getByRole("button", { name: project.name, exact: true }).click();
  await page.locator(".stepper").getByRole("button", { name: /Review/ }).click();
  await expect(page.getByRole("button", { name: "Confirm and build model" })).toHaveCount(0);
  await page.getByRole("button", { name: "Confirm dimensions", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Estimate", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Calculate estimate", exact: true })).toBeEnabled();
  await expect(page.locator(".stepper").getByRole("button", { name: /3D model/ })).toBeDisabled();
});