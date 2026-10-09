import { readFileSync } from "node:fs";
import { join } from "node:path";

import { createInstance, type TFunction } from "i18next";
import { http, HttpResponse } from "msw";
import { beforeAll, beforeEach, describe, expect, it } from "vitest";

import en from "@/i18n/en.json";
import vi from "@/i18n/vi.json";
import {
  fetchAccountReportData,
  type AccountReportData,
} from "@/features/account/api/accountReportData";
import { buildAccountReport } from "@/features/account/model/accountReport";
import { groupDeviceSessions } from "@/features/account/model/deviceSessions";
import { reportSheets } from "@/features/account/export/reportXlsx";
import { createReportPdf } from "@/features/account/export/reportPdf";
import { useAuthStore } from "@/stores/authStore";
import { server } from "./msw-server";

let tVi: TFunction;
let tEn: TFunction;

beforeAll(async () => {
  const viInstance = createInstance();
  await viInstance.init({
    lng: "vi",
    resources: { vi: { translation: vi } },
    interpolation: { escapeValue: false },
  });
  tVi = viInstance.t;
  const enInstance = createInstance();
  await enInstance.init({
    lng: "en",
    resources: { en: { translation: en } },
    interpolation: { escapeValue: false },
  });
  tEn = enInstance.t;
});

const profile = {
  id: "user-1",
  email: "hoc.vien@example.com",
  status: "active" as const,
  created_at: "2026-08-01T10:00:00Z",
  updated_at: "2026-08-10T10:00:00Z",
  profile: {
    display_name: "Nguyễn Thị Hương",
    country: "VN",
    timezone: "Asia/Ho_Chi_Minh",
  },
};

function fullData(): AccountReportData {
  return {
    generatedAt: new Date(2026, 9, 8, 9, 30),
    profile,
    preferences: {
      locale: "vi",
      theme: "dark",
      daily_goal_minutes: 30,
      notification_channels: ["in_app"],
      ai_processing_opt_out: false,
      updated_at: "2026-08-10T10:00:00Z",
    },
    learningProfile: {
      declared_level: "B1",
      target_level: "B2",
      target_exam: "ielts",
      weekly_minutes_goal: 150,
      motivations: [],
      created_at: "2026-08-01T10:00:00Z",
      updated_at: "2026-08-01T10:00:00Z",
    },
    progress: {
      courses: [
        {
          course_id: "c-1",
          status: "completed",
          completed_activities: 40,
          total_activities: 40,
          percentage: 100,
          score: 90,
          completed_at: "2026-09-01T10:00:00Z",
        },
        {
          course_id: "c-2",
          status: "in_progress",
          completed_activities: 10,
          total_activities: 40,
          percentage: 25,
          score: null,
        },
      ],
      skills: [
        {
          skill: "vocabulary",
          level: "B1",
          confidence: 0.85,
          updated_at: "2026-09-02T10:00:00Z",
        },
      ],
    },
    courses: [
      {
        id: "c-1",
        slug: "unlikely-slug-without-translation",
        title: "Tiếng Anh giao tiếp cơ bản",
        cefr_from: "A2",
        cefr_to: "B1",
        status: "published",
        estimated_hours: 10,
      },
    ],
    gamification: {
      total_xp: 1250,
      level: 5,
      level_start_xp: 1000,
      next_level_xp: 1500,
      xp_today: 40,
      daily_goal_xp: 50,
      streak: {
        current: 7,
        longest: 12,
        freezes_available: 1,
        hours_remaining: 5,
      },
      badges: [
        {
          code: "first",
          name: "Bước đầu tiên",
          tier: "bronze",
          earned_at: "2026-08-02T10:00:00Z",
        },
      ],
      quests: [],
      league: "silver",
    },
    weeklyPlan: {
      week_start: "2026-10-05",
      minutes_goal: 150,
      items: [],
      progress: { minutes: 95, items_done: 3, items_total: 7 },
    },
    dueReviews: 14,
    forecast: {
      days: [
        { date: "2026-10-09", due_count: 5 },
        { date: "2026-10-10", due_count: 7 },
      ],
    },
    decks: [
      {
        id: "d-1",
        owner_id: "user-1",
        slug: "tu-cua-toi",
        name: "Từ của tôi",
        is_public: false,
        item_count: 32,
        created_at: "2026-08-03T10:00:00Z",
      },
    ],
    exams: {
      attempts: [
        {
          id: "a-1",
          exam_id: "e-1",
          exam_title: "TOEIC Mock (B1)",
          level: "B1",
          mode: "exam",
          chosen_duration_minutes: 75,
          unlimited: false,
          started_at: "2026-09-14T09:00:00Z",
          deadline_at: "2026-09-14T10:15:00Z",
          remaining_seconds: 0,
          current_section: 4,
          status: "completed",
          submitted_at: "2026-09-14T10:05:00Z",
          server_time: "2026-09-14T10:05:00Z",
        },
      ],
      total: 1,
      scores: {
        "a-1": {
          attempt_id: "a-1",
          mode: "exam",
          status: "ready",
          overall_score: 72,
          overall_band: "B1",
          per_section: [],
          integrity_signals: [],
          disclaimer: "",
          elapsed_seconds: 3900,
        },
      },
    },
    unavailable: [],
  };
}

