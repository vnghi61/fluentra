import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it, vi } from "vitest";

import i18n, { initI18n, loadLocale } from "@/i18n";
import { MyWritingPage } from "@/routes/MyWritingPage";
import { ExerciseWriting } from "@/features/learning/components/Runner/ExerciseWriting";
import {
  clearWritingDraft,
  getWritingDraft,
  saveWritingDraft,
  WritingFeedbackView,
  type WritingFeedback,
} from "@/features/writing";
import { useAuthStore } from "@/stores/authStore";

import { server } from "./msw-server";

const mockFeedback: WritingFeedback = {
  attempt_id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
  user_id: "user-123",
  overall_band: 7.0,
  score: 78,
  criteria: [
    {
      name: "task_response",
      band: 7.5,
      comment_en: "Well developed response addressing all parts.",
      comment_vi: "Phát triển ý tốt và trả lời đầy đủ đề bài.",
    },
    {
      name: "coherence_cohesion",
      band: 7.0,
      comment_en: "Good logical sequencing.",
      comment_vi: "Tính liên kết và mạch lạc tốt.",
    },
    {
      name: "lexical_resource",
      band: 7.0,
      comment_en: "Varied vocabulary with few errors.",
      comment_vi: "Từ vựng đa dạng, ít lỗi.",
    },
    {
      name: "grammatical_range_accuracy",
      band: 6.5,
      comment_en: "Mix of complex sentences.",
      comment_vi: "Kết hợp câu phức tốt.",
    },
  ],
  annotations: [
    {
      quoted_text: "rapid advancement",
      start_offset: 4,
      end_offset: 21,
      comment_en: "Strong collocation.",
      comment_vi: "Cụm từ tốt.",
    },
  ],
  feedback_en: "Solid essay with clear arguments and good structure.",
  feedback_vi: "Bài viết vững với luận điểm rõ ràng và cấu trúc tốt.",
  prompt_version: "writing_grade.v2",
  model: "gpt-4o-mini",
  created_at: "2026-09-11T12:00:00Z",
};

const mockSubmissions = {
  items: [
    {
      attempt_id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
      status: "graded",
      overall_band: 7.0,
      score: 78,
      feedback_en: "Solid essay with clear arguments and good structure.",
      feedback_vi: "Bài viết vững với luận điểm rõ ràng và cấu trúc tốt.",
      created_at: "2026-09-11T12:00:00Z",
    },
  ],
  total: 1,
  page: 1,
  page_size: 10,
};

