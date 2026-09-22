import React from "react";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { I18nextProvider } from "react-i18next";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";

import i18n, { initI18n } from "@/i18n";
import { FoundationPathView } from "@/features/learning/components/Foundation/FoundationPathView";
import type { FoundationPathNode } from "@/features/learning/api/foundation";

async function renderWithRouter(ui: React.ReactElement) {
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => (
      <I18nextProvider i18n={i18n}>{ui}</I18nextProvider>
    ),
  });

  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

describe("FoundationPathView", () => {
  beforeEach(async () => {
    await initI18n("en");
    await i18n.changeLanguage("en");
  });

  const sampleNodes: FoundationPathNode[] = [
    {
      id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
      namespace: "grammar",
      code: "SENTENCE_STRUCTURE",
      label: "Sentence Structure",
      cefr_level: "A1",
      attempts: 6,
      score: 0.85,
      mastered: true,
      next: false,
    },
    {
      id: "0199a1c2-3d4e-7f80-9abc-def01234567b",
      namespace: "grammar",
      code: "PRESENT_SIMPLE",
      label: "Present Simple",
      cefr_level: "A1",
      attempts: 5,
      score: 0.9,
      mastered: true,
      next: false,
    },
    {
      id: "0199a1c2-3d4e-7f80-9abc-def01234567c",
      namespace: "grammar",
      code: "PRESENT_CONTINUOUS",
      label: "Present Continuous",
      cefr_level: "A1",
      attempts: 1,
      score: 0.5,
      mastered: false,
      next: true,
    },
    {
      id: "0199a1c2-3d4e-7f80-9abc-def01234567d",
      namespace: "grammar",
      code: "PRESENT_PERFECT",
      label: "Present Perfect",
      cefr_level: "B1",
      attempts: 0,
      score: 0,
      mastered: false,
      next: false,
    },
  ];

  it("renders prerequisite chain with mastery badges and next marker", async () => {
    await renderWithRouter(
      <FoundationPathView
        items={sampleNodes}
        targetCode="PRESENT_PERFECT"
        currentCode="PRESENT_PERFECT"
      />,
    );

    expect(
      await screen.findByText("Prerequisite Learning Path"),
    ).toBeInTheDocument();
    expect(screen.getByText("Sentence Structure")).toBeInTheDocument();
    expect(screen.getByText("Present Simple")).toBeInTheDocument();
    expect(screen.getByText("Present Continuous")).toBeInTheDocument();
    expect(screen.getByText("Present Perfect")).toBeInTheDocument();

    // 2 mastered nodes
    const masteredBadges = screen.getAllByText("Mastered");
    expect(masteredBadges).toHaveLength(2);

    // 1 next node
    expect(screen.getByText("Next to Learn")).toBeInTheDocument();

    // 1 target badge
    expect(screen.getByText("Target")).toBeInTheDocument();
  });

  it("renders empty state when no items provided", async () => {
    await renderWithRouter(
      <FoundationPathView items={[]} targetCode="PRESENT_PERFECT" />,
    );

    expect(
      await screen.findByText(
        "No prerequisite path recorded for this topic.",
      ),
    ).toBeInTheDocument();
  });
});