function summaryValue(
  report: ReturnType<typeof buildAccountReport>,
  label: string,
) {
  for (const group of report.summary) {
    const row = group.rows.find((r) => r.label === label);
    if (row) return row.value;
  }
  return undefined;
}

describe("groupDeviceSessions", () => {
  it("puts every sign-in and trust record of one device in one row", () => {
    const groups = groupDeviceSessions(
      [
        {
          id: "s-1",
          current: false,
          device_label: "Chrome on Windows",
          created_at: "2026-08-01T00:00:00Z",
          last_seen_at: "2026-08-02T00:00:00Z",
        },
        {
          id: "s-2",
          current: true,
          device_label: "chrome on windows ",
          created_at: "2026-08-03T00:00:00Z",
          last_seen_at: "2026-08-04T00:00:00Z",
        },
        // A repeated row is counted once.
        {
          id: "s-2",
          current: true,
          device_label: "Chrome on Windows",
          created_at: "2026-08-03T00:00:00Z",
          last_seen_at: "2026-08-04T00:00:00Z",
        },
        {
          id: "s-3",
          current: false,
          device_label: null,
          created_at: "2026-08-05T00:00:00Z",
          last_seen_at: "2026-08-09T00:00:00Z",
        },
      ],
      [
        {
          id: "d-1",
          current: true,
          label: "Chrome on Windows",
          trusted_at: "2026-08-01T00:00:00Z",
          last_seen_at: "2026-08-04T00:00:00Z",
          idle_expires_at: "2026-11-01T00:00:00Z",
          absolute_expires_at: "2027-02-01T00:00:00Z",
        },
      ],
    );

    expect(groups).toHaveLength(2);
    // The device in hand first, even though the unlabelled one is more recent.
    expect(groups[0]?.current).toBe(true);
    expect(groups[0]?.sessions.map((s) => s.id)).toEqual(["s-2", "s-1"]);
    expect(groups[0]?.trusted.map((d) => d.id)).toEqual(["d-1"]);
    expect(groups[1]?.label).toBeNull();
    expect(groups[1]?.sessions.map((s) => s.id)).toEqual(["s-3"]);
  });
});

