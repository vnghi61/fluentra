import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import {
  ExerciseReading,
  type ExerciseReadingProps,
  type ItemResult,
  type ReadingQuestionItem,
  type ReadingSubmissionPayload,
} from "@/features/learning";

const sampleQuestions: ReadingQuestionItem[] = [
  {
    id: "q1",
    type: "multiple_choice",
    prompt: "What is the primary benefit mentioned in the text?",
    options: [
      { id: "opt_a", text: "Better focus and mindfulness" },
      { id: "opt_b", text: "Higher processing speed" },
    ],
  },
  {
    id: "q2",
    type: "true_false_not_given",
    prompt: "The author started photography in college.",
  },
  {
    id: "q3",
    type: "gap_fill",
    prompt: "Film photography requires significant _____ to master.",
  },
];

const defaultProps: ExerciseReadingProps = {
  passageTitle: "The Art of Slow Living",
  passage:
    "Film photography forces you to slow down and consider every shot before pressing the shutter. In a world obsessed with instant gratification, this deliberate constraint brings immense creative satisfaction and peace of mind.",
  questions: sampleQuestions,
  isSubmitted: false,
  onSubmit: vi.fn(),
  onContinue: vi.fn(),
};

describe("ExerciseReading Two-Phase Runner", () => {
  it("renders phase 1 (passage) and hides questions until learner finishes reading", () => {
    render(<ExerciseReading {...defaultProps} />);

    expect(screen.getByText("The Art of Slow Living")).toBeInTheDocument();
    expect(
      screen.getByText(/Film photography forces you to slow down/),
    ).toBeInTheDocument();

    // "I have finished reading" button should be visible
    expect(
      screen.getByRole("button", { name: /finished reading/i }),
    ).toBeInTheDocument();

    // Questions should not yet be rendered
    expect(
      screen.queryByText("What is the primary benefit mentioned in the text?"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("The author started photography in college."),
    ).not.toBeInTheDocument();
  });

  it("transitions to phase 2 when learner clicks 'finished reading', recording reading_ms", async () => {
    const onSubmit = vi.fn();
    render(<ExerciseReading {...defaultProps} onSubmit={onSubmit} />);

    // Click finished reading
    await userEvent.click(
      screen.getByRole("button", { name: /finished reading/i }),
    );

    // Questions should now appear
    expect(
      screen.getByText("What is the primary benefit mentioned in the text?"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("The author started photography in college."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Film photography requires significant _____ to master."),
    ).toBeInTheDocument();

    // Answering questions
    // 1. Multiple choice
    await userEvent.click(
      screen.getByRole("radio", { name: /Better focus and mindfulness/i }),
    );

    // 2. True/False/Not Given
    await userEvent.click(screen.getByRole("radio", { name: /^true$/i }));

    // 3. Gap fill
    const gapInput = screen.getByPlaceholderText(/type your answer/i);
    await userEvent.type(gapInput, "patience");

    // Submit
    const checkBtn = screen.getByRole("button", { name: /check/i });
    expect(checkBtn).not.toBeDisabled();
    await userEvent.click(checkBtn);

    expect(onSubmit).toHaveBeenCalledTimes(1);
    const firstCall = onSubmit.mock.calls[0];
    expect(firstCall).toBeDefined();
    const payload = firstCall?.[0] as ReadingSubmissionPayload;
    expect(typeof payload).toBe("object");
    if (typeof payload === "object") {
      expect(payload.answers).toEqual({
        q1: "opt_a",
        q2: "true",
        q3: "patience",
      });
      expect(typeof payload.reading_ms).toBe("number");
      expect(payload.reading_ms).toBeGreaterThanOrEqual(0);
    }
  });

  it("renders per-question item_results verdicts and revealed correct answers when submitted", () => {
    const itemResults: ItemResult[] = [
      { id: "q1", correct: true },
      { id: "q2", correct: false, correct_answer: "not_given" },
      { id: "q3", correct: true },
    ];

    render(
      <ExerciseReading
        {...defaultProps}
        isSubmitted
        isCorrect={false}
        itemResults={itemResults}
        feedback="You got 2 out of 3 questions right."
        explanation={{
          text: "The passage does not state when the author began photography.",
          text_vi: "Đoạn văn không nói tác giả bắt đầu chụp ảnh khi nào.",
        }}
      />,
    );

    // In submitted mode, questions and verdicts are displayed
    expect(
      screen.getByText("What is the primary benefit mentioned in the text?"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("You got 2 out of 3 questions right."),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "The passage does not state when the author began photography.",
      ),
    ).toBeInTheDocument();

    // Revealed correct answer for q2
    expect(screen.getByText("not_given")).toBeInTheDocument();
  });
});
