import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it, vi } from "vitest";

import i18n, { initI18n } from "@/i18n";
import { MySpeakingPage } from "@/routes/MySpeakingPage";
import { ExerciseSpeaking } from "@/features/learning/components/Runner/ExerciseSpeaking";
import {
  computeWordDiff,
  type SpeakingFeedback,
  SpeakingFeedbackView,
  tokenizeWords,
} from "@/features/speaking";
import { useAuthStore } from "@/stores/authStore";

import { server } from "./msw-server";

const mockReadAloudFeedback: SpeakingFeedback = {
  id: "fb-123",
  attempt_id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
  user_id: "user-123",
  transcript: "The quick brown fox jumps over the lazy dog",
  read_aloud_accuracy: 92.5,
  words_per_minute: 135,
  audio_url: "https://storage.example.com/audio/test-attempt.webm?token=xyz",
  recording_key: "speaking/user-123/test-attempt.webm",
  criteria: [
    {
      name: "pronunciation",
      band: 7.0,
      comment_en: "Clear articulation and rhythm.",
      comment_vi: "Phát âm rõ ràng và giữ nhịp tốt.",
    },
    {
      name: "fluency",
      band: 7.5,
      comment_en: "Natural pacing without hesitation.",
      comment_vi: "Tốc độ tự nhiên, không ngập ngừng.",
    },
  ],
  feedback_en: "Excellent read-aloud delivery with high fidelity.",
  feedback_vi: "Đọc to rất xuất sắc với độ chính xác cao.",
  prompt_version: "speaking_grade.v1",
  asr_model: "whisper-large-v3",
  model: "gpt-4o-mini",
  created_at: "2026-09-15T10:00:00Z",
  updated_at: "2026-09-15T10:00:00Z",
};

const { audio_url: _unusedAudioUrl, ...purgedBaseFeedback } =
  mockReadAloudFeedback;
const mockPurgedFeedback: SpeakingFeedback = {
  ...purgedBaseFeedback,
  id: "fb-456",
  attempt_id: "0199a1c2-3d4e-7f80-9abc-def01234567b",
  recording_deleted_at: "2026-09-18T00:00:00Z",
};

const mockSubmissionsList = {
  items: [
    {
      attempt_id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
      status: "graded",
      task_type: "read_aloud",
      overall_band: 7.5,
      score: 93,
      feedback_en: "Excellent read-aloud delivery with high fidelity.",
      feedback_vi: "Đọc to rất xuất sắc với độ chính xác cao.",
      has_recording: true,
      created_at: "2026-09-15T10:00:00Z",
    },
    {
      attempt_id: "0199a1c2-3d4e-7f80-9abc-def01234567b",
      status: "graded",
      task_type: "respond",
      overall_band: 6.5,
      score: 65,
      feedback_en: "Cohesive response with good vocabulary.",
      feedback_vi: "Câu trả lời mạch lạc với từ vựng tốt.",
      has_recording: false,
      created_at: "2026-09-10T08:30:00Z",
    },
  ],
  total: 2,
  page: 1,
  page_size: 10,
};

