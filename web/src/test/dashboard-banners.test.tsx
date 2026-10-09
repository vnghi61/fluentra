import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEventDefault from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from "@tanstack/react-router";

import i18n, { initI18n, loadLocale } from "@/i18n";
import { DashboardBanners, isSafeBannerUrl } from "@/features/banners";
import { SLIDE_AUTOPLAY_MS } from "@/features/banners/components/SlideCarousel";
import { useAuthStore } from "@/stores/authStore";
import type { components } from "@/types/api";
import { server } from "./msw-server";

type ActiveBanner = components["schemas"]["ActiveBanner"];

const USER_ID = "0199a1c2-3d4e-7f80-9abc-def012345678";
const STORAGE_KEY = `fluentra.banners.dismissed.${USER_ID}`;

function banner(overrides: Partial<ActiveBanner> = {}): ActiveBanner {
  return {
    id: "0199c3e4-5f60-7a82-9cde-f01234567890",
    kind: "announcement",
    title: "IELTS mock week",
    body: "Take a full mock test free until Sunday.",
    cta_label: "Start a mock test",
    cta_url: "/exams",
    tone: "promo",
    dismissible: true,
    ...overrides,
  };
}

function serve(items: ActiveBanner[]) {
  server.use(
    http.get("/api/v1/banners/active", () =>
      HttpResponse.json({
        items,
      } satisfies components["schemas"]["ActiveBannerList"]),
    ),
  );
}

