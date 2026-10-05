import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { beforeEach, expect, it } from "vitest";
import i18n, { initI18n, loadLocale } from "@/i18n";
import { SubmissionStatusModal } from "@/features/studio";
import { studioApi } from "@/features/studio/api/studioApi";
import { server } from "./msw-server";

beforeEach(async () => {
  await initI18n("en");
  await loadLocale("en");
  await i18n.changeLanguage("en");
});

it("reads the latest submission and typed Gate 1 failures from draft detail", async () => {
  let reads = 0;
  server.use(
    http.get("/api/v1/studio/courses/draft-1", () => {
      reads++;
      return HttpResponse.json({
        latest_submission: {
          id: "submission-1",
          draft_id: "draft-1",
          version: 1,
          status: "changes_requested",
          submitted_at: "2026-10-05T00:00:00Z",
          verification_report: {
            passed: false,
            items_checked: 3,
            blind_solved: 0,
            blind_solve_rejects: 0,
            failures: [
              {
                unit_index: 0,
                lesson_index: 0,
                activity_index: 1,
                check: "material_ready",
                message: "Video is still processing",
              },
            ],
          },
        },
      });
    }),
  );
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <SubmissionStatusModal
        draftId="draft-1"
        draftTitle="Course"
        isOpen
        onClose={() => {}}
      />
    </QueryClientProvider>,
  );
  expect(
    await screen.findByText("Video is still processing"),
  ).toBeInTheDocument();
  expect(screen.getByText("material_ready")).toBeInTheDocument();
  expect(reads).toBe(1);
});

it("returns no submissions when draft detail has never been submitted", async () => {
  server.use(
    http.get("/api/v1/studio/courses/draft-1", () => HttpResponse.json({})),
  );
  expect(await studioApi.getDraftSubmissions("draft-1")).toEqual([]);
});

it("shows a load error instead of pretending the draft has no submissions", async () => {
  server.use(
    http.get(
      "/api/v1/studio/courses/draft-1",
      () => new HttpResponse(null, { status: 500 }),
    ),
  );
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <SubmissionStatusModal
        draftId="draft-1"
        draftTitle="Course"
        isOpen
        onClose={() => {}}
      />
    </QueryClientProvider>,
  );
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "Could not load submission status.",
  );
});
