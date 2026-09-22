import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
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

describe("AdminQuestionBank", () => {
  let generated: Record<string, unknown>[] = [];

  beforeEach(async () => {
    generated = [];
    await initI18n("en");
    await i18n.changeLanguage("en");

    server.use(
      http.get("/api/v1/admin/questions", () =>
        HttpResponse.json({ items: [question], total: 1, limit: 20, offset: 0 }),
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

    const row = await screen.findByRole("button", {
      name: /grammar_tense_choice/,
    });
    await userEvent.click(row);

    expect(await screen.findByText("0.68")).toBeInTheDocument();
    expect(screen.getByText("0.45")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
  });

  it("generates drafts with the node codes the form carries", async () => {
    renderBank();
    await screen.findByRole("button", { name: /grammar_tense_choice/ });

    await userEvent.click(
      screen.getByRole("button", { name: /Generate drafts/i }),
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
    expect(
      await screen.findByText(/drafts generated/i),
    ).toBeInTheDocument();
  });

  it("shows the coverage number and marks the bottleneck part", async () => {
    renderBank();
    await screen.findByRole("button", { name: /grammar_tense_choice/ });

    await userEvent.click(screen.getByRole("button", { name: /Coverage/i }));

    // The headline number appears beside the "2" test counts in the table.
    expect((await screen.findAllByText("2")).length).toBeGreaterThan(0);
    expect(screen.getByText(/Bottleneck/i)).toBeInTheDocument();
    expect(screen.getByText(/TOEIC_LR_2026/)).toBeInTheDocument();
  });
});
