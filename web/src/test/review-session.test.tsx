import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEventDefault from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";

import i18n, { initI18n } from "@/i18n";
import { ReviewPage } from "@/routes/ReviewPage";
import type { ReviewGrade, ReviewSessionResponse } from "@/features/review";
import { server } from "./msw-server";

async function renderReview() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const rootRoute = createRootRoute();
  const reviewRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/practice/review",
    component: () => (
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <ReviewPage />
        </QueryClientProvider>
      </I18nextProvider>
    ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([reviewRoute]),
    history: createMemoryHistory({ initialEntries: ["/practice/review"] }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

/**
 * The card's word, asserted across both faces.
 *
 * FlipCard lays the front and the back out together and rotates the container —
 * that is what makes the turn continuous rather than a cross-fade between two
 * heights — so the word is in the DOM twice, once per face. The face pointing
 * away is `aria-hidden`, which is what matters to a screen reader; to a text
 * query both are simply there.
 */
async function expectWordOnCard(word: string): Promise<void> {
  const shown = await screen.findAllByText(word);
  expect(shown.length).toBeGreaterThan(0);
}

describe("ReviewPage SRS Session (P10.4)", () => {
  // The card carries its content, because GET /reviews/session resolves the
  // version behind it. An earlier version of this suite used a fixture with no
  // content and still asserted `findByText("meticulous")` — it passed only
  // because the component substituted a hard-coded word for every card, which is
  // a test that could not fail for the thing it claimed to check.
  const cardWithContent: ReviewSessionResponse["cards"][number] = {
    id: "card-1",
    user_id: "0199a1c2-3d4e-7f80-9abc-def012345678",
    content_version_id: "0199a1c2-3d4e-7f80-9abc-def01234567b",
    skill: "vocabulary",
    stability: 2.4,
    difficulty: 4.5,
    due_at: "2026-08-24T09:00:00Z",
    reps: 2,
    lapses: 0,
    state: "review",
    content: {
      kind: "vocab_flashcard",
      cefr_level: "B2",
      body: {
        word: "meticulous",
        pos: "adjective",
        ipa: "/m\u0259\u02c8t\u026akj\u0259l\u0259s/",
        definition: "Showing great attention to detail.",
        definition_vi: "T\u1ec9 m\u1ec9, c\u1ea9n th\u1eadn.",
        example_sentence: "She kept meticulous records of every transaction.",
      },
    },
  };

  const mockSession: ReviewSessionResponse = {
    cards: [cardWithContent],
    total_due: 1,
  };

  let capturedGrades: { cardId: string; grade: ReviewGrade }[] = [];

  function serveSession(session: ReviewSessionResponse) {
    server.use(
      http.get("/api/v1/reviews/session", () => HttpResponse.json(session)),
      http.post("/api/v1/reviews/:id/answer", async ({ params, request }) => {
        const requestBody = (await request.json()) as { grade: ReviewGrade };
        capturedGrades.push({
          cardId: String(params.id ?? "1"),
          grade: requestBody.grade,
        });
        return HttpResponse.json({
          card: session.cards[0],
          next_due_at: "2026-08-27T09:00:00Z",
          interval_days: 3,
        });
      }),
    );
  }

  beforeEach(async () => {
    initI18n("en");
    await i18n.changeLanguage("en");
    capturedGrades = [];
    serveSession(mockSession);
  });

  it("renders the word the API returned, not a placeholder", async () => {
    await renderReview();

    await expectWordOnCard("meticulous");
    // The IPA sits on both faces too, for the same reason the word does.
    expect(
      screen.getAllByText("/m\u0259\u02c8t\u026akj\u0259l\u0259s/").length,
    ).toBeGreaterThan(0);
  });

  it("flips card and grades using 4 grade buttons", async () => {
    const user = userEventDefault.setup();
    await renderReview();

    await expectWordOnCard("meticulous");

    await user.click(screen.getByRole("button", { name: /meticulous/i }));

    expect(
      await screen.findByText(/Showing great attention to detail/i),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Good/i })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /Good/i }));

    expect(capturedGrades.length).toBe(1);
    expect(capturedGrades[0]?.grade).toBe("good");

    expect(
      await screen.findByText("Review Session Summary"),
    ).toBeInTheDocument();
    expect(screen.getByText("Cards Reviewed")).toBeInTheDocument();
  });

  it("grades using keyboard shortcuts 1-4 with exact string enum payload", async () => {
    const user = userEventDefault.setup();
    await renderReview();

    await expectWordOnCard("meticulous");

    await user.keyboard(" ");

    expect(
      await screen.findByText(/Showing great attention to detail/i),
    ).toBeInTheDocument();

    // The digit is the affordance; the enum is the contract.
    await user.keyboard("4");

    expect(capturedGrades.length).toBe(1);
    expect(capturedGrades[0]?.grade).toBe("easy");
  });

  it("clears a whole queue with the keyboard alone", async () => {
    const user = userEventDefault.setup();
    serveSession({
      cards: [
        cardWithContent,
        { ...cardWithContent, id: "card-2" },
        { ...cardWithContent, id: "card-3" },
      ],
      total_due: 3,
    });
    await renderReview();

    await expectWordOnCard("meticulous");

    for (const digit of ["3", "2", "4"]) {
      await user.keyboard(" ");
      await user.keyboard(digit);
    }

    expect(capturedGrades.map((g) => g.grade)).toEqual([
      "good",
      "hard",
      "easy",
    ]);
    expect(
      await screen.findByText("Review Session Summary"),
    ).toBeInTheDocument();
  });

  it("renders 320px responsive Vietnamese grade buttons cleanly", async () => {
    await i18n.changeLanguage("vi");
    const user = userEventDefault.setup();
    await renderReview();

    await expectWordOnCard("meticulous");
    await user.click(screen.getByRole("button", { name: /meticulous/i }));

    expect(await screen.findByText("L\u1ea1i")).toBeInTheDocument();
    expect(screen.getByText("Kh\u00f3")).toBeInTheDocument();
    expect(screen.getByText("T\u1ed1t")).toBeInTheDocument();
    expect(screen.getByText("D\u1ec5")).toBeInTheDocument();
  });

  it("says so when a card arrives with no content, rather than inventing a word", async () => {
    const { content: _dropped, ...withoutContent } = cardWithContent;
    serveSession({ cards: [withoutContent], total_due: 1 });
    await renderReview();

    expect(
      await screen.findByText("This card has no content yet"),
    ).toBeInTheDocument();
    expect(screen.queryAllByText("meticulous")).toHaveLength(0);

    // The schedule is real, so the card is still gradable.
    expect(screen.getByRole("button", { name: /Good/i })).toBeInTheDocument();
  });

  it("says so when the content body is missing the word, rather than inventing one", async () => {
    // The half-authored case, which is the one the hard-coded fallback used to
    // sit in: the version resolved, so `content` is present, but its body has no
    // word to put on the front.
    serveSession({
      cards: [
        {
          ...cardWithContent,
          content: { kind: "vocab_flashcard", body: { pos: "adjective" } },
        },
      ],
      total_due: 1,
    });
    await renderReview();

    expect(
      await screen.findByText("This card has no content yet"),
    ).toBeInTheDocument();
    expect(screen.queryAllByText("meticulous")).toHaveLength(0);
  });

  it("keeps the card on screen when a grade fails to reach the server", async () => {
    const user = userEventDefault.setup();
    server.use(
      http.get("/api/v1/reviews/session", () => HttpResponse.json(mockSession)),
      http.post("/api/v1/reviews/:id/answer", () =>
        HttpResponse.json({ title: "boom" }, { status: 500 }),
      ),
    );
    await renderReview();

    await expectWordOnCard("meticulous");
    await user.keyboard(" ");
    await user.keyboard("3");

    // Not counted, not skipped: a grade that did not land has not happened.
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(
      screen.queryByText("Review Session Summary"),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Good/i })).toBeInTheDocument();
  });

  it("renders an empty state when nothing is due", async () => {
    serveSession({ cards: [], total_due: 0 });
    await renderReview();

    expect(
      await screen.findByText("Nothing due right now"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Review Session Summary"),
    ).not.toBeInTheDocument();
  });
});
