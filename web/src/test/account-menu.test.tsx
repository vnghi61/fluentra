import { render, screen } from "@testing-library/react";
import { describe, expect, it, beforeEach } from "vitest";

import AccountMenu from "@/components/layout/AccountMenu";
import i18n, { initI18n } from "@/i18n";

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
});
