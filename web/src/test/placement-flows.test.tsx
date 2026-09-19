import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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
import { useAuthStore } from "@/stores/authStore";
import { PlacementInviteCard } from "@/features/learning/components/Dashboard/PlacementInviteCard";
import { WeeklyPlanCard } from "@/features/learning/components/Dashboard/WeeklyPlanCard";
import { LearningProfileSettings } from "@/features/account/components/LearningProfileSettings";
import { WelcomePage } from "@/pages/WelcomePage";
import { PlacementPage } from "@/routes/PlacementPage";
import { server } from "./msw-server";

function renderWithProviders(ui: React.ReactElement) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => (
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>{ui}</QueryClientProvider>
      </I18nextProvider>
    ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });
  return render(<RouterProvider router={router} />);
}

const SESSION_ID = "0199a1c2-3d4e-7f80-9abc-def012345684";
const ACTIVITY_ID = "0199a1c2-3d4e-7f80-9abc-def01234567b";

const inProgressSession = {
  id: SESSION_ID,
  status: "in_progress",
  stage: "vocabulary_grammar",
  started_at: "2026-09-14T03:00:00Z",
  deadline_at: "2026-09-14T03:20:00Z",
  remaining_seconds: 1150,
  responses: 3,
  max_responses: 25,
  current_item: {
    activity_id: ACTIVITY_ID,
    content_version_id: "0199a1c2-3d4e-7f80-9abc-def01234567c",
    kind: "grammar_tense_choice",
    skill: "grammar",
    config: {
      prompt: "She ___ in Hanoi since 2020.",
      options: [
        { id: "A", text: "lives" },
        { id: "B", text: "has lived" },
        { id: "C", text: "lived" },
        { id: "D", text: "is living" },
      ],
    },
  },
  productive_status: "offered",
  productive_deadline_at: null,
  productive_remaining_seconds: 0,
  productive_items: [],
  result: null,
};

const completedSession = {
  ...inProgressSession,
  status: "completed",
  stage: "done",
  remaining_seconds: 0,
  responses: 20,
  current_item: null,
  result: {
    id: "0199a1c2-3d4e-7f80-9abc-def012345690",
    session_id: SESSION_ID,
    level: "B1",
    per_skill: {
      vocabulary: { band: "B1", responses: 7 },
      grammar: { band: "B2", responses: 7 },
      reading: { band: "A2", responses: 3 },
      listening: { band: "B1", responses: 3 },
    },
    taken_at: "2026-09-14T03:16:00Z",
  },
};

