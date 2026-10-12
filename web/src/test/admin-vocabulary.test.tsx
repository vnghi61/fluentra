import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";

import i18n, { initI18n } from "@/i18n";
import { AdminVocabulary } from "@/features/admin/components/AdminVocabulary";
import { server } from "./msw-server";

describe("AdminVocabulary responsive lists", () => {
  beforeEach(async () => {
    await initI18n("en");
    server.use(
      http.get("/api/v1/me/permissions", () =>
        HttpResponse.json({ roles: ["admin"], permissions: ["content.edit"] }),
      ),
      http.get("/api/v1/admin/vocabulary/words", () =>
        HttpResponse.json({
          items: [
            {
              id: "word-1",
              lemma: "serendipity",
              pos: "noun",
              cefr_level: "C1",
              is_uploaded: false,
              senses_count: 1,
              created_at: "2026-01-01T00:00:00Z",
              updated_at: "2026-01-01T00:00:00Z",
            },
          ],
          total: 1,
          limit: 15,
          offset: 0,
        }),
      ),
      http.get("/api/v1/admin/vocabulary/queue", () =>
        HttpResponse.json({ items: [], total: 0, limit: 15, offset: 0 }),
      ),
    );
  });

  it("renders API-backed dictionary cards and confirms destructive withdrawals", async () => {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <AdminVocabulary />
        </QueryClientProvider>
      </I18nextProvider>,
    );
    await screen.findByTestId("vocabulary-word-cards");
    expect(screen.getByTestId("vocabulary-word-cards")).toHaveTextContent(
      "serendipity",
    );
    await userEvent.click(
      within(screen.getByTestId("vocabulary-word-cards")).getByRole("button", {
        name: /withdraw/i,
      }),
    );
    expect(await screen.findByRole("alertdialog")).toBeInTheDocument();
  });
});
