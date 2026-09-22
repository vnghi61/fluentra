/**
 * The activity kinds a creator may author, and how each one's editor fields
 * become the `body` Gate 1 grades and the `config` the lesson runner renders.
 *
 * The kind names and body shapes are the backend's, not ours: the list is
 * `AllowedActivityKinds` in internal/modules/studio/domain/models.go, and each
 * shape is what learning/service/practice_pool.go and exam_pool.go parse. The
 * editor used to offer its own names ("multiple_choice", "fill_blank", ...) and
 * save only a prompt, so every draft failed Gate 1 on kind and on empty body.
 *
 * `config` is the body itself, as every other authoring path in the codebase
 * does: the lesson service redacts answer keys (`correct_option_id`,
 * `correct_answer`, `correct_pairs`, `model_answer`) before a learner sees it.
 */

export const ACTIVITY_KINDS = [
  "vocab_multiple_choice",
  "vocab_gap_fill",
  "vocab_flashcard",
  "vocab_listen_type",
  "vocab_match",
  "vocab_reorder",
  "vocab_context_choice",
  "grammar_tense_choice",
  "grammar_sentence_transform",
  "reading_comprehension",
  "writing_prompt",
  "speaking_task",
  "lesson_material",
] as const;

export type ActivityKind = (typeof ACTIVITY_KINDS)[number];

export interface ChoiceFields {
  prompt: string;
  /** Exactly four, as Gate 1 requires. */
  options: [string, string, string, string];
  correctIndex: number;
  explanationVi: string;
}

export interface ReadingQuestionFields {
  prompt: string;
  options: [string, string, string, string];
  correctIndex: number;
  explanationVi: string;
}

export interface ActivityFields {
  prompt: string;
  /** Choice kinds. */
  options: [string, string, string, string];
  correctIndex: number;
  explanationVi: string;
  /** Gap fill: the words either side of the blank. The answer goes in `answer`. */
  sentenceBefore: string;
  sentenceAfter: string;
  /** The typed / spoken / rebuilt answer, or the flashcard's word. */
  answer: string;
  /** Sentence transform: other answers that also count. One per line. */
  acceptable: string;
  /** Flashcard. */
  ipa: string;
  definition: string;
  definitionVi: string;
  example: string;
  /** Match: a word and its meaning per row. */
  pairs: { word: string; meaning: string }[];
  /** Context choice: the sentence the word sits in. */
  sentence: string;
  /** Reading. */
  passageTitle: string;
  passage: string;
  questions: ReadingQuestionFields[];
  /** Writing. */
  modelAnswer: string;
  minWords: number;
  /** Speaking. */
  taskType: "read_aloud" | "respond";
  referenceText: string;
  /**
   * Lesson material (a document or a video). `resourceId` is the creator's own
   * private resource; `materialStatus` is the editor's own view of its
   * processing, never saved to the server.
   */
  materialKind: "document" | "video";
  materialTitle: string;
  materialDescription: string;
  resourceId: string;
  rightsConfirmed: boolean;
  materialStatus: "idle" | "uploading" | "processing" | "ready" | "failed";
}

export interface DraftActivity {
  id: string;
  kind: ActivityKind;
  title: string;
  fields: ActivityFields;
}

const blankOptions = (): [string, string, string, string] => ["", "", "", ""];

export const blankQuestion = (): ReadingQuestionFields => ({
  prompt: "",
  options: blankOptions(),
  correctIndex: 0,
  explanationVi: "",
});

export function emptyFields(): ActivityFields {
  return {
    prompt: "",
    options: blankOptions(),
    correctIndex: 0,
    explanationVi: "",
    sentenceBefore: "",
    sentenceAfter: "",
    answer: "",
    acceptable: "",
    ipa: "",
    definition: "",
    definitionVi: "",
    example: "",
    pairs: [
      { word: "", meaning: "" },
      { word: "", meaning: "" },
      { word: "", meaning: "" },
    ],
    sentence: "",
    passageTitle: "",
    passage: "",
    questions: [
      blankQuestion(),
      blankQuestion(),
      blankQuestion(),
      blankQuestion(),
    ],
    modelAnswer: "",
    minWords: 80,
    taskType: "read_aloud",
    referenceText: "",
    materialKind: "document",
    materialTitle: "",
    materialDescription: "",
    resourceId: "",
    rightsConfirmed: false,
    materialStatus: "idle",
  };
}

