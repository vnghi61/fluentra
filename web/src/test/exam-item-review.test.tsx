import { render, screen } from "@testing-library/react";
import { afterEach, beforeAll, describe, expect, it } from "vitest";

import { Explanation } from "@/features/exam/components/ExamItemReview";
import i18n, { initI18n, loadLocale } from "@/i18n";

describe("Explanation", () => {
  beforeAll(async () => {
    await initI18n("en");
  });

  afterEach(async () => {
    await i18n.changeLanguage("en");
  });

  const sampleExplanation = {
    text: "This question tests present perfect continuous tense.",
    text_vi: "Cau hoi nay kiem tra thi hien tai hoan thanh tiep dien.",
  };

  it("renders English text and omits Vietnamese text when interface language is English", () => {
    render(<Explanation explanation={sampleExplanation} />);

    expect(screen.getByText(sampleExplanation.text)).toBeInTheDocument();
    expect(
      screen.queryByText(sampleExplanation.text_vi),
    ).not.toBeInTheDocument();
  });

  it("renders Vietnamese text and omits English text when interface language is Vietnamese", async () => {
    await loadLocale("vi");
    await i18n.changeLanguage("vi");

    render(<Explanation explanation={sampleExplanation} />);

    expect(screen.getByText(sampleExplanation.text_vi)).toBeInTheDocument();
    expect(screen.queryByText(sampleExplanation.text)).not.toBeInTheDocument();
  });

  it("falls back to the available language when chosen language is empty", async () => {
    await loadLocale("vi");
    await i18n.changeLanguage("vi");

    render(
      <Explanation
        explanation={{
          text: "Fallback English explanation.",
          text_vi: "",
        }}
      />,
    );

    expect(
      screen.getByText("Fallback English explanation."),
    ).toBeInTheDocument();
  });

  it("returns null when explanation text is empty", () => {
    const { container } = render(
      <Explanation explanation={{ text: "", text_vi: "" }} />,
    );

    expect(container).toBeEmptyDOMElement();
  });
});
