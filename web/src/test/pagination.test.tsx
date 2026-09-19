import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { Pagination, pageWindow } from "@/components/ui/pagination";

/**
 * The pager replaced two hand-rolled footers, one of which told the reader
 * nothing about where they were — the complaint that produced it. These tests
 * hold the two things that made it worth extracting: it reports the position,
 * and it only offers page numbers to a caller that can actually reach a page.
 */

describe("pageWindow", () => {
  it("lists every page while they fit", () => {
    expect(pageWindow(1, 5)).toEqual([1, 2, 3, 4, 5]);
  });

  it("keeps the first and last page either side of a gap", () => {
    const window = pageWindow(10, 20);
    expect(window[0]).toBe(1);
    expect(window.at(-1)).toBe(20);
    expect(window).toContain(null);
    expect(window).toContain(10);
  });

  it("holds its width at both ends of the range", () => {
    // A window that shrank at page 1 moved every control beside it.
    expect(pageWindow(1, 20).length).toBe(pageWindow(20, 20).length);
  });
});

const base = {
  page: 2,
  pageCount: 5,
  total: 68,
  rangeFrom: 16,
  rangeTo: 30,
  canPrevious: true,
  canNext: true,
  onPrevious: vi.fn(),
  onNext: vi.fn(),
};

describe("Pagination", () => {
  it("says which page of how many, and which rows", () => {
    render(<Pagination {...base} />);

    // i18n is not initialised in this suite, so t() returns the key. The point
    // under test is that the position is reported at all — the learner list
    // rendered neither number before.
    expect(screen.getByText(/admin\.pageOf/)).toBeTruthy();
    expect(screen.getByText(/admin\.showingRange/)).toBeTruthy();
  });

  it("offers no page numbers when the caller cannot jump", () => {
    // /admin/users is cursor-paged: there is no offset to jump to, so a row of
    // numbers would be a row of dead buttons.
    render(<Pagination {...base} />);
    expect(screen.queryByRole("list")).toBeNull();
  });

  it("offers page numbers when the caller can jump, and reports the choice", async () => {
    const onPageSelect = vi.fn();
    render(<Pagination {...base} onPageSelect={onPageSelect} />);

    // The accessible name is an i18n key here, not "Go to page 4", so the
    // number on the face of the button is what identifies it.
    const four = screen
      .getAllByRole("button")
      .find((button) => button.textContent === "4");
    expect(four).toBeTruthy();
    await userEvent.click(four!);

    expect(onPageSelect).toHaveBeenCalledWith(4);
  });

  it("marks the current page for a screen reader", () => {
    render(<Pagination {...base} onPageSelect={vi.fn()} />);
    const current = screen.getByRole("button", { current: "page" });
    expect(current.textContent).toBe("2");
  });

  it("refuses to step past either end", () => {
    const onPrevious = vi.fn();
    render(
      <Pagination
        {...base}
        page={1}
        canPrevious={false}
        onPrevious={onPrevious}
      />,
    );

    const previous = screen.getByRole("button", { name: /common\.previous/ });
    expect(previous.hasAttribute("disabled")).toBe(true);
  });
});
