import { fireEvent, render, screen, waitFor } from "@testing-library/react";
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

  it("uploads an active take on unmount without creating an orphan playback URL", async () => {
    class MockRecorder {
      static instance: MockRecorder | undefined;
      static isTypeSupported = () => true;
      state = "inactive";
      mimeType = "audio/webm";
      ondataavailable: ((event: BlobEvent) => void) | null = null;
      onstop: ((event: Event) => void) | null = null;
      constructor() {
        MockRecorder.instance = this;
      }
      start() {
        this.state = "recording";
      }
      stop() {
        this.state = "inactive";
        this.onstop?.(new Event("stop"));
      }
    }
    const track = { stop: vi.fn() };
    Object.defineProperty(window, "isSecureContext", {
      configurable: true,
      value: true,
    });
    Object.defineProperty(navigator, "mediaDevices", {
      configurable: true,
      value: {
        getUserMedia: vi.fn().mockResolvedValue({ getTracks: () => [track] }),
      },
    });
    vi.stubGlobal("MediaRecorder", MockRecorder);
    const createUrl = vi
      .spyOn(URL, "createObjectURL")
      .mockReturnValue("blob:take");
    const upload = vi.fn().mockResolvedValue("take-key");
    const complete = vi.fn();
    const view = render(
      <I18nextProvider i18n={i18n}>
        <AudioRecorder
          taskType="respond"
          speakingTimeSeconds={300}
          mode="practice"
          onUpload={upload}
          onRecordingComplete={complete}
        />
      </I18nextProvider>,
    );
    await screen.findByText("05:00");
    fireEvent.click(screen.getByRole("button", { name: "Start Recording" }));
    await waitFor(() => expect(MockRecorder.instance?.state).toBe("recording"));
    view.unmount();
    expect(upload).toHaveBeenCalledOnce();
    expect(createUrl).not.toHaveBeenCalled();
    expect(track.stop).toHaveBeenCalledOnce();
    await waitFor(() => expect(complete).toHaveBeenCalledWith("take-key"));
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });
});
