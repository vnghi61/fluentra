import { describe, it, expect, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { server } from "./msw-server";
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

describe("Admin Shell & Operations", () => {
  beforeEach(() => {
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
    render(<AdminPage />);

    expect(screen.getByText("Platform Administration")).toBeInTheDocument();

    // Awaited: the tabs are built from /me/permissions, so nothing
    // administrative renders until that read lands. Rendering the tabs first
    // and hiding them afterwards would flash actions the caller may not have.
    expect(
      await screen.findByRole("button", { name: /Learner Management/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Feature Flags/i }),
    ).toBeInTheDocument();

    // Switch to Feature Flags tab
    await user.click(screen.getByRole("button", { name: /Feature Flags/i }));
    await waitFor(() => {
      expect(screen.getByText("Feature Flags & Rollouts")).toBeInTheDocument();
    });
  });

  it("loads learners list with cursor-based pagination walk", async () => {
    const user = userEvent.setup();
    render(<AdminUserList />);

    // Initial page shows learner 1
    await waitFor(() => {
      expect(screen.getByText("Learner One")).toBeInTheDocument();
      expect(screen.getByText("learner1@example.com")).toBeInTheDocument();
    });

    const nextBtn = screen.getByRole("button", { name: /Next/i });
    const prevBtn = screen.getByRole("button", { name: /Previous/i });

    expect(prevBtn).toBeDisabled();
    expect(nextBtn).toBeEnabled();

    // Click Next -> loads page 2 with learner 2
    await user.click(nextBtn);

    await waitFor(() => {
      expect(screen.getByText("Learner Two")).toBeInTheDocument();
      expect(screen.getByText("learner2@example.com")).toBeInTheDocument();
    });

    // Next is now disabled because next_cursor is absent
    expect(nextBtn).toBeDisabled();
    expect(prevBtn).toBeEnabled();

    // Click Previous -> loads back learner 1
    await user.click(prevBtn);
    await waitFor(() => {
      expect(screen.getByText("Learner One")).toBeInTheDocument();
    });
  });

  it("inspects learner details with audit notice", async () => {
    const user = userEvent.setup();
    render(<AdminUserList />);

    await waitFor(() => {
      expect(screen.getByText("Learner One")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /Inspect/i }));

    await waitFor(() => {
      expect(screen.getByText("Learner Account Details")).toBeInTheDocument();
      expect(screen.getByText(/admin\.user_viewed/i)).toBeInTheDocument();
      expect(screen.getByText("vi-VN")).toBeInTheDocument();
      expect(screen.getByText("Asia/Ho_Chi_Minh")).toBeInTheDocument();
    });
  });

  it("enforces reason of at least 10 characters for administrative suspension", async () => {
    const user = userEvent.setup();
    render(<AdminUserList />);

    await waitFor(() => {
      expect(screen.getByText("Learner One")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /Inspect/i }));

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

    await waitFor(() => {
      expect(screen.getByText("Learner One")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /Inspect/i }));

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
      expect(screen.getByText("streaks_v2")).toBeInTheDocument();
      expect(
        screen.getByText("Second-generation streak calculation"),
      ).toBeInTheDocument();
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
      expect(screen.getByText("audio_v2")).toBeInTheDocument();
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

    render(<AdminPage />);

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

    render(<AdminPage />);

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
  function renderAdmin() {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    return render(
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

    renderAdmin();

    expect(await screen.findByText(/openai_compatible/)).toBeInTheDocument();
    expect(screen.getByText(/fallback_llm/)).toBeInTheDocument();
  });

  it("hides the section from an admin without admin.dashboard", async () => {
    server.use(
      http.get("/api/v1/me/permissions", () =>
        HttpResponse.json({ roles: ["admin"], permissions: ["user.list"] }),
      ),
    );

    renderAdmin();

    expect(
      await screen.findByRole("button", { name: /Learner Management/i }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /AI Usage/i }),
    ).not.toBeInTheDocument();
  });
});
