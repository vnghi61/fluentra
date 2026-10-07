import React from "react";
import { describe, it, expect, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { server } from "./msw-server";
import i18n, { initI18n, loadLocale } from "@/i18n";
import { AdminUserList } from "@/features/admin/components/AdminUserList";
import { AdminFeatureFlags } from "@/features/admin/components/AdminFeatureFlags";
import { AdminPage } from "@/pages/AdminPage";

const sampleUser1 = {
  id: "11111111-1111-1111-1111-111111111111",
  email: "learner1@example.com",
  display_name: "Learner One",
  avatar_url: null,
  status: "active",
  created_at: "2026-08-01T10:00:00Z",
};

const sampleUser2 = {
  id: "22222222-2222-2222-2222-222222222222",
  email: "learner2@example.com",
  display_name: "Learner Two",
  avatar_url: null,
  status: "suspended",
  created_at: "2026-08-05T10:00:00Z",
};

const sampleUser3 = {
  id: "33333333-3333-3333-3333-333333333333",
  email: "learner3@example.com",
  display_name: "Learner Three",
  avatar_url: null,
  status: "active",
  created_at: "2026-08-07T10:00:00Z",
};

const sampleUserDetail = {
  id: "11111111-1111-1111-1111-111111111111",
  email: "learner1@example.com",
  display_name: "Learner One",
  avatar_url: null,
  locale: "vi-VN",
  timezone: "Asia/Ho_Chi_Minh",
  status: "active",
  created_at: "2026-08-01T10:00:00Z",
  updated_at: "2026-08-10T12:00:00Z",
};

const sampleFlag = {
  key: "streaks_v2",
  description: "Second-generation streak calculation",
  enabled: true,
  rollout_percent: 25,
  owner: "@backend-team",
  expires_on: "2026-12-31",
  created_at: "2026-08-01T10:00:00Z",
  updated_at: "2026-08-01T10:00:00Z",
};

/**
 * AdminPage reads which section to show from the path, so it needs a router.
 * Both routes are mounted because the page serves both: /admin picks the first
 * section the permissions allow, /admin/$section names one.
 */
async function renderAdminPage(
  initialPath = "/admin",
  ui?: React.ReactElement,
) {
  const rootRoute = createRootRoute();
  const page = () => ui ?? <AdminPage />;
  const adminIndexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/admin",
    component: page,
  });
  const adminSectionRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/admin/$section",
    component: page,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([adminIndexRoute, adminSectionRoute]),
    history: createMemoryHistory({ initialEntries: [initialPath] }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

/** Opens the actions menu of one learner's row in the desktop table. */
async function openRowMenu(
  user: ReturnType<typeof userEvent.setup>,
  name = "Learner One",
) {
  const table = await screen.findByTestId("learner-table");
  await user.click(
    within(table).getByRole("button", { name: `Actions for ${name}` }),
  );
}

describe("Admin Shell & Operations", () => {
  beforeEach(async () => {
    await initI18n("en");
    await loadLocale("en");
    await i18n.changeLanguage("en");

    // Default mock handlers for admin endpoints
    server.use(
      // The admin screens ask what the caller may do before rendering any of
      // it, so every admin test needs this. An administrator holding the full
      // set is the case these tests are about; the gating itself is covered in
      // its own test below.
      http.get("/api/v1/me/permissions", () =>
        HttpResponse.json({
          roles: ["admin"],
          permissions: [
            "user.list",
            "user.read",
            "user.suspend",
            "user.reinstate",
            "user.manage_sessions",
            "user.delete",
            "system.flags",
            "admin.dashboard",
          ],
        }),
      ),
      http.get("/api/v1/admin/users", ({ request }) => {
        const url = new URL(request.url);
        const cursor = url.searchParams.get("cursor");

        if (cursor === "cursor_page_2") {
          return HttpResponse.json({
            items: [sampleUser2],
            next_cursor: undefined, // End of pages
          });
        }

        return HttpResponse.json({
          items: [sampleUser1],
          next_cursor: "cursor_page_2",
        });
      }),

      http.get("/api/v1/admin/users/:id", () => {
        return HttpResponse.json(sampleUserDetail);
      }),

      http.post("/api/v1/admin/users/:id/suspend", async ({ request }) => {
        const body = (await request.json()) as { reason: string };
        if (!body.reason || body.reason.trim().length < 10) {
          return HttpResponse.json(
            {
              code: "VALIDATION_FAILED",
              message: "Reason too short",
              status: 422,
            },
            { status: 422 },
          );
        }
        return HttpResponse.json({
          id: sampleUserDetail.id,
          status: "suspended",
        });
      }),

      http.post("/api/v1/admin/users/:id/reinstate", () => {
        return HttpResponse.json({
          id: sampleUserDetail.id,
          status: "active",
        });
      }),

      http.post("/api/v1/admin/users/:id/sessions/revoke", () => {
        return HttpResponse.json({
          id: sampleUserDetail.id,
          revoked: true,
        });
      }),

      http.get("/api/v1/admin/flags", () => {
        return HttpResponse.json({
          items: [sampleFlag],
        });
      }),

      http.post("/api/v1/admin/flags", async ({ request }) => {
        const body = (await request.json()) as typeof sampleFlag;
        return HttpResponse.json(
          {
            ...body,
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          },
          { status: 201 },
        );
      }),

      http.patch("/api/v1/admin/flags/:key", async ({ request }) => {
        const body = (await request.json()) as Partial<typeof sampleFlag>;
        return HttpResponse.json({
          ...sampleFlag,
          ...body,
          updated_at: new Date().toISOString(),
        });
      }),

      http.delete("/api/v1/admin/flags/:key", () => {
        return new HttpResponse(null, { status: 204 });
      }),
    );
  });

  it("renders Admin page and switches tabs", async () => {
    const user = userEvent.setup();
    await renderAdminPage();

    // Awaited: the tabs are built from /me/permissions, so nothing
    // administrative renders until that read lands. Rendering the tabs first
    // and hiding them afterwards would flash actions the caller may not have.
    expect(
      await screen.findByRole("button", { name: /Learner Management/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Learner Management" }),
    ).toBeInTheDocument();
    // Below md the sections live behind the section picker; the desktop
    // sidebar is drawn by the shell, which this page test does not mount.
    await user.click(
      screen.getByRole("button", { name: /Section:\s*Learner Management/i }),
    );
    await user.click(
      within(screen.getByRole("dialog")).getByRole("button", {
        name: "Feature Flags",
      }),
    );
    await waitFor(() => {
      expect(screen.getByText("Feature Flags & Rollouts")).toBeInTheDocument();
    });
  });

  it("loads learners list with cursor-based pagination walk", async () => {
    const user = userEvent.setup();
    render(<AdminUserList />);

    // Initial page shows learner 1
    await screen.findAllByText("Learner One");
    expect(screen.getAllByText("learner1@example.com").length).toBeGreaterThan(
      0,
    );

    const nextBtn = screen.getByRole("button", { name: /Next/i });
    const prevBtn = screen.getByRole("button", { name: /Previous/i });

    expect(prevBtn).toBeDisabled();
    expect(nextBtn).toBeEnabled();

    // Click Next -> loads page 2 with learner 2
    await user.click(nextBtn);

    await screen.findAllByText("Learner Two");
    expect(screen.getAllByText("learner2@example.com").length).toBeGreaterThan(
      0,
    );

    // Next is now disabled because next_cursor is absent
    expect(nextBtn).toBeDisabled();
    expect(prevBtn).toBeEnabled();

    // Click Previous -> loads back learner 1
    await user.click(prevBtn);
    await screen.findAllByText("Learner One");
  });

  it("renders the learner in both the table and the mobile cards, name before id", async () => {
    render(<AdminUserList />);
    await screen.findAllByText("Learner One");

    const table = screen.getByTestId("learner-table");
    const cards = screen.getByTestId("learner-cards");
    expect(table).toHaveClass("hidden", "md:block");
    expect(cards).toHaveClass("md:hidden");
    expect(within(table).getByText("Learner One")).toBeInTheDocument();
    expect(within(cards).getByText("Learner One")).toBeInTheDocument();
    expect(within(cards).getByText("learner1@example.com")).toBeInTheDocument();
    expect(table.querySelector("tbody tr")).toHaveClass("h-14");

    const card = within(cards).getAllByRole("listitem")[0]!;
    const text = card.textContent ?? "";
    expect(text.indexOf("Learner One")).toBeGreaterThanOrEqual(0);
    expect(text.indexOf("Learner One")).toBeLessThan(text.indexOf("11111111…"));
  });

  it("opens mobile learner actions and applies the status filter sheet", async () => {
    const user = userEvent.setup();
    server.use(
      http.get("/api/v1/admin/users", ({ request }) => {
        const status = new URL(request.url).searchParams.get("status");
        return HttpResponse.json({
          items: status === "suspended" ? [sampleUser2] : [sampleUser1],
          next_cursor: undefined,
        });
      }),
    );
    render(<AdminUserList />);
    await screen.findAllByText("Learner One");
    const cards = screen.getByTestId("learner-cards");
    await user.click(
      within(cards).getByRole("button", { name: /Actions for Learner One/i }),
    );
    expect(
      screen.getByRole("menuitem", { name: /Lock login/i }),
    ).toBeInTheDocument();
    await user.keyboard("{Escape}");
    await user.click(screen.getByRole("button", { name: "Filters" }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    await user.selectOptions(
      screen.getByRole("combobox", { name: /Filter by status/i }),
      "suspended",
    );
    await user.click(screen.getByRole("button", { name: "Apply" }));
    await screen.findAllByText("Learner Two");
    expect(screen.queryByText("Learner One")).not.toBeInTheDocument();
  });

  it("offers no soft delete on the active row menu without user.delete, and keeps suspend", async () => {
    server.use(
      http.get("/api/v1/me/permissions", () =>
        HttpResponse.json({
          roles: ["admin"],
          permissions: ["admin.dashboard", "user.list", "user.suspend"],
        }),
      ),
    );
    render(<AdminUserList />);
    await openRowMenu(userEvent.setup());
    expect(
      screen.getByRole("menuitem", { name: /Lock login/i }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: /Soft-delete/i }),
    ).not.toBeInTheDocument();
  });

  it("shows a skeleton, not a page spinner, while loading", async () => {
    render(<AdminUserList />);
    const skeleton = screen.getByTestId("learner-skeleton");
    expect(skeleton).toHaveAttribute("aria-busy", "true");
    expect(skeleton.querySelector(".animate-spin")).toBeNull();
    await screen.findAllByText("Learner One");
    expect(screen.queryByTestId("learner-skeleton")).toBeNull();
  });

  it("retries the learners request from the error state", async () => {
    let calls = 0;
    server.use(
      http.get("/api/v1/admin/users", () => {
        calls += 1;
        if (calls === 1) {
          return HttpResponse.json(
            { code: "INTERNAL", message: "boom", status: 500 },
            { status: 500 },
          );
        }
        return HttpResponse.json({
          items: [sampleUser1],
          total: 1,
          next_cursor: undefined,
        });
      }),
    );
    const user = userEvent.setup();
    render(<AdminUserList />);

    const alert = await screen.findByRole("alert");
    await user.click(within(alert).getByRole("button", { name: /Try again/i }));

    await screen.findAllByText("Learner One");
    expect(calls).toBe(2);
  });

  it("inspects learner details with audit notice", async () => {
    const user = userEvent.setup();
    render(<AdminUserList />);

    await screen.findAllByText("Learner One");

    await openRowMenu(user);
    await user.click(
      screen.getByRole("menuitem", { name: /Inspect account/i }),
    );

    await waitFor(() => {
      expect(screen.getByText("Learner Account Details")).toBeInTheDocument();
      expect(screen.getByText(/admin\.user_viewed/i)).toBeInTheDocument();
      expect(screen.getByText("vi-VN")).toBeInTheDocument();
      expect(screen.getByText("Asia/Ho_Chi_Minh")).toBeInTheDocument();
    });
  });

  it("locks a login straight from the row, and records why", async () => {
    let suspended: { id: string; reason: string } | null = null;
    server.use(
      http.post(
        "/api/v1/admin/users/:id/suspend",
        async ({ params, request }) => {
          const body = (await request.json()) as { reason: string };
          suspended = { id: String(params["id"]), reason: body.reason };
          return HttpResponse.json({ id: params["id"], status: "suspended" });
        },
      ),
    );

    const user = userEvent.setup();
    render(<AdminUserList />);

    await screen.findAllByText("Learner One");

    await openRowMenu(user);
    await user.click(screen.getByRole("menuitem", { name: /Lock login/i }));

    // The reason is not optional decoration: the server refuses under ten
    // characters, so the row action has to collect one before it can fire.
    const textarea = await screen.findByPlaceholderText(
      /State the justification/i,
    );
    await user.type(textarea, "Repeated spam reports confirmed by moderation");
    await user.click(screen.getByRole("button", { name: /Suspend User/i }));

    await waitFor(() => {
      expect(suspended).not.toBeNull();
    });
    expect(suspended!.id).toBe("11111111-1111-1111-1111-111111111111");
    expect(suspended!.reason.length).toBeGreaterThanOrEqual(10);
  });

  it("soft-deletes an account from the row, and says what that costs", async () => {
    let deleted: { id: string; reason: string } | null = null;
    server.use(
      http.post(
        "/api/v1/admin/users/:id/delete",
        async ({ params, request }) => {
          const body = (await request.json()) as { reason: string };
          deleted = { id: String(params["id"]), reason: body.reason };
          return HttpResponse.json({
            id: params["id"],
            status: "pending_deletion",
          });
        },
      ),
    );

    const user = userEvent.setup();
    render(<AdminUserList />);

    await screen.findAllByText("Learner One");

    await openRowMenu(user);
    await user.click(
      screen.getByRole("menuitem", { name: /Soft-delete account/i }),
    );

    // The administrator is told the two things that are easy to get wrong: the
    // sessions end now, and they cannot undo it afterwards.
    expect(
      await screen.findByText(/erased after 30 days unless its owner cancels/i),
    ).toBeInTheDocument();

    const textarea = screen.getByPlaceholderText(/State the justification/i);
    await user.type(textarea, "Account closure requested by the owner");
    // Scoped to the dialog: the row icon that opened it carries the same
    // accessible name, which is right for both and ambiguous for a global query.
    await user.click(
      within(screen.getByRole("dialog")).getByRole("button", {
        name: /Soft-delete account/i,
      }),
    );

    await waitFor(() => {
      expect(deleted).not.toBeNull();
    });
    expect(deleted!.id).toBe("11111111-1111-1111-1111-111111111111");
  });

  it("reports how many accounts match, not how many are on the page", async () => {
    server.use(
      http.get("/api/v1/admin/users", () =>
        HttpResponse.json({
          items: [sampleUser1],
          total: 1284,
          next_cursor: "cursor_page_2",
        }),
      ),
    );

    render(<AdminUserList />);

    // The footer counted the rows it was handed, so it read "1 learner" for a
    // search matching 1284 — the size of one page presented as the whole set.
    expect(await screen.findByText(/1284/)).toBeInTheDocument();
  });

  it("locks a batch under one justification, and refuses to invent one", async () => {
    const suspended: string[] = [];
    server.use(
      http.get("/api/v1/admin/users", () =>
        HttpResponse.json({
          items: [sampleUser1, sampleUser3],
          total: 2,
          next_cursor: undefined,
        }),
      ),
      http.post("/api/v1/admin/users/:id/suspend", ({ params }) => {
        suspended.push(String(params["id"]));
        return HttpResponse.json({ id: params["id"], status: "suspended" });
      }),
    );

    const user = userEvent.setup();
    render(<AdminUserList />);

    await screen.findAllByText("Learner One");

    // Re-queried each time: ticking a box re-renders the rows, so a node
    // captured before the first click is detached by the second.
    const picks = () =>
      within(screen.getByTestId("learner-table")).getAllByLabelText(
        /Select this account/i,
      );
    expect(picks()).toHaveLength(2);
    for (let i = 0; i < 2; i++) {
      await user.click(picks()[i]!);
    }

    await user.selectOptions(
      screen.getByLabelText(/Action for 2 selected/i),
      "suspend",
    );

    // The same sentence is recorded against every account in the batch, so the
    // modal asks for one before anything happens rather than after.
    const textarea = await screen.findByPlaceholderText(
      /State the justification/i,
    );
    await user.type(
      textarea,
      "Coordinated spam campaign from one address book",
    );
    await user.click(
      within(screen.getByRole("dialog")).getByRole("button", {
        name: /Lock login for the selected/i,
      }),
    );

    await waitFor(() => {
      expect(suspended).toHaveLength(2);
    });
    expect(suspended).toContain(sampleUser1.id);
    expect(suspended).toContain(sampleUser3.id);
  });

  it("names the accounts a batch could not change rather than counting them", async () => {
    server.use(
      http.get("/api/v1/admin/users", () =>
        HttpResponse.json({
          items: [sampleUser1, sampleUser3],
          total: 2,
          next_cursor: undefined,
        }),
      ),
      http.post("/api/v1/admin/users/:id/suspend", ({ params }) => {
        // One of the two refuses. A batch that half-applied and says only
        // "1 failed" leaves the administrator to find out which by running it
        // again — on the account that already succeeded.
        if (String(params["id"]) === sampleUser3.id) {
          return HttpResponse.json(
            { status: 422, code: "INVALID_STATE_TRANSITION" },
            { status: 422 },
          );
        }
        return HttpResponse.json({ id: params["id"], status: "suspended" });
      }),
    );

    const user = userEvent.setup();
    render(<AdminUserList />);

    await screen.findAllByText("Learner One");

    // Re-queried each time: ticking a box re-renders the rows, so a node
    // captured before the first click is detached by the second.
    const picks = () =>
      within(screen.getByTestId("learner-table")).getAllByLabelText(
        /Select this account/i,
      );
    expect(picks()).toHaveLength(2);
    for (let i = 0; i < 2; i++) {
      await user.click(picks()[i]!);
    }
    await user.selectOptions(
      screen.getByLabelText(/Action for 2 selected/i),
      "suspend",
    );
    await user.type(
      await screen.findByPlaceholderText(/State the justification/i),
      "Coordinated spam campaign from one address book",
    );
    await user.click(
      within(screen.getByRole("dialog")).getByRole("button", {
        name: /Lock login for the selected/i,
      }),
    );

    // The banner names the one that failed. "Learner Three" is also its own
    // table row, so the assertion is on the list item the banner renders.
    expect(await screen.findByText(/1 of 2 succeeded/i)).toBeInTheDocument();
    const failedItems = screen
      .getAllByRole("listitem")
      .map((li) => li.textContent);
    expect(failedItems).toContain("Learner Three");
    expect(failedItems).not.toContain("Learner One");
  });

  it("offers no soft delete to an administrator without user.delete", async () => {
    server.use(
      http.get("/api/v1/me/permissions", () =>
        HttpResponse.json({
          roles: ["admin"],
          permissions: [
            "admin.dashboard",
            "user.list",
            "user.read",
            "user.suspend",
          ],
        }),
      ),
    );

    render(<AdminUserList />);

    await screen.findAllByText("Learner One");

    // Hiding it is half the control; the endpoint requires the permission too.
    // But a control that answers 403 is the fault this screen was written to
    // avoid, and soft deletion is the one where a 403 arrives after the
    // administrator has already typed a justification.
    await openRowMenu(userEvent.setup());
    expect(
      screen.queryByRole("menuitem", { name: /Soft-delete account/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.getAllByRole("menuitem", { name: /Lock login/i }).length,
    ).toBeGreaterThan(0);
  });

  it("offers a suspended account reinstatement, and never a second suspension", async () => {
    // sampleUser2 is suspended, and the default handler only serves it on the
    // second page. Serving it first keeps this about the row, not the paging.
    server.use(
      http.get("/api/v1/admin/users", () =>
        HttpResponse.json({ items: [sampleUser2], next_cursor: undefined }),
      ),
    );

    render(<AdminUserList />);

    await screen.findAllByText("Learner Two");

    await openRowMenu(userEvent.setup(), "Learner Two");

    // Exact names, not substrings: "Unlock login (reinstate)" contains "Lock
    // login", so a loose matcher here would pass for the wrong reason.
    expect(
      screen.getByRole("menuitem", { name: "Unlock login (reinstate)" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: "Lock login (suspend)" }),
    ).not.toBeInTheDocument();
  });

  it("enforces reason of at least 10 characters for administrative suspension", async () => {
    const user = userEvent.setup();
    render(<AdminUserList />);

    await screen.findAllByText("Learner One");

    await openRowMenu(user);
    await user.click(
      screen.getByRole("menuitem", { name: /Inspect account/i }),
    );

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /Suspend User/i }),
      ).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /Suspend User/i }));

    expect(screen.getByText("Suspend Account")).toBeInTheDocument();

    const textarea = screen.getByPlaceholderText(/State the justification/i);
    const confirmBtn = screen.getByRole("button", {
      name: /Confirm Suspension/i,
    });

    // Try too short reason (e.g. "spam")
    await user.type(textarea, "spam");
    await user.click(confirmBtn);

    await waitFor(() => {
      expect(
        screen.getByText(/Reason must be at least 10 characters/i),
      ).toBeInTheDocument();
    });

    // Type valid reason >= 10 chars
    await user.clear(textarea);
    await user.type(textarea, "Repeated community guidelines violation");
    await user.click(confirmBtn);

    // Modal closes and user becomes suspended
    await waitFor(() => {
      expect(screen.queryByText("Suspend Account")).not.toBeInTheDocument();
    });
  });

  it("handles 403 SELF_ADMIN_ACTION_FORBIDDEN with plain user explanation", async () => {
    // Override handler to return 403 SELF_ADMIN_ACTION_FORBIDDEN
    server.use(
      http.post("/api/v1/admin/users/:id/suspend", () => {
        return HttpResponse.json(
          {
            code: "SELF_ADMIN_ACTION_FORBIDDEN",
            message: "Self-administration is forbidden",
            status: 403,
          },
          { status: 403 },
        );
      }),
    );

    const user = userEvent.setup();
    render(<AdminUserList />);

    await screen.findAllByText("Learner One");

    await openRowMenu(user);
    await user.click(
      screen.getByRole("menuitem", { name: /Inspect account/i }),
    );

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /Suspend User/i }),
      ).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /Suspend User/i }));

    const textarea = screen.getByPlaceholderText(/State the justification/i);
    await user.type(textarea, "Attempting self suspension test");
    await user.click(
      screen.getByRole("button", { name: /Confirm Suspension/i }),
    );

    await waitFor(() => {
      expect(
        screen.getByText(
          /Self-administration is forbidden: you cannot suspend/i,
        ),
      ).toBeInTheDocument();
    });
  });

  it("lists feature flags and creates a new flag", async () => {
    const user = userEvent.setup();
    render(<AdminFeatureFlags />);

    await waitFor(() => {
      expect(screen.getAllByText("streaks_v2").length).toBeGreaterThan(0);
      expect(
        screen.getAllByText("Second-generation streak calculation").length,
      ).toBeGreaterThan(0);
      expect(screen.getByText("25%")).toBeInTheDocument();
    });

    // Open create modal
    await user.click(screen.getByRole("button", { name: /Create Flag/i }));
    expect(screen.getByText("Create Feature Flag")).toBeInTheDocument();

    await user.type(screen.getByLabelText(/Flag Identifier Key/i), "audio_v2");
    await user.type(
      screen.getByLabelText(/Description/i),
      "Audio engine overhaul",
    );
    await user.click(screen.getByRole("button", { name: "Save Feature Flag" }));

    await waitFor(() => {
      expect(screen.getAllByText("audio_v2").length).toBeGreaterThan(0);
    });
  });
});

