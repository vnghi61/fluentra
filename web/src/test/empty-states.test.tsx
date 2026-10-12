import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { render, screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { afterAll, beforeEach, describe, expect, it } from "vitest";

import i18n, { initI18n, loadLocale } from "@/i18n";
import { MyWritingPage } from "@/routes/MyWritingPage";
import { MySpeakingPage } from "@/routes/MySpeakingPage";
import { ProgressPage } from "@/routes/ProgressPage";
import { MyResourcesPage } from "@/routes/MyResourcesPage";
import { useAuthStore } from "@/stores/authStore";
import { server } from "./msw-server";

async function renderRoute(
  initialPath: string,
  component: React.ComponentType,
) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const rootRoute = createRootRoute();
  const pageRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: initialPath,
    component: () => (
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          {React.createElement(component)}
        </QueryClientProvider>
      </I18nextProvider>
    ),
  });
  const learnRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/learn",
    component: () => <div>Learn Destination</div>,
  });
  const practiceRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/practice",
    component: () => <div>Practice Destination</div>,
  });
  const resourceDetailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/my-resources/$resourceId",
    component: () => <div>Resource Detail</div>,
  });

  const router = createRouter({
    routeTree: rootRoute.addChildren([
      pageRoute,
      learnRoute,
      practiceRoute,
      resourceDetailRoute,
    ]),
    history: createMemoryHistory({ initialEntries: [initialPath] }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

describe("Empty States with real CTAs under vi locale (TASK-W2-EMPTY)", () => {
  beforeEach(async () => {
    useAuthStore.getState().setAuthSession({
      access_token: "valid-test-token",
      token_type: "Bearer",
      expires_in: 900,
      user_id: "user-123",
      role: "user",
    });
    await initI18n("vi");
    await loadLocale("vi");
    await i18n.changeLanguage("vi");
  });

  afterAll(async () => {
    await loadLocale("en");
    await i18n.changeLanguage("en");
  });

  describe("MyWritingPage", () => {
    it("renders empty state with short explanation and CTA link to /learn under vi locale when 0 submissions", async () => {
      server.use(
        http.get("/api/v1/writing/submissions", () =>
          HttpResponse.json({
            items: [],
            total: 0,
            page: 1,
            page_size: 10,
          }),
        ),
      );

      await renderRoute("/my-writing", MyWritingPage);

      expect(
        await screen.findByText("Chưa có bài viết nào"),
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          "Hoàn thành các bài tập viết trong bài học để nhận điểm IELTS và nhận xét chi tiết từ AI.",
        ),
      ).toBeInTheDocument();

      const cta = screen.getByRole("link", { name: "Khám phá bài học" });
      expect(cta).toBeInTheDocument();
      expect(cta).toHaveAttribute("href", "/learn");
    });

    it("does not render empty state when submissions exist", async () => {
      server.use(
        http.get("/api/v1/writing/submissions", () =>
          HttpResponse.json({
            items: [
              {
                attempt_id: "0199a1c2-3d4e-7f80-9abc-def012345601",
                task_id: "task-1",
                status: "graded",
                overall_band: 6.5,
                created_at: "2026-09-20T10:00:00Z",
                feedback_en: "Good essay",
                feedback_vi: "Bài luận tốt",
              },
            ],
            total: 1,
            page: 1,
            page_size: 10,
          }),
        ),
      );

      await renderRoute("/my-writing", MyWritingPage);

      expect(await screen.findByText("Band 6.5")).toBeInTheDocument();
      expect(
        screen.queryByText("Chưa có bài viết nào"),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole("link", { name: "Khám phá bài học" }),
      ).not.toBeInTheDocument();
    });
  });

  describe("MySpeakingPage", () => {
    it("renders empty state with short explanation and CTA link to /practice under vi locale when 0 submissions", async () => {
      server.use(
        http.get("/api/v1/speaking/submissions", () =>
          HttpResponse.json({
            items: [],
            total: 0,
            page: 1,
            page_size: 10,
          }),
        ),
      );

      await renderRoute("/my-speaking", MySpeakingPage);

      expect(
        await screen.findByText("Chưa có bài nói nào"),
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          "Hoàn thành các hoạt động nói trong bài học để kiểm tra tốc độ, độ chính xác và nhận phản hồi từ AI.",
        ),
      ).toBeInTheDocument();

      const cta = screen.getByRole("link", { name: "Khám phá luyện tập" });
      expect(cta).toBeInTheDocument();
      expect(cta).toHaveAttribute("href", "/practice");
    });

    it("does not render empty state when submissions exist", async () => {
      server.use(
        http.get("/api/v1/speaking/submissions", () =>
          HttpResponse.json({
            items: [
              {
                attempt_id: "0199a1c2-3d4e-7f80-9abc-def012345602",
                activity_id: "act-1",
                status: "graded",
                score: 85,
                created_at: "2026-09-20T10:00:00Z",
              },
            ],
            total: 1,
            page: 1,
            page_size: 10,
          }),
        ),
      );

      await renderRoute("/my-speaking", MySpeakingPage);

      expect(await screen.findByText(/85 \/ 100/)).toBeInTheDocument();
      expect(screen.queryByText("Chưa có bài nói nào")).not.toBeInTheDocument();
      expect(
        screen.queryByRole("link", { name: "Khám phá luyện tập" }),
      ).not.toBeInTheDocument();
    });
  });

  describe("ProgressPage", () => {
    beforeEach(() => {
      server.use(
        http.get("/api/v1/courses", () => HttpResponse.json({ courses: [] })),
      );
    });

    it("renders empty state with explanation and CTA link to /learn under vi locale when courses and skills are empty", async () => {
      server.use(
        http.get("/api/v1/me/progress", () =>
          HttpResponse.json({
            courses: [],
            skills: [],
          }),
        ),
      );

      await renderRoute("/progress", ProgressPage);

      expect(await screen.findByText("Tiến độ học tập")).toBeInTheDocument();
      expect(screen.getByText("Chưa có tiến độ học tập")).toBeInTheDocument();
      expect(
        screen.getByText(
          "Bắt đầu học bài để theo dõi tiến độ, năng lực kỹ năng và độ hoàn thành khoá học.",
        ),
      ).toBeInTheDocument();

      const cta = screen.getByRole("link", { name: "Bắt đầu học" });
      expect(cta).toBeInTheDocument();
      expect(cta).toHaveAttribute("href", "/learn");
    });

    it("does not render empty state when progress exists", async () => {
      server.use(
        http.get("/api/v1/me/progress", () =>
          HttpResponse.json({
            courses: [
              {
                course_id: "0199a1c2-3d4e-7f80-9abc-def012345678",
                status: "in_progress",
                completed_activities: 12,
                total_activities: 40,
                percentage: 30,
                score: 88,
              },
            ],
            skills: [],
          }),
        ),
      );

      await renderRoute("/progress", ProgressPage);

      expect(await screen.findByText("Tiến độ học tập")).toBeInTheDocument();
      expect(
        screen.queryByText("Chưa có tiến độ học tập"),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole("link", { name: "Bắt đầu học" }),
      ).not.toBeInTheDocument();
    });
  });

  describe("MyResourcesPage", () => {
    it("renders merged empty state with upload action under vi locale when 0 resources", async () => {
      server.use(
        http.get("/api/v1/me/resources", () =>
          HttpResponse.json({
            items: [],
            total: 0,
            page: 1,
            page_size: 100,
          }),
        ),
      );

      const { container } = await renderRoute("/my-resources", MyResourcesPage);

      expect(await screen.findByText("Chưa có tài liệu")).toBeInTheDocument();
      expect(
        screen.getByText(
          "Tải lên PDF, bản ghi âm bài học hoặc video từ khoá học của bạn và nó sẽ xuất hiện ở đây.",
        ),
      ).toBeInTheDocument();

      // Upload action is merged into the single empty state
      expect(
        screen.getByRole("button", { name: /Chọn tệp/i }),
      ).toBeInTheDocument();
      const fileInput = container.querySelector('input[type="file"]');
      expect(fileInput).toBeInTheDocument();

      // Only one upload action exists on the page
      const chooseButtons = screen.getAllByRole("button", {
        name: /Chọn tệp/i,
      });
      expect(chooseButtons.length).toBe(1);
    });

    it("does not render empty state when resources exist", async () => {
      server.use(
        http.get("/api/v1/me/resources", () =>
          HttpResponse.json({
            items: [
              {
                id: "0199a1c2-3d4e-7f80-9abc-def0123456a1",
                kind: "file",
                title: "grammar-notes.pdf",
                status: "validated",
                failure_reason: "",
                original_filename: "grammar-notes.pdf",
                declared_mime: "application/pdf",
                detected_mime: "application/pdf",
                byte_size: 2458120,
                checksum: null,
                source_url: null,
                download_url: "https://storage.example/download",
                created_at: "2026-09-21T00:00:00Z",
                updated_at: "2026-09-21T00:01:00Z",
                validated_at: "2026-09-21T00:01:00Z",
                renditions: [],
              },
            ],
            total: 1,
            page: 1,
            page_size: 100,
          }),
        ),
      );

      await renderRoute("/my-resources", MyResourcesPage);

      expect(await screen.findByText("grammar-notes.pdf")).toBeInTheDocument();
      expect(screen.queryByText("Chưa có tài liệu")).not.toBeInTheDocument();
    });
  });
});
