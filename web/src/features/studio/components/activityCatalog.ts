import {
  ArrowDownUp,
  ArrowLeftRight,
  BookOpen,
  Clock,
  Headphones,
  Layers,
  ListChecks,
  Mic,
  Paperclip,
  PenLine,
  Quote,
  Repeat,
  TextCursorInput,
  type LucideIcon,
} from "lucide-react";

import type { ActivityKind } from "../model/activityKinds";

/** One icon per kind, so a long lesson reads at a glance. */
export const KIND_ICON: Record<ActivityKind, LucideIcon> = {
  vocab_multiple_choice: ListChecks,
  vocab_gap_fill: TextCursorInput,
  vocab_flashcard: Layers,
  vocab_listen_type: Headphones,
  vocab_match: ArrowLeftRight,
  vocab_reorder: ArrowDownUp,
  vocab_context_choice: Quote,
  grammar_tense_choice: Clock,
  grammar_sentence_transform: Repeat,
  reading_comprehension: BookOpen,
  writing_prompt: PenLine,
  speaking_task: Mic,
  lesson_material: Paperclip,
};

/** The graded kinds, grouped as the "add exercise" menu shows them. */
export const EXERCISE_GROUPS: {
  key: "vocab" | "grammar" | "skills";
  kinds: ActivityKind[];
}[] = [
  {
    key: "vocab",
    kinds: [
      "vocab_multiple_choice",
      "vocab_gap_fill",
      "vocab_flashcard",
      "vocab_listen_type",
      "vocab_match",
      "vocab_reorder",
      "vocab_context_choice",
    ],
  },
  {
    key: "grammar",
    kinds: ["grammar_tense_choice", "grammar_sentence_transform"],
  },
  {
    key: "skills",
    kinds: ["reading_comprehension", "writing_prompt", "speaking_task"],
  },
];

export const EXERCISE_KINDS: ActivityKind[] = EXERCISE_GROUPS.flatMap(
  (g) => g.kinds,
);
