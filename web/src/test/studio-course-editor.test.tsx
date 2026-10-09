import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";

import i18n, { initI18n, loadLocale } from "@/i18n";
import { checkMaterialFile } from "@/features/resource";
import { CourseEditor } from "@/features/studio/components/CourseEditor";
import {
  courseMetrics,
  newActivity,
  newLesson,
  newUnit,
  priceInBounds,
  PRICE,
  slugify,
} from "@/features/studio/model/courseStructure";
import { server } from "./msw-server";

describe("course outline model", () => {
  it("slugifies Vietnamese titles without dropping accented letters", () => {
    expect(slugify("Giao tiếp cơ bản – Đời sống")).toBe(
      "giao-tiep-co-ban-doi-song",
    );
  });

  it("accepts 5.000 ₫ as the lowest paid price", () => {
    expect(PRICE.min).toBe(5_000);
    expect(priceInBounds(false, 5_000)).toBe(true);
    expect(priceInBounds(false, 4_999)).toBe(false);
    expect(priceInBounds(false, 5_000_001)).toBe(false);
    expect(priceInBounds(true, 0)).toBe(true);
  });

  it("counts materials apart from exercises, as Gate 1 does", () => {
    const unit = newUnit("U1");
    const lesson = newLesson("L1");
    lesson.activities.push(newActivity("lesson_material", "PDF"));
    unit.lessons.push(lesson);
    const m = courseMetrics([unit]);
    expect(m.exercises).toBe(0);
    expect(m.materials).toBe(1);
    // A lesson with one material is valid, but the material has no file yet.
    expect(m.invalidLessons).toBe(0);
    expect(m.materialsNotReady).toBe(1);
  });
});

describe("checkMaterialFile", () => {
  const file = (name: string, size: number) =>
    new File([new Uint8Array(size)], name);

  it("derives the kind and the declared MIME from the extension", () => {
    expect(checkMaterialFile(file("bang-diem.xlsx", 10))).toEqual({
      ok: true,
      kind: "document",
      mime: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    });
    expect(checkMaterialFile(file("anh.PNG", 10))).toMatchObject({
      ok: true,
      kind: "image",
    });
  });

  it("refuses unsupported types, empty files and oversize images", () => {
    expect(checkMaterialFile(file("virus.exe", 10))).toMatchObject({
      ok: false,
      reason: "type",
    });
    expect(checkMaterialFile(file("macro.xlsm", 10))).toMatchObject({
      ok: false,
      reason: "type",
    });
    expect(checkMaterialFile(file("empty.pdf", 0))).toMatchObject({
      ok: false,
      reason: "empty",
    });
    expect(
      checkMaterialFile(file("big.jpg", 10 * 1024 * 1024 + 1)),
    ).toMatchObject({ ok: false, reason: "size" });
  });
});

async function renderEditor() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const rootRoute = createRootRoute();
  const editorRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/studio/courses/new",
    component: () => (
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <CourseEditor />
        </QueryClientProvider>
      </I18nextProvider>
    ),
  });
  const studioRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/studio",
    component: () => <p>studio home</p>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([editorRoute, studioRoute]),
    history: createMemoryHistory({ initialEntries: ["/studio/courses/new"] }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

describe("CourseEditor", () => {
  beforeEach(async () => {
    await initI18n("en");
    await loadLocale("en");
    await i18n.changeLanguage("en");
  });

  it("fills the URL from a Vietnamese title", async () => {
    const user = userEvent.setup();
    await renderEditor();
    await user.type(
      await screen.findByLabelText(/course title/i),
      "Tiếng Anh cơ bản",
    );
    expect(screen.getByLabelText(/^url$/i)).toHaveValue("tieng-anh-co-ban");
  });

  it("rejects a paid price under 5.000 ₫ and accepts 5.000 ₫", async () => {
    const user = userEvent.setup();
    await renderEditor();
    await user.click(await screen.findByText(/one-time payment/i));
    const price = screen.getByLabelText(/course price/i);
    await user.clear(price);
    await user.type(price, "4000");
    await user.tab();
    expect(await screen.findByRole("alert")).toHaveTextContent(/whole number/i);
    await user.clear(price);
    await user.type(price, "5000");
    await user.tab();
    await waitFor(() =>
      expect(screen.queryByRole("alert")).not.toBeInTheDocument(),
    );
  });

  it("adds a reference material with a drop zone to a lesson", async () => {
    const user = userEvent.setup();
    await renderEditor();
    await user.click(
      await screen.findByRole("button", { name: /add material/i }),
    );
    expect(
      screen.getByText(/drop a file here or click to choose/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/excel/i, { selector: "span" }),
    ).toBeInTheDocument();
  });

  it("saves a new draft with the price the creator chose", async () => {
    let body: Record<string, unknown> | null = null;
    server.use(
      http.post("/api/v1/studio/courses", async ({ request }) => {
        body = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json(
          { id: "0199a1c2-3d4e-7f80-9abc-def012345678", ...body },
          { status: 201 },
        );
      }),
      http.get("/api/v1/studio/courses/:id", () =>
        HttpResponse.json({ message: "not found" }, { status: 404 }),
      ),
    );
    const user = userEvent.setup();
    await renderEditor();
    await user.type(
      await screen.findByLabelText(/course title/i),
      "Daily English",
    );
    await user.click(screen.getByText(/one-time payment/i));
    await user.click(
      within(
        screen.getByRole("group", { name: /suggested prices/i }),
      ).getByRole("button", { name: /19\.000/ }),
    );
    await user.click(screen.getByRole("button", { name: /save draft/i }));
    await waitFor(() => expect(body).not.toBeNull());
    expect(body).toMatchObject({
      title: "Daily English",
      slug: "daily-english",
      price_vnd: 19_000,
    });
  });
});
