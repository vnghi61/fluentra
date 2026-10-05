import { render, screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it, vi } from "vitest";
import i18n, { initI18n } from "@/i18n";
import {
  AudioRecorder,
  formatRecorderTime,
} from "@/features/speaking/components/AudioRecorder";
import { server } from "./msw-server";

describe("speaking recorder", () => {
  beforeEach(async () => {
    await initI18n("en");
    server.use(
      http.get("*/api/v1/speaking/consent", () =>
        HttpResponse.json({
          consented: true,
          consented_at: "2026-09-19T08:00:00Z",
        }),
      ),
    );
  });

  it.each([
    [300, "05:00"],
    [120, "02:00"],
    [83, "01:23"],
    [0, "00:00"],
    [-4, "00:00"],
  ])("formats %i seconds as %s", (seconds, expected) =>
    expect(formatRecorderTime(seconds)).toBe(expected),
  );

  it("shows an idle mm:ss duration and a minute-based limit", async () => {
    render(
      <I18nextProvider i18n={i18n}>
        <AudioRecorder
          taskType="respond"
          speakingTimeSeconds={300}
          mode="practice"
          onRecordingComplete={vi.fn()}
        />
      </I18nextProvider>,
    );
    expect(await screen.findByText("05:00")).toBeInTheDocument();
    expect(screen.getByText("5 min")).toBeInTheDocument();
    expect(screen.queryByText("00:300")).not.toBeInTheDocument();
  });
});
