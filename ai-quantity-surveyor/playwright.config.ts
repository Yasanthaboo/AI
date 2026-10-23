import { defineConfig, devices } from "@playwright/test";

const webPort = Number(process.env.PLAYWRIGHT_WEB_PORT || 3001);
const webURL = `http://localhost:${webPort}`;
const apiURL = process.env.PLAYWRIGHT_API_URL || "http://localhost:8080";

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  use: {
    baseURL: webURL,
    trace: "on-first-retry",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: [
    { command: `npm run dev -- --port ${webPort}`, url: webURL, env: { NEXT_PUBLIC_API_URL: apiURL }, reuseExistingServer: true, timeout: 120_000 },
    { command: "go run ./api/cmd/server", url: `${apiURL}/health`, reuseExistingServer: true, timeout: 120_000 },
  ],
});
