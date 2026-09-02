import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import React from "react";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";

import { WordAutocomplete } from "@/features/vocabulary";
import i18n, { initI18n } from "@/i18n";
import { server } from "./msw-server";

function renderAutocomplete(props: React.ComponentProps<typeof WordAutocomplete>) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>
        <WordAutocomplete {...props} />
      </QueryClientProvider>
    </I18nextProvider>,
  );
}

describe("WordAutocomplete Component (WP13)", () => {
  beforeEach(async () => {
    initI18n("en");
    await i18n.changeLanguage("en");
  });

  it("renders search input and suggests matching dictionary words", async () => {
    const user = userEvent.setup();

    server.use(
      http.get("/api/v1/vocabulary/search", ({ request }) => {
        const url = new URL(request.url);
        const q = url.searchParams.get("q");
        if (q === "met") {
          return HttpResponse.json({
            results: [
              {
                id: "0199a1c2-3d4e-7f80-9abc-def01234567b",
                lemma: "meticulous",
                pos: "adjective",
                cefr_level: "C1",
                ipa: "/məˈtɪk.jə.ləs/",
              },
              {
                id: "0199a1c2-3d4e-7f80-9abc-def01234567c",
                lemma: "method",
                pos: "noun",
                cefr_level: "A2",
                ipa: "/ˈmeθ.əd/",
              },
            ],
            total: 2,
          });
        }
        return HttpResponse.json({ results: [], total: 0 });
      }),
    );

    renderAutocomplete({});

    const input = screen.getByRole("combobox");
    await user.type(input, "met");

    // Wait for search suggestions to show
    expect(await screen.findByText("meticulous")).toBeInTheDocument();
    expect(screen.getByText("method")).toBeInTheDocument();
    expect(screen.getByText("C1")).toBeInTheDocument();
    expect(screen.getByText("A2")).toBeInTheDocument();
  });

  it("selecting an existing suggestion selects the sense with zero LLM model calls", async () => {
    const user = userEvent.setup();
    let selectedWordLemma = "";
    let aiModelCalled = false;

    server.use(
      http.get("/api/v1/vocabulary/search", () =>
        HttpResponse.json({
          results: [
            {
              id: "0199a1c2-3d4e-7f80-9abc-def01234567b",
              lemma: "bank",
              pos: "noun",
              cefr_level: "A1",
              ipa: "/bæŋk/",
            },
          ],
          total: 1,
        }),
      ),
      // If any AI endpoint is hit, record failure
      http.post("/api/v1/ai/*", () => {
        aiModelCalled = true;
        return HttpResponse.json({ error: "AI should not be called" }, { status: 500 });
      }),
    );

    renderAutocomplete({
      onSelectWord: (word) => {
        selectedWordLemma = word.lemma;
      },
    });

    const input = screen.getByRole("combobox");
    await user.type(input, "ban");

    const option = await screen.findByText("bank");
    await user.click(option);

    expect(selectedWordLemma).toBe("bank");
    expect(aiModelCalled).toBe(false);
  });

  it("shows custom word option when no dictionary results match", async () => {
    const user = userEvent.setup();
    let customSubmittedTerm = "";

    server.use(
      http.get("/api/v1/vocabulary/search", () =>
        HttpResponse.json({ results: [], total: 0 }),
      ),
    );

    renderAutocomplete({
      onCustomSubmit: (term) => {
        customSubmittedTerm = term;
      },
    });

    const input = screen.getByRole("combobox");
    await user.type(input, "unheardwordxyz");

    expect(
      await screen.findByText(/"unheardwordxyz" is not in the dictionary yet/i),
    ).toBeInTheDocument();

    const customButton = screen.getByRole("button", { name: /Add as new custom word/i });
    await user.click(customButton);

    expect(customSubmittedTerm).toBe("unheardwordxyz");
  });
});
