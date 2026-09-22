import { fireEvent, render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ExerciseMaterial } from "@/features/learning/components/Runner/ExerciseMaterial";
import i18n, { initI18n } from "@/i18n";

const video = [
  { url: "https://cdn.example/fluentra-media/v720.mp4?sig=1", height: 720 },
  { url: "https://cdn.example/fluentra-media/v360.mp4?sig=1", height: 360 },
];

function renderMaterial(
  props: Partial<React.ComponentProps<typeof ExerciseMaterial>>,
) {
  return render(
    <I18nextProvider i18n={i18n}>
      <ExerciseMaterial
        materialKind="video"
        title="Intro"
        isSubmitted={false}
        onSubmit={vi.fn()}
        onContinue={vi.fn()}
        {...props}
      />
    </I18nextProvider>,
  );
}

describe("ExerciseMaterial", () => {
  beforeEach(async () => {
    await initI18n("en");
  });

  it("falls back to 360p, then refetches the lesson once when the URLs have expired", () => {
    const onRefetchLesson = vi.fn();
    const { container } = renderMaterial({
      sources: { video },
      onRefetchLesson,
    });
    const player = container.querySelector("video") as HTMLVideoElement;

    expect(player.getAttribute("src")).toContain("v720.mp4");

    // An error on a <source> child never reaches <video>; one src makes it do so.
    fireEvent.error(player);
    expect(player.getAttribute("src")).toContain("v360.mp4");
    expect(onRefetchLesson).not.toHaveBeenCalled();

    fireEvent.error(player);
    expect(onRefetchLesson).toHaveBeenCalledTimes(1);
    fireEvent.error(player);
    expect(onRefetchLesson).toHaveBeenCalledTimes(1);
  });

  it("offers a video no link to its raw upload", () => {
    renderMaterial({ sources: { video } });
    expect(
      screen.queryByRole("link", { name: "Open document" }),
    ).not.toBeInTheDocument();
  });

  it("opens a document in a new tab and completes on mark as done", () => {
    const onSubmit = vi.fn();
    renderMaterial({
      materialKind: "document",
      sources: {
        document: { url: "https://cdn.example/fluentra-media/doc.pdf?sig=1" },
      },
      onSubmit,
    });

    const link = screen.getByRole("link", { name: "Open document" });
    expect(link).toHaveAttribute("target", "_blank");
    fireEvent.click(screen.getByRole("button", { name: /Mark as done/ }));
    expect(onSubmit).toHaveBeenCalledWith(true);
  });
});