describe("buildAccountReport", () => {
  it("summarises only what the API returned, in Vietnamese", () => {
    const report = buildAccountReport(fullData(), tVi, "vi");

    expect(report.title).toBe("Báo cáo học tập Fluentra");
    expect(report.summaryTitle).toBe("Tổng quan");
    expect(report.fileBaseName).toBe("fluentra-report-2026-10-08");
    expect(summaryValue(report, "Tên hiển thị")).toBe("Nguyễn Thị Hương");
    expect(summaryValue(report, "Khoá học đã đăng ký")).toBe(2);
    expect(summaryValue(report, "Khoá học đã hoàn thành")).toBe(1);
    expect(summaryValue(report, "Hoạt động đã hoàn thành")).toBe(50);
    expect(
      summaryValue(report, "Tổng hoạt động trong các khoá đã đăng ký"),
    ).toBe(80);
    expect(summaryValue(report, "Mức hoàn thành chung")).toBe(62.5);
    // Only courses with a score count towards the average.
    expect(summaryValue(report, "Điểm trung bình khoá học")).toBe(90);
    expect(summaryValue(report, "Tổng XP")).toBe(1250);
    expect(summaryValue(report, "Chuỗi hiện tại (ngày)")).toBe(7);
    expect(summaryValue(report, "Giải đấu")).toBe("Bạc");
    expect(summaryValue(report, "Thời gian học tuần này (phút)")).toBe(95);
    expect(summaryValue(report, "Thẻ cần ôn ngay")).toBe(14);
    expect(summaryValue(report, "Thẻ đến hạn trong 30 ngày tới")).toBe(12);
    expect(summaryValue(report, "Số từ trong bộ từ của tôi")).toBe(32);
    expect(summaryValue(report, "Điểm tổng trung bình")).toBe(72);
    expect(report.notes).toEqual([]);

    const courses = report.tables.find((t) => t.id === "courses");
    expect(courses?.rows[0]?.[0]).toBe("Tiếng Anh giao tiếp cơ bản");
    expect(courses?.rows[0]?.[1]).toBe("Đã hoàn thành");
    // A course missing from the catalogue keeps its id rather than a made-up name.
    expect(courses?.rows[1]?.[0]).toBe("c-2");
    const exams = report.tables.find((t) => t.id === "exams");
    expect(exams?.rows[0]?.slice(6)).toEqual([72, "B1", null, 65]);
  });

  it("leaves out what could not be loaded instead of printing zeros", () => {
    const data: AccountReportData = {
      ...fullData(),
      gamification: null,
      exams: null,
      dueReviews: null,
      unavailable: ["gamification", "exams", "reviews"],
    };
    const report = buildAccountReport(data, tEn, "en");

    const titles = report.summary.map((g) => g.title);
    expect(titles).not.toContain("XP & streak");
    expect(titles).not.toContain("Mock tests");
    expect(summaryValue(report, "Total XP")).toBeUndefined();
    expect(summaryValue(report, "Review cards due now")).toBeUndefined();
    expect(report.tables.map((t) => t.id)).not.toContain("exams");
    expect(report.notes[0]).toContain("XP and streak, mock tests, reviews due");
  });
});

describe("reportSheets (Excel)", () => {
  it("starts with the summary sheet, then one sheet per section with headers and widths", () => {
    const sheets = reportSheets(buildAccountReport(fullData(), tVi, "vi"));

    expect(sheets.map((s) => s.sheet)).toEqual([
      "Tổng quan",
      "Khoá học",
      "Kỹ năng",
      "Thi thử",
      "Huy hiệu",
      "Lịch ôn tập",
      "Bộ từ của tôi",
    ]);
    const [summary, courses] = sheets;
    expect(summary?.data[0]?.[0]).toMatchObject({
      value: "Báo cáo học tập Fluentra",
      fontWeight: "bold",
    });
    expect(courses?.stickyRowsCount).toBe(1);
    expect(courses?.data[0]?.[0]).toMatchObject({
      value: "Khoá học",
      fontWeight: "bold",
    });
    expect(courses?.columns).toHaveLength(7);
    for (const column of courses?.columns ?? []) {
      expect(column.width).toBeGreaterThanOrEqual(10);
    }
    // Dates stay dates and numbers stay numbers, so Excel can sort and sum.
    expect(courses?.data[1]?.[2]).toMatchObject({ value: 40, type: Number });
    expect(courses?.data[1]?.[6]).toMatchObject({ type: Date });
  });
});