/** The kinds the editor offered before it spoke the backend's names. */
const LEGACY_KINDS: Record<
  string,
  { kind: ActivityKind; taskType?: "read_aloud" | "respond" }
> = {
  multiple_choice: { kind: "vocab_multiple_choice" },
  fill_blank: { kind: "vocab_gap_fill" },
  matching: { kind: "vocab_match" },
  ordering: { kind: "vocab_reorder" },
  translation: { kind: "grammar_sentence_transform" },
  error_correction: { kind: "grammar_sentence_transform" },
  open_response: { kind: "writing_prompt" },
  pronunciation: { kind: "speaking_task", taskType: "read_aloud" },
  dialogue: { kind: "speaking_task", taskType: "respond" },
  dictation: { kind: "vocab_listen_type" },
  short_answer: { kind: "vocab_gap_fill" },
};

function isKind(value: unknown): value is ActivityKind {
  return (
    typeof value === "string" &&
    (ACTIVITY_KINDS as readonly string[]).includes(value)
  );
}

/**
 * Reads an activity out of a saved draft: the current shape, which carries its
 * editor `fields`, or the first shape, which carried only a prompt and an
 * expected answer under the editor's own kind names.
 */
export function readActivity(raw: unknown, fallbackId: string): DraftActivity {
  const obj = (raw && typeof raw === "object" ? raw : {}) as Record<
    string,
    unknown
  >;
  const id = typeof obj.id === "string" ? obj.id : fallbackId;
  const title = typeof obj.title === "string" ? obj.title : "";
  const fields = emptyFields();

  const saved = obj.fields;
  if (saved && typeof saved === "object") {
    Object.assign(fields, saved);
  } else {
    if (typeof obj.prompt === "string") fields.prompt = obj.prompt;
    if (typeof obj.expected_answer === "string")
      fields.answer = obj.expected_answer;
  }

  // A material saved before its editor fields existed still carries the shape
  // the server reads; read it back so the file picker is not empty.
  const material = obj.material;
  if (material && typeof material === "object") {
    const m = material as Record<string, unknown>;
    if (typeof m.resource_id === "string") fields.resourceId = m.resource_id;
    if (m.material_kind === "video" || m.material_kind === "document") {
      fields.materialKind = m.material_kind;
    }
    if (typeof m.title === "string") fields.materialTitle = m.title;
    if (typeof m.description === "string")
      fields.materialDescription = m.description;
    if (m.rights_confirmed === true) fields.rightsConfirmed = true;
    if (fields.resourceId) fields.materialStatus = "processing";
  }

  if (isKind(obj.kind)) return { id, kind: obj.kind, title, fields };
  const legacy =
    typeof obj.kind === "string" ? LEGACY_KINDS[obj.kind] : undefined;
  if (legacy?.taskType) fields.taskType = legacy.taskType;
  return { id, kind: legacy?.kind ?? "vocab_multiple_choice", title, fields };
}

const OPTION_IDS = ["a", "b", "c", "d"] as const;

function choiceOptions(options: readonly string[]) {
  return options.map((text, i) => ({
    id: OPTION_IDS[i] ?? String(i),
    text: text.trim(),
  }));
}

/**
 * A shuffle that gives the same order for the same activity on every save, so
 * re-saving a draft does not reshuffle what a reviewer already looked at. It
 * never leaves the order unchanged: the runner shows tokens and meanings in the
 * order they are stored, and an unshuffled reorder is already solved.
 */
