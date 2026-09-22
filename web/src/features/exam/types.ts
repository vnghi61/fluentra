export type ExamLevel = "A2" | "B1" | "B2";
export type ExamMode = "exam" | "practice";
export type ExamStatus = "in_progress" | "completed" | "expired";
export type ReportStatus = "pending" | "ready" | "partial";
export type ExamSkill = "listening" | "reading" | "writing" | "speaking";

export const EXAM_LEVELS: readonly ExamLevel[] = ["A2", "B1", "B2"];
export const EXAM_SKILLS: readonly ExamSkill[] = [
  "listening",
  "reading",
  "writing",
  "speaking",
];

export interface ExamSectionTemplate {
  id: string;
  exam_id: string;
  position: number;
  skill: ExamSkill;
  exam_duration_minutes: number;
  item_count: number;
  item_kinds: string[];
}

export interface ExamTemplate {
  id: string;
  slug: string;
  title_en: string;
  title_vi: string;
  description_en?: string;
  description_vi?: string;
  level: ExamLevel;
  format: string;
  total_minutes: number;
  sections?: ExamSectionTemplate[];
}

export interface ChoiceOption {
  id: string;
  text: string;
}

export interface ChoiceQuestion {
  id: string;
  prompt: string;
  type?: string;
  options?: ChoiceOption[];
}

/** An item's learner-facing body. The server has removed every answer key. */
export interface SittingActivityConfig {
  title?: string;
  passage?: string;
  questions?: ChoiceQuestion[];
  prompt?: string;
  topic?: string;
  min_words?: number;
  task_type?: "read_aloud" | "respond";
  reference_text?: string;
  speaking_time_seconds?: number;
  options?: ChoiceOption[];
  statements?: ChoiceOption[];
  responses?: ChoiceOption[];
  sentence?: string;
  image_url?: string;
}

export interface SittingActivity {
  id: string;
  kind: string;
  content_version_id: string;
  config?: SittingActivityConfig;
  weight: number;
}

export interface SectionActivities {
  section_position: number;
  skill: ExamSkill;
  activities: SittingActivity[];
}

/** Answer shapes, as each kind's grader reads them. */
export interface ChoiceAnswer {
  answers: Record<string, string>;
}
export interface SingleChoiceAnswer {
  selected_option_id: string;
}
export interface EssayAnswer {
  text_answer: string;
}
export interface RewriteAnswer {
  answer: string;
}
export interface RecordingAnswer {
  audio_object_key: string;
}
export type SittingAnswer =
  | ChoiceAnswer
  | SingleChoiceAnswer
  | EssayAnswer
  | RewriteAnswer
  | RecordingAnswer;
export type DraftAnswers = Record<string, SittingAnswer>;


export interface ExamAttempt {
  id: string;
  exam_id: string;
  exam_slug?: string;
  exam_title?: string;
  level?: ExamLevel;
  mode: ExamMode;
  chosen_duration_minutes: number;
  /** Practice mode only: true when the sitting opted out of a visible time limit. */
  unlimited: boolean;
  started_at: string;
  deadline_at: string;
  remaining_seconds: number;
  current_section: number;
  section_deadline_at?: string;
  section_remaining_seconds?: number;
  status: ExamStatus;
  submitted_at?: string;
  section_activities?: SectionActivities[];
  draft_answers?: DraftAnswers;
  server_time: string;
}

export interface ExamAttemptListResponse {
  items: ExamAttempt[];
  total: number;
  sittings_today: number;
  daily_limit: number;
}

export interface StartSittingRequest {
  mode: ExamMode;
  chosen_duration_minutes?: number;
  /** Practice mode only: skips chosen_duration_minutes and runs without a visible time limit. */
  unlimited?: boolean;
  /** Practice mode only: the section positions to sit. Omitted means all. */
  sections?: number[];
}

export type IntegrityKind = "tab_hidden" | "window_blurred" | "paste";

export interface IntegrityEvent {
  kind: IntegrityKind;
}

export interface SaveAnswersRequest {
  section_number?: number;
  answers: DraftAnswers;
  integrity_events?: IntegrityEvent[];
}

export interface SaveAnswersResult {
  saved: boolean;
  remaining_seconds: number;
  current_section: number;
  section_remaining_seconds?: number;
}

export interface CompleteSectionResult {
  current_section: number;
  remaining_seconds: number;
  section_remaining_seconds?: number;
  submitted: boolean;
}

export interface SubmitExamResult {
  attempt_id: string;
  status: ExamStatus;
  report_status: ReportStatus;
}

export interface AnswerExplanation {
  text: string;
  text_vi: string;
}

export interface QuestionResult {
  id: string;
  correct: boolean;
  correct_answer?: string;
  explanation?: AnswerExplanation;
}

export type ItemStatus = "graded" | "pending" | "failed" | "unanswered";
export type SectionStatus = "scored" | "pending" | "not_scored";

export interface ExamItemOutcome {
  activity_id: string;
  content_version_id: string;
  kind: string;
  attempt_id?: string;
  status: ItemStatus;
  score: number;
  max_score: number;
  item_results?: QuestionResult[];
  /** The grader's own message, e.g. an essay under its word count. */
  feedback?: string;
  /** What the learner answered. Present once the sitting is submitted. */
  response?: Record<string, unknown>;
  /** The item as authored, with answers and explanations. Submitted sittings only. */
  content?: Record<string, unknown>;
}

export interface ExamSectionOutcome {
  position: number;
  skill: ExamSkill;
  status: SectionStatus;
  /** 0–100, present only when the section is scored. */
  score?: number;
  max_score: number;
  items: ExamItemOutcome[];
}

export interface IntegritySignal {
  kind: IntegrityKind;
  count: number;
}

export interface ScoreReport {
  attempt_id: string;
  mode: ExamMode;
  submitted_by?: "learner" | "expiry";
  status: ReportStatus;
  overall_score: number;
  overall_band?: string;
  per_section: ExamSectionOutcome[];
  integrity_signals: IntegritySignal[];
  disclaimer: string;
  /** Seconds between started_at and submitted_at — how long the learner took. */
  elapsed_seconds?: number;
}

export interface ListeningPlayResult {
  audio_url: string;
  plays_used: number;
  plays_allowed: number;
  expires_at: string;
}

export interface SpeakingUploadIntentResult {
  upload_url: string;
  object_key: string;
  expires_at: string;
  daily_recordings_used: number;
  daily_recordings_limit: number;
}

export interface SpeakingCriterion {
  name: string;
  band: number;
  comment_en: string;
  comment_vi: string;
}

export interface SpeakingFeedback {
  attempt_id: string;
  transcript: string;
  criteria: SpeakingCriterion[];
  read_aloud_accuracy?: number;
  words_per_minute?: number;
  feedback_en: string;
  feedback_vi: string;
  recording_deleted_at?: string;
}
