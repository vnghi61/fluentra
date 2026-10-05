import { render, screen } from "@testing-library/react";
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

import AccountMenu from "@/components/layout/AccountMenu";
import i18n, { initI18n, loadLocale } from "@/i18n";

async function renderWithRouter(ui: ReactElement) {
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => ui,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

/**
 * The account menu's trigger is the only place a learner sees their own avatar
 * outside the settings screen they uploaded it on.
 *
 * It drew a generic person icon for everybody, always — the comment above it
 * said "the trigger is an avatar" and it was not one, and the URL was sitting in
 * a profile response the shell had already fetched.
 */
describe("AccountMenu trigger", () => {
  beforeEach(async () => {
    await initI18n("en");
    await loadLocale("en");
    await i18n.changeLanguage("en");
  });

  it("shows the uploaded avatar when there is one", () => {
    render(
      <AccountMenu
        role="user"
        displayName="Nghi"
        avatarUrl="https://cdn.example/avatars/nghi.png"
      />,
    );

    const image = screen.getByRole("button").querySelector("img");
    expect(image).not.toBeNull();
    expect(image?.getAttribute("src")).toBe(
      "https://cdn.example/avatars/nghi.png",
    );
  });

  it("falls back to the learner's own initial, not a generic icon", () => {
    // The middle rung matters: an initial is theirs, it is legible at 44 px, and
    // it tells two accounts on the same browser apart. The person icon did none
    // of that.
    render(<AccountMenu role="user" displayName="Nghi" />);

    const trigger = screen.getByRole("button");
    expect(trigger.querySelector("img")).toBeNull();
    expect(trigger.textContent).toBe("N");
  });

  it("still renders a trigger when the profile has not loaded", () => {
    // No name and no avatar is the ordinary state for the first paint after a
    // reload, and the menu has to be reachable during it.
    render(<AccountMenu role="user" />);

    expect(
      screen.getByRole("button", { name: /Account/i }),
    ).toBeInTheDocument();
  });

  it("opens menu and renders destinations and greeting for a user", async () => {
    const user = userEvent.setup();
    await renderWithRouter(<AccountMenu role="user" displayName="Nghi" />);

    const trigger = screen.getByRole("button", { name: /Account/i });
    await user.click(trigger);

    expect(await screen.findByText(/Hello/i)).toBeInTheDocument();
    expect(screen.getByText("Nghi")).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /Settings/i })).toHaveAttribute(
      "href",
      "/settings",
    );
    expect(
      screen.getByRole("menuitem", { name: /My Writing/i }),
    ).toHaveAttribute("href", "/my-writing");
    expect(
      screen.getByRole("menuitem", { name: /My Speaking/i }),
    ).toHaveAttribute("href", "/my-speaking");
    expect(
      screen.queryByRole("menuitem", { name: /Admin/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: /Sign out/i }),
    ).toBeInTheDocument();
  });

  it("includes admin destination and badge for admin role and fires onLogout", async () => {
    const user = userEvent.setup();
    const handleLogout = vi.fn();
    await renderWithRouter(
      <AccountMenu
        role="admin"
        displayName="Admin User"
        onLogout={handleLogout}
      />,
    );

    const trigger = screen.getByRole("button", { name: /Account/i });
    await user.click(trigger);

    expect(await screen.findByText("admin")).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /Admin/i })).toHaveAttribute(
      "href",
      "/admin",
    );

    const signOut = screen.getByRole("menuitem", { name: /Sign out/i });
    await user.click(signOut);
    expect(handleLogout).toHaveBeenCalledTimes(1);
  });
});
