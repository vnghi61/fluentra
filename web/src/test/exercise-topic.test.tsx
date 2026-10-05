import { fireEvent, render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ExerciseTopic } from "@/features/learning/components/Runner/ExerciseTopic";
import i18n, { initI18n } from "@/i18n";

const body = {
  objective: "Use the present perfect for past actions with present relevance.",
  explanation: {
    en: "The present perfect links a past action to the present.",
    vi: "Thì hiện tại hoàn thành nối hành động quá khứ với hiện tại.",
  },
  examples: [{ text: "She has lived here for three years.", note: "Duration" }],
};

function renderTopic(
  props: Partial<React.ComponentProps<typeof ExerciseTopic>> = {},
) {
  return render(
    <I18nextProvider i18n={i18n}>
      <ExerciseTopic
        title="Present Perfect"
        body={body}
        isSubmitted={false}
        onSubmit={vi.fn()}
        onContinue={vi.fn()}
        {...props}
      />
    </I18nextProvider>,
  );
}

describe("ExerciseTopic", () => {
  beforeEach(async () => {
    await initI18n("en");
  });

  // The WO 22 Stage F gate: a lesson opening with a topic shows the
  // explanation and moves on with Continue.
  it("shows the explanation and moves on with Continue", () => {
    const onSubmit = vi.fn();
    const { rerender } = render(
      <I18nextProvider i18n={i18n}>
        <ExerciseTopic
          title="Present Perfect"
          body={body}
          isSubmitted={false}
          onSubmit={onSubmit}
          onContinue={vi.fn()}
        />
      </I18nextProvider>,
    );

    expect(
      screen.getByText(
        "The present perfect links a past action to the present.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("She has lived here for three years."),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Continue/ }));
    expect(onSubmit).toHaveBeenCalledTimes(1);

    // One press is enough: once the submit lands the step advances by itself,
    // exactly once, even across further renders.
    const onContinue = vi.fn();
    const submitted = (
      <I18nextProvider i18n={i18n}>
        <ExerciseTopic
          title="Present Perfect"
          body={body}
          isSubmitted
          onSubmit={vi.fn()}
          onContinue={onContinue}
        />
      </I18nextProvider>
    );
    rerender(submitted);
    rerender(submitted);
    expect(onContinue).toHaveBeenCalledTimes(1);
  });

  it("does not advance by itself when the topic was already submitted on arrival", () => {
    const onContinue = vi.fn();
    renderTopic({ isSubmitted: true, onContinue });
    expect(onContinue).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: /Continue/ }));
    expect(onContinue).toHaveBeenCalledTimes(1);
  });

  it("shows the Vietnamese explanation when the interface is Vietnamese", async () => {
    await i18n.changeLanguage("vi");
    renderTopic();
    expect(
      screen.getByText(
        "Thì hiện tại hoàn thành nối hành động quá khứ với hiện tại.",
      ),
    ).toBeInTheDocument();
  });
});
