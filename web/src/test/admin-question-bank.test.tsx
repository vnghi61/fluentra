import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";

import i18n, { initI18n } from "@/i18n";
import { AdminQuestionBank } from "@/features/admin/components/AdminQuestionBank";

import { server } from "./msw-server";

const question = {
  id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
  content_item_id: "0199a1c2-3d4e-7f80-9abc-def01234567b",
  activity_id: "0199a1c2-3d4e-7f80-9abc-def01234567c",
  exam_part_id: null,
  kind: "grammar_tense_choice",
  skill: "grammar",
  cefr_level: "B1",
  difficulty: null,
  question_count: 1,
  fingerprint:
    "7d2b45f1e8a93102efb132a0c49876543210fedcba9876543210fedcba987654",
  provenance: { model: "test-model" },
  status: "published",
  created_at: "2026-09-20T10:00:00Z",
  updated_at: "2026-09-20T10:00:00Z",
};

const coverage = {
  version_id: "20000000-0000-0000-0000-000000000001",
  exam_code: "TOEIC_LR_2026",
  distinct_tests_possible: 2,
  bottleneck_part_id: "20000000-0000-0000-0001-000000000002",
  parts: [
    {
      part_id: "20000000-0000-0000-0001-000000000001",
      part_number: 1,
      section: "listening",
      kind: "photo_description",
      question_count: 6,
      group_size: 1,
      published_groups_available: 12,
      groups_needed_per_test: 6,
      tests_possible: 2,
    },
    {
      part_id: "20000000-0000-0000-0001-000000000002",
      part_number: 2,
      section: "listening",
      kind: "question_response",
      question_count: 25,
      group_size: 1,
      published_groups_available: 50,
      groups_needed_per_test: 25,
      tests_possible: 2,
    },
  ],
};

function renderBank() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <AdminQuestionBank />
      </QueryClientProvider>
    </I18nextProvider>,
  );
}

/** Opens the desktop row menu and picks Performance. */
async function openStatsFromRowMenu() {
  const table = await screen.findByTestId("question-table");
  await userEvent.click(
    within(table).getByRole("button", {
      name: /Actions for Grammar tense choice/i,
    }),
  );
  await userEvent.click(
    await screen.findByRole("menuitem", { name: /Performance/i }),
  );
}

