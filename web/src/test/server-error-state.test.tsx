import type { ReactNode } from "react";
import { act, render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { ApiError } from "@/api/client";
import { describeLoadError } from "@/api/loadError";
import i18n, { initI18n, loadLocale } from "@/i18n";
import {
  ServerErrorState,
  type ServerErrorStateProps,
} from "@/components/layout/ServerErrorState";

// A plain anchor in place of the router's Link, so the component renders
// without a router and the countdown runs on fake timers from its first tick.
vi.mock("@tanstack/react-router", () => ({
  Link: ({
    to,
    children,
    className,
  }: {
    to: string;
    children: ReactNode;
    className?: string;
  }) => (
    <a href={to} className={className}>
      {children}
    </a>
  ),
}));

beforeAll(async () => {
  await initI18n("en");
  await loadLocale("en");
  await i18n.changeLanguage("en");
});

afterEach(() => {
  vi.useRealTimers();
});

function renderState(props: ServerErrorStateProps) {
  render(
    <I18nextProvider i18n={i18n}>
      <ServerErrorState {...props} />
    </I18nextProvider>,
  );
}

/** One second per act: each tick is scheduled by the render the last one caused. */
async function advance(seconds: number) {
  for (let i = 0; i < seconds; i++) {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000);
    });
  }
}

describe("ServerErrorState", () => {
  it("says the server is busy and retries by itself after the countdown", async () => {
    vi.useFakeTimers();
    const onRetry = vi.fn();
    renderState({
      kind: "server",
      onRetry,
      autoRetrySeconds: 3,
      requestId: "req-123",
    });

    expect(screen.getByRole("alert")).toHaveTextContent(
      "The server is a little busy",
    );
    expect(screen.getByText("Trying again in 3 seconds")).toBeInTheDocument();
    expect(screen.getByText("req-123")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Go to Learn" })).toHaveAttribute(
      "href",
      "/learn",
    );

    await advance(2);
    expect(screen.getByText("Trying again in 1 second")).toBeInTheDocument();
    expect(onRetry).not.toHaveBeenCalled();

    await advance(1);
    expect(onRetry).toHaveBeenCalledTimes(1);
    // Still shown means the retry failed too: the countdown starts over.
    expect(screen.getByText("Trying again in 3 seconds")).toBeInTheDocument();
  });

  it("does not retry a refused request on a timer", async () => {
    vi.useFakeTimers();
    const onRetry = vi.fn();
    renderState({ kind: "request", onRetry, autoRetrySeconds: 1 });

    expect(screen.getByRole("alert")).toHaveTextContent(
      "This page didn't load",
    );
    expect(screen.queryByText(/Trying again in/)).not.toBeInTheDocument();
    await advance(5);
    expect(onRetry).not.toHaveBeenCalled();
  });

  it("waits for the network when offline and hides the reference", async () => {
    const onRetry = vi.fn();
    renderState({ kind: "offline", onRetry, requestId: "req-9" });

    expect(screen.getByText("Waiting for a connection…")).toBeInTheDocument();
    expect(screen.queryByText("req-9")).not.toBeInTheDocument();
    await act(async () => {
      window.dispatchEvent(new Event("online"));
      await Promise.resolve();
    });
    expect(onRetry).toHaveBeenCalledTimes(1);
  });
});

describe("describeLoadError", () => {
  it("treats 5xx and 429 as the server, other 4xx as the request", () => {
    const problem = (status: number) =>
      new ApiError({ title: "x", status, request_id: "r1" });
    expect(describeLoadError(problem(503))).toEqual({
      kind: "server",
      requestId: "r1",
    });
    expect(describeLoadError(problem(429)).kind).toBe("server");
    expect(describeLoadError(problem(401)).kind).toBe("request");
  });

  it("is offline only when the browser says so", () => {
    // Online but unanswered (the host is down): waiting for an `online` event
    // would never end, so it retries on a timer like a busy server.
    expect(describeLoadError(new TypeError("Failed to fetch")).kind).toBe(
      "server",
    );
    const onLine = vi.spyOn(navigator, "onLine", "get").mockReturnValue(false);
    try {
      expect(describeLoadError(new TypeError("Failed to fetch")).kind).toBe(
        "offline",
      );
    } finally {
      onLine.mockRestore();
    }
  });
});
