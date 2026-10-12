import * as React from "react";
import { render, screen } from "@testing-library/react";
import { describe, it, expect, beforeAll, afterAll } from "vitest";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import i18n from "i18next";
import { initReactI18next, I18nextProvider } from "react-i18next";

import en from "@/i18n/en.json";
import vi from "@/i18n/vi.json";
import { LoginForm } from "@/features/auth/components/LoginForm";
import { RegisterForm } from "@/features/auth/components/RegisterForm";
import { PreferencesSettings } from "@/features/account/components/PreferencesSettings";
import type { UserPreferences } from "@/features/account/api/accountApi";

const mockPreferences: UserPreferences = {
  locale: "vi",
  theme: "system",
  daily_goal_minutes: 30,
  notification_channels: ["in_app", "email"],
  quiet_hours: {
    start: "22:00",
    end: "07:00",
  },
  ai_processing_opt_out: false,
  practice_level: "B1",
  updated_at: "2026-08-10T10:00:00Z",
};

async function renderWithRouter(ui: React.ReactElement) {
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => ui,
  });
  const registerRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/register",
    component: () => <div>Register</div>,
  });
  const loginRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/login",
    component: () => <div>Login</div>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute, registerRoute, loginRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });
  await router.load();
  return render(
    <I18nextProvider i18n={i18n}>
      <RouterProvider router={router} />
    </I18nextProvider>,
  );
}

describe("Vietnamese Chrome (TASK-I18N-A)", () => {
  beforeAll(async () => {
    if (!i18n.isInitialized) {
      await i18n.use(initReactI18next).init({
        lng: "vi",
        resources: {
          en: { translation: en },
          vi: { translation: vi },
        },
        fallbackLng: "en",
        interpolation: { escapeValue: false },
      });
    } else {
      i18n.addResourceBundle("en", "translation", en, true, true);
      i18n.addResourceBundle("vi", "translation", vi, true, true);
      await i18n.changeLanguage("vi");
    }
  });

  afterAll(async () => {
    if (i18n.isInitialized) {
      await i18n.changeLanguage("en");
    }
  });

  it("renders LoginForm under the vi locale with Vietnamese footer and without English text", async () => {
    await renderWithRouter(<LoginForm />);

    expect(screen.getByText(/Chưa có tài khoản\?/i)).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Đăng ký ngay/i }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/Don't have an account\?/i),
    ).not.toBeInTheDocument();
  });

  it("renders RegisterForm under the vi locale with Vietnamese footer and without English text", async () => {
    await renderWithRouter(<RegisterForm onChallengeIssued={() => {}} />);

    expect(screen.getByText(/Đã có tài khoản\?/i)).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Đăng nhập/i }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/Already have an account\?/i),
    ).not.toBeInTheDocument();
  });

  it("renders PreferencesSettings under the vi locale with Vietnamese theme labels and without English labels", () => {
    render(
      <I18nextProvider i18n={i18n}>
        <PreferencesSettings initialPreferences={mockPreferences} />
      </I18nextProvider>,
    );

    expect(screen.getByText("Hệ thống")).toBeInTheDocument();
    expect(screen.getByText("Tối")).toBeInTheDocument();
    expect(screen.getByText("Sáng")).toBeInTheDocument();
    expect(screen.queryByText("System")).not.toBeInTheDocument();

    // AI grading is always on: the switch and its description are gone.
    expect(
      screen.queryByText(/Đang bật: AI chấm câu trả lời mở/i),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText(/Chấm điểm & xử lý bằng AI/i),
    ).not.toBeInTheDocument();
    // Split into two cards: learning preferences and appearance.
    expect(screen.getByText("Sở thích học tập")).toBeInTheDocument();
    expect(screen.getByText("Giao diện")).toBeInTheDocument();
    expect(screen.getByText("Lưu giao diện")).toBeInTheDocument();
  });
});