describe("AdminQuestionBank", () => {
  let generated: Record<string, unknown>[] = [];

  beforeEach(async () => {
    generated = [];
    await initI18n("en");
    await i18n.changeLanguage("en");

    server.use(
      http.get("/api/v1/admin/questions", () =>
        HttpResponse.json({
          items: [question],
          total: 1,
          limit: 20,
          offset: 0,
        }),
      ),
      http.get("/api/v1/admin/questions/:id/stats", () =>
        HttpResponse.json({
          question_id: question.id,
          attempts: 42,
          p_value: 0.68,
          discrimination: 0.45,
          avg_time_ms: 15400,
          last_computed_at: "2026-09-20T10:00:00Z",
        }),
      ),
      http.post("/api/v1/admin/questions/generate", async ({ request }) => {
        generated.push((await request.json()) as Record<string, unknown>);
        return HttpResponse.json({ questions: [question] }, { status: 201 });
      }),
      http.get("/api/v1/exam-versions", () =>
        HttpResponse.json({
          items: [
            {
              id: coverage.version_id,
              exam_family: "toeic_lr",
              code: "TOEIC_LR_2026",
              title: "TOEIC Listening & Reading (2026)",
              total_minutes: 120,
              scoring: { type: "raw_with_estimate" },
              source_url: "https://example.test/toeic",
              verified_at: "2026-09-20",
              is_current: true,
              notes: "",
              blueprints: [],
            },
          ],
        }),
      ),
      http.get("/api/v1/admin/exams/versions/:id/coverage", () =>
        HttpResponse.json(coverage),
      ),
    );
  });

  it("lists bank items and shows an item's empirical statistics", async () => {
    renderBank();

    await openStatsFromRowMenu();

    expect(await screen.findByText("0.68")).toBeInTheDocument();
    expect(screen.getByText("0.45")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
  });

  it("says statistics are not computed yet instead of showing zeros", async () => {
    server.use(
      http.get("/api/v1/admin/questions/:id/stats", ({ params }) =>
        HttpResponse.json({
          question_id: params.id,
          attempts: 0,
          p_value: null,
          discrimination: null,
          avg_time_ms: 0,
          last_computed_at: null,
        }),
      ),
    );
    renderBank();
    await openStatsFromRowMenu();

    const dialog = await screen.findByRole("dialog");
    expect(
      await within(dialog).findByText(/No statistics yet/),
    ).toBeInTheDocument();
    expect(
      within(dialog).queryByText("Statistics could not be loaded."),
    ).toBeNull();
  });

  it("opens stats in a Modal dialog and closes it on Escape", async () => {
    renderBank();
    await openStatsFromRowMenu();

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Item performance")).toBeInTheDocument();
    expect(await within(dialog).findByText("0.68")).toBeInTheDocument();

    await userEvent.keyboard("{Escape}");
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
  });

  it("renders the kind in both the table and the cards, with a truncated id on the card", async () => {
    renderBank();
    await screen.findByTestId("question-table");

    const table = screen.getByTestId("question-table");
    const cards = screen.getByTestId("question-cards");
    expect(table).toHaveClass("hidden", "md:block");
    expect(within(table).getByText("Grammar tense choice")).toBeInTheDocument();
    expect(within(cards).getByText("Grammar tense choice")).toBeInTheDocument();
    expect(table.querySelector("tbody tr")).toHaveClass("h-14");
    expect(cards).toHaveTextContent("0199a1c2…");
    expect(cards).not.toHaveTextContent(question.id);
  });

  it("offers Retry after a 500 and refetches", async () => {
    let calls = 0;
    server.use(
      http.get("/api/v1/admin/questions", () => {
        calls += 1;
        if (calls === 1) {
          return HttpResponse.json(
            { code: "INTERNAL", message: "boom", status: 500 },
            { status: 500 },
          );
        }
        return HttpResponse.json({
          items: [question],
          total: 1,
          limit: 20,
          offset: 0,
        });
      }),
    );
    renderBank();

    const alert = await screen.findByRole("alert");
    await userEvent.click(
      within(alert).getByRole("button", { name: /Try again/i }),
    );

    await screen.findByTestId("question-table");
    expect(calls).toBe(2);
  });

  it("shows the empty state with active filters and Clear filters resets them", async () => {
    server.use(
      http.get("/api/v1/admin/questions", ({ request }) => {
        const hasStatus = new URL(request.url).searchParams.has("status");
        return HttpResponse.json({
          items: hasStatus ? [] : [question],
          total: hasStatus ? 0 : 1,
          limit: 20,
          offset: 0,
        });
      }),
    );
    renderBank();
    await screen.findByTestId("question-table");

    await userEvent.click(screen.getByRole("button", { name: /^Filters/i }));
    const sheet = await screen.findByTestId("filter-sheet");
    await userEvent.selectOptions(
      within(sheet).getByLabelText(/^Status/i),
      "draft",
    );

    expect(
      await screen.findByText("No questions match these filters."),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("question-table")).not.toBeInTheDocument();

    // The empty state's own action, not the chip bar's.
    const emptyAction = screen
      .getAllByRole("button", { name: /Clear filters/i })
      .at(-1)!;
    await userEvent.click(emptyAction);

    await screen.findByTestId("question-table");
    expect(
      screen.queryByTestId("applied-filter-chips"),
    ).not.toBeInTheDocument();
  });

  it("generates drafts with the node codes the form carries", async () => {
    renderBank();
    await screen.findByTestId("question-table");

    // Desktop toolbar and mobile header both render the button in jsdom.
    await userEvent.click(
      screen.getAllByRole("button", { name: /Generate drafts/i })[0]!,
    );
    await userEvent.type(
      screen.getByPlaceholderText(/PRESENT_PERFECT/),
      "PRESENT_PERFECT, PAST_SIMPLE",
    );
    await userEvent.click(screen.getByRole("button", { name: /^Generate$/i }));

    await waitFor(() => expect(generated).toHaveLength(1));
    expect(generated[0]).toMatchObject({
      kind: "grammar_tense_choice",
      cefr_level: "B1",
      count: 5,
      node_codes: ["PRESENT_PERFECT", "PAST_SIMPLE"],
    });
    expect(await screen.findByText(/drafts generated/i)).toBeInTheDocument();
  });

  it("shows the coverage number and marks the bottleneck part", async () => {
    renderBank();
    await screen.findByTestId("question-table");

    await userEvent.click(screen.getByRole("tab", { name: /Coverage/i }));

    // The headline number appears beside the "2" test counts in the table.
    expect((await screen.findAllByText("2")).length).toBeGreaterThan(0);
    // Rendered in both the mobile cards and the md+ table.
    expect(screen.getAllByText(/Bottleneck/i).length).toBeGreaterThan(0);
    expect(screen.getByTestId("coverage-cards")).toBeInTheDocument();
    expect(screen.getByText(/TOEIC_LR_2026/)).toBeInTheDocument();
  });

  it("renders mobile cards with summary/prompt first, status badge, action button, and secondary id", async () => {
    renderBank();

    const cardList = await screen.findByTestId("question-cards");
    expect(cardList).toBeInTheDocument();

    const card = within(cardList).getByTestId("question-card");
    expect(card).toBeInTheDocument();

    // Summary/prompt first (humanized question.kind)
    expect(card).toHaveTextContent("Grammar tense choice");
    // Text status badge (translated)
    expect(card).toHaveTextContent("Published");
    // Secondary small text: the id, truncated, never the full UUID
    expect(card).toHaveTextContent("0199a1c2…");
    expect(card).not.toHaveTextContent(question.id);

    // Action button
    const actionBtn = within(card).getByRole("button", {
      name: /Performance/i,
    });
    expect(actionBtn).toBeInTheDocument();

    // Clicking action button opens stats modal
    await userEvent.click(actionBtn);
    expect(await screen.findByText("0.68")).toBeInTheDocument();
  });

  it("opens filters in BottomSheet and shows an applied filter chip", async () => {
    renderBank();
    await screen.findByTestId("question-table");

    // Open filter sheet on mobile
    await userEvent.click(screen.getByRole("button", { name: /^Filters/i }));

    // Select a filter inside the sheet
    const sheet = await screen.findByTestId("filter-sheet");
    const statusSelect = within(sheet).getByLabelText(/^Status/i);
    expect(
      within(statusSelect).getByRole("option", { name: "Draft" }),
    ).toBeInTheDocument();
    await userEvent.selectOptions(statusSelect, "draft");

    await userEvent.click(within(sheet).getByRole("button", { name: "Close" }));

    // Applied filter chip appears with translated status
    const chipContainer = await screen.findByTestId("applied-filter-chips");
    expect(chipContainer).toHaveTextContent(/Status.*Draft/i);

    // Verify 44px touch target and translated aria-label on chip remove button
    const removeStatusBtn = within(chipContainer).getByRole("button", {
      name: /Remove status filter/i,
    });
    expect(removeStatusBtn).toHaveClass("min-h-[44px]", "min-w-[44px]");
    expect(removeStatusBtn).toHaveAttribute(
      "aria-label",
      "Remove Status filter",
    );
  });

  it("removes a chip and clear-filters resets the query params", async () => {
    let lastQuery = "";
    server.use(
      http.get("/api/v1/admin/questions", ({ request }) => {
        lastQuery = new URL(request.url).search;
        return HttpResponse.json({
          items: [question],
          total: 1,
          limit: 20,
          offset: 0,
        });
      }),
    );

    renderBank();
    await screen.findByTestId("question-table");

    // Open filter sheet and apply kind and status filters
    await userEvent.click(screen.getByRole("button", { name: /^Filters/i }));
    const sheet = await screen.findByTestId("filter-sheet");
    await userEvent.selectOptions(
      within(sheet).getByLabelText(/^Kind/i),
      "grammar_tense_choice",
    );
    await userEvent.selectOptions(
      within(sheet).getByLabelText(/^Status/i),
      "draft",
    );

    await waitFor(() => {
      expect(lastQuery).toContain("kind=grammar_tense_choice");
      expect(lastQuery).toContain("status=draft");
    });

    await userEvent.click(within(sheet).getByRole("button", { name: "Close" }));

    // Remove the kind filter chip
    const removeKindBtn = screen.getByRole("button", {
      name: /Remove kind filter/i,
    });
    expect(removeKindBtn).toHaveClass("min-h-[44px]", "min-w-[44px]");
    expect(removeKindBtn).toHaveAttribute("aria-label", "Remove Kind filter");
    await userEvent.click(removeKindBtn);

    await waitFor(() => {
      expect(lastQuery).not.toContain("kind=grammar_tense_choice");
      expect(lastQuery).toContain("status=draft");
    });

    // Click "Clear filters" to reset all filters
    const clearBtn = within(
      screen.getByTestId("applied-filter-chips"),
    ).getByRole("button", { name: /Clear filters/i });
    await userEvent.click(clearBtn);

    await waitFor(() => {
      expect(lastQuery).not.toContain("status=draft");
      expect(
        screen.queryByTestId("applied-filter-chips"),
      ).not.toBeInTheDocument();
    });
  });

  it("shows result count in role='status' saying 'loaded' when more pages exist and no total invented", async () => {
    // 1. With 1 item and total = 1 (no more pages):
    renderBank();
    await screen.findByTestId("question-table");

    const statusLine = screen.getByRole("status");
    expect(statusLine).toHaveTextContent("1 result");
    expect(statusLine).not.toHaveTextContent(/loaded/i);

    // 2. When more pages exist:
    server.use(
      http.get("/api/v1/admin/questions", () =>
        HttpResponse.json({
          items: [question],
          total: 10,
          limit: 1,
          offset: 0,
        }),
      ),
    );

    await userEvent.click(screen.getByRole("button", { name: /^Filters/i }));
    const sheet = await screen.findByTestId("filter-sheet");
    await userEvent.selectOptions(
      within(sheet).getByLabelText(/^Status/i),
      "published",
    );

    await userEvent.click(within(sheet).getByRole("button", { name: "Close" }));

    await waitFor(() => {
      const updatedStatus = screen.getByRole("status");
      expect(updatedStatus).toHaveTextContent("1 loaded");
      expect(updatedStatus).not.toHaveTextContent("10");
    });
  });
});
