import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { FlipCard } from "@/components/ui/flip-card";

describe("FlipCard", () => {
  it("lets taps reach only the face turned toward the learner", () => {
    // Phones hit-test a backface-hidden face; the back sits on top in DOM order,
    // so a tap on the front's speaker flipped the card instead of speaking.
    const { rerender } = render(
      <FlipCard flipped={false} front={<span>front</span>} back={<span>back</span>} onClick={() => {}} />,
    );
    expect(screen.getByText("back").parentElement).toHaveClass("pointer-events-none");
    expect(screen.getByText("front").parentElement).not.toHaveClass("pointer-events-none");

    rerender(
      <FlipCard flipped front={<span>front</span>} back={<span>back</span>} onClick={() => {}} />,
    );
    expect(screen.getByText("front").parentElement).toHaveClass("pointer-events-none");
    expect(screen.getByText("back").parentElement).not.toHaveClass("pointer-events-none");
  });

  it("still flips when the visible face is tapped", () => {
    let flips = 0;
    render(
      <FlipCard flipped={false} front={<span>front</span>} back={<span>back</span>} onClick={() => flips++} />,
    );
    fireEvent.click(screen.getByText("front"));
    expect(flips).toBe(1);
  });
});
