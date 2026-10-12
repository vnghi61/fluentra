import { apiFetch } from "@/api/client";
import type { components } from "@/types/api";
import {
  accountApi,
  type LearningProfile,
  type UserPreferences,
  type UserProfile,
} from "./accountApi";

type Schemas = components["schemas"];
export type ReportProgress = Schemas["ProgressResponse"];
export type ReportCourse = Schemas["CourseSummary"];
export type ReportGamification = Schemas["GamificationSummary"];
export type ReportWeeklyPlan = Schemas["WeeklyPlan"];
export type ReportForecast = Schemas["ForecastResponse"];
export type ReportDeck = Schemas["Deck"];
export type ReportExamAttempt = Schemas["ExamAttempt"];
export type ReportExamScore = Schemas["ExamScoreReport"];

/** The sections a report is assembled from; each comes from one endpoint. */
export type ReportSource =
  | "learningProfile"
  | "preferences"
  | "progress"
  | "courses"
  | "gamification"
  | "weeklyPlan"
  | "reviews"
  | "forecast"
  | "decks"
  | "exams";

/**
 * Everything the account report shows, exactly as the API returned it.
 *
 * A section that could not be read is `null` and named in `unavailable`, so the
 * report can say "not available" rather than print a zero it never received.
 */
export interface AccountReportData {
  generatedAt: Date;
  profile: UserProfile;
  preferences: UserPreferences | null;
  learningProfile: LearningProfile | null;
  progress: ReportProgress | null;
  courses: ReportCourse[] | null;
  gamification: ReportGamification | null;
  weeklyPlan: ReportWeeklyPlan | null;
  dueReviews: number | null;
  forecast: ReportForecast | null;
  /** Only the decks the learner owns; curated public decks are not theirs. */
  decks: ReportDeck[] | null;
  exams: {
    attempts: ReportExamAttempt[];
    total: number;
    /** Score reports by attempt id, for completed sittings. */
    scores: Record<string, ReportExamScore>;
  } | null;
  unavailable: ReportSource[];
}

/** Sittings are paged at 100; a report reads at most this many. */
const MAX_EXAM_ATTEMPTS = 300;
/** Score reports are one request each, so only the most recent are read. */
const MAX_EXAM_SCORES = 50;
const SCORE_CONCURRENCY = 4;

async function settle<T>(promise: Promise<T>): Promise<T | null> {
  try {
    return await promise;
  } catch {
    return null;
  }
}

async function listExamAttempts(): Promise<{
  attempts: ReportExamAttempt[];
  total: number;
}> {
  const attempts: ReportExamAttempt[] = [];
  let total = 0;
  for (let offset = 0; offset < MAX_EXAM_ATTEMPTS; offset += 100) {
    const page = await apiFetch<Schemas["ExamAttemptList"]>(
      `/api/v1/exam-attempts?limit=100&offset=${offset}`,
    );
    total = page.total;
    attempts.push(...page.items);
    if (page.items.length < 100 || attempts.length >= page.total) break;
  }
  return { attempts, total };
}

async function readScores(
  attempts: readonly ReportExamAttempt[],
): Promise<Record<string, ReportExamScore>> {
  const completed = attempts
    .filter((a) => a.status === "completed")
    .slice(0, MAX_EXAM_SCORES);
  const scores: Record<string, ReportExamScore> = {};
  for (let i = 0; i < completed.length; i += SCORE_CONCURRENCY) {
    const batch = completed.slice(i, i + SCORE_CONCURRENCY);
    const results = await Promise.all(
      batch.map((a) =>
        settle(
          apiFetch<ReportExamScore>(`/api/v1/exam-attempts/${a.id}/report`),
        ),
      ),
    );
    results.forEach((report, index) => {
      const attempt = batch[index];
      if (report && attempt) scores[attempt.id] = report;
    });
  }
  return scores;
}

/**
 * Reads the learner's own data from the endpoints the app already uses.
 *
 * Nothing here is computed on the server for the report: there is no export
 * endpoint that returns statistics (POST /me/export queues a ZIP that is sent by
 * e-mail). The account itself is required; every other section is optional.
 */
export async function fetchAccountReportData(): Promise<AccountReportData> {
  const [
    profile,
    preferences,
    learningProfile,
    progress,
    courses,
    gamification,
    weeklyPlan,
    due,
    forecast,
    decks,
    exams,
  ] = await Promise.all([
    accountApi.getMe(),
    settle(accountApi.getPreferences()),
    // `null` from the API means "no learning profile yet", not a failure.
    accountApi.getLearningProfile().then(
      (value) => ({ ok: true as const, value }),
      () => ({ ok: false as const, value: null }),
    ),
    settle(apiFetch<ReportProgress>("/api/v1/me/progress")),
    settle(apiFetch<Schemas["CourseList"]>("/api/v1/courses")),
    settle(apiFetch<ReportGamification>("/api/v1/me/gamification")),
    settle(apiFetch<ReportWeeklyPlan>("/api/v1/me/weekly-plan")),
    settle(apiFetch<Schemas["DueCountResponse"]>("/api/v1/reviews/due-count")),
    settle(apiFetch<ReportForecast>("/api/v1/reviews/forecast")),
    settle(apiFetch<Schemas["DeckListResponse"]>("/api/v1/vocabulary/decks")),
    settle(listExamAttempts()),
  ]);

  const scores = exams ? await readScores(exams.attempts) : {};

  const unavailable: ReportSource[] = [];
  const note = (source: ReportSource, missing: boolean) => {
    if (missing) unavailable.push(source);
  };
  note("preferences", preferences === null);
  note("learningProfile", !learningProfile.ok);
  note("progress", progress === null);
  note("courses", courses === null);
  note("gamification", gamification === null);
  note("weeklyPlan", weeklyPlan === null);
  note("reviews", due === null);
  note("forecast", forecast === null);
  note("decks", decks === null);
  note("exams", exams === null);

  return {
    generatedAt: new Date(),
    profile,
    preferences,
    learningProfile: learningProfile.value,
    progress,
    courses: courses?.courses ?? null,
    gamification,
    weeklyPlan,
    dueReviews: due?.due_count ?? null,
    forecast,
    decks: decks
      ? decks.decks.filter((deck) => deck.owner_id === profile.id)
      : null,
    exams: exams ? { ...exams, scores } : null,
    unavailable,
  };
}
