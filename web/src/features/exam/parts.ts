import type { DraftAnswers, ExamSkill, SectionActivities } from "./types";

/**
 * The question types a sitting is made of, the way a paper exam lists its parts.
 *
 * This is the composition the server draws every sitting with
 * (`examSittingPlan` in internal/modules/learning/service/exam_pool.go). The
 * counts are facts about that plan, and the tags describe the form of each
 * part — nothing here claims more about an item than the plan guarantees.
 */

export type PartKey =
  "listening" | "reading" | "essay" | "rewrite" | "readAloud" | "respond";

export interface ExamPart {
  key: PartKey;
  section: number;
  skill: ExamSkill;
  /** Items one sitting draws for this part. */
  items: number;
  /** The fewest questions each item holds, where the pool checks it. */
  minQuestionsPerItem?: number;
  /** i18n keys under `exam.parts`. */
  tags: readonly string[];
}

export const EXAM_PARTS: readonly ExamPart[] = [
  {
    key: "listening",
    section: 1,
    skill: "listening",
    items: 3,
    minQuestionsPerItem: 4,
    tags: ["tagAudio", "tagMultipleChoice"],
  },
  {
    key: "reading",
    section: 2,
    skill: "reading",
    items: 2,
    tags: ["tagPassage", "tagMultipleChoice"],
  },
  {
    key: "essay",
    section: 3,
    skill: "writing",
    items: 1,
    tags: ["tagWriting"],
  },
  {
    key: "rewrite",
    section: 3,
    skill: "writing",
    items: 3,
    tags: ["tagGrammar"],
  },
  {
    key: "readAloud",
    section: 4,
    skill: "speaking",
    items: 2,
    tags: ["tagPronunciation"],
  },
  {
    key: "respond",
    section: 4,
    skill: "speaking",
    items: 2,
    tags: ["tagSpeaking"],
  },
];

export function partsOfSection(section: number): ExamPart[] {
  return EXAM_PARTS.filter((part) => part.section === section);
}

/** Which part an item belongs to, from its kind and, for speaking, its task. */
export function partOf(
  kind: string,
  config?: Record<string, unknown> | { task_type?: unknown },
): PartKey | undefined {
  switch (kind) {
    case "listening_comprehension":
    case "photo_description":
    case "question_response":
      return "listening";
    case "reading_comprehension":
    case "mcq_gap":
    case "text_completion":
      return "reading";
    case "writing_prompt":
      return "essay";
    case "grammar_sentence_transform":
      return "rewrite";
    case "speaking_task":
      return config?.task_type === "read_aloud" ? "readAloud" : "respond";
    default:
      return undefined;
  }
}

export function partByKey(key: PartKey | undefined): ExamPart | undefined {
  return EXAM_PARTS.find((part) => part.key === key);
}

/** One numbered entry on a sitting's question list. */
export interface QuestionSlot {
  number: number;
  section: number;
  activityId: string;
  /** Present for a question inside a comprehension set. */
  questionId?: string;
  answered: boolean;
}

function hasText(value: unknown): boolean {
  return typeof value === "string" && value.trim() !== "";
}

/**
 * Numbers every question of a sitting in order, the way the report does: each
 * question of a listening or reading set is a number, every other item is one.
 */
export function questionSlots(
  sections: readonly SectionActivities[],
  answers: DraftAnswers,
): QuestionSlot[] {
  const slots: QuestionSlot[] = [];
  for (const section of sections) {
    for (const activity of section.activities) {
      const answer = answers[activity.id] as
        Record<string, unknown> | undefined;
      const questions = activity.config?.questions ?? [];
      if (questions.length > 0) {
        const chosen = (answer?.answers ?? {}) as Record<string, unknown>;
        for (const question of questions) {
          slots.push({
            number: slots.length + 1,
            section: section.section_position,
            activityId: activity.id,
            questionId: question.id,
            answered: hasText(chosen[question.id]),
          });
        }
        continue;
      }
      slots.push({
        number: slots.length + 1,
        section: section.section_position,
        activityId: activity.id,
        answered:
          hasText(answer?.text_answer) ||
          hasText(answer?.answer) ||
          hasText(answer?.audio_object_key),
      });
    }
  }
  return slots;
}

/** The DOM id a question list entry scrolls to. */
export function slotAnchor(slot: Pick<QuestionSlot, "number">): string {
  return `sitting-q-${slot.number}`;
}