export function stableShuffle<T>(items: readonly T[], seed: string): T[] {
  let h = 2166136261;
  for (let i = 0; i < seed.length; i++) {
    h ^= seed.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  const out = [...items];
  for (let i = out.length - 1; i > 0; i--) {
    h = Math.imul(h ^ (h >>> 15), 2246822507) >>> 0;
    const j = h % (i + 1);
    [out[i], out[j]] = [out[j] as T, out[i] as T];
  }
  if (out.length > 1 && out.every((item, i) => item === items[i])) {
    out.push(out.shift() as T);
  }
  return out;
}

const explanation = (vi: string) =>
  vi.trim() ? { explanation: { text_vi: vi.trim() } } : {};

/** The body Gate 1 verifies and publishes, which is also the runner's config. */
export function buildBody(activity: DraftActivity): Record<string, unknown> {
  const f = activity.fields;
  switch (activity.kind) {
    case "vocab_multiple_choice":
    case "grammar_tense_choice":
      return {
        prompt: f.prompt.trim(),
        options: choiceOptions(f.options),
        correct_option_id: OPTION_IDS[f.correctIndex],
        ...explanation(f.explanationVi),
      };
    case "vocab_context_choice":
      return {
        prompt: f.prompt.trim(),
        sentence: f.sentence.trim(),
        options: choiceOptions(f.options),
        correct_option_id: OPTION_IDS[f.correctIndex],
        ...explanation(f.explanationVi),
      };
    case "vocab_gap_fill":
      return {
        prompt: f.prompt.trim(),
        sentence_before: f.sentenceBefore.trim(),
        sentence_after: f.sentenceAfter.trim(),
        correct_answer: f.answer.trim(),
        ...explanation(f.explanationVi),
      };
    case "vocab_flashcard":
      return {
        prompt: f.prompt.trim(),
        target_word: f.answer.trim(),
        correct_answer: f.answer.trim(),
        ipa: f.ipa.trim(),
        definition: f.definition.trim(),
        definition_vi: f.definitionVi.trim(),
        example_sentence: f.example.trim(),
      };
    case "vocab_listen_type":
      return {
        prompt: f.prompt.trim(),
        // Spoken by the learner's browser, so it reaches the client by design.
        audio_text: f.answer.trim(),
        correct_answer: f.answer.trim(),
      };
    case "vocab_reorder": {
      const sentence = f.answer.trim();
      const tokens = sentence.split(/\s+/).filter(Boolean);
      return {
        prompt: f.prompt.trim(),
        tokens: stableShuffle(tokens, activity.id),
        correct_answer: sentence,
      };
    }
    case "vocab_match": {
      const pairs = f.pairs.filter((p) => p.word.trim() && p.meaning.trim());
      const words = pairs.map((p, i) => ({
        id: `w${i + 1}`,
        text: p.word.trim(),
      }));
      const definitions = pairs.map((p, i) => ({
        id: `d${i + 1}`,
        text: p.meaning.trim(),
      }));
      return {
        prompt: f.prompt.trim(),
        words,
        definitions: stableShuffle(definitions, activity.id),
        correct_pairs: Object.fromEntries(
          words.map((w, i) => [w.id, `d${i + 1}`]),
        ),
      };
    }
    case "grammar_sentence_transform":
      return {
        prompt: f.prompt.trim(),
        correct_answer: f.answer.trim(),
        acceptable: f.acceptable
          .split("\n")
          .map((line) => line.trim())
          .filter(Boolean),
        ...explanation(f.explanationVi),
      };
    case "reading_comprehension":
      return {
        passage_title: f.passageTitle.trim(),
        passage: f.passage.trim(),
        questions: f.questions.map((q, i) => ({
          id: `q${i + 1}`,
          prompt: q.prompt.trim(),
          options: choiceOptions(q.options),
          correct_option_id: OPTION_IDS[q.correctIndex],
          ...explanation(q.explanationVi),
        })),
      };
    case "writing_prompt":
      return {
        prompt: f.prompt.trim(),
        model_answer: f.modelAnswer.trim(),
        min_words: f.minWords,
      };
    case "speaking_task":
      return f.taskType === "read_aloud"
        ? {
            task_type: "read_aloud",
            prompt: f.prompt.trim(),
            reference_text: f.referenceText.trim(),
          }
        : { task_type: "respond", prompt: f.prompt.trim() };
    case "lesson_material":
      // The draft body carries the resource id and title only, so Gate 1's
      // non-empty check passes and no URL is ever written into it. The publish
      // step replaces it with the copied object keys. `resource_id` is omitted
      // when empty: the server decodes it as a UUID, and "" is not one.
      return {
        ...(f.resourceId.trim() ? { resource_id: f.resourceId.trim() } : {}),
        title: f.materialTitle.trim(),
      };
  }
}

/** What the draft stores for one activity: editor state plus what the server reads. */
export function serialiseActivity(
  activity: DraftActivity,
): Record<string, unknown> {
  const body = buildBody(activity);
  const f = activity.fields;
  return {
    id: activity.id,
    title: activity.title,
    kind: activity.kind,
    ...(activity.kind === "speaking_task" && {
      task_type: activity.fields.taskType,
    }),
    ...(activity.kind === "lesson_material" && {
      material: {
        ...(f.resourceId.trim() ? { resource_id: f.resourceId.trim() } : {}),
        material_kind: f.materialKind,
        title: f.materialTitle.trim(),
        description: f.materialDescription.trim(),
        rights_confirmed: f.rightsConfirmed,
      },
    }),
    // A material is completed, not scored (D20-8).
    weight: activity.kind === "lesson_material" ? 0 : 1,
    fields: activity.fields,
    body,
    config: body,
  };
}

const words = (text: string) => text.trim().split(/\s+/).filter(Boolean).length;

function choiceIssues(
  prompt: string,
  options: readonly string[],
  issues: string[],
) {
  if (!prompt.trim()) issues.push("prompt");
  if (options.some((o) => !o.trim())) issues.push("fourOptions");
  const norm = options.map((o) => o.trim().toLowerCase()).filter(Boolean);
  if (new Set(norm).size !== norm.length) issues.push("distinctOptions");
}

/**
 * The problems Gate 1 would reject this activity for, as i18n keys under
 * `studio.activity.issue`. Caught here so a creator is not told about them one
 * submission round-trip at a time.
 */
export function activityIssues(activity: DraftActivity): string[] {
  const f = activity.fields;
  const issues: string[] = [];
  switch (activity.kind) {
    case "vocab_multiple_choice":
      choiceIssues(f.prompt, f.options, issues);
      break;
    case "grammar_tense_choice":
      choiceIssues(f.prompt, f.options, issues);
      if (!f.explanationVi.trim()) issues.push("explanationVi");
      break;
    case "vocab_context_choice":
      choiceIssues(f.sentence || f.prompt, f.options, issues);
      break;
    case "vocab_gap_fill":
      if (!f.answer.trim()) issues.push("answer");
      if (!f.sentenceBefore.trim() && !f.sentenceAfter.trim())
        issues.push("sentence");
      break;
    case "vocab_flashcard":
      if (!f.answer.trim()) issues.push("word");
      if (!f.definition.trim()) issues.push("definition");
      break;
    case "vocab_listen_type":
      if (!f.answer.trim()) issues.push("answer");
      break;
    case "vocab_reorder":
      if (words(f.answer) < 2) issues.push("reorderWords");
      break;
    case "vocab_match":
      if (f.pairs.filter((p) => p.word.trim() && p.meaning.trim()).length < 2)
        issues.push("pairs");
      break;
    case "grammar_sentence_transform":
      if (!f.prompt.trim()) issues.push("prompt");
      if (!f.answer.trim()) issues.push("answer");
      if (!f.explanationVi.trim()) issues.push("explanationVi");
      if (
        f.answer.trim() &&
        f.prompt.toLowerCase().includes(f.answer.trim().toLowerCase())
      ) {
        issues.push("answerInPrompt");
      }
      break;
    case "reading_comprehension":
      if (!f.passage.trim()) issues.push("passage");
      if (f.questions.length < 4 || f.questions.length > 6)
        issues.push("questionCount");
      for (const q of f.questions) {
        choiceIssues(q.prompt, q.options, issues);
        if (!q.explanationVi.trim()) issues.push("explanationVi");
      }
      break;
    case "writing_prompt":
      if (words(f.prompt) < 15) issues.push("writingPrompt");
      if (words(f.modelAnswer) < 80) issues.push("modelAnswer");
      break;
    case "speaking_task":
      if (f.taskType === "read_aloud") {
        const n = words(f.referenceText);
        if (n < 15 || n > 100) issues.push("referenceText");
      } else if (words(f.prompt) < 8) {
        issues.push("respondPrompt");
      }
      break;
    case "lesson_material":
      if (!f.resourceId.trim()) {
        issues.push("materialFile");
      } else if (f.materialStatus !== "ready") {
        // A draft may be saved while a video processes; it may not be submitted
        // until the renditions the runner needs are ready.
        issues.push("materialProcessing");
      }
      if (!f.materialTitle.trim()) issues.push("materialTitle");
      if (!f.rightsConfirmed) issues.push("materialRights");
      break;
  }
  return [...new Set(issues)];
}
