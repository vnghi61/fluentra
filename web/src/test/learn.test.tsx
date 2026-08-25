import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";

import i18n, { initI18n } from "@/i18n";
import { LearnPage } from "@/routes/LearnPage";
import type { CourseDetail, CourseList } from "@/features/lesson";
import { server } from "./msw-server";

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
        description: "Comprehensive curriculum covering vocabulary and core grammar.",
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
    description: "Comprehensive curriculum covering vocabulary and core grammar.",
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
            lock_reason: "Complete Academic Word List - Topic 1 first",
          },
        ],
      },
    ],
  };

  beforeEach(async () => {
    initI18n("en");
    await i18n.changeLanguage("en");
  });

  it("renders intentional empty state when no courses exist", async () => {
    server.use(
      http.get("/api/v1/courses", () => HttpResponse.json({ courses: [] })),
    );

    await renderLearn();

    expect(await screen.findByText("No Courses Available Yet")).toBeInTheDocument();
    expect(screen.getByText(/Curriculum content is currently being prepared/i)).toBeInTheDocument();
  });

  it("renders course syllabus with units and lessons", async () => {
    server.use(
      http.get("/api/v1/courses", () => HttpResponse.json(mockCourseList)),
      http.get("/api/v1/courses/0199a1c2-3d4e-7f80-9abc-def012345678", () => HttpResponse.json(mockCourseDetail)),
    );

    await renderLearn();

    // Course Header
    expect(await screen.findByText("IELTS Foundation 5.0 - 6.5")).toBeInTheDocument();
    expect(screen.getByText("B1 → B2")).toBeInTheDocument();
    expect(screen.getByText("40h")).toBeInTheDocument();

    // Unit 1
    expect(screen.getByText("Everyday Campus Life")).toBeInTheDocument();

    // Lesson 1 (Unlocked)
    expect(screen.getByText("Academic Word List - Topic 1")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Start Lesson/i })).toBeInTheDocument();

    // Lesson 2 (Locked with reason)
    expect(screen.getByText("Present Perfect in Research Contexts")).toBeInTheDocument();
    expect(screen.getByText(/Complete Academic Word List - Topic 1 first/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Locked/i })).toBeDisabled();
  });

  it("renders correctly in Vietnamese (vi)", async () => {
    await i18n.changeLanguage("vi");

    server.use(
      http.get("/api/v1/courses", () => HttpResponse.json(mockCourseList)),
      http.get("/api/v1/courses/0199a1c2-3d4e-7f80-9abc-def012345678", () => HttpResponse.json(mockCourseDetail)),
    );

    await renderLearn();

    expect(await screen.findByText("IELTS Foundation 5.0 - 6.5")).toBeInTheDocument();
    expect(screen.getByText(/Giáo trình khóa học/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Bắt đầu bài học/i })).toBeInTheDocument();
  });
});