describe("Admin permission gating", () => {
  it("offers only the sections the caller's permissions allow", async () => {
    // An administrator without system.flags. The route lets them in — they are
    // an admin — but the Feature Flags section is not theirs, and every action
    // in it would answer 403.
    server.use(
      http.get("/api/v1/me/permissions", () =>
        HttpResponse.json({ roles: ["admin"], permissions: ["user.list"] }),
      ),
    );

    await renderAdminPage();

    expect(
      await screen.findByRole("button", { name: /Learner Management/i }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Feature Flags/i }),
    ).not.toBeInTheDocument();
  });

  it("offers nothing when the permission read fails", async () => {
    server.use(
      http.get("/api/v1/me/permissions", () =>
        HttpResponse.json(
          { title: "Server error", status: 500 },
          { status: 500 },
        ),
      ),
    );

    await renderAdminPage();

    // A read that did not happen is not evidence of a permission.
    expect(
      await screen.findByText(/no administrative permissions/i),
    ).toBeInTheDocument();
  });
});

describe("Admin AI usage", () => {
  // The component existed, was exported, and was rendered by nothing.
  //
  // That is the third time on this project: the gamification widgets shipped
  // with no directory to render them, the avatar route was advertised with
  // nothing mounted at it, and this screen was reachable only by importing it
  // in a test. An admin surface nobody can open tells nobody anything, which is
  // the entire point of a view whose job is to show a provider running out
  // before a learner meets a queued word.
  //
  // It is also the only admin component that uses react-query, so it needs the
  // provider the app root supplies in main.tsx. The sibling screens fetch by
  // hand and render without one, which is why no existing test needed this.
  async function renderAdmin() {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    return renderAdminPage(
      "/admin",
      <QueryClientProvider client={client}>
        <AdminPage />
      </QueryClientProvider>,
    );
  }

  it("shows today's usage and marks an exhausted provider", async () => {
    server.use(
      // Only admin.dashboard: the AI tab is then the sole section, so it is the
      // one shown and no other screen's requests enter the picture.
      http.get("/api/v1/me/permissions", () =>
        HttpResponse.json({
          roles: ["admin"],
          permissions: ["admin.dashboard"],
        }),
      ),
      http.get("/api/v1/admin/ai/usage", () =>
        HttpResponse.json({
          items: [
            {
              provider: "openai_compatible",
              task: "vocab_verify",
              requests_today: 45,
              tokens_today: 12050,
              daily_request_limit: 1000,
              daily_token_limit: 500000,
              is_exhausted: false,
            },
            {
              provider: "fallback_llm",
              task: "vocab_verify",
              requests_today: 200,
              tokens_today: 98000,
              daily_request_limit: 200,
              daily_token_limit: 500000,
              is_exhausted: true,
            },
          ],
        }),
      ),
    );

    await renderAdmin();

    expect(
      (await screen.findAllByText(/openai_compatible/)).length,
    ).toBeGreaterThan(0);
    expect((await screen.findAllByText(/fallback_llm/)).length).toBeGreaterThan(
      0,
    );
  });

  it("hides the section from an admin without admin.dashboard", async () => {
    server.use(
      http.get("/api/v1/me/permissions", () =>
        HttpResponse.json({ roles: ["admin"], permissions: ["user.list"] }),
      ),
    );

    await renderAdmin();

    expect(
      await screen.findByRole("button", { name: /Section:/i }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /AI Usage/i }),
    ).not.toBeInTheDocument();
  });
});

