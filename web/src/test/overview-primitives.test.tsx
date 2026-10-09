import { render, screen } from "@testing-library/react";
import { Flame, Layers } from "lucide-react";
import { describe, expect, it } from "vitest";

import { ProgressRing } from "@/components/ui/progress-ring";
import { StackedCardsArt } from "@/components/ui/stacked-cards-art";
import { StatTile } from "@/components/ui/stat-tile";

describe("ProgressRing", () => {
  it("is a named progressbar with a clamped value", () => {
    render(<ProgressRing value={140} label="Today's goal" />);
    const ring = screen.getByRole("progressbar", { name: "Today's goal" });
    expect(ring).toHaveAttribute("aria-valuenow", "100");
    expect(ring).toHaveAttribute("aria-valuemin", "0");
    expect(ring).toHaveAttribute("aria-valuemax", "100");
  });

  it("stays out of the accessibility tree when decorative", () => {
    render(
      <ProgressRing value={0} decorative>
        <span>10</span>
      </ProgressRing>,
    );
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
  });
});

describe("StatTile", () => {
  it("renders the label and figure in the requested order", () => {
    render(
      <StatTile
        icon={Flame}
        label="Activities completed"
        value={21}
        valueTestId="kpi"
        labelFirst
      />,
    );
    const value = screen.getByTestId("kpi");
    const label = screen.getByText("Activities completed");
    expect(value).toHaveTextContent("21");
    expect(
      label.compareDocumentPosition(value) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });
});

describe("StackedCardsArt", () => {
  it("is decorative", () => {
    const { container } = render(<StackedCardsArt icon={Layers} />);
    expect(container.firstElementChild).toHaveAttribute("aria-hidden", "true");
  });
});
