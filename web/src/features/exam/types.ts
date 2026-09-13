export type ExamLevel = "A2" | "B1" | "B2";
export type ExamMode = "exam" | "practice";
export type ExamStatus = "in_progress" | "completed" | "expired";
export type ReportStatus = "pending" | "ready" | "partial";

export interface ExamSectionTemplate {
  id: string;
  exam_id: string;
  position: number;
  skill: "listening" | "reading" | "writing" | "speaking";
  exam_duration_minutes: number;
  item_count: number;
  item_kinds: string[];
}

export interface ExamTemplate {
  id: string;
  slug: string;
  title_en: string;
  title_vi: string;
  description_en: string;
  description_vi: string;
  level: ExamLevel;
  format: string;
  total_minutes: number;
  sections?: ExamSectionTemplate[];
}

export interface SittingActivity {
  id: string;
  kind: string;
  content_version_id: string;
  config?: any;
  weight: number;
}

export interface SectionActivities {
  section_position: number;
  skill: "listening" | "reading" | "writing" | "speaking";
  activities: SittingActivity[];
}

export interface ExamAttempt {
  id: string;
  exam_id: string;
  exam_slug?: string;
  exam_title?: string;
  mode: ExamMode;
  chosen_duration_minutes: number;
  started_at: string;
  deadline_at: string;
  remaining_seconds: number;
  current_section: number;
  status: ExamStatus;
  section_activities?: SectionActivities[];
  draft_answers?: Record<string, any>;
  server_time: string;
}

export interface ExamAttemptListResponse {
  attempts: ExamAttempt[];
  total: number;
}

export interface StartSittingRequest {
  mode: ExamMode;
  chosen_duration_minutes?: number;
}

export interface IntegrityEvent {
  kind: "tab_hidden" | "window_blurred" | "paste";
  occurred_at: string;
  metadata?: Record<string, any>;
}

export interface SaveAnswersRequest {
  section_number?: number;
  answers: Record<string, any>;
  integrity_events?: IntegrityEvent[];
}

export interface SaveAnswersResult {
  saved: boolean;
  remaining_seconds: number;
}

export interface CompleteSectionResult {
  current_section: number;
  remaining_seconds: number;
}

export interface SubmitExamResult {
  attempt_id: string;
  status: string;
  report_status: string;
}

export interface SectionScore {
  skill: string;
  score: number;
  max_score: number;
  band?: string;
  status?: "scored" | "not_scored" | "pending";
  item_results?: any;
}

export interface ScoreReport {
  attempt_id: string;
  status: ReportStatus;
  overall_score: number;
  overall_band: string;
  per_section: SectionScore[];
  feedback?: {
    writing?: any;
    speaking?: any;
    [key: string]: any;
  };
  integrity_signals?: Array<Record<string, any>>;
  disclaimer: string;
}

export interface ListeningPlayResult {
  audio_url: string;
  plays_used: number;
  plays_allowed: number;
  expires_at: string;
}

export interface SpeakingUploadIntentResult {
  upload_url: string;
  key: string;
  max_bytes: number;
  expires_at: string;
}
