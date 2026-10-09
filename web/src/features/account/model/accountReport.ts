import type { TFunction } from "i18next";
import type { AccountReportData, ReportSource } from "../api/accountReportData";

/**
 * The learner's report as plain data, shared by the Excel and PDF writers.
 *
 * Every number here is one the API returned, or a count/sum/average of such
 * numbers. A section the API could not provide is left out and named in
 * `notes`; it is never filled with zeros.
 */
export type ReportCellKind =
  "text" | "number" | "percent" | "date" | "datetime";
export type ReportCellValue = string | number | Date | null;

export interface ReportSummaryRow {
  label: string;
  value: ReportCellValue;
  kind: ReportCellKind;
}

export interface ReportSummaryGroup {
  title: string;
  rows: ReportSummaryRow[];
}

export interface ReportColumn {
  header: string;
  kind: ReportCellKind;
}

export interface ReportTable {
  id: string;
  title: string;
  columns: ReportColumn[];
  rows: ReportCellValue[][];
}

export interface AccountReport {
  title: string;
  generatedAt: Date;
  generatedLabel: string;
  locale: string;
  summaryTitle: string;
  summary: ReportSummaryGroup[];
  tables: ReportTable[];
  notes: string[];
  emptyText: string;
  /** File name without extension, e.g. `fluentra-report-2026-10-08`. */
  fileBaseName: string;
}

function toDate(iso: string | null | undefined): Date | null {
  if (!iso) return null;
  const date = new Date(iso);
  return Number.isNaN(date.getTime()) ? null : date;
}

/** A calendar day ("2026-10-08") as a local date, not UTC midnight. */
function toDay(day: string | null | undefined): Date | null {
  if (!day) return null;
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(day);
  if (!match) return toDate(day);
  return new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
}

function sum(values: readonly number[]): number {
  return values.reduce((total, value) => total + value, 0);
}

function round1(value: number): number {
  return Math.round(value * 10) / 10;
}

