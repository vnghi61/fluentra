import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";

import i18n, { initI18n } from "@/i18n";
import { MyWordsPage } from "@/routes/MyWordsPage";
import { useAuthStore } from "@/stores/authStore";

import { server } from "./msw-server";

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <MyWordsPage />
      </QueryClientProvider>
    </I18nextProvider>,
  );
}

function signIn() {
  useAuthStore.getState().setAuthSession({
    access_token: "valid-test-token",
    token_type: "Bearer",
    expires_in: 900,
    user_id: "user-123",
    role: "user",
  });
}

describe("MyWordsPage", () => {
  let submitted: string[] = [];

  beforeEach(async () => {
    signIn();
    initI18n("en");
    await i18n.changeLanguage("en");
    submitted = [];

    server.use(
      http.get("/api/v1/me/vocabulary/uploads", () =>
        HttpResponse.json({
          items: [
            {
              id: "0199a1c2-3d4e-7f80-9abc-def0123456aa",
              status: "completed",
              item_count: 3,
              verified_count: 2,
              rejected_count: 1,
              pending_count: 0,
              created_at: "2026-08-30T09:00:00Z",
            },
          ],
        }),
      ),
      http.get("/api/v1/me/vocabulary/uploads/:id", () =>
        HttpResponse.json({
          id: "0199a1c2-3d4e-7f80-9abc-def0123456aa",
          status: "completed",
          item_count: 3,
          verified_count: 2,
          rejected_count: 1,
          pending_count: 0,
          created_at: "2026-08-30T09:00:00Z",
          items: [
            {
              term: "leisure",
              provided_meaning: "thời gian rảnh",
              status: "verified",
              reason: "",
            },
            {
              term: "asdfgh",
              provided_meaning: "",
              status: "rejected",
              reason: 'We could not find "asdfgh" as an English word.',
            },
          ],
        }),
      ),
      http.post("/api/v1/me/vocabulary/uploads", async ({ request }) => {
        const body = (await request.json()) as { text: string };
        submitted.push(body.text);
        return HttpResponse.json(
          {
            id: "0199a1c2-3d4e-7f80-9abc-def0123456bb",
            status: "pending",
            item_count: 2,
            verified_count: 0,
            rejected_count: 0,
            pending_count: 2,
            created_at: "2026-08-30T11:00:00Z",
          },
          { status: 202 },
        );
      }),
    );
  });

  it("counts the words it will actually store, not the lines pasted", async () => {
    const user = userEvent.setup();
    renderPage();

    const box = await screen.findByLabelText(/paste your words/i);
    // Five lines: one duplicate, one page number, one divider.
    await user.type(
      box,
      "leisure{Enter}habit{Enter}leisure{Enter}42{Enter}---",
    );

    // Three of five is what the server's parser will find, and seeing it live
    // is less alarming than being told afterwards.
    await waitFor(() => {
      expect(screen.getByText(/2 words found/i)).toBeInTheDocument();
    });
  });

  it("cannot be submitted with nothing to add", async () => {
    renderPage();
    const button = await screen.findByRole("button", {
      name: /add these words/i,
    });
    expect(button).toBeDisabled();
  });

  it("sends the paste and clears the box", async () => {
    const user = userEvent.setup();
    renderPage();

    const box = await screen.findByLabelText(/paste your words/i);
    await user.type(box, "leisure - free time{Enter}habit");
    await user.click(screen.getByRole("button", { name: /add these words/i }));

    await waitFor(() => expect(submitted).toHaveLength(1));
    expect(submitted[0]).toContain("leisure - free time");
    await waitFor(() => expect(box).toHaveValue(""));
  });

  it("shows what became of an earlier upload", async () => {
    renderPage();

    // The counts lead: the question after pasting is "did it work".
    expect(await screen.findByText(/2 added/i)).toBeInTheDocument();
    expect(screen.getByText(/1 not found/i)).toBeInTheDocument();
  });

  it("loads the words themselves only once a row is opened", async () => {
    const user = userEvent.setup();
    renderPage();

    expect(screen.queryByText("leisure")).not.toBeInTheDocument();

    await user.click(await screen.findByRole("button", { name: /3 words/i }));

    expect(await screen.findByText("leisure")).toBeInTheDocument();
    // The rejection reason is written for the learner, so it is shown rather
    // than reduced to a status chip.
    expect(
      screen.getByText(/could not find "asdfgh" as an English word/i),
    ).toBeInTheDocument();
  });
});
