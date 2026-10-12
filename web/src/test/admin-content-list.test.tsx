import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, it, vi } from "vitest";
import i18n, { initI18n, loadLocale } from "@/i18n";
import {
  adminApi,
  type AdminContentItemDetail,
  type ContentItem,
} from "@/features/admin/api/adminApi";
import { AdminContentList } from "@/features/admin/components/AdminContentList";
import { CreateContentModal } from "@/features/admin/components/CreateContentModal";
import { server } from "./msw-server";

// Derived from the adminListContent OpenAPI example, expanded deterministically
// to prove totals are not mistaken for the size of one page.
const contentItems: ContentItem[] = Array.from({ length: 40 }, (_, index) => ({
  id: `0199a1c2-3d4e-7f80-9abc-def012345${String(index + 1).padStart(3, "0")}`,
  slug: `environment-n-${String(index + 1).padStart(2, "0")}`,
  kind: index < 20 ? "vocab_word" : "grammar_exercise",
  status: index < 25 ? "published" : index < 35 ? "in_review" : "draft",
  owner_id: "0199a1c2-3d4e-7f80-9abc-def012345603",
  created_at: "2026-08-01T09:00:00Z",
  updated_at: "2026-08-02T11:30:00Z",
}));
const firstItem = contentItems[0]!;
const createdItem: ContentItem = {
  ...firstItem,
  id: "0199a1c2-3d4e-7f80-9abc-def01234567d",
  slug: "new-content-1",
  status: "draft",
};
const requests: URL[] = [];

function renderList() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <AdminContentList />
    </QueryClientProvider>,
  );
}
function renderCreate() {
  const onCreated = vi.fn();
  const onOpenChange = vi.fn();
  render(
    <CreateContentModal
      open
      onOpenChange={onOpenChange}
      onCreated={onCreated}
      kindOptions={[{ value: "vocab_word", label: "Vocabulary word" }]}
    />,
  );
  return { onCreated, onOpenChange };
}
function table() {
  return screen.getByRole("table", { name: "Content library" });
}

beforeEach(async () => {
  await initI18n("en");
  await loadLocale("en");
  await i18n.changeLanguage("en");
  requests.length = 0;
  server.use(
    http.get("/api/v1/me/permissions", () =>
      HttpResponse.json({
        permissions: [
          "content.edit",
          "content.create",
          "content.review",
          "content.publish",
        ],
      }),
    ),
    http.get("/api/v1/admin/content", ({ request }) => {
      const url = new URL(request.url);
      requests.push(url);
      const status = url.searchParams.get("status");
      const kind = url.searchParams.get("kind");
      const query = url.searchParams.get("q");
      const limit = Number(url.searchParams.get("limit") ?? 10);
      const offset = Number(url.searchParams.get("offset") ?? 0);
      const matches = contentItems.filter(
        (item) =>
          (!status || item.status === status) &&
          (!kind || item.kind === kind) &&
          (!query || item.slug.startsWith(query)),
      );
      return HttpResponse.json({
        items: matches.slice(offset, offset + limit),
        total: matches.length,
        limit,
        offset,
      });
    }),
    http.get("/api/v1/admin/content/:id", ({ params }) => {
      const item = params["id"] === createdItem.id ? createdItem : firstItem;
      const detail: AdminContentItemDetail = { ...item, versions: [] };
      return HttpResponse.json(detail);
    }),
    http.post("/api/v1/admin/content", () =>
      HttpResponse.json(createdItem, { status: 201 }),
    ),
  );
});

