import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";

import i18n, { initI18n } from "@/i18n";
import { AdminReviewQueue } from "@/features/admin/components/AdminReviewQueue";
import type { AdminReviewQueueItem } from "@/features/admin/api/adminApi";

import { server } from "./msw-server";

const item: AdminReviewQueueItem = {
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

async function openItem() {
  const table = await screen.findByRole("table", {
    name: "Content Review Queue",
  });
  const button = within(table).getByRole("button", { name: item.slug });
  await userEvent.click(button);
  return button;
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
      http.get("/api/v1/me/permissions", () =>
        HttpResponse.json({
          permissions: ["content.review", "content.publish"],
        }),
      ),
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

    await openItem();

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
    await openItem();

    await userEvent.click(
      screen.getByRole("button", { name: /Approve and publish/i }),
    );

    await waitFor(() => expect(decisions).toHaveLength(1));
    expect(decisions[0]?.decision).toBe("approved");
    expect(published).toEqual([item.item_id]);
  });

  it("requires a note before requesting changes", async () => {
    renderQueue();
    await openItem();

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

  it("shows real pending count, metadata and creation dates without unsupported bulk controls", async () => {
    renderQueue();
    const table = await screen.findByRole("table", {
      name: "Content Review Queue",
    });
    expect(within(table).getByText("PRESENT_PERFECT")).toBeInTheDocument();
    expect(within(table).getByText("B1")).toBeInTheDocument();
    expect(
      within(table).getByRole("columnheader", { name: "Created" }),
    ).toBeInTheDocument();
    expect(screen.getByText("(1)")).toBeInTheDocument();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    expect(document.querySelector("button button")).toBeNull();
  });

  it("filters the current page by slug or ID without issuing a fake server search", async () => {
    let requests = 0;
    server.use(
      http.get("/api/v1/admin/review-queue", () => {
        requests++;
        return HttpResponse.json({ items: [item], total: 30 });
      }),
    );
    renderQueue();
    await screen.findByRole("table");
    const user = userEvent.setup();
    await user.type(screen.getByRole("searchbox"), "nonexistent");
    expect(screen.getByText("No matches on this page.")).toBeInTheDocument();
    expect(screen.getByText("Showing 1–1 of 30")).toBeInTheDocument();
    expect(requests).toBe(1);
    await user.click(screen.getByRole("button", { name: "Reset filters" }));
    expect(await screen.findByRole("table")).toBeInTheDocument();
    await user.type(screen.getByRole("searchbox"), item.item_id);
    expect(screen.getByText(/1 match on this page\./)).toBeInTheDocument();
  });

  it("uses offset pages, jumps to a page and resets page search on navigation", async () => {
    const requests: URL[] = [];
    server.use(
      http.get("/api/v1/admin/review-queue", ({ request }) => {
        const url = new URL(request.url);
        requests.push(url);
        const offset = Number(url.searchParams.get("offset"));
        const limit = Number(url.searchParams.get("limit"));
        return HttpResponse.json({
          items: Array.from(
            { length: Math.min(limit, 35 - offset) },
            (_, index) => ({
              ...item,
              id: `${item.id}-${offset + index}`,
              slug: `review-${offset + index + 1}`,
            }),
          ),
          total: 35,
        });
      }),
    );
    renderQueue();
    await screen.findByRole("table");
    const user = userEvent.setup();
    await user.type(screen.getByRole("searchbox"), "review-1");
    await user.click(screen.getByRole("button", { name: "Next" }));
    await waitFor(() =>
      expect(
        within(screen.getByRole("table")).getByText("review-16"),
      ).toBeInTheDocument(),
    );
    expect(screen.getByRole("searchbox")).toHaveValue("");
    await user.click(screen.getByRole("button", { name: "Go to page 3" }));
    await waitFor(() =>
      expect(
        within(screen.getByRole("table")).getByText("review-31"),
      ).toBeInTheDocument(),
    );
    await user.selectOptions(
      screen.getByRole("combobox", { name: "Rows per page" }),
      "25",
    );
    await waitFor(() =>
      expect(
        within(screen.getByRole("table")).getAllByRole("row"),
      ).toHaveLength(26),
    );
    expect(requests.at(-1)?.searchParams.get("offset")).toBe("0");
  });

  it("applies mobile purpose/kind filters only after confirmation and cancels drafts", async () => {
    const requests: URL[] = [];
    server.use(
      http.get("/api/v1/admin/review-queue", ({ request }) => {
        requests.push(new URL(request.url));
        return HttpResponse.json({ items: [item], total: 1 });
      }),
    );
    const user = userEvent.setup();
    renderQueue();
    await screen.findByRole("table");
    await user.click(screen.getByRole("button", { name: "Filters" }));
    let dialog = screen.getByRole("dialog", { name: "Filters" });
    await user.selectOptions(
      within(dialog).getByRole("combobox", { name: "Purpose" }),
      "foundation",
    );
    await user.keyboard("{Escape}");
    expect(requests).toHaveLength(1);
    await user.click(screen.getByRole("button", { name: "Filters" }));
    dialog = screen.getByRole("dialog", { name: "Filters" });
    expect(
      within(dialog).getByRole("combobox", { name: "Purpose" }),
    ).toHaveValue("");
    await user.selectOptions(
      within(dialog).getByRole("combobox", { name: "Purpose" }),
      "bank",
    );
    await user.selectOptions(
      within(dialog).getByRole("combobox", { name: "Kind" }),
      "grammar_tense_choice",
    );
    await user.click(within(dialog).getByRole("button", { name: "Apply" }));
    await waitFor(() =>
      expect(requests.at(-1)?.searchParams.get("purpose")).toBe("bank"),
    );
    expect(requests.at(-1)?.searchParams.get("kind")).toBe(
      "grammar_tense_choice",
    );
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("does not confuse queue errors with an empty queue or expose raw errors", async () => {
    server.use(
      http.get("/api/v1/admin/review-queue", () =>
        HttpResponse.json(
          { status: 500, title: "private database error" },
          { status: 500 },
        ),
      ),
    );
    renderQueue();
    expect(
      await screen.findByRole("heading", {
        name: "The review queue could not be loaded.",
      }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Nothing is waiting for review."),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("private database error"),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Try again" }),
    ).toBeInTheDocument();
  });

  it("distinguishes batch and sample failures from their empty states", async () => {
    server.use(
      http.get("/api/v1/admin/review-queue/batches", () =>
        HttpResponse.json({ status: 500 }, { status: 500 }),
      ),
      http.get("/api/v1/admin/review-queue/samples", () =>
        HttpResponse.json({ status: 500 }, { status: 500 }),
      ),
    );
    renderQueue();
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Batches" }));
    expect(
      await screen.findByRole("heading", {
        name: "The review queue could not be loaded.",
      }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("No generation run is waiting."),
    ).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Samples" }));
    expect(
      await screen.findByRole("heading", {
        name: "The review queue could not be loaded.",
      }),
    ).toBeInTheDocument();
    expect(screen.queryByText("No sample is waiting.")).not.toBeInTheDocument();
  });

  it("opens the review from the mobile menu and restores focus after Escape", async () => {
    renderQueue();
    await screen.findByRole("table");
    const user = userEvent.setup();
    const menu = within(screen.getByTestId("review-cards")).getByRole(
      "button",
      { name: `Actions for ${item.slug}` },
    );
    await user.click(menu);
    await user.click(screen.getByRole("menuitem", { name: "Review content" }));
    expect(screen.getByRole("dialog", { name: item.slug })).toBeInTheDocument();
    await user.keyboard("{Escape}");
    await waitFor(() => expect(menu).toHaveFocus());
  });

  it("locks inputs and dismissal while a review is being saved", async () => {
    let finish: () => void = () => {};
    const saving = new Promise<void>((resolve) => {
      finish = resolve;
    });
    server.use(
      http.post("/api/v1/admin/content/:id/review", async () => {
        await saving;
        return HttpResponse.json({ ...item, status: "approved" });
      }),
    );
    renderQueue();
    await openItem();
    const user = userEvent.setup();
    await user.click(
      screen.getByRole("button", { name: "Approve and publish" }),
    );
    expect(screen.getByLabelText(/Note/)).toBeDisabled();
    expect(screen.getByRole("button", { name: "Close" })).toBeDisabled();
    await user.keyboard("{Escape}");
    expect(screen.getByRole("dialog", { name: item.slug })).toBeInTheDocument();
    await act(async () => {
      finish();
      await saving;
    });
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
  });

  it("explains that an author cannot approve their own content", async () => {
    let published = 0;
    server.use(
      http.post("/api/v1/admin/content/:id/review", () =>
        HttpResponse.json(
          {
            status: 403,
            title: "Authors cannot approve their own content.",
            code: "SELF_APPROVAL_FORBIDDEN",
          },
          { status: 403 },
        ),
      ),
      http.post("/api/v1/admin/content/:id/publish", () => {
        published += 1;
        return HttpResponse.json({ ...item, status: "published" });
      }),
    );
    renderQueue();
    await openItem();
    const user = userEvent.setup();
    await user.click(
      screen.getByRole("button", { name: "Approve and publish" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "You created this content, so you cannot approve it yourself.",
    );
    expect(published).toBe(0);
  });

  it("retries failed publication without sending a second approval", async () => {
    let attempts = 0;
    server.use(
      http.post("/api/v1/admin/content/:id/publish", () =>
        ++attempts === 1
          ? HttpResponse.json(
              { status: 500, title: "private database error" },
              { status: 500 },
            )
          : HttpResponse.json({ ...item, status: "published" }),
      ),
    );
    renderQueue();
    await openItem();
    const user = userEvent.setup();
    await user.click(
      screen.getByRole("button", { name: "Approve and publish" }),
    );
    const retry = await screen.findByRole("button", {
      name: "Retry publication",
    });
    await waitFor(() => expect(retry).toBeEnabled());
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Approved, but not yet published.",
    );
    expect(
      screen.queryByText("private database error"),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Request changes" }),
    ).toBeDisabled();
    await user.click(retry);
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    expect(decisions).toHaveLength(1);
    expect(attempts).toBe(2);
  });

  it("allows requesting changes but not publication without content.publish permission", async () => {
    server.use(
      http.get("/api/v1/me/permissions", () =>
        HttpResponse.json({ permissions: ["content.review"] }),
      ),
    );
    renderQueue();
    await openItem();
    await userEvent.type(
      screen.getByLabelText(/Note/),
      "Please check this draft.",
    );
    expect(
      screen.getByRole("button", { name: "Request changes" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Approve and publish" }),
    ).toBeDisabled();
  });
});
