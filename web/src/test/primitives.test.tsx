import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";

describe("Card Component (P6.3)", () => {
  it("renders card with all slots correctly", () => {
    render(
      <Card data-testid="test-card">
        <CardHeader data-testid="test-header">
          <CardTitle data-testid="test-title">Unit 1: Introductions</CardTitle>
          <CardDescription data-testid="test-desc">
            Learn basic greetings and introductions
          </CardDescription>
        </CardHeader>
        <CardContent data-testid="test-content">
          <p>Lesson content goes here</p>
        </CardContent>
        <CardFooter data-testid="test-footer">
          <button type="button">Start Lesson</button>
        </CardFooter>
      </Card>,
    );

    const card = screen.getByTestId("test-card");
    expect(card).toBeInTheDocument();
    expect(card.className).toContain("bg-surface-card");
    expect(card.className).toContain("border-border-subtle");

    expect(screen.getByTestId("test-header")).toBeInTheDocument();
    expect(screen.getByTestId("test-title")).toHaveTextContent(
      "Unit 1: Introductions",
    );
    expect(screen.getByTestId("test-desc")).toHaveTextContent(
      "Learn basic greetings and introductions",
    );
    expect(screen.getByTestId("test-content")).toHaveTextContent(
      "Lesson content goes here",
    );
    expect(screen.getByTestId("test-footer")).toBeInTheDocument();
  });
});

describe("Badge Component (P6.3)", () => {
  const variants = [
    "primary",
    "secondary",
    "outline",
    "success",
    "warning",
    "danger",
  ] as const;

  for (const variant of variants) {
    it(`renders ${variant} variant with correct text and class`, () => {
      render(
        <Badge variant={variant} data-testid={`badge-${variant}`}>
          {variant.toUpperCase()}
        </Badge>,
      );

      const badge = screen.getByTestId(`badge-${variant}`);
      expect(badge).toBeInTheDocument();
      expect(badge).toHaveTextContent(variant.toUpperCase());
    });
  }

  it("renders with an icon and text label (WCAG 1.4.1 non-color indicator)", () => {
    render(
      <Badge
        variant="success"
        icon={<span data-testid="badge-icon">✓</span>}
        data-testid="badge-with-icon"
      >
        Completed
      </Badge>,
    );

    const badge = screen.getByTestId("badge-with-icon");
    expect(badge).toHaveTextContent("Completed");
    expect(screen.getByTestId("badge-icon")).toBeInTheDocument();
  });
});

describe("Progress Component (P6.3)", () => {
  it("renders role='progressbar' with accessible ARIA attributes", () => {
    render(
      <Progress
        value={60}
        min={0}
        max={100}
        aria-label="Course completion"
        data-testid="progress-bar"
      />,
    );

    const progress = screen.getByRole("progressbar");
    expect(progress).toBeInTheDocument();
    expect(progress).toHaveAttribute("aria-valuenow", "60");
    expect(progress).toHaveAttribute("aria-valuemin", "0");
    expect(progress).toHaveAttribute("aria-valuemax", "100");
    expect(progress).toHaveAttribute("aria-valuetext", "60%");
    expect(progress).toHaveAttribute("aria-label", "Course completion");
  });

  it("clamps values exceeding max or below min", () => {
    const { rerender } = render(
      <Progress
        value={150}
        min={0}
        max={100}
        aria-label="Test progress"
        data-testid="progress-clamp"
      />,
    );

    let progress = screen.getByRole("progressbar");
    expect(progress).toHaveAttribute("aria-valuenow", "100");
    expect(progress).toHaveAttribute("aria-valuetext", "100%");

    rerender(
      <Progress value={-20} min={0} max={100} aria-label="Test progress" data-testid="progress-clamp" />,
    );
    progress = screen.getByRole("progressbar");
    expect(progress).toHaveAttribute("aria-valuenow", "0");
    expect(progress).toHaveAttribute("aria-valuetext", "0%");
  });

  it("supports custom getValueLabel", () => {
    render(
      <Progress
        value={3}
        min={0}
        max={10}
        aria-label="Exercise progress"
        getValueLabel={(val, max) => `${val} of ${max} exercises`}
      />,
    );

    const progress = screen.getByRole("progressbar");
    expect(progress).toHaveAttribute("aria-valuetext", "3 of 10 exercises");
  });

  it("supports variant styling and motion-reduce animation class", () => {
    const { container } = render(<Progress value={45} variant="success" aria-label="Variant test" />);

    const indicator = container.querySelector(".bg-success");
    expect(indicator).not.toBeNull();
    expect(indicator?.className).toContain("motion-reduce:transition-none");
  });
});

describe("Skeleton Component (P6.3)", () => {
  it("renders with animate-pulse and respects prefers-reduced-motion via motion-reduce:animate-none", () => {
    render(<Skeleton data-testid="skeleton-item" className="h-6 w-32" />);

    const skeleton = screen.getByTestId("skeleton-item");
    expect(skeleton).toBeInTheDocument();
    expect(skeleton).toHaveAttribute("aria-hidden", "true");
    expect(skeleton.className).toContain("animate-pulse");
    expect(skeleton.className).toContain("bg-surface-muted");
    expect(skeleton.className).toContain("motion-reduce:animate-none");
  });
});