async function renderBanners() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const rootRoute = createRootRoute({
    component: () => (
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <Outlet />
        </QueryClientProvider>
      </I18nextProvider>
    ),
  });
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => (
      <div>
        <DashboardBanners />
        <p>dashboard body</p>
      </div>
    ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

const SESSION_KEY = `fluentra.banners.seen.${USER_ID}`;

function slide(n: number, overrides: Partial<ActiveBanner> = {}): ActiveBanner {
  return banner({
    id: `0199c3e4-5f60-7a82-9cde-f0123456780${n}`,
    kind: "slide",
    title: `Promo ${n}`,
    body: "",
    cta_label: null,
    cta_url: null,
    dismissible: false,
    image_url: `https://storage.test/signed/promo-${n}.jpg`,
    image_alt: `Promo picture ${n}`,
    image_width: 1500,
    image_height: 500,
    ...overrides,
  });
}

describe("DashboardBanners: announcements", () => {
  beforeEach(async () => {
    await initI18n("en");
    await loadLocale("en");
    await i18n.changeLanguage("en");
    window.localStorage.clear();
    window.sessionStorage.clear();
    useAuthStore.setState({
      status: "authenticated",
      user: { userId: USER_ID, role: "user" },
    });
  });

  afterEach(() => {
    window.localStorage.clear();
    window.sessionStorage.clear();
  });

  it("opens an announcement as a dialog with its call to action as a router link", async () => {
    serve([banner()]);
    await renderBanners();

    const dialog = await screen.findByRole("dialog", {
      name: "IELTS mock week",
    });
    expect(
      within(dialog).getByText("Take a full mock test free until Sunday."),
    ).toBeInTheDocument();
    const cta = within(dialog).getByRole("link", { name: /Start a mock test/ });
    expect(cta).toHaveAttribute("href", "/exams");
    expect(cta).not.toHaveAttribute("target");
  });

  it("opens an https call to action in a new tab without an opener", async () => {
    serve([banner({ cta_url: "https://fluentra.dev/ielts" })]);
    await renderBanners();

    const cta = await screen.findByRole("link", { name: /Start a mock test/ });
    expect(cta).toHaveAttribute("href", "https://fluentra.dev/ielts");
    expect(cta).toHaveAttribute("target", "_blank");
    expect(cta).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("renders the text as text, never as HTML", async () => {
    serve([
      banner({ title: "<b>bold</b>", body: "<img src=x onerror=alert(1)>" }),
    ]);
    await renderBanners();

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("<b>bold</b>")).toBeInTheDocument();
    expect(dialog.querySelector("b")).toBeNull();
    expect(dialog.querySelector("img")).toBeNull();
  });

  it("hides a dismissible announcement for good once it is closed", async () => {
    const user = userEventDefault.setup();
    serve([banner()]);
    const first = await renderBanners();

    await user.click(
      await screen.findByRole("button", {
        name: "Close announcement: IELTS mock week",
      }),
    );
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    expect(
      JSON.parse(window.localStorage.getItem(STORAGE_KEY) ?? "[]"),
    ).toEqual([banner().id]);

    // A later visit, even in a new browser session, keeps it closed.
    first.unmount();
    window.sessionStorage.clear();
    await renderBanners();
    expect(await screen.findByText("dashboard body")).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
  });

  it("brings a non-dismissible announcement back on the next visit, not within one", async () => {
    const user = userEventDefault.setup();
    serve([banner({ dismissible: false })]);
    const first = await renderBanners();

    await user.click(await screen.findByRole("button", { name: /^Close$/ }));
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    expect(window.localStorage.getItem(STORAGE_KEY)).toBeNull();
    expect(
      JSON.parse(window.sessionStorage.getItem(SESSION_KEY) ?? "[]"),
    ).toEqual([banner().id]);

    // Back to the dashboard in the same session: no second pop-up.
    first.unmount();
    const second = await renderBanners();
    expect(await screen.findByText("dashboard body")).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );

    // A new session: it opens again.
    second.unmount();
    window.sessionStorage.clear();
    await renderBanners();
    expect(
      await screen.findByRole("dialog", { name: "IELTS mock week" }),
    ).toBeInTheDocument();
  });

  it("opens one announcement per visit, highest priority first", async () => {
    const user = userEventDefault.setup();
    serve([
      banner(),
      banner({
        id: "0199c3e4-5f60-7a82-9cde-f01234567891",
        title: "Maintenance tonight",
      }),
    ]);
    await renderBanners();

    await screen.findByRole("dialog", { name: "IELTS mock week" });
    await user.click(screen.getByRole("button", { name: /^Close$/ }));
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    // Closing one does not open the next straight away.
    expect(screen.queryByText("Maintenance tonight")).not.toBeInTheDocument();
  });

  it("renders nothing when the request fails", async () => {
    server.use(
      http.get("/api/v1/banners/active", () =>
        HttpResponse.json(
          { title: "Bad request", status: 400 },
          { status: 400 },
        ),
      ),
    );
    await renderBanners();

    expect(await screen.findByText("dashboard body")).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("survives storage that refuses to work", async () => {
    const getItem = vi
      .spyOn(Storage.prototype, "getItem")
      .mockImplementation(() => {
        throw new Error("denied");
      });
    try {
      serve([banner()]);
      await renderBanners();
      expect(
        await screen.findByRole("dialog", { name: "IELTS mock week" }),
      ).toBeInTheDocument();
    } finally {
      getItem.mockRestore();
    }
  });
});

describe("DashboardBanners: slides", () => {
  beforeEach(async () => {
    await initI18n("en");
    await loadLocale("en");
    await i18n.changeLanguage("en");
    window.localStorage.clear();
    window.sessionStorage.clear();
    useAuthStore.setState({
      status: "authenticated",
      user: { userId: USER_ID, role: "user" },
    });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("shows a slide as a picture, linked when it has a link, with nothing to dismiss", async () => {
    serve([slide(1, { cta_url: "/pricing" })]);
    await renderBanners();

    const picture = await screen.findByRole("img", { name: "Promo picture 1" });
    expect(picture.closest("a")).toHaveAttribute("href", "/pricing");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Close|Dismiss/ }),
    ).not.toBeInTheDocument();
    // One slide: no carousel controls.
    expect(
      screen.queryByRole("button", { name: "Next announcement" }),
    ).not.toBeInTheDocument();
  });

  it("moves between slides with next, previous and the dots, wrapping round", async () => {
    const user = userEventDefault.setup();
    serve([slide(1), slide(2)]);
    await renderBanners();

    expect(
      await screen.findByRole("img", { name: "Promo picture 1" }),
    ).toBeInTheDocument();
    // The other slide is out of view and out of the accessibility tree.
    expect(
      screen.queryByRole("img", { name: "Promo picture 2" }),
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Next announcement" }));
    expect(
      await screen.findByRole("img", { name: "Promo picture 2" }),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Next announcement" }));
    expect(
      await screen.findByRole("img", { name: "Promo picture 1" }),
    ).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: "Previous announcement" }),
    );
    expect(
      await screen.findByRole("img", { name: "Promo picture 2" }),
    ).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: "Show announcement 1 of 2" }),
    );
    expect(
      screen.getByRole("button", { name: "Show announcement 1 of 2" }),
    ).toHaveAttribute("aria-current", "true");
  });

  it("advances on its own and stops once paused", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    serve([slide(1), slide(2)]);
    await renderBanners();
    await screen.findByRole("img", { name: "Promo picture 1" });

    act(() => {
      vi.advanceTimersByTime(SLIDE_AUTOPLAY_MS);
    });
    expect(
      await screen.findByRole("img", { name: "Promo picture 2" }),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Pause rotating announcements" }),
    );
    act(() => {
      vi.advanceTimersByTime(SLIDE_AUTOPLAY_MS * 3);
    });
    expect(
      screen.getByRole("img", { name: "Promo picture 2" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Resume rotating announcements" }),
    ).toBeInTheDocument();
  });

  it("follows a horizontal swipe and ignores a vertical one", async () => {
    serve([slide(1), slide(2)]);
    await renderBanners();
    const picture = await screen.findByRole("img", { name: "Promo picture 1" });
    const viewport = picture.closest("[aria-live]") as HTMLElement;

    const swipe = (dx: number, dy: number) => {
      fireEvent.pointerDown(viewport, {
        pointerId: 1,
        pointerType: "touch",
        clientX: 200,
        clientY: 100,
      });
      fireEvent.pointerMove(viewport, {
        pointerId: 1,
        pointerType: "touch",
        clientX: 200 + dx,
        clientY: 100 + dy,
      });
      fireEvent.pointerUp(viewport, {
        pointerId: 1,
        pointerType: "touch",
        clientX: 200 + dx,
        clientY: 100 + dy,
      });
    };

    swipe(0, -120); // a scroll
    expect(
      screen.getByRole("img", { name: "Promo picture 1" }),
    ).toBeInTheDocument();

    swipe(-120, 4); // leftwards: the next slide
    expect(
      await screen.findByRole("img", { name: "Promo picture 2" }),
    ).toBeInTheDocument();
  });

  it("shows slides and opens an announcement side by side", async () => {
    serve([banner(), slide(1)]);
    await renderBanners();

    expect(
      await screen.findByRole("dialog", { name: "IELTS mock week" }),
    ).toBeInTheDocument();
    // The carousel stays on the page behind the dialog.
    expect(document.querySelector('img[alt="Promo picture 1"]')).not.toBeNull();
  });
});

describe("isSafeBannerUrl", () => {
  it.each([
    ["/exams", true],
    ["https://fluentra.dev/ielts", true],
    ["//evil.example", false],
    ["/\\evil.example", false],
    ["javascript:alert(1)", false],
    ["data:text/html,x", false],
    ["http://example.com", false],
    ["java\nscript:alert(1)", false],
  ])("%s -> %s", (url, safe) => {
    expect(isSafeBannerUrl(url)).toBe(safe);
  });
});
