import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";

import i18n, { initI18n } from "@/i18n";
import { AdminReviewQueue } from "@/features/admin/components/AdminReviewQueue";

import { server } from "./msw-server";

const item = {
  id: "0199a1c2-3d4e-7f80-9abc-def012345602",
  item_id: "0199a1c2-3d4e-7f80-9abc-def012345601",
  slug: "foundation-b1-grammar-tense-choice-a1b2c3d4",
  kind: "grammar_tense_choice",
  cefr_level: "B1",
  status: "draft",
  body: {
    prompt: "She ___ lived here for three years.",
    options: [
      { id: "A", text: "has" },
      { id: "B", text: "have" },
      { id: "C", text: "had" },
      { id: "D", text: "having" },
    ],
    correct_option_id: "A",
  },
  blind_solve_answer: { selected_option_id: "A" },
  cefr_reasoning: "Present perfect corresponds to B1.",
  provenance: { model: "test-model", prompt_version: "item_generate.v1" },
  node_codes: ["PRESENT_PERFECT"],
  created_at: "2026-09-21T10:00:00Z",
};

function renderQueue() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <AdminReviewQueue />
      </QueryClientProvider>
    </I18nextProvider>,
  );
}

describe("AdminReviewQueue", () => {
  let decisions: { decision: string; comments?: string }[] = [];
  let published: string[] = [];

  beforeEach(async () => {
    decisions = [];
    published = [];
    await initI18n("en");
    await i18n.changeLanguage("en");

    server.use(
      http.get("/api/v1/admin/review-queue", () =>
        HttpResponse.json({ items: [item], total: 1 }),
      ),
      http.post(
        "/api/v1/admin/content/:id/review",
        async ({ request, params }) => {
          const body = (await request.json()) as {
            decision: string;
            comments?: string;
          };
          decisions.push(body);
          expect(params.id).toBe(item.item_id);
          return HttpResponse.json({ ...item, status: "approved" });
        },
      ),
      http.post("/api/v1/admin/content/:id/publish", ({ params }) => {
        published.push(String(params.id));
        return HttpResponse.json({ ...item, status: "published" });
      }),
    );
  });

  it("shows a generated draft with its key and the blind solver's answer", async () => {
    renderQueue();

    expect(
      await screen.findByText("foundation-b1-grammar-tense-choice-a1b2c3d4"),
    ).toBeInTheDocument();

    await userEvent.click(
      screen.getByText("foundation-b1-grammar-tense-choice-a1b2c3d4"),
    );

    // The learner's renderer, with the key revealed.
    expect(screen.getByText(/She ___ lived here/)).toBeInTheDocument();
    expect(screen.getByText("has")).toBeInTheDocument();
    expect(
      screen.getByText(/Present perfect corresponds to B1/),
    ).toBeInTheDocument();
    expect(screen.getByText(/test-model/)).toBeInTheDocument();
  });

  it("approves and publishes through the content review endpoints", async () => {
    renderQueue();
    await userEvent.click(
      await screen.findByText("foundation-b1-grammar-tense-choice-a1b2c3d4"),
    );

    await userEvent.click(
      screen.getByRole("button", { name: /Approve and publish/i }),
    );

    await waitFor(() => expect(decisions).toHaveLength(1));
    expect(decisions[0]?.decision).toBe("approved");
    expect(published).toEqual([item.item_id]);
  });

  it("requires a note before requesting changes", async () => {
    renderQueue();
    await userEvent.click(
      await screen.findByText("foundation-b1-grammar-tense-choice-a1b2c3d4"),
    );

    const requestChanges = screen.getByRole("button", {
      name: /Request changes/i,
    });
    expect(requestChanges).toBeDisabled();

    await userEvent.type(
      screen.getByLabelText(/Note/i),
      "The distractor is ambiguous.",
    );
    expect(requestChanges).toBeEnabled();
    await userEvent.click(requestChanges);

    await waitFor(() => expect(decisions).toHaveLength(1));
    expect(decisions[0]).toEqual({
      decision: "changes_requested",
      comments: "The distractor is ambiguous.",
    });
    expect(published).toEqual([]);
  });
});