describe("work order 13 screens", () => {
  beforeEach(async () => {
    await initI18n("en");
    await i18n.changeLanguage("en");
    useAuthStore.setState({
      status: "authenticated",
      user: { userId: "0199a1c2-3d4e-7f80-9abc-def012345678", role: "user" },
    });
  });

  describe("PlacementInviteCard", () => {
    it("invites while the server says the invitation is available", async () => {
      server.use(
        http.get("/api/v1/me/placement", () =>
          HttpResponse.json({
            result: null,
            active_session: null,
            retake_available_at: null,
            invite_available: true,
          }),
        ),
      );
      renderWithProviders(<PlacementInviteCard />);
      expect(
        await screen.findByRole("link", { name: /take the placement test/i }),
      ).toBeInTheDocument();
    });

    it("stays hidden when the flag is off or the learner is placed", async () => {
      const { container } = renderWithProviders(<PlacementInviteCard />);
      await waitFor(() => {
        expect(container.querySelector("a[href='/placement']")).toBeNull();
      });
    });
  });

  describe("WeeklyPlanCard", () => {
    it("shows the week's items, the focus item and the minutes studied", async () => {
      server.use(
        http.get("/api/v1/me/weekly-plan", () =>
          HttpResponse.json({
            week_start: "2026-09-14",
            minutes_goal: 90,
            items: [
              {
                kind: "lesson",
                minutes: 10,
                lesson_id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
                title: "Making plans",
                skill: "grammar",
                done: true,
              },
              { kind: "reviews", minutes: 5, reviews: 15, done: false },
              {
                kind: "daily_practice",
                minutes: 10,
                skill: "listening",
                focus: true,
                done: false,
              },
            ],
            progress: { minutes: 25, items_done: 1, items_total: 3 },
          }),
        ),
      );
      renderWithProviders(<WeeklyPlanCard />);

      expect(await screen.findByText("This week's plan")).toBeInTheDocument();
      expect(
        screen.getByRole("link", { name: "Making plans" }),
      ).toBeInTheDocument();
      expect(screen.getByText("25 of 90 minutes")).toBeInTheDocument();
      expect(screen.getByText("15 reviews")).toBeInTheDocument();
      expect(screen.getByText("Focus")).toBeInTheDocument();
    });
  });

  describe("LearningProfileSettings", () => {
    it("shows the placed level and when a retake becomes available", async () => {
      server.use(
        http.get("/api/v1/me/learning-profile", () =>
          HttpResponse.json({
            declared_level: null,
            target_level: "B2",
            target_exam: "ielts",
            weekly_minutes_goal: 120,
            motivations: [],
            created_at: "2026-09-14T00:00:00Z",
            updated_at: "2026-09-14T00:00:00Z",
          }),
        ),
        http.get("/api/v1/me/placement", () =>
          HttpResponse.json({
            result: completedSession.result,
            active_session: null,
            retake_available_at: "2999-10-14T03:16:00Z",
            invite_available: false,
          }),
        ),
      );
      renderWithProviders(<LearningProfileSettings />);

      expect(await screen.findByText("Placement")).toBeInTheDocument();
      expect(screen.getAllByText("B1").length).toBeGreaterThan(0);
      expect(screen.getByText(/Placed on/)).toBeInTheDocument();
      expect(
        screen.getByText(/You can retake the test on/),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("link", { name: /retake placement/i }),
      ).toBeNull();
    });
  });

  describe("WelcomePage", () => {
    it("asks for a level when the placement test is not open", async () => {
      const user = userEvent.setup();
      server.use(
        http.get("/api/v1/me/learning-profile", () =>
          HttpResponse.json(
            { code: "LEARNING_PROFILE_NOT_FOUND" },
            { status: 404 },
          ),
        ),
      );
      renderWithProviders(<WelcomePage />);

      expect(
        await screen.findByText("What are you learning English for?"),
      ).toBeInTheDocument();
      await user.click(screen.getByRole("button", { name: "Continue" }));
      expect(
        await screen.findByText("How much time do you have each week?"),
      ).toBeInTheDocument();
      await user.click(screen.getByRole("button", { name: "Continue" }));

      expect(
        await screen.findByText(/The placement test is not open yet/),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: "Take the placement test" }),
      ).toBeNull();
    });
  });

  describe("PlacementPage", () => {
    it("answers the current item with a key and shows the result that comes back", async () => {
      const user = userEvent.setup();
      let seenKey: string | null = null;
      let seenBody: unknown = null;
      server.use(
        http.get("/api/v1/me/placement", () =>
          HttpResponse.json({
            result: null,
            active_session: {
              id: SESSION_ID,
              stage: "vocabulary_grammar",
              deadline_at: "2026-09-14T03:20:00Z",
              remaining_seconds: 1150,
            },
            retake_available_at: null,
            invite_available: false,
          }),
        ),
        http.get(`/api/v1/me/placement/sessions/${SESSION_ID}`, () =>
          HttpResponse.json(inProgressSession),
        ),
        http.post(
          `/api/v1/me/placement/sessions/${SESSION_ID}/answers`,
          async ({ request }) => {
            seenKey = request.headers.get("Idempotency-Key");
            seenBody = await request.json();
            return HttpResponse.json(completedSession);
          },
        ),
      );
      renderWithProviders(<PlacementPage />);

      expect(
        await screen.findByText("She ___ in Hanoi since 2020."),
      ).toBeInTheDocument();
      const next = screen.getByRole("button", { name: "Next" });
      expect(next).toBeDisabled();

      await user.click(screen.getByRole("button", { name: /has lived/ }));
      await user.click(next);

      expect(await screen.findByText("Your level")).toBeInTheDocument();
      expect(
        screen.getByRole("heading", { level: 1, name: "B1" }),
      ).toBeInTheDocument();
      expect(seenKey).toMatch(/^[0-9a-f-]{36}$/);
      expect(seenBody).toEqual({
        activity_id: ACTIVITY_ID,
        response: { selected_option_id: "B" },
      });
    });
  });
});