async function renderMyWritingPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const rootRoute = createRootRoute();
  const myWritingRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/my-writing",
    component: () => (
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <MyWritingPage />
        </QueryClientProvider>
      </I18nextProvider>
    ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([myWritingRoute]),
    history: createMemoryHistory({ initialEntries: ["/my-writing"] }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

function signIn() {
  useAuthStore.getState().setAuthSession({
    access_token: "valid-test-token",
    token_type: "Bearer",
    expires_in: 900,
    user_id: "user-123",
    role: "user",
  });
}

describe("Writing Screens & Drafts (§3.5)", () => {
  beforeEach(async () => {
    await initI18n();
    await loadLocale("en");
    await i18n.changeLanguage("en");
    useAuthStore.getState().clearAuth();
    localStorage.clear();
  });

  describe("Draft persistence in localStorage", () => {
    it("saves, retrieves, and clears drafts by user and activity id", () => {
      saveWritingDraft("user-1", "act-1", "My draft essay content");
      expect(getWritingDraft("user-1", "act-1")).toBe("My draft essay content");
      expect(getWritingDraft("user-2", "act-1")).toBe("");

      clearWritingDraft("user-1", "act-1");
      expect(getWritingDraft("user-1", "act-1")).toBe("");
    });

    it("clears all writing drafts on sign-out", () => {
      signIn();
      saveWritingDraft("user-123", "act-1", "Draft 1");
      saveWritingDraft("user-123", "act-2", "Draft 2");

      expect(getWritingDraft("user-123", "act-1")).toBe("Draft 1");
      expect(getWritingDraft("user-123", "act-2")).toBe("Draft 2");

      useAuthStore.getState().clearAuth();

      expect(getWritingDraft("user-123", "act-1")).toBe("");
      expect(getWritingDraft("user-123", "act-2")).toBe("");
    });
  });

  describe("WritingFeedbackView component", () => {
    it("renders overall band, 4 IELTS criteria, summary, and model answer", async () => {
      const user = userEvent.setup();
      render(
        <I18nextProvider i18n={i18n}>
          <WritingFeedbackView
            feedback={mockFeedback}
            essayText="The rapid advancement of tech is fast."
            sampleAnswer="Technology has advanced rapidly over the past decade..."
          />
        </I18nextProvider>,
      );

      // Check Overall Band and Score
      expect(screen.getByText(/Overall Band/i)).toBeInTheDocument();
      expect(screen.getByText("7.0")).toBeInTheDocument();
      expect(screen.getByText(/78 \/ 100/i)).toBeInTheDocument();

      // Check 4 Criteria
      expect(screen.getByText(/Task Response/i)).toBeInTheDocument();
      expect(screen.getByText("Band 7.5")).toBeInTheDocument();
      expect(screen.getByText(/Coherence & Cohesion/i)).toBeInTheDocument();
      expect(screen.getAllByText("Band 7.0")).toHaveLength(2);
      expect(screen.getByText(/Lexical Resource/i)).toBeInTheDocument();
      expect(screen.getByText(/Grammatical Range/i)).toBeInTheDocument();
      expect(screen.getByText("Band 6.5")).toBeInTheDocument();

      // Check Summary
      expect(
        screen.getByText(/Solid essay with clear arguments/i),
      ).toBeInTheDocument();

      // Check Highlighted Annotation button
      const annotationBtn = screen.getByRole("button", {
        name: /rapid advancement/i,
      });
      expect(annotationBtn).toBeInTheDocument();

      // Click annotation to inspect comment
      await user.click(annotationBtn);
      expect(screen.getByText("Strong collocation.")).toBeInTheDocument();

      // Toggle model answer
      const modelAnswerToggle = screen.getByText(/Model Answer/i);
      await user.click(modelAnswerToggle);
      expect(
        screen.getByText(/Technology has advanced rapidly/i),
      ).toBeInTheDocument();
    });
  });

  describe("ExerciseWriting component", () => {
    it("loads saved draft on mount and autosaves on edit", async () => {
      const user = userEvent.setup();
      saveWritingDraft("user-1", "act-1", "Existing draft text");

      render(
        <I18nextProvider i18n={i18n}>
          <ExerciseWriting
            prompt="Discuss pros and cons of remote work."
            userId="user-1"
            activityId="act-1"
            isSubmitted={false}
            onSubmit={() => {}}
            onContinue={() => {}}
          />
        </I18nextProvider>,
      );

      const textarea = screen.getByRole("textbox");
      expect(textarea).toHaveValue("Existing draft text");

      await user.type(textarea, " with added words");
      expect(getWritingDraft("user-1", "act-1")).toContain("with added words");
    });

    it("renders marking state when isMarking is true", () => {
      render(
        <I18nextProvider i18n={i18n}>
          <ExerciseWriting
            prompt="Discuss pros and cons of remote work."
            isSubmitted={false}
            isMarking={true}
            onSubmit={() => {}}
            onContinue={() => {}}
          />
        </I18nextProvider>,
      );

      expect(screen.getByText(/Marking your essay/i)).toBeInTheDocument();
      expect(
        screen.getByText(/Our AI examiner is evaluating your writing/i),
      ).toBeInTheDocument();
    });

    it("renders timeout message when markingTimedOut is true", () => {
      render(
        <I18nextProvider i18n={i18n}>
          <ExerciseWriting
            prompt="Discuss pros and cons of remote work."
            isSubmitted={true}
            markingTimedOut={true}
            onNavigateToMyWriting={() => {}}
            onSubmit={() => {}}
            onContinue={() => {}}
          />
        </I18nextProvider>,
      );

      expect(
        screen.getByText(/Marking continues in the background/i),
      ).toBeInTheDocument();
      expect(screen.getByText(/View My Writing/i)).toBeInTheDocument();
    });

    it("renders visible label linked to textarea with prompt above it", () => {
      render(
        <I18nextProvider i18n={i18n}>
          <ExerciseWriting
            prompt="Discuss pros and cons of remote work."
            minWords={40}
            isSubmitted={false}
            onSubmit={() => {}}
            onContinue={() => {}}
          />
        </I18nextProvider>,
      );

      expect(
        screen.getByRole("heading", {
          name: /Discuss pros and cons of remote work/i,
        }),
      ).toBeInTheDocument();
      const label = screen.getByText("Your writing");
      expect(label).toBeVisible();
      expect(label).toHaveAttribute("for", "writing-response");
      const textarea = screen.getByRole("textbox", { name: "Your writing" });
      expect(textarea).toHaveAttribute("id", "writing-response");
      expect(textarea).toHaveClass(
        "min-h-[220px]",
        "md:min-h-[320px]",
        "text-base",
      );
    });

    it("displays counter with min from props (e.g. 150 and 40) and plain counter for minWords=0", () => {
      const { rerender } = render(
        <I18nextProvider i18n={i18n}>
          <ExerciseWriting
            prompt="Essay task"
            minWords={150}
            isSubmitted={false}
            onSubmit={() => {}}
            onContinue={() => {}}
          />
        </I18nextProvider>,
      );

      expect(
        screen.getByText("0 words · minimum 150 words"),
      ).toBeInTheDocument();

      rerender(
        <I18nextProvider i18n={i18n}>
          <ExerciseWriting
            prompt="Short response"
            minWords={40}
            isSubmitted={false}
            onSubmit={() => {}}
            onContinue={() => {}}
          />
        </I18nextProvider>,
      );

      expect(
        screen.getByText("0 words · minimum 40 words"),
      ).toBeInTheDocument();

      rerender(
        <I18nextProvider i18n={i18n}>
          <ExerciseWriting
            prompt="Freeform task"
            minWords={0}
            isSubmitted={false}
            onSubmit={() => {}}
            onContinue={() => {}}
          />
        </I18nextProvider>,
      );

      expect(screen.getByText("0 words")).toBeInTheDocument();
      expect(screen.queryByText(/minimum/i)).not.toBeInTheDocument();
    });

    it("shows helper count and disables submit button when below minimum words", async () => {
      const user = userEvent.setup();
      render(
        <I18nextProvider i18n={i18n}>
          <ExerciseWriting
            prompt="Write an essay"
            minWords={40}
            isSubmitted={false}
            onSubmit={() => {}}
            onContinue={() => {}}
          />
        </I18nextProvider>,
      );

      const checkBtn = screen.getByRole("button", { name: /check/i });
      expect(checkBtn).toBeDisabled();
      expect(
        screen.getByText("Write 40 more words to check"),
      ).toBeInTheDocument();

      const textarea = screen.getByRole("textbox");
      await user.type(textarea, "This is only five words.");

      expect(
        screen.getByText("5 words · minimum 40 words"),
      ).toBeInTheDocument();
      expect(
        screen.getByText("Write 35 more words to check"),
      ).toBeInTheDocument();
      expect(checkBtn).toBeDisabled();
    });

    it("enables submit when minimum words is reached", async () => {
      const user = userEvent.setup();
      render(
        <I18nextProvider i18n={i18n}>
          <ExerciseWriting
            prompt="Write a short response"
            minWords={5}
            isSubmitted={false}
            onSubmit={() => {}}
            onContinue={() => {}}
          />
        </I18nextProvider>,
      );

      const checkBtn = screen.getByRole("button", { name: /check/i });
      expect(checkBtn).toBeDisabled();

      const textarea = screen.getByRole("textbox");
      await user.type(textarea, "One two three four five.");

      expect(screen.getByText("5 words · minimum 5 words")).toBeInTheDocument();
      expect(
        screen.queryByText(/more words to check/i),
      ).not.toBeInTheDocument();
      expect(checkBtn).toBeEnabled();
    });

    it("blocks synchronous double click on submit button and calls onSubmit only once", async () => {
      const user = userEvent.setup();
      const onSubmit = vi.fn();

      render(
        <I18nextProvider i18n={i18n}>
          <ExerciseWriting
            prompt="Write a quick thought"
            minWords={3}
            isSubmitted={false}
            onSubmit={onSubmit}
            onContinue={() => {}}
          />
        </I18nextProvider>,
      );

      const textarea = screen.getByRole("textbox");
      await user.type(textarea, "One two three.");

      const checkBtn = screen.getByRole("button", { name: /check/i });
      expect(checkBtn).toBeEnabled();

      // Two synchronous clicks call onSubmit only once
      fireEvent.click(checkBtn);
      fireEvent.click(checkBtn);

      expect(onSubmit).toHaveBeenCalledTimes(1);
      expect(onSubmit).toHaveBeenCalledWith("One two three.");
    });

    it("releases pending submit when isLoading never toggles (fake timers)", () => {
      vi.useFakeTimers();
      const onSubmit = vi.fn();

      render(
        <I18nextProvider i18n={i18n}>
          <ExerciseWriting
            prompt="Write a quick thought"
            minWords={3}
            isSubmitted={false}
            onSubmit={onSubmit}
            onContinue={() => {}}
          />
        </I18nextProvider>,
      );

      const textarea = screen.getByRole("textbox");
      fireEvent.change(textarea, { target: { value: "One two three." } });

      const checkBtn = screen.getByRole("button", { name: /check/i });
      expect(checkBtn).toBeEnabled();

      // Submit once
      fireEvent.click(checkBtn);
      expect(onSubmit).toHaveBeenCalledTimes(1);
      expect(onSubmit).toHaveBeenCalledWith("One two three.");
      expect(checkBtn).toBeDisabled();

      // Next tick releases pending state
      act(() => {
        vi.advanceTimersByTime(0);
      });
      expect(checkBtn).toBeEnabled();

      // Second click calls onSubmit again
      fireEvent.click(checkBtn);
      expect(onSubmit).toHaveBeenCalledTimes(2);

      vi.useRealTimers();
    });

    it("supports Vietnamese locale for label, counter, and helper", async () => {
      await loadLocale("vi");
      await i18n.changeLanguage("vi");

      const user = userEvent.setup();
      render(
        <I18nextProvider i18n={i18n}>
          <ExerciseWriting
            prompt="Đề bài viết tiếng Anh"
            minWords={40}
            isSubmitted={false}
            onSubmit={() => {}}
            onContinue={() => {}}
          />
        </I18nextProvider>,
      );

      expect(screen.getByText("Bài viết của bạn")).toBeVisible();
      expect(screen.getByText("0 từ · tối thiểu 40 từ")).toBeInTheDocument();
      expect(
        screen.getByText("Viết thêm 40 từ để kiểm tra"),
      ).toBeInTheDocument();

      const textarea = screen.getByRole("textbox");
      await user.type(textarea, "Một hai ba bốn năm.");

      expect(screen.getByText("5 từ · tối thiểu 40 từ")).toBeInTheDocument();
      expect(
        screen.getByText("Viết thêm 35 từ để kiểm tra"),
      ).toBeInTheDocument();
    });
  });

  describe("MyWritingPage screen", () => {
    it("renders paginated submissions and opens feedback modal on click", async () => {
      signIn();
      const user = userEvent.setup();

      server.use(
        http.get("*/api/v1/writing/submissions", () => {
          return HttpResponse.json(mockSubmissions);
        }),
        http.get("*/api/v1/writing/attempts/:id/feedback", () => {
          return HttpResponse.json(mockFeedback);
        }),
        http.get("*/api/v1/attempts/:id", () => {
          return HttpResponse.json({
            id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
            activity_id: "act-1",
            user_id: "user-123",
            status: "graded",
            response: { text_answer: "The rapid advancement of technology." },
          });
        }),
      );

      await renderMyWritingPage();

      // Check submission card
      expect(await screen.findByText(/Graded/i)).toBeInTheDocument();
      expect(screen.getByText("Band 7.0")).toBeInTheDocument();

      // Click "View Feedback"
      const viewFeedbackBtn = screen.getByRole("button", {
        name: /View Feedback/i,
      });
      await user.click(viewFeedbackBtn);

      // Verify modal opened with feedback view
      expect(await screen.findByRole("dialog")).toBeInTheDocument();
      expect(
        await screen.findByText(/Criteria Breakdown/i),
      ).toBeInTheDocument();
      expect(screen.getByText(/Task Response/i)).toBeInTheDocument();

      // Close modal
      const closeButtons = screen.getAllByRole("button", { name: /Close/i });
      const firstCloseBtn = closeButtons[0];
      expect(firstCloseBtn).toBeDefined();
      if (firstCloseBtn) {
        await user.click(firstCloseBtn);
      }

      await waitFor(() => {
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      });
    });
  });
});
