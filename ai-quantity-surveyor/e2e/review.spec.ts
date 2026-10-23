import { expect, test, type Page } from "@playwright/test";
import type { Candidate } from "../src/app/api";

const meters = (value: number) => ({ value, unit: "m" });
const project = { id: "review-project", name: "Dimension review sample", createdAt: "2026-10-23T00:00:00Z", status: "AnalysisReady" };
const user = { id: "review-user", aud: "authenticated", role: "authenticated", email: "review@example.test", app_metadata: { provider: "email", providers: ["email"] }, user_metadata: {}, identities: [], created_at: project.createdAt };

async function setup(page: Page) {
  const candidate: Candidate = {
    id: "review-analysis", floorPlanId: "review-plan", analysisState: "AnalysisReady", engine: "demo",
    rooms: [
      { id: "living", name: "Living room", length: meters(6), width: meters(4), wallHeight: meters(2.7), confidence: 0.7, uncertainty: "Check length" },
      { id: "bedroom", name: "Bedroom", length: null, width: meters(3), wallHeight: null, confidence: 0.6, uncertainty: "Missing length" },
    ],
    doors: [{ id: "door", roomId: "living", width: null, height: meters(2), confidence: 0.8 }],
    windows: [],
    features: [{ id: "stair", type: "stair", label: "Staircase", disposition: "pending", origin: "engine" }],
  };
  const summary = { project, stage: "Analyzed", floorPlan: { id: "review-plan", fileName: "plan.png", fileSize: 1 }, analysisId: candidate.id, roomCount: 2, openingCount: 1, estimateCount: 0, updatedAt: project.createdAt };
  let confirmed: Candidate | null = null;
  await page.route("**/auth/v1/token**", (route) => route.fulfill({ json: { access_token: "review-fixture-token", token_type: "bearer", expires_in: 3600, refresh_token: "review-fixture-refresh", user } }));
  await page.route("**/api/**", (route) => {
    const request = route.request(), path = new URL(request.url()).pathname;
    if (path === "/api/account/plan") return route.fulfill({ json: { plan: { id: "pro", name: "Pro", features: ["model3d", "aiAnalysis"] }, assignmentSource: "manual", demoSwitchingEnabled: false } });
    if (path === "/api/analysis-engines") return route.fulfill({ json: ["demo"] });
    if (path === "/api/projects") return route.fulfill({ json: [summary] });
    if (path.endsWith("/summary")) return route.fulfill({ json: summary });
    if (path === `/api/analyses/${candidate.id}`) return route.fulfill({ json: candidate });
    if (path.endsWith("/estimates")) return route.fulfill({ json: [] });
    if (path.endsWith("/confirmed-data") && request.method() === "PUT") {
      confirmed = request.postDataJSON() as Candidate;
      return route.fulfill({ json: { ...confirmed, confirmationRevision: 1 } });
    }
    return route.fulfill({ status: 404, json: { message: "Fixture endpoint unavailable" } });
  });
  await page.goto("/");
  await page.getByLabel("Email").fill(user.email);
  await page.getByLabel("Password", { exact: true }).fill("fixture-password-1");
  await page.getByRole("button", { name: "Sign in", exact: true }).last().click();
  await page.getByRole("button", { name: project.name, exact: true }).click();
  await page.getByRole("button", { name: "Continue: Review", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Review the extraction" })).toBeVisible();
  return { candidate, confirmed: () => confirmed };
}

test("bulk room review requires complete dimensions and saves only on explicit confirmation", async ({ page }) => {
  const fixture = await setup(page);
  const bulk = page.getByRole("button", { name: "All looks right", exact: true });
  const confirm = page.getByRole("button", { name: "Confirm and build model", exact: true });
  await expect(page.getByRole("button", { name: "Looks right", exact: true })).toHaveCount(2);
  await expect(bulk).toBeDisabled();
  await page.getByLabel("Bedroom length", { exact: true }).fill("4.5");
  await page.getByLabel("Living room width", { exact: true }).fill("0");
  await expect(bulk).toBeDisabled();
  await page.getByLabel("Living room width", { exact: true }).fill("4");
  await page.getByLabel("Bedroom wall height", { exact: true }).fill("");
  await expect(bulk).toBeDisabled();
  await page.getByLabel("Bedroom wall height", { exact: true }).fill("2.8");
  await page.getByRole("textbox", { name: "Room name", exact: true }).last().fill(" ");
  await expect(bulk).toBeDisabled();
  await page.getByRole("textbox", { name: "Room name", exact: true }).last().fill("Bedroom");
  await expect(bulk).toBeEnabled();
  await bulk.click();
  await expect(page.getByRole("button", { name: "Looks right", exact: true })).toHaveCount(0);
  await expect(page.locator("#room-review-help")).toContainText("No room flags remain");
  await expect(bulk).toBeDisabled();
  await expect(confirm).toBeDisabled();
  expect(fixture.confirmed()).toBeNull();
  expect(fixture.candidate.rooms![1].length).toBeNull();
  await page.getByRole("spinbutton", { name: "Door width", exact: true }).fill("0.9");
  await expect(confirm).toBeDisabled();
  await expect(page.getByLabel("stair disposition")).toHaveValue("pending");
  await page.getByLabel("stair disposition").selectOption("acknowledged");
  await expect(confirm).toBeEnabled();
  await confirm.click();
  await expect(page.getByRole("heading", { name: "3D model", exact: true })).toBeVisible();
  expect(fixture.confirmed()?.rooms).toMatchObject([
    { id: "living", length: meters(6), width: meters(4), confidence: 1 },
    { id: "bedroom", length: meters(4.5), wallHeight: meters(2.8), confidence: 1 },
  ]);
  expect(fixture.confirmed()?.doors?.[0].width).toEqual(meters(0.9));
  expect(fixture.confirmed()?.features?.[0].disposition).toBe("acknowledged");
});

test("individual room review remains available alongside bulk review on mobile", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const fixture = await setup(page);
  await page.getByRole("button", { name: "Looks right", exact: true }).first().click();
  await expect(page.getByRole("button", { name: "Looks right", exact: true })).toHaveCount(1);
  await expect(page.getByRole("button", { name: "All looks right", exact: true })).toBeDisabled();
  await page.getByLabel("Bedroom length", { exact: true }).fill("4");
  await page.getByRole("button", { name: "All looks right", exact: true }).click();
  await expect(page.locator(".row--flagged")).toHaveCount(0);
  expect(fixture.confirmed()).toBeNull();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});
