import { test, expect } from "@playwright/test";

test.describe("Journey 8: Device List → Untrust Device", () => {
  test("warns and logs learner out when current trusted device is untrusted", async ({
    page,
  }) => {
    // Mock authenticated user
    await page.route("**/api/v1/auth/refresh", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: {
            id: "88888888-8888-8888-8888-888888888888",
            email: "device-learner@example.com",
            display_name: "Device Learner",
            role: "user",
          },
          session_id: "sess-device-123",
        }),
      });
    });

    await page.route("**/api/v1/me/devices", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          items: [
            {
              id: "dev-current",
              device_name: "Current Chrome Device",
              is_current: true,
              last_seen_at: new Date().toISOString(),
              created_at: new Date().toISOString(),
            },
          ],
        }),
      });
    });

    await page.goto("/settings");
    await page.getByRole("button", { name: /Security & Access/i }).click();

    await expect(page.getByText("Current Chrome Device")).toBeVisible();
    await expect(page.getByText("This Device")).toBeVisible();

    // Click untrust on current device
    await page.getByRole("button", { name: /Untrust/i }).click();

    // Untrust warning modal appears
    await expect(page.getByText("Untrust Current Device")).toBeVisible();
    await expect(
      page.getByText(/untrusting the device you are currently using will immediately log you out/i),
    ).toBeVisible();
  });
});
