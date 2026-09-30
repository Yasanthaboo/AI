import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  use: {
    baseURL: "http://localhost:3001",
    trace: "on-first-retry",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: [
    { command: "npm run dev -- --port 3001", url: "http://localhost:3001", reuseExistingServer: true, timeout: 120_000 },
    { command: "go run ./api/cmd/server", url: "http://localhost:8080/health", reuseExistingServer: true, timeout: 120_000 },
  ],
});
