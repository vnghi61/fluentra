import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";
import i18n, { initI18n } from "@/i18n";
import { AdminAIUsage } from "@/features/admin/components/AdminAIUsage";
import { quotaStatus } from "@/features/admin/model/aiUsage";
import { server } from "./msw-server";

const usage = [
  {
    provider: "openai",
    task: "grading",
    requests_today: 70,
    tokens_today: 10,
    daily_request_limit: 100,
    daily_token_limit: 100,
    is_exhausted: false,
  },
  {
    provider: "openai",
    task: "translation",
    requests_today: 20,
    tokens_today: 20,
    daily_request_limit: null,
    daily_token_limit: null,
    is_exhausted: false,
  },
  {
    provider: "anthropic",
    task: "review",
    requests_today: 96,
    tokens_today: 12,
    daily_request_limit: 100,
    daily_token_limit: 100,
    is_exhausted: false,
  },
];

describe("AdminAIUsage", () => {
  beforeEach(async () => {
    await initI18n("en");
    server.use(
      http.get("/api/v1/admin/ai/usage", () =>
        HttpResponse.json({ items: usage }),
      ),
    );
  });
  it("has no table, groups providers, and filters to the empty state", async () => {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <AdminAIUsage />
        </QueryClientProvider>
      </I18nextProvider>,
    );
    expect(await screen.findByText("grading")).toBeInTheDocument();
    expect(screen.getAllByText("openai").length).toBeGreaterThan(0);
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    await userEvent.type(
      screen.getByRole("textbox", { name: "Task" }),
      "no-such-task",
    );
    expect(await screen.findByText("Nothing here yet")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Reset filters" }),
    ).toBeInTheDocument();
  });
  it("classifies quota thresholds and exhaustion", () => {
    expect(quotaStatus(70)).toBe("normal");
    expect(quotaStatus(71)).toBe("highlight");
    expect(quotaStatus(86)).toBe("warning");
    expect(quotaStatus(96)).toBe("critical");
    expect(quotaStatus(1, true)).toBe("critical");
  });
});
