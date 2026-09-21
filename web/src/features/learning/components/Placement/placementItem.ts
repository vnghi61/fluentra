import type { PlacementItem, PlacementResponse } from "../../api/placement";

export interface ChoiceOption {
  id: string;
  text: string;
}

export interface ItemQuestion {
  id: string;
  prompt: string;
  options: ChoiceOption[];
}

/** What the screen reads from an item's redacted body. */
export interface ParsedItem {
  title: string;
  prompt: string;
  passage: string;
  options: ChoiceOption[];
  questions: ItemQuestion[];
  minWords: number;
  speakingSeconds: number;
}

/** The learner's answer so far: one choice, or one per question. */
export interface ItemAnswer {
  selected: string | null;
  answers: Readonly<Record<string, string>>;
}

export const emptyAnswer: ItemAnswer = { selected: null, answers: {} };

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function text(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function count(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function options(value: unknown): ChoiceOption[] {
  if (!Array.isArray(value)) return [];
  return value.flatMap((option: unknown) =>
    isRecord(option) && text(option.id)
      ? [{ id: text(option.id), text: text(option.text) }]
      : [],
  );
}

function questions(value: unknown): ItemQuestion[] {
  if (!Array.isArray(value)) return [];
  return value.flatMap((question: unknown) =>
    isRecord(question) && text(question.id)
      ? [
          {
            id: text(question.id),
            prompt: text(question.prompt),
            options: options(question.options),
          },
        ]
      : [],
  );
}

export function parseItem(config: Record<string, unknown>): ParsedItem {
  return {
    title: text(config.title) || text(config.passage_title),
    prompt: text(config.prompt),
    passage: text(config.passage),
    options: options(config.options),
    questions: questions(config.questions),
    minWords: count(config.min_words),
    speakingSeconds: count(config.speaking_time_seconds),
  };
}

/** A passage or a clip is answered question by question. */
export function hasQuestions(item: PlacementItem): boolean {
  return item.skill === "reading" || item.skill === "listening";
}

export function isComplete(item: PlacementItem, answer: ItemAnswer): boolean {
  if (!hasQuestions(item)) return answer.selected !== null;
  const parsed = parseItem(item.config);
  return (
    parsed.questions.length > 0 &&
    parsed.questions.every((q) => answer.answers[q.id] !== undefined)
  );
}

/** The response shape each kind's grader reads. */
export function toResponse(
  item: PlacementItem,
  answer: ItemAnswer,
): PlacementResponse {
  if (hasQuestions(item)) return { answers: answer.answers };
  return { selected_option_id: answer.selected ?? "" };
}

export function formatClock(totalSeconds: number): string {
  const safe = Math.max(0, Math.floor(totalSeconds));
  const minutes = Math.floor(safe / 60);
  const seconds = safe % 60;
  return `${minutes.toString().padStart(2, "0")}:${seconds.toString().padStart(2, "0")}`;
}

export function wordCount(value: string): number {
  const trimmed = value.trim();
  return trimmed ? trimmed.split(/\s+/).length : 0;
}
