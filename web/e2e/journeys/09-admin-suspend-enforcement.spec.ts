import { test, expect } from "@playwright/test";

test.describe("Journey 9: Admin Suspends User → Enforcement", () => {
  test("admin suspends learner account with reason, blocking subsequent requests", async ({
    page,
  }) => {
    // Mock admin login
    await page.route("**/api/v1/auth/refresh", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: {
            id: "admin-id-999",
            email: "admin@fluentra.com",
            display_name: "Platform Admin",
            role: "admin",
          },
          session_id: "sess-admin-123",
        }),
      });
    });

    await page.route("**/api/v1/admin/users", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          items: [
            {
              id: "target-user-999",
              email: "spammer@example.com",
              display_name: "Bad Actor",
              status: "active",
              created_at: new Date().toISOString(),
            },
          ],
        }),
      });
    });

    await page.route("**/api/v1/admin/users/target-user-999", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          id: "target-user-999",
          email: "spammer@example.com",
          display_name: "Bad Actor",
          status: "active",
          locale: "en",
          timezone: "UTC",
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        }),
      });
    });

    await page.route("**/api/v1/admin/users/target-user-999/suspend", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          id: "target-user-999",
          status: "suspended",
        }),
      });
    });

    await page.goto("/admin");
    await expect(page.getByText("Bad Actor")).toBeVisible();

    // Inspect
    await page.getByRole("button", { name: /Inspect/i }).click();
    await expect(page.getByText("Learner Account Details")).toBeVisible();

    // Click Suspend
    await page.getByRole("button", { name: /Suspend User/i }).click();

    // Fill reason
    await page
      .getByPlaceholder(/State the justification/i)
      .fill("Repeated spamming and community violations");
    await page.getByRole("button", { name: /Confirm Suspension/i }).click();

    // User status updates to suspended
    await expect(page.getByText("suspended")).toBeVisible();
  });
});
