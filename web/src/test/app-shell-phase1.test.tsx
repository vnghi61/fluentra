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
import { beforeEach, describe, expect, it } from "vitest";

import { AppShell } from "@/components/layout/AppShell";
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
  return render(<RouterProvider router={router} />);
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
    expect(screen.getByRole("link", { name: /Back to home/i })).toHaveAttribute(
      "href",
      "/",
    );
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

  it("renders the passed controls in the More sheet when signed in", async () => {
    const user = userEvent.setup();
    await renderAt("/", () => (
      <AppShell
        status="authenticated"
        user={{ role: "user" }}
        controls={<button type="button">Theme toggle</button>}
      >
        <div>content</div>
      </AppShell>
    ));

    const nav = await screen.findByRole("navigation", {
      name: /Mobile navigation/i,
    });
    expect(within(nav).getAllByRole("link")).toHaveLength(4);
    await user.click(within(nav).getByRole("button", { name: /More/i }));

    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByText(/Appearance & language/i),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByRole("button", { name: "Theme toggle" }),
    ).toBeInTheDocument();
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
