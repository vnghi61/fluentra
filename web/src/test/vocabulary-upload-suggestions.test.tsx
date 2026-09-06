import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";

import { UploadForm } from "@/features/vocabulary";
import i18n, { initI18n } from "@/i18n";
import { server } from "./msw-server";

/**
 * The suggestions have to be reachable from the screen a learner actually opens.
 *
 * WordAutocomplete was written, exported and unit-tested, and rendered by
 * nothing — the fourth time in this project that a finished component shipped
 * with no route to it. A test against the component alone could not see that,
 * because the component was never the broken part. This one renders the form
 * the My Words page renders, and would fail if the field were unmounted again.
 */
function renderUploadForm() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>
        <UploadForm />
      </QueryClientProvider>
    </I18nextProvider>,
  );
}

function suggest(lemma: string) {
  server.use(
    http.get("/api/v1/vocabulary/search", () =>
      HttpResponse.json({
        results: [
          {
            id: "0199a1c2-3d4e-7f80-9abc-def012345001",
            lemma,
            pos: "noun",
            cefr_level: "A2",
            ipa: "/wɜːd/",
          },
        ],
        total: 1,
      }),
    ),
  );
}

describe("Adding a word with suggestions", () => {
  beforeEach(async () => {
    initI18n("en");
    await i18n.changeLanguage("en");
  });

  it("offers the dictionary from the upload screen itself", async () => {
    const user = userEvent.setup();
    suggest("word");
    renderUploadForm();

    const search = screen.getByRole("combobox");
    await user.type(search, "wor");

    expect(await screen.findByText("word")).toBeInTheDocument();
  });

  it("drops the chosen word into the list rather than submitting on its own", async () => {
    const user = userEvent.setup();
    suggest("word");
    renderUploadForm();

    await user.type(screen.getByRole("combobox"), "wor");
    await user.click(await screen.findByText("word"));

    // The box below is what gets submitted. Picking six words is still one
    // upload, and the learner can edit the list before sending it.
    const list = screen.getByLabelText(/paste your words/i);
    expect(list).toHaveValue("word\n");
  });

  it("leaves a word it has never heard of alone", async () => {
    const user = userEvent.setup();
    server.use(
      http.get("/api/v1/vocabulary/search", () =>
        HttpResponse.json({ results: [], total: 0 }),
      ),
    );
    renderUploadForm();

    // Nothing matched, and the learner's own spelling is still the answer:
    // the whole point of the upload feature is words the dictionary lacks.
    await user.type(screen.getByRole("combobox"), "worldbuilding{Enter}");

    const list = screen.getByLabelText(/paste your words/i);
    expect(list).toHaveValue("worldbuilding\n");
  });

  it("does not add a word the list already has", async () => {
    const user = userEvent.setup();
    suggest("word");
    renderUploadForm();

    // Typed by hand first, in the casing a learner would actually use.
    const list = screen.getByLabelText(/paste your words/i);
    await user.type(list, "Word");

    await user.type(screen.getByRole("combobox"), "wor");
    await user.click(await screen.findByText("word"));

    // The parser drops duplicates anyway, so appending one would leave the
    // count beside the button unchanged after a visible edit — which reads as
    // a button that did nothing.
    expect(list).toHaveValue("Word");
  });
});
