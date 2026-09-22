import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { fireEvent, render, screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";

import i18n, { initI18n } from "@/i18n";
import { ResourceDetailPage } from "@/routes/ResourceDetailPage";
import { MyResourcesPage } from "@/routes/MyResourcesPage";
import { useAuthStore } from "@/stores/authStore";

import { server } from "./msw-server";

const validatedResource = {
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
  extraction: {
    source: "pdf_text",
    char_count: 1540,
    truncated: false,
    excerpt:
      "Unit 1: Present Perfect Tense. She has lived here for three years.",
  },
  classification: {
    cefr_estimate: "B1",
    skill: "grammar",
    nodes: [{ code: "PRESENT_PERFECT", label: "Present Perfect" }],
  },
};

const rejectedResource = {
  ...validatedResource,
  id: "0199a1c2-3d4e-7f80-9abc-def0123456a2",
  title: "scan.png",
  status: "rejected",
  failure_reason:
    "We could not read this file. Its contents did not match its type.",
  detected_mime: "image/png",
  renditions: [],
  extraction: undefined,
  classification: undefined,
};

function resourcesPage(initialEntry = "/my-resources") {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const rootRoute = createRootRoute();
  const listRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/my-resources",
    component: () => (
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <MyResourcesPage />
        </QueryClientProvider>
      </I18nextProvider>
    ),
  });
  const detailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/my-resources/$resourceId",
    component: () => (
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <ResourceDetailPage />
        </QueryClientProvider>
      </I18nextProvider>
    ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([listRoute, detailRoute]),
    history: createMemoryHistory({ initialEntries: [initialEntry] }),
  });
  return { client, router };
}

async function renderPage(initialEntry = "/my-resources") {
  const { router } = resourcesPage(initialEntry);
  await router.load();
  return render(<RouterProvider router={router} />);
}

describe("MyResourcesPage", () => {
  beforeEach(async () => {
    useAuthStore.getState().setAuthSession({
      access_token: "valid-test-token",
      token_type: "Bearer",
      expires_in: 900,
      user_id: "user-123",
      role: "user",
    });
    await initI18n("en");
    await i18n.changeLanguage("en");

    server.use(
      http.get("/api/v1/me/resources", () =>
        HttpResponse.json({
          items: [validatedResource, rejectedResource],
          total: 2,
          page: 1,
          page_size: 100,
        }),
      ),
    );
  });

  it("lists the learner's resources with their state and the quota", async () => {
    await renderPage();

    expect(await screen.findByText("grammar-notes.pdf")).toBeInTheDocument();
    expect(screen.getByText("scan.png")).toBeInTheDocument();
    // A validated file with no renditions yet is still being processed, and a
    // rejection says why rather than showing a bare status.
    expect(screen.getAllByText("Processing").length).toBeGreaterThan(0);
    expect(screen.getByText(/did not match its type/i)).toBeInTheDocument();
    expect(screen.getByText(/of 50 files/)).toBeInTheDocument();
  });

  it("refuses an oversized file before uploading it", async () => {
    const { container } = await renderPage();
    await screen.findByText("grammar-notes.pdf");

    const input = container.querySelector('input[type="file"]');
    expect(input).not.toBeNull();
    const file = new File([new Uint8Array(10)], "huge.pdf", {
      type: "application/pdf",
    });
    Object.defineProperty(file, "size", { value: 51 * 1024 * 1024 });
    fireEvent.change(input as HTMLInputElement, { target: { files: [file] } });

    expect(await screen.findByRole("alert")).toHaveTextContent(
      /50 MB or smaller/,
    );
  });
});

describe("ResourceDetailPage", () => {
  beforeEach(async () => {
    useAuthStore.getState().setAuthSession({
      access_token: "valid-test-token",
      token_type: "Bearer",
      expires_in: 900,
      user_id: "user-123",
      role: "user",
    });
    await initI18n("en");
    await i18n.changeLanguage("en");

    server.use(
      http.get("/api/v1/me/resources/:id", () =>
        HttpResponse.json(validatedResource),
      ),
    );
  });

  it("shows the extraction, the estimate and the processing note", async () => {
    await renderPage(`/my-resources/${validatedResource.id}`);

    // Twice: once as the excerpt's text and once as the spine node's label.
    expect(
      (await screen.findAllByText(/present perfect/i)).length,
    ).toBeGreaterThan(0);
    expect(screen.getByText(/Around B1/)).toBeInTheDocument();
    expect(screen.getByText(/1540 characters/)).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent(
      /preparing this file/i,
    );
  });
});
