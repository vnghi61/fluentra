import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  isColdStartStatus,
  isNetworkError,
  resetWakeState,
  wakeStatus,
  wakeUp,
} from "@/api/wake";

/**
 * The API sleeps on Render's free tier. These pin the two judgements the rest
 * of the client makes on top of that: what counts as "still booting", and that
 * a page firing several queries at a cold host starts one boot rather than one
 * per query.
 */
describe("cold-start detection", () => {
  it("treats the platform's boot-time statuses as a sleeping host", () => {
    expect(isColdStartStatus(502)).toBe(true);
    expect(isColdStartStatus(503)).toBe(true);
    expect(isColdStartStatus(504)).toBe(true);
  });

  /**
   * The important half. A 4xx means the server answered and had an opinion —
   * retrying it is wrong, and waiting out a wake budget before showing the user
   * their own 403 would be worse.
   */
  it("does not treat an answered request as a sleeping host", () => {
    for (const status of [200, 204, 400, 401, 403, 404, 409, 422, 500]) {
      expect(isColdStartStatus(status)).toBe(false);
    }
  });

  it("recognises a connection that never opened", () => {
    expect(isNetworkError(new TypeError("Failed to fetch"))).toBe(true);
    expect(isNetworkError(new Error("something else"))).toBe(false);
  });
});

describe("wakeUp", () => {
  beforeEach(() => {
    resetWakeState();
    vi.restoreAllMocks();
  });

  afterEach(() => {
    resetWakeState();
  });

  it("reports the host awake once the ping answers", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(new Response("{}", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(wakeUp()).resolves.toBe(true);
    expect(wakeStatus()).toBe("awake");
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  /**
   * Five queries against a cold host must start one boot. Without the
   * single-flight the page would open five connections to a service that is
   * already struggling to start.
   */
  it("starts one boot for concurrent callers", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(new Response("{}", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    const results = await Promise.all([wakeUp(), wakeUp(), wakeUp()]);

    expect(results).toEqual([true, true, true]);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("does not ping again once the host has answered", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(new Response("{}", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    await wakeUp();
    await wakeUp();

    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  /**
   * A server that answers 404 is up. More pinging does not improve a routing
   * mistake, and holding the UI in "starting the server" would misdescribe it.
   */
  it("stops pinging when the server answers with an opinion", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(new Response("nope", { status: 404 }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(wakeUp()).resolves.toBe(true);
    expect(wakeStatus()).toBe("awake");
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});
