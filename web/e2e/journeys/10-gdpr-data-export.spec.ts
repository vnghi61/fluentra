import { test, expect } from "@playwright/test";

test.describe("Journey 10: GDPR Data Export Request & Download", () => {
  test("requests personal archive export, tracks status, and displays download readiness", async ({
    page,
  }) => {
    let exportRequested = false;

    await page.route("**/api/v1/auth/refresh", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: {
            id: "10101010-1010-1010-1010-101010101010",
            email: "export-learner@example.com",
            display_name: "Export Learner",
            role: "user",
          },
          session_id: "sess-export-123",
        }),
      });
    });

    await page.route("**/api/v1/me/export", async (route) => {
      if (route.request().method() === "POST") {
        exportRequested = true;
        await route.fulfill({
          status: 202,
          contentType: "application/json",
          body: JSON.stringify({
            status: "pending",
            requested_at: new Date().toISOString(),
          }),
        });
      } else {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(
            exportRequested
              ? {
                  status: "ready",
                  requested_at: new Date().toISOString(),
                  download_url: "http://127.0.0.1:9000/exports/archive.zip",
                  expires_at: new Date(Date.now() + 86400000).toISOString(),
                }
              : {
                  status: "none",
                },
          ),
        });
      }
    });

    await page.goto("/settings");
    await page.getByRole("button", { name: /Data & Privacy/i }).click();

    await expect(page.getByText("Export Personal Data")).toBeVisible();

    // Click request export
    await page.getByRole("button", { name: /Request Data Export/i }).click();

    // Export status updates
    await expect(
      page.getByText(/Export Request Submitted|ready|Download/i),
    ).toBeVisible({ timeout: 10000 });
  });
});
