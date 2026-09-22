import { fireEvent, render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ActivityFieldsEditor } from "@/features/studio/components/ActivityFieldsEditor";
import i18n, { initI18n } from "@/i18n";
import {
  activityIssues,
  emptyFields,
  readActivity,
  serialiseActivity,
  stableShuffle,
  type DraftActivity,
} from "@/features/studio/model/activityKinds";

const activity = (
  kind: DraftActivity["kind"],
  fields: Partial<DraftActivity["fields"]>,
): DraftActivity => ({
  id: "a-1",
  kind,
  title: "Activity 1",
  fields: { ...emptyFields(), ...fields },
});

describe("studio activity kinds", () => {
  it("saves a multiple choice as the body Gate 1 parses: four options and a key", () => {
    const saved = serialiseActivity(
      activity("vocab_multiple_choice", {
        prompt: "Pick the synonym of 'big'.",
        options: ["large", "tiny", "slow", "cold"],
        correctIndex: 0,
      }),
    );

    expect(saved.kind).toBe("vocab_multiple_choice");
    expect(saved.body).toEqual({
      prompt: "Pick the synonym of 'big'.",
      options: [
        { id: "a", text: "large" },
        { id: "b", text: "tiny" },
        { id: "c", text: "slow" },
        { id: "d", text: "cold" },
      ],
      correct_option_id: "a",
    });
    // The runner renders config; the server redacts the key out of it.
    expect(saved.config).toEqual(saved.body);
  });

  it("shuffles a reorder's words so it is not already solved, and keeps the sentence as the key", () => {
    const body = serialiseActivity(
      activity("vocab_reorder", { answer: "She goes to school" }),
    ).body as { tokens: string[]; correct_answer: string };

    expect(body.correct_answer).toBe("She goes to school");
    expect([...body.tokens].sort()).toEqual(["She", "goes", "school", "to"].sort());
    expect(body.tokens.join(" ")).not.toBe("She goes to school");
  });

  it("pairs each word with its meaning by id, whatever order the meanings are shown in", () => {
    const body = serialiseActivity(
      activity("vocab_match", {
        pairs: [
          { word: "cat", meaning: "con mèo" },
          { word: "dog", meaning: "con chó" },
          { word: "", meaning: "" },
        ],
      }),
    ).body as {
      words: { id: string; text: string }[];
      definitions: { id: string; text: string }[];
      correct_pairs: Record<string, string>;
    };

    expect(body.words).toHaveLength(2);
    const meaningOf = (word: string) => {
      const w = body.words.find((x) => x.text === word);
      return body.definitions.find((d) => d.id === body.correct_pairs[w?.id ?? ""])?.text;
    };
    expect(meaningOf("cat")).toBe("con mèo");
    expect(meaningOf("dog")).toBe("con chó");
  });

  it("marks a read-aloud speaking task with its task type", () => {
    const saved = serialiseActivity(
      activity("speaking_task", { taskType: "read_aloud", referenceText: "x" }),
    );
    expect(saved.task_type).toBe("read_aloud");
    expect(saved.body).toMatchObject({ task_type: "read_aloud", reference_text: "x" });
  });

  it("opens a draft saved by the old editor under the backend's kind names", () => {
    const read = readActivity(
      { id: "a-9", kind: "fill_blank", title: "Old", prompt: "Fill it", expected_answer: "go" },
      "fallback",
    );
    expect(read.kind).toBe("vocab_gap_fill");
    expect(read.fields.prompt).toBe("Fill it");
    expect(read.fields.answer).toBe("go");

    expect(readActivity({ kind: "pronunciation" }, "x").fields.taskType).toBe("read_aloud");
    expect(readActivity({ kind: "dialogue" }, "x").fields.taskType).toBe("respond");
  });

  it("reports what Gate 1 would reject before the creator submits", () => {
    expect(activityIssues(activity("vocab_multiple_choice", {}))).toEqual(
      expect.arrayContaining(["prompt", "fourOptions"]),
    );
    expect(
      activityIssues(
        activity("grammar_tense_choice", {
          prompt: "q",
          options: ["a", "b", "c", "d"],
        }),
      ),
    ).toEqual(["explanationVi"]);
    expect(activityIssues(activity("writing_prompt", { prompt: "too short" }))).toEqual(
      expect.arrayContaining(["writingPrompt", "modelAnswer"]),
    );
  });

  it("shuffles the same way on every save", () => {
    expect(stableShuffle([1, 2, 3, 4, 5], "a-1")).toEqual(stableShuffle([1, 2, 3, 4, 5], "a-1"));
  });
});

describe("ActivityFieldsEditor", () => {
  beforeEach(async () => {
    await initI18n("vi");
  });

  const renderKind = (kind: DraftActivity["kind"]) =>
    render(
      <I18nextProvider i18n={i18n}>
        <ActivityFieldsEditor kind={kind} fields={emptyFields()} idPrefix="a1" onChange={() => {}} />
      </I18nextProvider>,
    );

  it("shows different inputs for each kind, in Vietnamese", () => {
    const { unmount } = renderKind("vocab_multiple_choice");
    expect(screen.getByLabelText("Câu hỏi")).toBeInTheDocument();
    expect(screen.getAllByRole("radio")).toHaveLength(4);
    unmount();

    const flash = renderKind("vocab_flashcard");
    expect(screen.getByLabelText("Từ vựng")).toBeInTheDocument();
    expect(screen.getByLabelText("Nghĩa tiếng Việt")).toBeInTheDocument();
    expect(screen.queryAllByRole("radio")).toHaveLength(0);
    flash.unmount();

    renderKind("reading_comprehension");
    expect(screen.getByLabelText("Đoạn văn")).toBeInTheDocument();
    expect(screen.getByText("Câu hỏi 4")).toBeInTheDocument();
  });

  it("reports an edit as a patch of the field it belongs to", () => {
    const onChange = vi.fn();
    render(
      <I18nextProvider i18n={i18n}>
        <ActivityFieldsEditor kind="vocab_gap_fill" fields={emptyFields()} idPrefix="a1" onChange={onChange} />
      </I18nextProvider>,
    );
    fireEvent.change(screen.getByLabelText("Từ điền vào chỗ trống"), { target: { value: "went" } });
    expect(onChange).toHaveBeenCalledWith({ answer: "went" });
  });
});