describe("content library", () => {
  it("shows API-backed totals across all pages, not the ten displayed rows", async () => {
    renderList();
    await screen.findByRole("table", { name: "Content library" });
    expect(within(table()).getAllByRole("row")).toHaveLength(11);
    await waitFor(() =>
      expect(screen.getByTestId("content-stat-total")).toHaveTextContent("40"),
    );
    expect(screen.getByTestId("content-stat-published")).toHaveTextContent(
      "25",
    );
    expect(screen.getByTestId("content-stat-review")).toHaveTextContent("10");
    expect(screen.getByTestId("content-stat-other")).toHaveTextContent("5");
    expect(screen.getByText("Showing 1–10 of 40")).toBeInTheDocument();
  });

  it("walks offset pages, jumps to a numbered page and resets when page size changes", async () => {
    const user = userEvent.setup();
    renderList();
    await screen.findByRole("table", { name: "Content library" });
    await user.click(screen.getByRole("button", { name: "Next" }));
    await waitFor(() =>
      expect(within(table()).getByText("environment-n-11")).toBeInTheDocument(),
    );
    await user.click(screen.getByRole("button", { name: "Go to page 4" }));
    await waitFor(() =>
      expect(within(table()).getByText("environment-n-31")).toBeInTheDocument(),
    );
    await user.selectOptions(
      screen.getByRole("combobox", { name: /Rows per page/i }),
      "25",
    );
    await waitFor(() =>
      expect(within(table()).getAllByRole("row")).toHaveLength(26),
    );
    expect(screen.getByText("Showing 1–25 of 40")).toBeInTheDocument();
  });

  it("submits a slug-prefix search once and resets it without pretending to search titles or ids", async () => {
    const user = userEvent.setup();
    renderList();
    await screen.findByRole("table", { name: "Content library" });
    const before = requests.length;
    await user.type(screen.getByRole("searchbox"), "environment-n-3");
    expect(requests).toHaveLength(before);
    await user.click(screen.getByRole("button", { name: "Search" }));
    await waitFor(() =>
      expect(screen.getByTestId("content-stat-total")).toHaveTextContent("10"),
    );
    expect(
      requests.some((url) => url.searchParams.get("q") === "environment-n-3"),
    ).toBe(true);
    await user.click(screen.getByRole("button", { name: "Reset filters" }));
    await waitFor(() =>
      expect(screen.getByTestId("content-stat-total")).toHaveTextContent("40"),
    );
    expect(screen.getByRole("searchbox")).toHaveValue("");
  });

  it("applies mobile status/kind filters only on confirmation, with counts scoped to kind but all statuses", async () => {
    const user = userEvent.setup();
    renderList();
    await screen.findByRole("table", { name: "Content library" });
    await user.click(screen.getByRole("button", { name: "Filters" }));
    const dialog = screen.getByRole("dialog", { name: "Filters" });
    await user.selectOptions(
      within(dialog).getByRole("combobox", { name: "Filter by status" }),
      "in_review",
    );
    await user.selectOptions(
      within(dialog).getByRole("combobox", { name: "Filter by content kind" }),
      "grammar_exercise",
    );
    expect(
      within(screen.getByTestId("content-cards")).getByText(firstItem.slug),
    ).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Apply" }));
    await waitFor(() =>
      expect(screen.getByText("Showing 1–10 of 10")).toBeInTheDocument(),
    );
    expect(within(table()).queryByText(firstItem.slug)).not.toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByTestId("content-stat-total")).toHaveTextContent("20"),
    );
    expect(screen.getByTestId("content-stat-published")).toHaveTextContent("5");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("cancels mobile filter changes without applying them", async () => {
    const user = userEvent.setup();
    renderList();
    await screen.findByRole("table", { name: "Content library" });
    await user.click(screen.getByRole("button", { name: "Filters" }));
    await user.selectOptions(
      within(screen.getByRole("dialog")).getByRole("combobox", {
        name: "Filter by status",
      }),
      "draft",
    );
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Reset filters" }),
    ).not.toBeInTheDocument();
    expect(within(table()).getByText(firstItem.slug)).toBeInTheDocument();
  });

  it("opens detail from a keyboard-accessible name and from the mobile action menu", async () => {
    const user = userEvent.setup();
    renderList();
    await screen.findByRole("table", { name: "Content library" });
    const titleButton = within(table()).getByRole("button", {
      name: firstItem.slug,
    });
    titleButton.focus();
    await user.keyboard("{Enter}");
    await screen.findByRole("dialog");
    await user.keyboard("{Escape}");
    const cards = screen.getByTestId("content-cards");
    await user.click(
      within(cards).getByRole("button", {
        name: `Actions for ${firstItem.slug}`,
      }),
    );
    await user.click(screen.getByRole("menuitem", { name: "Edit & review" }));
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
  });

  it("never nests the ID copy button inside a content button", async () => {
    const clipboard = vi.fn().mockResolvedValue(undefined);
    const user = userEvent.setup();
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText: clipboard },
    });
    renderList();
    await screen.findByRole("table", { name: "Content library" });
    expect(document.querySelector("button button")).toBeNull();
    await user.click(
      within(screen.getByTestId("content-cards")).getByRole("button", {
        name: `Copy ID: ${firstItem.id}`,
      }),
    );
    expect(clipboard).toHaveBeenCalledWith(firstItem.id);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("renders a mobile loading state and shows no fake zero summary while requests are pending", () => {
    renderList();
    expect(
      screen.getByRole("status", { name: "Loading content items…" }),
    ).toHaveAttribute("aria-busy", "true");
    expect(screen.queryByTestId("content-stat-total")).not.toBeInTheDocument();
  });

  it("shows summary failure as unknown counts while keeping the content list usable", async () => {
    server.use(
      http.get("/api/v1/admin/content", ({ request }) => {
        if (new URL(request.url).searchParams.get("limit") === "1")
          return HttpResponse.json({ status: 500 }, { status: 500 });
        return HttpResponse.json({
          items: [firstItem],
          total: 40,
          limit: 10,
          offset: 0,
        });
      }),
    );
    renderList();
    await screen.findByRole("table", { name: "Content library" });
    expect(
      await screen.findByText(
        "Content counts could not be loaded. The list is still available.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByTestId("content-stat-total")).toHaveTextContent("—");
    expect(
      screen.getByRole("button", { name: "Retry counts" }),
    ).toBeInTheDocument();
  });

  it("distinguishes a failed list from an empty result and retries without exposing server text", async () => {
    let failed = true;
    server.use(
      http.get("/api/v1/admin/content", () =>
        failed
          ? HttpResponse.json(
              { status: 500, title: "private database error" },
              { status: 500 },
            )
          : HttpResponse.json({ items: [], total: 0, limit: 10, offset: 0 }),
      ),
    );
    const user = userEvent.setup();
    renderList();
    await screen.findByRole("heading", {
      name: "Failed to load content items.",
    });
    expect(
      screen.queryByText("private database error"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("No content items match these filters."),
    ).not.toBeInTheDocument();
    failed = false;
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(
      await screen.findByText("No content items match these filters."),
    ).toBeInTheDocument();
  });

  it("hides creation unless the administrator holds content.create", async () => {
    server.use(
      http.get("/api/v1/me/permissions", () =>
        HttpResponse.json({ permissions: ["content.edit"] }),
      ),
    );
    renderList();
    await screen.findByRole("table", { name: "Content library" });
    expect(
      screen.queryByRole("button", { name: "Add content" }),
    ).not.toBeInTheDocument();
  });

  it("uses Vietnamese copy for filters, summary and the create dialog", async () => {
    await loadLocale("vi");
    await i18n.changeLanguage("vi");
    const user = userEvent.setup();
    renderList();
    await screen.findByRole("table", { name: "Thư viện nội dung" });
    expect(
      screen.getByRole("region", { name: "Thống kê nội dung" }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Thêm nội dung" }));
    const dialog = screen.getByRole("dialog", { name: "Thêm nội dung" });
    expect(
      within(dialog).getByRole("button", { name: "Đóng" }),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByRole("button", { name: "Tạo bản nháp" }),
    ).toBeInTheDocument();
  });
});

describe("create content draft", () => {
  it("validates required identifiers and JSON objects before sending a request", async () => {
    const user = userEvent.setup();
    const { onCreated } = renderCreate();
    await user.clear(screen.getByLabelText("Content body (JSON)"));
    await user.paste("[]");
    await user.click(screen.getByRole("button", { name: "Create draft" }));
    expect(
      await screen.findByText("This field is required."),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Enter valid JSON containing an object/),
    ).toBeInTheDocument();
    expect(onCreated).not.toHaveBeenCalled();
  });

  it("posts a real draft and opens its detail without auto-publishing", async () => {
    let posted: unknown;
    server.use(
      http.post("/api/v1/admin/content", async ({ request }) => {
        posted = await request.json();
        return HttpResponse.json(createdItem, { status: 201 });
      }),
    );
    const user = userEvent.setup();
    renderList();
    await user.click(
      await screen.findByRole("button", { name: "Add content" }),
    );
    await user.type(
      screen.getByRole("textbox", { name: "Slug" }),
      createdItem.slug,
    );
    await user.click(screen.getByRole("button", { name: "Create draft" }));
    expect(
      await screen.findByRole("dialog", { name: createdItem.slug }),
    ).toBeInTheDocument();
    expect(posted).toEqual({
      slug: createdItem.slug,
      kind: "vocab_word",
      cefr_level: "A1",
      body: {},
    });
  });

  it("retains the form after a failed save", async () => {
    server.use(
      http.post("/api/v1/admin/content", () =>
        HttpResponse.json({ status: 409, code: "CONFLICT" }, { status: 409 }),
      ),
    );
    const user = userEvent.setup();
    const { onCreated } = renderCreate();
    await user.type(
      screen.getByRole("textbox", { name: "Slug" }),
      "existing-slug",
    );
    await user.click(screen.getByRole("button", { name: "Create draft" }));
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Slug" })).toHaveValue(
      "existing-slug",
    );
    expect(onCreated).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Create draft" })).toBeEnabled();
  });

  it("shows a localized safe message instead of raw server errors", async () => {
    server.use(
      http.post("/api/v1/admin/content", () =>
        HttpResponse.json(
          { status: 500, title: "private database error" },
          { status: 500 },
        ),
      ),
    );
    const user = userEvent.setup();
    renderCreate();
    await user.type(
      screen.getByRole("textbox", { name: "Slug" }),
      createdItem.slug,
    );
    await user.click(screen.getByRole("button", { name: "Create draft" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Content could not be created. Please try again.",
    );
    expect(
      screen.queryByText("private database error"),
    ).not.toBeInTheDocument();
  });

  it("locks edits and dismissal during save", async () => {
    let finish: (item: ContentItem) => void = () => {};
    const response = new Promise<ContentItem>((resolve) => {
      finish = resolve;
    });
    const create = vi
      .spyOn(adminApi, "createContent")
      .mockReturnValue(response);
    const user = userEvent.setup();
    const { onCreated, onOpenChange } = renderCreate();
    await user.type(
      screen.getByRole("textbox", { name: "Slug" }),
      createdItem.slug,
    );
    await user.click(screen.getByRole("button", { name: "Create draft" }));
    expect(screen.getByRole("textbox", { name: "Slug" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Close" })).toBeDisabled();
    await user.keyboard("{Escape}");
    expect(onOpenChange).not.toHaveBeenCalled();
    await act(async () => {
      finish(createdItem);
      await response;
    });
    expect(onCreated).toHaveBeenCalledWith(createdItem);
    expect(onOpenChange).toHaveBeenCalledWith(false);
    create.mockRestore();
  });
});