describe("createReportPdf", () => {
  it("embeds a Unicode font so Vietnamese letters survive", () => {
    const fontDir = join(process.cwd(), "public", "fonts", "report");
    const fonts = {
      regular: readFileSync(join(fontDir, "Inter-Regular.ttf")).toString(
        "base64",
      ),
      bold: readFileSync(join(fontDir, "Inter-Bold.ttf")).toString("base64"),
    };
    const doc = createReportPdf(
      buildAccountReport(fullData(), tVi, "vi"),
      fonts,
    );

    expect(doc.getFontList()["Inter"]).toEqual(
      expect.arrayContaining(["normal", "bold"]),
    );
    const pdf = doc.output();
    expect(pdf).toContain("/FontFile2");
    expect(pdf).toContain("Inter");
    // Glyphs are written through an Identity-H CID font, not WinAnsi, which
    // is what lets "ế", "ộ", "đ" and "ư" render.
    expect(pdf).toContain("/Identity-H");
    expect(doc.getNumberOfPages()).toBeGreaterThanOrEqual(1);
  });
});

describe("fetchAccountReportData", () => {
  beforeEach(() => {
    useAuthStore.getState().setAuthSession({
      access_token: "valid-test-token",
      token_type: "Bearer",
      expires_in: 900,
      user_id: "user-1",
      role: "user",
    });
  });

  it("keeps the learner's own decks only and records sections it could not read", async () => {
    server.use(
      http.get("/api/v1/me", () => HttpResponse.json(profile)),
      http.get("/api/v1/me/preferences", () =>
        HttpResponse.json(fullData().preferences),
      ),
      http.get("/api/v1/me/learning-profile", () =>
        HttpResponse.json({ status: 404, title: "Not Found" }, { status: 404 }),
      ),
      http.get("/api/v1/me/progress", () =>
        HttpResponse.json(fullData().progress),
      ),
      http.get("/api/v1/courses", () =>
        HttpResponse.json({ courses: fullData().courses }),
      ),
      http.get("/api/v1/me/gamification", () =>
        HttpResponse.json({ status: 500, title: "Boom" }, { status: 500 }),
      ),
      http.get("/api/v1/me/weekly-plan", () =>
        HttpResponse.json(fullData().weeklyPlan),
      ),
      http.get("/api/v1/reviews/due-count", () =>
        HttpResponse.json({ due_count: 3 }),
      ),
      http.get("/api/v1/reviews/forecast", () =>
        HttpResponse.json(fullData().forecast),
      ),
      http.get("/api/v1/vocabulary/decks", () =>
        HttpResponse.json({
          decks: [
            ...(fullData().decks ?? []),
            {
              id: "curated",
              owner_id: null,
              slug: "ielts",
              name: "IELTS",
              is_public: true,
              item_count: 500,
              created_at: "2026-01-01T00:00:00Z",
            },
          ],
        }),
      ),
      http.get("/api/v1/exam-attempts", () =>
        HttpResponse.json({
          items: fullData().exams?.attempts,
          total: 1,
          sittings_today: 0,
          daily_limit: 5,
        }),
      ),
      http.get("/api/v1/exam-attempts/:id/report", () =>
        HttpResponse.json(fullData().exams?.scores["a-1"]),
      ),
    );

    const data = await fetchAccountReportData();

    expect(data.decks?.map((d) => d.id)).toEqual(["d-1"]);
    // No learning profile yet is not a failure.
    expect(data.learningProfile).toBeNull();
    expect(data.gamification).toBeNull();
    expect(data.unavailable).toEqual(["gamification"]);
    expect(data.dueReviews).toBe(3);
    expect(data.exams?.scores["a-1"]?.overall_score).toBe(72);
  });
});