async function renderMySpeakingPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const rootRoute = createRootRoute();
  const mySpeakingRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/my-speaking",
    component: () => (
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <MySpeakingPage />
        </QueryClientProvider>
      </I18nextProvider>
    ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([mySpeakingRoute]),
    history: createMemoryHistory({ initialEntries: ["/my-speaking"] }),
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

describe("Speaking Screens & Components (§3.6)", () => {
  beforeEach(async () => {
    await initI18n();
    useAuthStore.getState().clearAuth();
    localStorage.clear();

    server.use(
      // The recorder reads consent from the server now (BR-SPEAKING-03), so
      // every test that mounts it needs an answer. Default: already consented,
      // which is the state the recording tests are about.
      http.get("*/api/v1/speaking/consent", () =>
        HttpResponse.json({
          consented: true,
          consented_at: "2026-09-19T08:00:00Z",
        }),
      ),
      http.post("*/api/v1/speaking/consent", () =>
        HttpResponse.json({
          consented: true,
          consented_at: "2026-09-19T08:00:00Z",
        }),
      ),
      http.post("*/api/v1/speaking/upload-intent", () => {
        return HttpResponse.json({
          upload_url: "https://storage.example.com/upload",
          object_key: "speaking/test.webm",
          daily_recordings_used: 1,
          daily_recordings_limit: 10,
        });
      }),
    );
  });

  describe("computeWordDiff algorithm", () => {
    it("handles exact match correctly", () => {
      const diff = computeWordDiff(
        "the quick brown fox",
        "the quick brown fox",
      );
      expect(diff).toHaveLength(4);
      expect(diff.every((d) => d.type === "match")).toBe(true);
      expect(diff.map((d) => d.text)).toEqual(["the", "quick", "brown", "fox"]);
    });

    it("detects omissions, additions, and substitutions", () => {
      const diff = computeWordDiff(
        "the quick brown fox jumps",
        "the fast brown fox runs leaps",
      );

      // quick -> fast (substitution)
      const substitution = diff.find((d) => d.type === "substitution");
      expect(substitution).toBeDefined();
      expect(substitution?.text).toBe("fast");
      expect(substitution?.expected).toBe("quick");

      // jumps -> runs leaps (substitution or addition)
      expect(
        diff.some((d) => d.type === "addition" || d.type === "substitution"),
      ).toBe(true);
    });

    it("splits the way the Go scorer does, so the diff matches the score", () => {
      // domain.TokenizeWords ends a token at any punctuation, so "don't" is two
      // words to the percentage shown beside this diff. Splitting on whitespace
      // here counted it as one, and the explanation disagreed with the number.
      expect(tokenizeWords("I don't like it")).toEqual([
        "I",
        "don",
        "t",
        "like",
        "it",
      ]);
      expect(tokenizeWords("a well-known fact")).toEqual([
        "a",
        "well",
        "known",
        "fact",
      ]);
      // A symbol is dropped without ending the token, as the Go loop does.
      expect(tokenizeWords("a+b")).toEqual(["ab"]);
    });

    it("detects omissions when learner skips words", () => {
      const diff = computeWordDiff(
        "read all these words carefully",
        "read these carefully",
      );
      const omissions = diff.filter((d) => d.type === "omission");
      expect(omissions.length).toBeGreaterThanOrEqual(1);
      expect(
        omissions.some((o) => o.text === "all" || o.text === "words"),
      ).toBe(true);
    });
  });

  describe("SpeakingFeedbackView component", () => {
    it("renders read-aloud alignment diff, audio player, delivery numbers, and criteria", async () => {
      const user = userEvent.setup();
      render(
        <I18nextProvider i18n={i18n}>
          <SpeakingFeedbackView
            feedback={mockReadAloudFeedback}
            taskType="read_aloud"
            referenceText="The quick brown fox jumps over the lazy dog"
          />
        </I18nextProvider>,
      );

      // Read aloud evaluation header
      expect(screen.getByText(/Read Aloud Evaluation/i)).toBeInTheDocument();
      expect(screen.getAllByText(/93%/i).length).toBeGreaterThanOrEqual(1);

      // Audio player
      const audioElement = document.querySelector("audio");
      expect(audioElement).toBeInTheDocument();
      expect(audioElement).toHaveAttribute(
        "src",
        mockReadAloudFeedback.audio_url,
      );

      // Transcript and alignment diff
      expect(
        screen.getByText(/Transcript & Word Alignment/i),
      ).toBeInTheDocument();
      expect(screen.getByText("quick")).toBeInTheDocument();

      // Pacing & Delivery Metrics
      expect(
        screen.getByText(/Pacing & Delivery Measurements/i),
      ).toBeInTheDocument();
      expect(screen.getByText(/135\s+WPM/i)).toBeInTheDocument();
      expect(screen.getByText(/Speaking Rate/i)).toBeInTheDocument();

      // BR-SPEAKING-09 disclaimer
      expect(
        screen.getByText(/Note on Pronunciation Assessment/i),
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          /Pronunciation, intonation, and vowel clarity are not directly measured/i,
        ),
      ).toBeInTheDocument();

      // Criteria is expanded by default
      expect(screen.getByText(/Band 7.0/i)).toBeInTheDocument();
      expect(
        screen.getByText(/Clear articulation and rhythm/i),
      ).toBeInTheDocument();

      // Clicking accordion toggle collapses it
      const criteriaToggle = screen.getByRole("button", {
        name: /Criteria Assessment/i,
      });
      await user.click(criteriaToggle);
      expect(screen.queryByText(/Band 7.0/i)).not.toBeInTheDocument();

      // Clicking again expands it back
      await user.click(criteriaToggle);
      expect(screen.getByText(/Band 7.0/i)).toBeInTheDocument();
    });

    it("lets the learner tap a word they got wrong and hear it said correctly", async () => {
      const speak = vi.fn();
      class FakeUtterance {
        text: string;
        lang = "";
        rate = 1;
        onend: (() => void) | null = null;
        onerror: (() => void) | null = null;
        constructor(text: string) {
          this.text = text;
        }
      }
      vi.stubGlobal("speechSynthesis", { speak, cancel: vi.fn() });
      vi.stubGlobal("SpeechSynthesisUtterance", FakeUtterance);

      try {
        const user = userEvent.setup();
        render(
          <I18nextProvider i18n={i18n}>
            <SpeakingFeedbackView
              feedback={{
                ...mockReadAloudFeedback,
                // "quick" dropped, "lazy" heard as "crazy".
                transcript: "The brown fox jumps over the crazy dog",
              }}
              taskType="read_aloud"
              referenceText="The quick brown fox jumps over the lazy dog"
            />
          </I18nextProvider>,
        );

        // The word that was skipped: tapping it says the word the learner
        // should have said, which a tooltip on a phone never could.
        const missed = screen.getByRole("button", { name: /quick/i });
        await user.click(missed);
        expect(speak).toHaveBeenCalledTimes(1);
        expect((speak.mock.calls[0]![0] as FakeUtterance).text).toBe("quick");

        // The word that was replaced plays the expected one, not what was heard.
        const swapped = screen.getByRole("button", { name: /lazy/i });
        await user.click(swapped);
        expect((speak.mock.calls[1]![0] as FakeUtterance).text).toBe("lazy");
      } finally {
        vi.unstubAllGlobals();
      }
    });

    it("says plainly when nothing transcribed the recording", () => {
      render(
        <I18nextProvider i18n={i18n}>
          <SpeakingFeedbackView
            feedback={{ ...mockReadAloudFeedback, asr_model: "mock" }}
            taskType="read_aloud"
            referenceText="The quick brown fox jumps over the lazy dog"
          />
        </I18nextProvider>,
      );

      // A score computed against a fixed sample sentence is not an assessment,
      // and the screen has to be the thing that says so.
      expect(screen.getByText(/not a real transcription/i)).toBeInTheDocument();
    });

    it("displays 90-day privacy purged notice when recording has been purged", () => {
      render(
        <I18nextProvider i18n={i18n}>
          <SpeakingFeedbackView
            feedback={mockPurgedFeedback}
            taskType="read_aloud"
          />
        </I18nextProvider>,
      );

      expect(screen.getByText(/Audio purged \(90d\)/i)).toBeInTheDocument();
      expect(
        screen.getByText(
          /Audio recording was removed per 90-day privacy retention policy/i,
        ),
      ).toBeInTheDocument();
      expect(document.querySelector("audio")).toBeNull();
    });
  });

  describe("ExerciseSpeaking runner component", () => {
    it("asks for consent when the server says it was never given (BR-SPEAKING-03)", async () => {
      // The consent record lives on the server now. A learner who agreed on
      // another device is not asked again, and one who never agreed is — which
      // a localStorage flag could get wrong in both directions.
      server.use(
        http.get("*/api/v1/speaking/consent", () =>
          HttpResponse.json({ consented: false, consented_at: null }),
        ),
      );

      render(
        <I18nextProvider i18n={i18n}>
          <ExerciseSpeaking
            prompt="Read this sentence aloud."
            isSubmitted={false}
            onSubmit={() => {}}
            onContinue={() => {}}
          />
        </I18nextProvider>,
      );

      const record = await screen.findByRole("button", { name: /record/i });
      await userEvent.click(record);

      expect(await screen.findByText(/Voice Recording Consent/i)).toBeVisible();
    });

    it("records consent on the server when the learner accepts", async () => {
      let posted = false;
      server.use(
        http.get("*/api/v1/speaking/consent", () =>
          HttpResponse.json({ consented: false, consented_at: null }),
        ),
        http.post("*/api/v1/speaking/consent", () => {
          posted = true;
          return HttpResponse.json({
            consented: true,
            consented_at: "2026-09-19T08:00:00Z",
          });
        }),
      );

      render(
        <I18nextProvider i18n={i18n}>
          <ExerciseSpeaking
            prompt="Read this sentence aloud."
            isSubmitted={false}
            onSubmit={() => {}}
            onContinue={() => {}}
          />
        </I18nextProvider>,
      );

      const record = await screen.findByRole("button", { name: /record/i });
      await userEvent.click(record);
      await userEvent.click(
        await screen.findByRole("button", { name: /I Consent & Continue/i }),
      );

      await waitFor(() => expect(posted).toBe(true));
    });

    it("renders marking spinner when isMarking is true", () => {
      render(
        <I18nextProvider i18n={i18n}>
          <ExerciseSpeaking
            prompt="Read this sentence aloud."
            isSubmitted={false}
            isMarking={true}
            onSubmit={() => {}}
            onContinue={() => {}}
          />
        </I18nextProvider>,
      );

      expect(screen.getByText(/Grading your recording/i)).toBeInTheDocument();
      expect(
        screen.getByText(/Our speech engine is transcribing and analyzing/i),
      ).toBeInTheDocument();
    });

    it("renders background marking timeout notice and link to My Speaking", () => {
      render(
        <I18nextProvider i18n={i18n}>
          <ExerciseSpeaking
            prompt="Read this sentence aloud."
            isSubmitted={true}
            markingTimedOut={true}
            onNavigateToMySpeaking={() => {}}
            onSubmit={() => {}}
            onContinue={() => {}}
          />
        </I18nextProvider>,
      );

      expect(
        screen.getByText(/Marking continues in the background/i),
      ).toBeInTheDocument();
      expect(screen.getByText(/View My Speaking/i)).toBeInTheDocument();
    });
  });

  describe("MySpeakingPage screen", () => {
    it("renders paginated submissions and opens feedback modal on click", async () => {
      signIn();
      const user = userEvent.setup();

      server.use(
        http.get("*/api/v1/speaking/submissions", () => {
          return HttpResponse.json(mockSubmissionsList);
        }),
        http.get("*/api/v1/speaking/attempts/:id/feedback", () => {
          return HttpResponse.json(mockReadAloudFeedback);
        }),
      );

      await renderMySpeakingPage();

      // Verify list items rendered
      expect(await screen.findByText(/Read Aloud/i)).toBeInTheDocument();
      expect(screen.getByText(/Band 7.5/i)).toBeInTheDocument();
      expect(screen.getByText(/Spoken Response/i)).toBeInTheDocument();
      expect(screen.getByText(/Band 6.5/i)).toBeInTheDocument();

      // Click "View Feedback" on first submission
      const viewFeedbackButtons = screen.getAllByRole("button", {
        name: /View Feedback/i,
      });
      expect(viewFeedbackButtons.length).toBeGreaterThanOrEqual(1);
      const firstBtn = viewFeedbackButtons[0];
      if (firstBtn) {
        await user.click(firstBtn);
      }

      // Dialog opens
      expect(await screen.findByRole("dialog")).toBeInTheDocument();
      expect(
        await screen.findByText(/Pacing & Delivery Measurements/i),
      ).toBeInTheDocument();

      // Close modal
      const closeButtons = screen.getAllByRole("button", { name: /Close/i });
      const firstCloseBtn = closeButtons[0];
      if (firstCloseBtn) {
        await user.click(firstCloseBtn);
      }

      await waitFor(() => {
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      });
    });
  });
});