describe("Mobile admin section picker", () => {
  it("renders a Section button that opens a BottomSheet with all permitted sections", async () => {
    const user = userEvent.setup();
    server.use(
      http.get("/api/v1/me/permissions", () =>
        HttpResponse.json({
          roles: ["admin"],
          permissions: ["user.list", "system.flags", "questionbank.read"],
        }),
      ),
    );

    await renderAdminPage("/admin");

    // Section picker button exists and has min height >= 44px
    const pickerBtn = await screen.findByRole("button", {
      name: /Section:\s*Learner Management/i,
    });
    expect(pickerBtn).toBeInTheDocument();
    expect(pickerBtn).toHaveClass("min-h-[44px]");

    // Open BottomSheet
    await user.click(pickerBtn);

    const dialog = screen.getByRole("dialog");
    expect(dialog).toBeInTheDocument();
    expect(within(dialog).getByText("Admin Sections")).toBeInTheDocument();

    const dialogScope = within(dialog);

    // Permitted sections appear with icon and label
    const userItem = dialogScope.getByRole("button", {
      name: "Learner Management",
    });
    const flagsItem = dialogScope.getByRole("button", {
      name: "Feature Flags",
    });
    const questionsItem = dialogScope.getByRole("button", {
      name: "Question Bank",
    });

    expect(userItem).toBeInTheDocument();
    expect(flagsItem).toBeInTheDocument();
    expect(questionsItem).toBeInTheDocument();

    // Current section is marked with aria-current
    expect(userItem).toHaveAttribute("aria-current", "page");
    expect(flagsItem).not.toHaveAttribute("aria-current");
    expect(questionsItem).not.toHaveAttribute("aria-current");

    // Unpermitted sections do not appear
    expect(
      dialogScope.queryByRole("button", { name: "AI Usage" }),
    ).not.toBeInTheDocument();
    expect(
      dialogScope.queryByRole("button", { name: "Review Queue" }),
    ).not.toBeInTheDocument();
    expect(
      dialogScope.queryByRole("button", { name: "Creator Payouts" }),
    ).not.toBeInTheDocument();
  });

  it("navigates on section selection and updates current section", async () => {
    const user = userEvent.setup();
    server.use(
      http.get("/api/v1/me/permissions", () =>
        HttpResponse.json({
          roles: ["admin"],
          permissions: ["user.list", "system.flags"],
        }),
      ),
      http.get("/api/v1/admin/users", () =>
        HttpResponse.json({ items: [sampleUser1] }),
      ),
      http.get("/api/v1/admin/flags", () =>
        HttpResponse.json({ items: [sampleFlag] }),
      ),
    );

    await renderAdminPage("/admin");

    const pickerBtn = await screen.findByRole("button", {
      name: /Section:\s*Learner Management/i,
    });
    await user.click(pickerBtn);

    const dialog = screen.getByRole("dialog");
    const flagsItem = within(dialog).getByRole("button", {
      name: "Feature Flags",
    });

    // Choosing a section navigates to its path
    await user.click(flagsItem);

    // BottomSheet closes
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    // Button label reflects new current section
    expect(
      await screen.findByRole("button", {
        name: /Section:\s*Feature Flags/i,
      }),
    ).toBeInTheDocument();

    // The destination section panel is loaded
    expect((await screen.findAllByText("streaks_v2")).length).toBeGreaterThan(
      0,
    );

    // Reopening the picker marks the new section with aria-current
    await user.click(
      screen.getByRole("button", { name: /Section:\s*Feature Flags/i }),
    );
    const reopenedDialog = screen.getByRole("dialog");
    const reopenedScope = within(reopenedDialog);
    expect(
      reopenedScope.getByRole("button", { name: "Feature Flags" }),
    ).toHaveAttribute("aria-current", "page");
    expect(
      reopenedScope.getByRole("button", { name: "Learner Management" }),
    ).not.toHaveAttribute("aria-current");
  });

  it("renders localized picker in Vietnamese", async () => {
    await loadLocale("vi");
    await i18n.changeLanguage("vi");

    const user = userEvent.setup();
    server.use(
      http.get("/api/v1/me/permissions", () =>
        HttpResponse.json({
          roles: ["admin"],
          permissions: ["user.list", "system.flags"],
        }),
      ),
      http.get("/api/v1/admin/users", () =>
        HttpResponse.json({ items: [sampleUser1] }),
      ),
      http.get("/api/v1/admin/flags", () =>
        HttpResponse.json({ items: [sampleFlag] }),
      ),
    );

    await renderAdminPage("/admin");

    const pickerBtn = await screen.findByRole("button", {
      name: /Mục:\s*Quản lý người học/i,
    });
    expect(pickerBtn).toBeInTheDocument();

    await user.click(pickerBtn);

    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("Các mục quản trị")).toBeInTheDocument();

    const userItem = within(dialog).getByRole("button", {
      name: "Quản lý người học",
    });
    expect(userItem).toHaveAttribute("aria-current", "page");

    const flagsItem = within(dialog).getByRole("button", {
      name: "Feature flag",
    });
    expect(flagsItem).toBeInTheDocument();
    expect(flagsItem).not.toHaveAttribute("aria-current");
  });
});