function localDay(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

/** Formats a cell for text output (PDF); Excel keeps the typed value. */
export function formatReportCell(
  value: ReportCellValue,
  kind: ReportCellKind,
  locale: string,
): string {
  if (value === null || value === "") return "—";
  if (value instanceof Date) {
    return kind === "datetime"
      ? value.toLocaleString(locale, {
          day: "2-digit",
          month: "2-digit",
          year: "numeric",
          hour: "2-digit",
          minute: "2-digit",
        })
      : value.toLocaleDateString(locale, {
          day: "2-digit",
          month: "2-digit",
          year: "numeric",
        });
  }
  if (typeof value === "number") {
    const text = value.toLocaleString(locale, { maximumFractionDigits: 1 });
    return kind === "percent" ? `${text}%` : text;
  }
  return value;
}

export function buildAccountReport(
  data: AccountReportData,
  t: TFunction,
  locale: string,
): AccountReport {
  const r = (key: string, fallback: string, options?: object) =>
    t(`account.report.${key}`, { defaultValue: fallback, ...options });

  const summary: ReportSummaryGroup[] = [];
  const add = (
    group: ReportSummaryGroup,
    label: string,
    value: ReportCellValue | undefined,
    kind: ReportCellKind = "number",
  ) => {
    if (value === undefined) return;
    group.rows.push({ label, value, kind });
  };

  // Account
  const { profile } = data;
  const account: ReportSummaryGroup = {
    title: r("groupAccount", "Account"),
    rows: [],
  };
  add(
    account,
    r("displayName", "Display name"),
    profile.profile.display_name,
    "text",
  );
  add(account, r("email", "Email"), profile.email, "text");
  add(
    account,
    r("memberSince", "Member since"),
    toDate(profile.created_at),
    "date",
  );
  add(
    account,
    r("accountStatus", "Account status"),
    r(`statusValue.${profile.status}`, profile.status),
    "text",
  );
  add(
    account,
    r("country", "Country"),
    profile.profile.country ?? null,
    "text",
  );
  add(account, r("timezone", "Time zone"), profile.profile.timezone, "text");
  summary.push(account);

  // Learning profile and the goals the learner set
  const goals: ReportSummaryGroup = {
    title: r("groupLearningProfile", "Learning profile"),
    rows: [],
  };
  const lp = data.learningProfile;
  if (lp) {
    add(
      goals,
      r("declaredLevel", "Current level (declared)"),
      lp.declared_level ?? null,
      "text",
    );
    add(
      goals,
      r("targetLevel", "Target level"),
      lp.target_level ?? null,
      "text",
    );
    add(
      goals,
      r("targetExam", "Target exam"),
      r(`targetExamValue.${lp.target_exam}`, lp.target_exam),
      "text",
    );
    add(
      goals,
      r("weeklyGoal", "Weekly study goal (minutes)"),
      lp.weekly_minutes_goal ?? null,
    );
  }
  if (data.preferences) {
    add(
      goals,
      r("dailyGoal", "Daily goal (minutes)"),
      data.preferences.daily_goal_minutes,
    );
    add(
      goals,
      r("interfaceLanguage", "Interface language"),
      data.preferences.locale === "vi"
        ? "Tiếng Việt"
        : data.preferences.locale === "en"
          ? "English"
          : data.preferences.locale,
      "text",
    );
  }
  if (goals.rows.length > 0) summary.push(goals);

  // Courses and activities
  const courseTitles = new Map<string, string>();
  for (const course of data.courses ?? []) {
    courseTitles.set(
      course.id,
      t(`courses.${course.slug}.title`, { defaultValue: course.title }),
    );
  }
  const courseRows = data.progress?.courses ?? [];
  if (data.progress) {
    const group: ReportSummaryGroup = {
      title: r("groupProgress", "Courses & activities"),
      rows: [],
    };
    const done = sum(courseRows.map((c) => c.completed_activities));
    const total = sum(courseRows.map((c) => c.total_activities));
    const scored = courseRows
      .map((c) => c.score)
      .filter((s): s is number => typeof s === "number");
    add(group, r("coursesEnrolled", "Courses enrolled"), courseRows.length);
    add(
      group,
      r("coursesCompleted", "Courses completed"),
      courseRows.filter((c) => c.status === "completed").length,
    );
    add(
      group,
      r("coursesInProgress", "Courses in progress"),
      courseRows.filter((c) => c.status === "in_progress").length,
    );
    add(group, r("activitiesCompleted", "Activities completed"), done);
    add(group, r("activitiesTotal", "Activities in enrolled courses"), total);
    if (total > 0) {
      add(
        group,
        r("overallCompletion", "Overall completion"),
        round1((done / total) * 100),
        "percent",
      );
    }
    if (scored.length > 0) {
      add(
        group,
        r("averageCourseScore", "Average course score"),
        round1(sum(scored) / scored.length),
      );
    }
    summary.push(group);
  }

  // XP, level and streak
  const g = data.gamification;
  if (g) {
    const group: ReportSummaryGroup = {
      title: r("groupAchievements", "XP & streak"),
      rows: [],
    };
    add(group, r("totalXp", "Total XP"), g.total_xp);
    add(group, r("level", "Level"), g.level);
    add(group, r("xpToday", "XP today"), g.xp_today);
    add(group, r("dailyXpGoal", "Daily XP goal"), g.daily_goal_xp);
    add(group, r("currentStreak", "Current streak (days)"), g.streak.current);
    add(group, r("longestStreak", "Longest streak (days)"), g.streak.longest);
    add(
      group,
      r("freezes", "Streak freezes available"),
      g.streak.freezes_available,
    );
    add(
      group,
      r("league", "League"),
      r(`leagueValue.${g.league}`, g.league),
      "text",
    );
    add(group, r("badgesEarned", "Badges earned"), g.badges.length);
    summary.push(group);
  }

  // This week's plan
  const plan = data.weeklyPlan;
  if (plan) {
    const group: ReportSummaryGroup = {
      title: r("groupWeek", "This week"),
      rows: [],
    };
    add(group, r("weekStart", "Week starting"), toDay(plan.week_start), "date");
    add(
      group,
      r("weekStudyMinutes", "Study time this week (minutes)"),
      plan.progress.minutes,
    );
    add(
      group,
      r("weekGoalMinutes", "Weekly plan goal (minutes)"),
      plan.minutes_goal,
    );
    add(
      group,
      r("weekItems", "Plan items done"),
      `${plan.progress.items_done} / ${plan.progress.items_total}`,
      "text",
    );
    summary.push(group);
  }

  // Reviews and vocabulary
  const reviews: ReportSummaryGroup = {
    title: r("groupReviews", "Reviews & vocabulary"),
    rows: [],
  };
  if (data.dueReviews !== null) {
    add(reviews, r("dueNow", "Review cards due now"), data.dueReviews);
  }
  if (data.forecast) {
    add(
      reviews,
      r("dueNext30", "Reviews due in the next 30 days"),
      sum(data.forecast.days.map((d) => d.due_count)),
    );
  }
  if (data.decks) {
    add(reviews, r("myDecks", "My vocabulary decks"), data.decks.length);
    add(
      reviews,
      r("wordsInDecks", "Words in my decks"),
      sum(data.decks.map((d) => d.item_count)),
    );
  }
  if (reviews.rows.length > 0) summary.push(reviews);

  // Mock tests
  const exams = data.exams;
  const examScores = exams
    ? exams.attempts
        .map((a) => exams.scores[a.id]?.overall_score)
        .filter((s): s is number => typeof s === "number")
    : [];
  if (exams) {
    const group: ReportSummaryGroup = {
      title: r("groupExams", "Mock tests"),
      rows: [],
    };
    add(group, r("examSittings", "Sittings started"), exams.total);
    add(
      group,
      r("examCompleted", "Sittings submitted"),
      exams.attempts.filter((a) => a.status === "completed").length,
    );
    if (examScores.length > 0) {
      add(
        group,
        r("examAverage", "Average overall score"),
        round1(sum(examScores) / examScores.length),
      );
      add(group, r("examBest", "Best overall score"), Math.max(...examScores));
    }
    summary.push(group);
  }

  // Detail tables
  const tables: ReportTable[] = [];
  if (data.progress) {
    tables.push({
      id: "courses",
      title: r("sheetCourses", "Courses"),
      columns: [
        { header: r("colCourse", "Course"), kind: "text" },
        { header: r("colStatus", "Status"), kind: "text" },
        { header: r("colActivitiesDone", "Activities done"), kind: "number" },
        { header: r("colActivitiesTotal", "Total activities"), kind: "number" },
        { header: r("colProgress", "Progress"), kind: "percent" },
        { header: r("colScore", "Score"), kind: "number" },
        { header: r("colCompletedOn", "Completed on"), kind: "date" },
      ],
      rows: courseRows.map((c) => [
        courseTitles.get(c.course_id) ?? c.course_id,
        r(`courseStatus.${c.status}`, c.status),
        c.completed_activities,
        c.total_activities,
        c.percentage,
        c.score ?? null,
        toDate(c.completed_at),
      ]),
    });
    tables.push({
      id: "skills",
      title: r("sheetSkills", "Skills"),
      columns: [
        { header: r("colSkill", "Skill"), kind: "text" },
        { header: r("colLevel", "Level"), kind: "text" },
        { header: r("colConfidence", "Confidence"), kind: "percent" },
        { header: r("colUpdated", "Updated"), kind: "date" },
      ],
      rows: data.progress.skills.map((s) => [
        t(`skills.${s.skill}`, { defaultValue: s.skill }),
        s.level,
        round1(s.confidence * 100),
        toDate(s.updated_at),
      ]),
    });
  }
  if (exams) {
    tables.push({
      id: "exams",
      title: r("sheetExams", "Mock tests"),
      columns: [
        { header: r("colExam", "Test"), kind: "text" },
        { header: r("colLevel", "Level"), kind: "text" },
        { header: r("colMode", "Mode"), kind: "text" },
        { header: r("colStatus", "Status"), kind: "text" },
        { header: r("colStarted", "Started"), kind: "datetime" },
        { header: r("colSubmitted", "Submitted"), kind: "datetime" },
        { header: r("colOverallScore", "Overall score"), kind: "number" },
        { header: r("colBand", "Band"), kind: "text" },
        { header: r("colPublishedScore", "Score on exam scale"), kind: "text" },
        { header: r("colMinutes", "Time taken (min)"), kind: "number" },
      ],
      rows: exams.attempts.map((a) => {
        const score = exams.scores[a.id];
        const published =
          score?.published_score !== undefined && score.published_score !== null
            ? `${score.published_score}${score.published_scale ? ` (${score.published_scale})` : ""}`
            : null;
        return [
          a.exam_title ?? a.exam_slug ?? a.exam_id,
          a.level ?? null,
          r(`examMode.${a.mode}`, a.mode),
          r(`examStatus.${a.status}`, a.status),
          toDate(a.started_at),
          toDate(a.submitted_at),
          score?.overall_score ?? null,
          score?.overall_band ?? null,
          published,
          typeof score?.elapsed_seconds === "number"
            ? round1(score.elapsed_seconds / 60)
            : null,
        ];
      }),
    });
  }
  if (g) {
    tables.push({
      id: "badges",
      title: r("sheetBadges", "Badges"),
      columns: [
        { header: r("colBadge", "Badge"), kind: "text" },
        { header: r("colTier", "Tier"), kind: "text" },
        { header: r("colEarnedOn", "Earned on"), kind: "date" },
        { header: r("colDescription", "Description"), kind: "text" },
      ],
      rows: g.badges.map((b) => [
        b.name,
        r(`tierValue.${b.tier}`, b.tier),
        toDate(b.earned_at),
        b.description ?? null,
      ]),
    });
  }
  if (data.forecast) {
    tables.push({
      id: "forecast",
      title: r("sheetForecast", "Review forecast"),
      columns: [
        { header: r("colDate", "Date"), kind: "date" },
        { header: r("colCardsDue", "Cards due"), kind: "number" },
      ],
      rows: data.forecast.days.map((d) => [toDay(d.date), d.due_count]),
    });
  }
  if (data.decks) {
    tables.push({
      id: "decks",
      title: r("sheetDecks", "My decks"),
      columns: [
        { header: r("colDeck", "Deck"), kind: "text" },
        { header: r("colWords", "Words"), kind: "number" },
        { header: r("colCreated", "Created"), kind: "date" },
      ],
      rows: data.decks.map((d) => [d.name, d.item_count, toDate(d.created_at)]),
    });
  }

  const notes: string[] = [];
  const missing = data.unavailable.map((source: ReportSource) =>
    r(`source.${source}`, source),
  );
  if (missing.length > 0) {
    notes.push(
      r(
        "unavailableNote",
        `Could not be loaded: ${missing.join(", ")}. These figures are left out rather than shown as zero.`,
        { sections: missing.join(", ") },
      ),
    );
  }
  if (exams && exams.attempts.length < exams.total) {
    notes.push(
      r(
        "examsTruncatedNote",
        `Only the ${exams.attempts.length} most recent of ${exams.total} sittings are listed.`,
        { shown: exams.attempts.length, total: exams.total },
      ),
    );
  }

  const generatedText = formatReportCell(data.generatedAt, "datetime", locale);
  return {
    title: r("title", "Fluentra learning report"),
    generatedAt: data.generatedAt,
    generatedLabel: r("generatedAt", `Generated ${generatedText}`, {
      date: generatedText,
    }),
    locale,
    summaryTitle: r("sheetSummary", "Summary"),
    summary,
    tables,
    notes,
    emptyText: r("emptyTable", "No data."),
    fileBaseName: `fluentra-report-${localDay(data.generatedAt)}`,
  };
}
