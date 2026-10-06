import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { Users } from "lucide-react";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AppShell } from "@/components/layout/AppShell";
import { AdminSidebarNav } from "@/features/admin/components/AdminSidebarNav";
import i18n, { initI18n, loadLocale } from "@/i18n";

vi.mock("@/features/admin/model/sections", () => ({
  useVisibleAdminSections: () => ({
    sections: [
      {
        key: "users",
        path: "/admin/users",
        labelKey: "admin.users",
        labelFallback: "Users",
        Icon: Users,
        anyOf: ["user.list"],
      },
    ],
    isLoading: false,
    error: null,
  }),
}));

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

describe("AppShell collapsible desktop sidebar", () => {
  beforeEach(async () => {
    localStorage.clear();
    await initI18n("en");
    await loadLocale("en");
    await i18n.changeLanguage("en");
  });

  it("renders default expanded with aria-expanded=true and default styling", async () => {
    await renderAt("/", () => (
      <AppShell status="authenticated" user={{ role: "admin" }}>
        <div>content</div>
      </AppShell>
    ));

    const toggle = screen.getByRole("button", { name: /Collapse sidebar/i });
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(toggle).toHaveAttribute("aria-label", "Collapse sidebar");
    expect(toggle).toHaveClass("h-11", "w-11", "rounded-xl");

    const aside = toggle.closest("aside");
    expect(aside).not.toBeNull();
    expect(aside).toHaveAttribute("data-collapsed", "false");
    expect(aside).toHaveClass(
      "w-[232px]",
      "duration-200",
      "motion-reduce:transition-none",
      "md:sticky",
      "md:top-0",
      "md:h-screen",
      "md:self-start",
    );
    expect(aside).not.toHaveClass("overflow-y-auto");
    const scrollers = (aside as HTMLElement).querySelectorAll(
      ".overflow-y-auto",
    );
    expect(scrollers).toHaveLength(1);
    expect(scrollers[0]?.tagName).toBe("NAV");
    expect(scrollers[0]).toHaveClass("min-h-0", "flex-1");

    const brandText = within(aside as HTMLElement).getByText("Fluentra");
    expect(brandText).not.toHaveClass("sr-only");
    expect(screen.getByText(/Keep the momentum!/i)).toBeInTheDocument();
  });

  it("collapses when toggle button is clicked and expands when clicked again", async () => {
    const user = userEvent.setup();
    await renderAt("/", () => (
      <AppShell status="authenticated" user={{ role: "user" }}>
        <div>content</div>
      </AppShell>
    ));

    const toggle = screen.getByRole("button", { name: /Collapse sidebar/i });
    const aside = toggle.closest("aside");
    expect(aside).not.toBeNull();

    // Click toggle to collapse
    await user.click(toggle);

    expect(
      screen.getByRole("button", { name: /Expand sidebar/i }),
    ).toBeInTheDocument();
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(toggle).toHaveAttribute("aria-label", "Expand sidebar");
    expect(aside).toHaveAttribute("data-collapsed", "true");
    expect(aside).toHaveClass("w-[72px]");
    expect(localStorage.getItem("fluentra.sidebarCollapsed")).toBe("true");

    expect(within(aside as HTMLElement).getByText("Fluentra")).toHaveClass(
      "sr-only",
    );
    expect(screen.queryByText(/Keep the momentum!/i)).not.toBeInTheDocument();

    // Click toggle again to expand
    await user.click(toggle);

    expect(
      screen.getByRole("button", { name: /Collapse sidebar/i }),
    ).toBeInTheDocument();
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(toggle).toHaveAttribute("aria-label", "Collapse sidebar");
    expect(aside).toHaveAttribute("data-collapsed", "false");
    expect(aside).toHaveClass("w-[232px]");
    expect(localStorage.getItem("fluentra.sidebarCollapsed")).toBe("false");

    expect(within(aside as HTMLElement).getByText("Fluentra")).not.toHaveClass(
      "sr-only",
    );
    expect(screen.getByText(/Keep the momentum!/i)).toBeInTheDocument();
  });

  it("keeps nav links reachable by accessible name without title attribute when collapsed", async () => {
    const user = userEvent.setup();
    await renderAt("/", () => (
      <AppShell status="authenticated" user={{ role: "admin" }}>
        <div>content</div>
      </AppShell>
    ));

    const toggle = screen.getByRole("button", { name: /Collapse sidebar/i });
    const aside = toggle.closest("aside");
    expect(aside).not.toBeNull();
    await user.click(toggle);

    const mainNav = screen.getByRole("navigation", {
      name: /Main navigation/i,
    });

    const dashboardLink = within(mainNav).getByRole("link", {
      name: /Dashboard/i,
    });
    expect(dashboardLink).toBeInTheDocument();
    expect(dashboardLink).not.toHaveAttribute("title");
    expect(dashboardLink).toHaveClass("justify-center", "px-0");

    const learnLink = within(mainNav).getByRole("link", { name: /Learn/i });
    expect(learnLink).toBeInTheDocument();
    expect(learnLink).not.toHaveAttribute("title");
    expect(learnLink).toHaveClass("justify-center", "px-0");

    const settingsLink = within(mainNav).getByRole("link", {
      name: /Settings/i,
    });
    expect(settingsLink).toBeInTheDocument();
    expect(settingsLink).not.toHaveAttribute("title");
    expect(settingsLink).toHaveClass("justify-center", "px-0");

    const adminLink = within(mainNav).getByRole("link", { name: /Admin/i });
    expect(adminLink).toBeInTheDocument();
    expect(adminLink).not.toHaveAttribute("title");
    expect(adminLink).toHaveClass("justify-center", "px-0");

    const brandLink = within(aside as HTMLElement).getByRole("link", {
      name: /Fluentra/i,
    });
    expect(brandLink).toBeInTheDocument();
    expect(brandLink).not.toHaveAttribute("title");
  });

  it("starts collapsed when 'fluentra.sidebarCollapsed' is pre-stored as 'true'", async () => {
    localStorage.setItem("fluentra.sidebarCollapsed", "true");

    await renderAt("/", () => (
      <AppShell status="authenticated" user={{ role: "user" }}>
        <div>content</div>
      </AppShell>
    ));

    const toggle = screen.getByRole("button", { name: /Expand sidebar/i });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(toggle).toHaveAttribute("aria-label", "Expand sidebar");

    const aside = toggle.closest("aside");
    expect(aside).not.toBeNull();
    expect(aside).toHaveAttribute("data-collapsed", "true");
    expect(aside).toHaveClass("w-[72px]");
    expect(screen.queryByText(/Keep the momentum!/i)).not.toBeInTheDocument();
  });

  it("keeps AdminSidebarNav link labels in DOM with group-data-[collapsed=true]:sr-only when collapsed", async () => {
    const user = userEvent.setup();
    await renderAt("/admin", () => (
      <AppShell
        status="authenticated"
        user={{ role: "admin" }}
        adminNav={<AdminSidebarNav />}
      >
        <div>content</div>
      </AppShell>
    ));

    const toggle = screen.getByRole("button", { name: /Collapse sidebar/i });
    await user.click(toggle);

    const aside = toggle.closest("aside");
    expect(aside).not.toBeNull();
    expect(aside).toHaveAttribute("data-collapsed", "true");

    const adminLink = within(aside as HTMLElement).getByRole("link", {
      name: /^Admin$/i,
    });
    expect(adminLink).toBeInTheDocument();
    expect(adminLink).not.toHaveAttribute("title");

    const usersLink = within(aside as HTMLElement).getByRole("link", {
      name: /Users/i,
    });
    expect(usersLink).toBeInTheDocument();
    expect(usersLink).not.toHaveAttribute("title");

    // The visible labels are demoted to sr-only when collapsed, not removed.
    for (const link of [adminLink, usersLink]) {
      const label = Array.from(link.querySelectorAll("span")).find((s) =>
        s.className.includes("group-data-[collapsed=true]:sr-only"),
      );
      expect(label).toBeDefined();
      expect(label).toHaveTextContent(link === adminLink ? "Admin" : /Users/);
    }

    const submenu = usersLink.closest("ul");
    expect(submenu).toHaveClass(
      "group-data-[collapsed=true]:border-l-0",
      "group-data-[collapsed=true]:pl-0",
      "group-data-[collapsed=true]:ml-0",
    );
  });

  it("toggles the admin submenu with an aria-expanded disclosure button", async () => {
    const user = userEvent.setup();
    await renderAt("/", () => (
      <AppShell
        status="authenticated"
        user={{ role: "admin" }}
        adminNav={<AdminSidebarNav />}
      >
        <div>content</div>
      </AppShell>
    ));

    const disclosure = screen.getByRole("button", {
      name: /Toggle admin sections/i,
    });
    // Not under /admin, so closed by default.
    expect(disclosure).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("link", { name: /Users/i })).toBeNull();
    const submenuId = disclosure.getAttribute("aria-controls");
    expect(submenuId).toBeTruthy();

    await user.click(disclosure);
    expect(disclosure).toHaveAttribute("aria-expanded", "true");
    expect(document.getElementById(submenuId as string)).toBeVisible();
    expect(screen.getByRole("link", { name: /Users/i })).toBeInTheDocument();

    await user.click(disclosure);
    expect(disclosure).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("link", { name: /Users/i })).toBeNull();
  });

  it("opens the admin submenu by default under /admin", async () => {
    await renderAt("/admin", () => (
      <AppShell
        status="authenticated"
        user={{ role: "admin" }}
        adminNav={<AdminSidebarNav />}
      >
        <div>content</div>
      </AppShell>
    ));
    expect(
      screen.getByRole("button", { name: /Toggle admin sections/i }),
    ).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("link", { name: /Users/i })).toBeInTheDocument();
  });

  it("shows tooltip on hover and removes it on unhover when collapsed", async () => {
    const user = userEvent.setup();
    await renderAt("/", () => (
      <AppShell status="authenticated" user={{ role: "admin" }}>
        <div>content</div>
      </AppShell>
    ));

    const toggle = screen.getByRole("button", { name: /Collapse sidebar/i });
    await user.click(toggle);

    const mainNav = screen.getByRole("navigation", {
      name: /Main navigation/i,
    });
    const dashboardLink = within(mainNav).getByRole("link", {
      name: /Dashboard/i,
    });

    expect(screen.queryByRole("tooltip", { hidden: true })).toBeNull();

    await user.hover(dashboardLink);
    const tooltip = screen.getByRole("tooltip", { hidden: true });
    expect(tooltip).toBeInTheDocument();
    expect(tooltip).toHaveTextContent("Dashboard");

    await user.unhover(dashboardLink);
    expect(screen.queryByRole("tooltip", { hidden: true })).toBeNull();

    // Verify brand link tooltip on hover
    const aside = toggle.closest("aside");
    expect(aside).not.toBeNull();
    const brandLink = within(aside as HTMLElement).getByRole("link", {
      name: /Fluentra/i,
    });
    await user.hover(brandLink);
    expect(screen.getByRole("tooltip", { hidden: true })).toHaveTextContent(
      "Fluentra",
    );

    // Verify dismiss on Escape
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("tooltip", { hidden: true })).toBeNull();
  });

  it("shows tooltip on focus for nav links and the expand toggle when collapsed", async () => {
    const user = userEvent.setup();
    localStorage.setItem("fluentra.sidebarCollapsed", "true");

    await renderAt("/", () => (
      <AppShell status="authenticated" user={{ role: "admin" }}>
        <div>content</div>
      </AppShell>
    ));

    const toggle = screen.getByRole("button", { name: /Expand sidebar/i });
    act(() => {
      toggle.focus();
    });
    const toggleTooltip = screen.getByRole("tooltip", { hidden: true });
    expect(toggleTooltip).toBeInTheDocument();
    expect(toggleTooltip).toHaveTextContent("Expand sidebar");

    act(() => {
      toggle.blur();
    });
    expect(screen.queryByRole("tooltip", { hidden: true })).toBeNull();

    const mainNav = screen.getByRole("navigation", {
      name: /Main navigation/i,
    });
    const settingsLink = within(mainNav).getByRole("link", {
      name: /Settings/i,
    });
    act(() => {
      settingsLink.focus();
    });
    const settingsTooltip = screen.getByRole("tooltip", { hidden: true });
    expect(settingsTooltip).toBeInTheDocument();
    expect(settingsTooltip).toHaveTextContent("Settings");

    act(() => {
      settingsLink.blur();
    });
    expect(screen.queryByRole("tooltip", { hidden: true })).toBeNull();

    // Focus link and tab to next link
    const dashboardLink = within(mainNav).getByRole("link", {
      name: /Dashboard/i,
    });
    act(() => {
      dashboardLink.focus();
    });
    expect(screen.getByRole("tooltip", { hidden: true })).toHaveTextContent(
      "Dashboard",
    );

    await user.tab();
    expect(screen.getByRole("tooltip", { hidden: true })).toHaveTextContent(
      "Learn",
    );
  });

  it("renders no tooltip on hover or focus when expanded", async () => {
    const user = userEvent.setup();
    await renderAt("/", () => (
      <AppShell status="authenticated" user={{ role: "admin" }}>
        <div>content</div>
      </AppShell>
    ));

    const toggle = screen.getByRole("button", { name: /Collapse sidebar/i });
    const mainNav = screen.getByRole("navigation", {
      name: /Main navigation/i,
    });
    const dashboardLink = within(mainNav).getByRole("link", {
      name: /Dashboard/i,
    });

    await user.hover(dashboardLink);
    expect(screen.queryByRole("tooltip", { hidden: true })).toBeNull();

    act(() => {
      dashboardLink.focus();
    });
    expect(screen.queryByRole("tooltip", { hidden: true })).toBeNull();

    await user.hover(toggle);
    expect(screen.queryByRole("tooltip", { hidden: true })).toBeNull();

    act(() => {
      toggle.focus();
    });
    expect(screen.queryByRole("tooltip", { hidden: true })).toBeNull();
  });

  it("shows tooltip when hovering AdminSidebarNav links when collapsed at /admin", async () => {
    const user = userEvent.setup();
    localStorage.setItem("fluentra.sidebarCollapsed", "true");

    await renderAt("/admin", () => (
      <AppShell
        status="authenticated"
        user={{ role: "admin" }}
        adminNav={<AdminSidebarNav />}
      >
        <div>content</div>
      </AppShell>
    ));

    const aside = screen
      .getByRole("button", { name: /Expand sidebar/i })
      .closest("aside");
    expect(aside).not.toBeNull();

    const usersLink = within(aside as HTMLElement).getByRole("link", {
      name: /Users/i,
    });
    expect(usersLink).toBeInTheDocument();
    expect(usersLink).not.toHaveAttribute("title");

    await user.hover(usersLink);
    const usersTooltip = screen.getByRole("tooltip", { hidden: true });
    expect(usersTooltip).toBeInTheDocument();
    expect(usersTooltip).toHaveTextContent("Users");

    await user.unhover(usersLink);
    expect(screen.queryByRole("tooltip", { hidden: true })).toBeNull();

    const adminLink = within(aside as HTMLElement).getByRole("link", {
      name: /^Admin$/i,
    });
    expect(adminLink).toBeInTheDocument();
    expect(adminLink).not.toHaveAttribute("title");

    await user.hover(adminLink);
    const adminTooltip = screen.getByRole("tooltip", { hidden: true });
    expect(adminTooltip).toBeInTheDocument();
    expect(adminTooltip).toHaveTextContent("Admin");

    await user.unhover(adminLink);
    expect(screen.queryByRole("tooltip", { hidden: true })).toBeNull();
  });
});
