import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AppShell } from "@/components/layout/AppShell";
import { TooltipProvider } from "@/components/ui/tooltip";
import i18n, { initI18n, loadLocale } from "@/i18n";
import { NotFoundPage } from "@/routes/NotFoundPage";
import { classifyError } from "@/routes/RouteErrorPage";

async function renderAt(path: string, Component: () => ReactElement) {
  const rootRoute = createRootRoute();
  const pageRoute = createRoute({
    getParentRoute: () => rootRoute,
    path,
    component: Component,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([pageRoute]),
    history: createMemoryHistory({ initialEntries: [path] }),
  });
  await router.load();
  return render(
    <TooltipProvider delayDuration={0}>
      <RouterProvider router={router} />
    </TooltipProvider>,
  );
}

describe("Phase 1 shell and status pages", () => {
  beforeEach(async () => {
    await initI18n("en");
    await loadLocale("en");
    await i18n.changeLanguage("en");
  });

  it("classifies route errors as chunk, offline or generic", () => {
    expect(
      classifyError(
        new Error("Failed to fetch dynamically imported module: x"),
      ),
    ).toBe("chunk");
    expect(classifyError(new TypeError("Failed to fetch"))).toBe("offline");
    expect(classifyError(new Error("boom"))).toBe("generic");
  });

  it("renders the 404 page with a link home", async () => {
    await renderAt("/missing", NotFoundPage);

    expect(await screen.findByText("404")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Page not found", level: 1 }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Back to home/i })).toHaveAttribute(
      "href",
      "/",
    );
  });

  it("offers a way back from a More page: to the More menu, or up to its section", async () => {
    const user = userEvent.setup();
    const first = await renderAt("/settings", () => (
      <AppShell status="authenticated" user={{ role: "user" }}>
        <div>settings content</div>
      </AppShell>
    ));
    await user.click(
      await screen.findByRole("button", { name: "Back to More" }),
    );
    expect(
      await screen.findByRole("dialog", { name: "More" }),
    ).toBeInTheDocument();
    first.unmount();

    await renderAt("/exams/$attemptId", () => (
      <AppShell status="authenticated" user={{ role: "user" }}>
        <div>attempt</div>
      </AppShell>
    ));
    expect(
      await screen.findByRole("link", { name: "Back to Mock Exams" }),
    ).toHaveAttribute("href", "/exams");
  });

  it("keeps a page the More menu leads to when More is tapped again", async () => {
    const user = userEvent.setup();
    await renderAt("/settings", () => (
      <AppShell status="authenticated" user={{ role: "user" }}>
        <div>settings content</div>
      </AppShell>
    ));

    const nav = await screen.findByRole("navigation", {
      name: /Mobile navigation/i,
    });
    const more = within(nav).getByRole("button", { name: /More/i });
    await user.click(more);
    await user.click(more);

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByText("settings content")).toBeInTheDocument();
  });

  it("draws five mobile nav items and opens the More sheet", async () => {
    const user = userEvent.setup();
    await renderAt("/", () => (
      <AppShell status="authenticated" user={{ role: "user" }}>
        <div>content</div>
      </AppShell>
    ));

    const nav = await screen.findByRole("navigation", {
      name: /Mobile navigation/i,
    });
    expect(within(nav).getAllByRole("link")).toHaveLength(4);
    expect(within(nav).getAllByRole("button")).toHaveLength(1);

    await user.click(within(nav).getByRole("button", { name: /More/i }));

    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByRole("link", { name: /Exams/i }),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByRole("link", { name: /Studio/i }),
    ).toBeInTheDocument();
  });

  it("shows theme pickers and account sections on the More screen", async () => {
    const user = userEvent.setup();
    const onThemeChoice = vi.fn();
    const onLocale = vi.fn();
    await renderAt("/", () => (
      <AppShell
        status="authenticated"
        user={{ role: "user" }}
        themeChoice="light"
        onThemeChoice={onThemeChoice}
        locale="en"
        onLocale={onLocale}
      >
        <div>content</div>
      </AppShell>
    ));

    const nav = await screen.findByRole("navigation", {
      name: /Mobile navigation/i,
    });
    await user.click(within(nav).getByRole("button", { name: /More/i }));

    const dialog = await screen.findByRole("dialog", { name: "More" });
    expect(within(dialog).queryByText("Preview")).not.toBeInTheDocument();
    expect(
      within(dialog).queryByRole("link", { name: "Settings" }),
    ).not.toBeInTheDocument();
    expect(within(dialog).getAllByRole("radio")).toHaveLength(3);
    expect(within(dialog).getByRole("radio", { name: "Light" })).toBeChecked();
    await user.click(within(dialog).getByRole("radio", { name: "Dark" }));
    // The second argument is where the reveal starts; jsdom has no layout, so
    // there is no point to report.
    expect(onThemeChoice).toHaveBeenCalledWith("dark", undefined);
    await user.click(within(dialog).getByRole("radio", { name: "System" }));
    expect(onThemeChoice).toHaveBeenCalledWith("system", undefined);
    await user.click(
      within(dialog).getByRole("button", { name: /App language/i }),
    );
    expect(onLocale).toHaveBeenCalledWith("vi");
    expect(
      within(dialog).getByRole("link", { name: /Profile and avatar/i }),
    ).toHaveAttribute("href", "/settings?section=profile");
    expect(
      within(dialog).getByRole("link", { name: /Data and privacy/i }),
    ).toHaveAttribute("href", "/settings?section=privacy");
    expect(
      within(dialog)
        .getAllByRole("link")
        .filter((link) => link.getAttribute("href")?.startsWith("/settings")),
      // Profile now holds the learning preferences: no separate row.
    ).toHaveLength(4);

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
  it("renders all main navigation destinations in desktop sidebar when signed in as admin", async () => {
    await renderAt("/", () => (
      <AppShell status="authenticated" user={{ role: "admin" }}>
        <div>content</div>
      </AppShell>
    ));

    const mainNav = await screen.findByRole("navigation", {
      name: /Main navigation/i,
    });
    expect(
      within(mainNav).getByRole("link", { name: /Dashboard/i }),
    ).toHaveAttribute("href", "/");
    expect(
      within(mainNav).getByRole("link", { name: /Learn/i }),
    ).toHaveAttribute("href", "/learn");
    expect(
      within(mainNav).getByRole("link", { name: /Practice/i }),
    ).toHaveAttribute("href", "/practice");
    expect(
      within(mainNav).getByRole("link", { name: /Exams/i }),
    ).toHaveAttribute("href", "/exams");
    expect(
      within(mainNav).getByRole("link", { name: /Progress/i }),
    ).toHaveAttribute("href", "/progress");
    expect(
      within(mainNav).getByRole("link", { name: /Studio/i }),
    ).toHaveAttribute("href", "/studio");
    expect(
      within(mainNav).getByRole("link", { name: /Settings/i }),
    ).toHaveAttribute("href", "/settings");
    expect(
      within(mainNav).getByRole("link", { name: /Admin/i }),
    ).toHaveAttribute("href", "/admin");
  });

  it("renders guest navigation and auth links when unauthenticated", async () => {
    await renderAt("/", () => (
      <AppShell status="unauthenticated">
        <div>content</div>
      </AppShell>
    ));

    const mainNav = await screen.findByRole("navigation", {
      name: /Main navigation/i,
    });
    expect(
      within(mainNav).getByRole("link", { name: /Dashboard/i }),
    ).toHaveAttribute("href", "/");
    expect(
      within(mainNav).getByRole("link", { name: /Learn/i }),
    ).toHaveAttribute("href", "/learn");
    expect(
      within(mainNav).getByRole("link", { name: /Practice/i }),
    ).toHaveAttribute("href", "/practice");
    expect(
      within(mainNav).getByRole("link", { name: /Exams/i }),
    ).toHaveAttribute("href", "/exams");
    expect(
      within(mainNav).getByRole("link", { name: /Progress/i }),
    ).toHaveAttribute("href", "/progress");
    expect(
      within(mainNav).getByRole("link", { name: /Studio/i }),
    ).toHaveAttribute("href", "/studio");
    expect(
      within(mainNav).queryByRole("link", { name: /Settings/i }),
    ).not.toBeInTheDocument();
    expect(
      within(mainNav).queryByRole("link", { name: /Admin/i }),
    ).not.toBeInTheDocument();

    expect(
      screen.getByRole("link", { name: /Create account/i }),
    ).toHaveAttribute("href", "/register");
    expect(screen.getByRole("link", { name: /Sign in/i })).toHaveAttribute(
      "href",
      "/login",
    );
  });
});
