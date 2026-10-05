import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEventDefault from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { afterAll, beforeEach, describe, expect, it } from "vitest";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";

import i18n, { initI18n, loadLocale } from "@/i18n";
import { LearnPage } from "@/routes/LearnPage";
import type { CourseDetail, CourseList } from "@/features/lesson";
import type { CourseUnit } from "@/features/lesson/api/lessonApi";
import { UnitList } from "@/features/lesson/components/UnitList";
import { LessonRow } from "@/features/lesson/components/LessonRow";
import { server } from "./msw-server";

const userEvent = userEventDefault;

async function renderWithRouter(ui: React.ReactElement) {
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <I18nextProvider i18n={i18n}>{ui}</I18nextProvider>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

const mockMultiUnitCourse: CourseUnit[] = [
  {
    id: "unit-1",
    course_id: "course-1",
    position: 1,
    title: "Everyday Campus Life",
    description: "Campus vocabulary and phrases.",
    lessons: [
      {
        id: "lesson-1-1",
        unit_id: "unit-1",
        position: 1,
        title: "Academic Word List - Topic 1",
        skill_focus: "vocabulary",
        estimated_minutes: 15,
        status: "published",
        locked: false,
        completed: true,
        lock_reason: null,
      },
      {
        id: "lesson-1-2",
        unit_id: "unit-1",
        position: 2,
        title: "Campus Orientation",
        skill_focus: "listening",
        estimated_minutes: 10,
        status: "published",
        locked: false,
        completed: true,
        lock_reason: null,
      },
    ],
  },
  {
    id: "unit-2",
    course_id: "course-1",
    position: 2,
    title: "Research Methodologies",
    description: "Academic writing skills.",
    lessons: [
      {
        id: "lesson-2-1",
        unit_id: "unit-2",
        position: 1,
        title: "Literature Review Synthesis",
        skill_focus: "writing",
        estimated_minutes: 25,
        status: "published",
        locked: false,
        completed: false,
        lock_reason: null,
      },
      {
        id: "lesson-2-2",
        unit_id: "unit-2",
        position: 2,
        title: "Methodology Reporting",
        skill_focus: "grammar",
        estimated_minutes: 20,
        status: "published",
        locked: false,
        completed: false,
        lock_reason: null,
      },
    ],
  },
  {
    id: "unit-3",
    course_id: "course-1",
    position: 3,
    title: "Advanced Data Analysis",
    description: "Statistical reporting.",
    lessons: [
      {
        id: "lesson-3-1",
        unit_id: "unit-3",
        position: 1,
        title: "Statistical Findings Analysis",
        skill_focus: "reading",
        estimated_minutes: 30,
        status: "published",
        locked: true,
        completed: false,
        lock_reason: "Complete Unit 2 first",
      },
    ],
  },
];

async function renderLearn() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/learn",
    component: () => (
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <LearnPage />
        </QueryClientProvider>
      </I18nextProvider>
    ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute]),
    history: createMemoryHistory({ initialEntries: ["/learn"] }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

describe("LearnPage (P10.2)", () => {
  const mockCourseList: CourseList = {
    courses: [
      {
        id: "0199a1c2-3d4e-7f80-9abc-def012345678",
        slug: "ielts-foundation",
        title: "IELTS Foundation 5.0 - 6.5",
        description:
          "Comprehensive curriculum covering vocabulary and core grammar.",
        cefr_from: "B1",
        cefr_to: "B2",
        status: "published",
        estimated_hours: 40,
      },
    ],
  };

  const mockCourseDetail: CourseDetail = {
    id: "0199a1c2-3d4e-7f80-9abc-def012345678",
    slug: "ielts-foundation",
    title: "IELTS Foundation 5.0 - 6.5",
    description:
      "Comprehensive curriculum covering vocabulary and core grammar.",
    cefr_from: "B1",
    cefr_to: "B2",
    status: "published",
    estimated_hours: 40,
    units: [
      {
        id: "0199a1c2-3d4e-7f80-9abc-def012345679",
        course_id: "0199a1c2-3d4e-7f80-9abc-def012345678",
        position: 1,
        title: "Everyday Campus Life",
        description: "Essential academic words for university scenarios.",
        lessons: [
          {
            id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
            unit_id: "0199a1c2-3d4e-7f80-9abc-def012345679",
            position: 1,
            title: "Academic Word List - Topic 1",
            skill_focus: "vocabulary",
            estimated_minutes: 15,
            status: "published",
            locked: false,
            completed: false,
            lock_reason: null,
          },
          {
            id: "0199a1c2-3d4e-7f80-9abc-def01234567b",
            unit_id: "0199a1c2-3d4e-7f80-9abc-def012345679",
            position: 2,
            title: "Present Perfect in Research Contexts",
            skill_focus: "grammar",
            estimated_minutes: 20,
            status: "published",
            locked: true,
            completed: false,
            lock_reason: "Complete Academic Word List - Topic 1 first",
          },
        ],
      },
    ],
  };

  beforeEach(async () => {
    await initI18n("en");
    await loadLocale("en");
    await i18n.changeLanguage("en");
  });

  afterAll(async () => {
    await loadLocale("en");
    await i18n.changeLanguage("en");
  });

  it("renders intentional empty state when no courses exist", async () => {
    server.use(
      http.get("/api/v1/courses", () => HttpResponse.json({ courses: [] })),
    );

    await renderLearn();

    expect(
      await screen.findByText("No Courses Available Yet"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Curriculum content is currently being prepared/i),
    ).toBeInTheDocument();
  });

  it("renders course syllabus with units and lessons", async () => {
    server.use(
      http.get("/api/v1/courses", () => HttpResponse.json(mockCourseList)),
      // Addressed by slug, which is what `/courses/{slug}` declares. Mocking
      // the id path made this test pass while the real syllabus 404ed.
      http.get("/api/v1/courses/ielts-foundation", () =>
        HttpResponse.json(mockCourseDetail),
      ),
    );

    await renderLearn();

    // Course Header
    expect(
      await screen.findByText("IELTS Foundation 5.0 - 6.5"),
    ).toBeInTheDocument();
    expect(screen.getByText("B1 → B2")).toBeInTheDocument();
    expect(screen.getByText("40h")).toBeInTheDocument();

    // Unit 1
    expect(screen.getByText("Everyday Campus Life")).toBeInTheDocument();

    // Lesson 1 (Unlocked)
    expect(
      screen.getByText("Academic Word List - Topic 1"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Start Lesson/i }),
    ).toBeInTheDocument();

    // Lesson 2 (Locked with reason)
    expect(
      screen.getByText("Present Perfect in Research Contexts"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Complete Academic Word List - Topic 1 first/i),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Locked/i })).toBeDisabled();
  });

  it("renders correctly in Vietnamese (vi)", async () => {
    // Only the locale being read is bundled, so the Vietnamese
    // translations have to be fetched before the switch. `setLocale` does
    // this in the app; calling `changeLanguage` alone renders raw keys.
    await loadLocale("vi");
    await i18n.changeLanguage("vi");

    server.use(
      http.get("/api/v1/courses", () => HttpResponse.json(mockCourseList)),
      // Addressed by slug, which is what `/courses/{slug}` declares. Mocking
      // the id path made this test pass while the real syllabus 404ed.
      http.get("/api/v1/courses/ielts-foundation", () =>
        HttpResponse.json(mockCourseDetail),
      ),
    );

    await renderLearn();

    expect(
      await screen.findByText("IELTS Foundation 5.0 - 6.5"),
    ).toBeInTheDocument();
    expect(screen.getByText(/Giáo trình khóa học/i)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Bắt đầu bài học/i }),
    ).toBeInTheDocument();
  });

  /**
   * A finished lesson looks finished.
   *
   * LessonRow has drawn a tick, a "Completed" badge and a "Review" button since
   * WP10, and none of them had ever appeared: UnitList read completion from a
   * `completedLessonIds` prop that no caller passed, so the default empty set
   * made every lesson unstarted forever. The catalogue now reads
   * `lesson.completed` off the response, which is why this can be asserted at
   * all.
   */
  it("marks a finished lesson, and offers to review it rather than start it", async () => {
    server.use(
      http.get("/api/v1/courses", () => HttpResponse.json(mockCourseList)),
      http.get("/api/v1/courses/ielts-foundation", () =>
        HttpResponse.json({
          ...mockCourseDetail,
          // The first lesson of each unit is done; the rest are not. Mapped
          // rather than indexed because noUncheckedIndexedAccess is on, and a
          // fixture is not the place to start asserting non-null.
          units: mockCourseDetail.units.map((unit) => ({
            ...unit,
            lessons: unit.lessons.map((lesson, index) => ({
              ...lesson,
              completed: index === 0,
            })),
          })),
        }),
      ),
    );

    await renderLearn();

    expect(await screen.findByText("Completed")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Review/i })).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Start Lesson/i }),
    ).not.toBeInTheDocument();
  });

  it("expands the current unit first and keeps other units collapsed", async () => {
    await renderWithRouter(<UnitList units={mockMultiUnitCourse} />);

    // Unit 2 is current (first with unlocked, incomplete lesson) -> expanded
    const unit2Btn = screen.getByRole("button", {
      name: /Unit 2 · Research Methodologies/i,
    });
    expect(unit2Btn).toHaveAttribute("aria-expanded", "true");
    expect(unit2Btn).toHaveAttribute("aria-controls", "unit-content-unit-2");
    expect(screen.getByText("Literature Review Synthesis")).toBeInTheDocument();

    // Unit 1 (all completed) -> collapsed
    const unit1Btn = screen.getByRole("button", {
      name: /Unit 1 · Everyday Campus Life/i,
    });
    expect(unit1Btn).toHaveAttribute("aria-expanded", "false");
    expect(
      screen.queryByText("Academic Word List - Topic 1"),
    ).not.toBeInTheDocument();

    // Unit 3 (all locked) -> collapsed
    const unit3Btn = screen.getByRole("button", {
      name: /Unit 3 · Advanced Data Analysis/i,
    });
    expect(unit3Btn).toHaveAttribute("aria-expanded", "false");
    expect(
      screen.queryByText("Statistical Findings Analysis"),
    ).not.toBeInTheDocument();

    // Progress text: 'x/y completed'
    expect(screen.getByText("2/2 completed")).toBeInTheDocument();
    expect(screen.getByText("0/2 completed")).toBeInTheDocument();
    expect(screen.getByText("0/1 completed")).toBeInTheDocument();
  });

  it("toggles unit expansion when clicked", async () => {
    const user = userEvent.setup();
    await renderWithRouter(<UnitList units={mockMultiUnitCourse} />);

    const unit1Btn = screen.getByRole("button", {
      name: /Unit 1 · Everyday Campus Life/i,
    });
    expect(unit1Btn).toHaveAttribute("aria-expanded", "false");
    expect(
      screen.queryByText("Academic Word List - Topic 1"),
    ).not.toBeInTheDocument();

    // Click to expand Unit 1
    await user.click(unit1Btn);
    expect(unit1Btn).toHaveAttribute("aria-expanded", "true");
    expect(
      screen.getByText("Academic Word List - Topic 1"),
    ).toBeInTheDocument();

    // Click to collapse Unit 1 again
    await user.click(unit1Btn);
    expect(unit1Btn).toHaveAttribute("aria-expanded", "false");
    expect(
      screen.queryByText("Academic Word List - Topic 1"),
    ).not.toBeInTheDocument();
  });

  it("searches across units, auto-expands matching collapsed units, and normalizes diacritics", async () => {
    const user = userEvent.setup();
    await renderWithRouter(<UnitList units={mockMultiUnitCourse} />);

    // Unit 3 is initially collapsed
    expect(
      screen.queryByText("Statistical Findings Analysis"),
    ).not.toBeInTheDocument();

    const searchInput = screen.getByRole("searchbox", {
      name: /search lessons/i,
    });

    // Search for a lesson in collapsed Unit 3 (case-insensitive)
    await user.type(searchInput, "findings");

    // Unit 3 auto-expands
    const unit3Btn = screen.getByRole("button", {
      name: /Unit 3 · Advanced Data Analysis/i,
    });
    expect(unit3Btn).toHaveAttribute("aria-expanded", "true");
    expect(
      screen.getByText("Statistical Findings Analysis"),
    ).toBeInTheDocument();

    // Status announcement
    expect(screen.getByRole("status")).toHaveTextContent("1 lesson found");

    // Non-matching lessons in Unit 2 are filtered out
    expect(
      screen.queryByText("Literature Review Synthesis"),
    ).not.toBeInTheDocument();

    // Diacritics-insensitive match (e.g. "methodològy" matching "Methodology")
    await user.clear(searchInput);
    await user.type(searchInput, "methodològy");
    expect(screen.getByText("Methodology Reporting")).toBeInTheDocument();
  });

  it("shows no-results text when search has no matches", async () => {
    const user = userEvent.setup();
    await renderWithRouter(<UnitList units={mockMultiUnitCourse} />);

    const searchInput = screen.getByRole("searchbox", {
      name: /search lessons/i,
    });
    await user.type(searchInput, "nonexistentquery123");

    const statuses = screen.getAllByRole("status");
    expect(statuses[0]).toHaveTextContent("");
    expect(statuses[1]).toHaveTextContent("No lessons found");
    expect(screen.getByText("No lessons found")).toBeInTheDocument();
    expect(
      screen.getByText("Try searching with different keywords."),
    ).toBeInTheDocument();
  });

  it("keeps the search status region in the DOM before typing and updates its count after typing", async () => {
    const user = userEvent.setup();
    await renderWithRouter(<UnitList units={mockMultiUnitCourse} />);

    const status = screen.getByRole("status");
    expect(status).toBeInTheDocument();
    expect(status).toHaveTextContent("");

    const searchInput = screen.getByRole("searchbox", {
      name: /search lessons/i,
    });
    await user.type(searchInput, "findings");

    expect(status).toHaveTextContent("1 lesson found");

    await user.clear(searchInput);
    await user.type(searchInput, "ing");
    expect(status).toHaveTextContent("2 lessons found");
  });

  it("shows text status labels (Completed, Locked, Next, Not started) on lesson rows", async () => {
    const sampleLesson = {
      id: "lesson-sample",
      unit_id: "unit-1",
      position: 1,
      title: "Sample Lesson",
      skill_focus: "vocabulary",
      estimated_minutes: 10,
      status: "published" as const,
      locked: false,
      completed: false,
      lock_reason: null,
    };

    // 1. Completed
    const { unmount: u1 } = await renderWithRouter(
      <LessonRow lesson={sampleLesson} isCompleted={true} />,
    );
    expect(screen.getByText("Completed")).toBeInTheDocument();
    u1();

    // 2. Locked
    const { unmount: u2 } = await renderWithRouter(
      <LessonRow
        lesson={{
          ...sampleLesson,
          locked: true,
          lock_reason: "Pre-requisite required",
        }}
      />,
    );
    expect(screen.getAllByText("Locked").length).toBeGreaterThanOrEqual(1);
    u2();

    // 3. Next
    const { unmount: u3 } = await renderWithRouter(
      <LessonRow lesson={sampleLesson} isNext={true} />,
    );
    expect(screen.getByText("Next")).toBeInTheDocument();
    u3();

    // 4. Not started
    const { unmount: u4 } = await renderWithRouter(
      <LessonRow lesson={sampleLesson} />,
    );
    expect(screen.getByText("Not started")).toBeInTheDocument();
    u4();
  });
});
