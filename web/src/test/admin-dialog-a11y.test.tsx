import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";
import { http, HttpResponse } from "msw";

import i18n, { initI18n, loadLocale } from "@/i18n";
import { AdminActionReasonModal } from "@/features/admin/components/AdminActionReasonModal";
import { AdminContentDetailModal } from "@/features/admin/components/AdminContentDetailModal";
import { AdminUserDetailModal } from "@/features/admin/components/AdminUserDetailModal";
import { CreateFeatureFlagModal } from "@/features/admin/components/CreateFeatureFlagModal";
import type {
  AdminContentItemDetail,
  AdminUserDetail,
  ContentVersion,
} from "@/features/admin/api/adminApi";

import { server } from "./msw-server";

const userDetail = {
  id: "11111111-1111-1111-1111-111111111111",
  email: "learner1@example.com",
  display_name: "Learner One",
  avatar_url: null,
  locale: "vi-VN",
  timezone: "Asia/Ho_Chi_Minh",
  status: "active",
  created_at: "2026-08-01T10:00:00Z",
  updated_at: "2026-08-10T12:00:00Z",
} satisfies AdminUserDetail;

const contentDetail = {
  id: "0199a1c2-3d4e-7f80-9abc-def01234567b",
  slug: "grammar-past-perfect-1",
  kind: "grammar_exercise",
  status: "published",
  owner_id: "11111111-1111-1111-1111-111111111111",
  created_at: "2026-09-10T08:00:00Z",
  updated_at: "2026-09-20T10:00:00Z",
  versions: [
    {
      id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
      item_id: "0199a1c2-3d4e-7f80-9abc-def01234567b",
      version: 1,
      kind: "grammar_exercise",
      status: "published",
      cefr_level: "B1",
      body: { prompt: "Test prompt" } as unknown as ContentVersion["body"],
      tags: [],
    },
  ],
} satisfies AdminContentItemDetail;

function renderWithI18n(ui: React.ReactElement) {
  return render(<I18nextProvider i18n={i18n}>{ui}</I18nextProvider>);
}

function renderWithQuery(ui: React.ReactElement) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return renderWithI18n(
    <QueryClientProvider client={client}>{ui}</QueryClientProvider>,
  );
}

function expectIconCloseButton() {
  const button = screen
    .getAllByRole("button", { name: "Close" })
    .find((candidate) => candidate.textContent?.trim() === "");

  expect(button).toBeDefined();
  expect(button).toHaveAttribute("aria-label", "Close");
}

describe("admin dialog accessible names", () => {
  beforeEach(async () => {
    await initI18n("en");
    await loadLocale("en");
    await i18n.changeLanguage("en");

    server.use(
      http.get("/api/v1/me/permissions", () =>
        HttpResponse.json({
          permissions: [
            "content.read.published",
            "content.edit",
            "content.review",
            "content.publish",
            "user.suspend",
            "user.reinstate",
            "user.manage_sessions",
          ],
        }),
      ),
      http.get("/api/v1/admin/users/:id", () => HttpResponse.json(userDetail)),
      http.get("/api/v1/admin/content/:id", () =>
        HttpResponse.json(contentDetail),
      ),
    );
  });

  it("names the action-reason dialog from its supplied title", () => {
    renderWithI18n(
      <AdminActionReasonModal
        isOpen
        title="Suspend learner"
        description="Explain why this learner is being suspended."
        actionButtonLabel="Suspend"
        onClose={() => undefined}
        onConfirm={() => Promise.resolve()}
      />,
    );

    expect(
      screen.getByRole("dialog", { name: "Suspend learner" }),
    ).toBeInTheDocument();
    expectIconCloseButton();
  });

  it("names the content detail dialog from the loaded content slug", async () => {
    renderWithQuery(
      <AdminContentDetailModal
        itemId={contentDetail.id}
        onClose={() => undefined}
        onItemUpdated={() => undefined}
      />,
    );

    expect(
      await screen.findByRole("dialog", { name: contentDetail.slug }),
    ).toBeInTheDocument();
    expectIconCloseButton();
  });

  it("names the learner detail dialog and its icon-only close button", async () => {
    renderWithI18n(
      <AdminUserDetailModal
        userId={userDetail.id}
        onClose={() => undefined}
        onUserStatusChanged={() => undefined}
      />,
    );

    expect(
      await screen.findByRole("dialog", { name: "Learner Account Details" }),
    ).toBeInTheDocument();
    expectIconCloseButton();
  });

  it("names the feature-flag dialog and its icon-only close button", () => {
    renderWithI18n(
      <CreateFeatureFlagModal
        isOpen
        onClose={() => undefined}
        onSuccess={() => undefined}
      />,
    );

    expect(
      screen.getByRole("dialog", { name: "Create Feature Flag" }),
    ).toBeInTheDocument();
    expectIconCloseButton();
  });
});
