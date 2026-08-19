import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useViewport } from "@/hooks/useViewport";

describe("useViewport Hook", () => {
  const originalInnerWidth = window.innerWidth;
  const originalInnerHeight = window.innerHeight;
  const originalVisualViewport = window.visualViewport;

  beforeEach(() => {
    // Reset window dimensions to default desktop
    Object.defineProperty(window, "innerWidth", {
      writable: true,
      configurable: true,
      value: 1280,
    });
    Object.defineProperty(window, "innerHeight", {
      writable: true,
      configurable: true,
      value: 800,
    });
  });

  afterEach(() => {
    Object.defineProperty(window, "innerWidth", {
      writable: true,
      configurable: true,
      value: originalInnerWidth,
    });
    Object.defineProperty(window, "innerHeight", {
      writable: true,
      configurable: true,
      value: originalInnerHeight,
    });
    Object.defineProperty(window, "visualViewport", {
      writable: true,
      configurable: true,
      value: originalVisualViewport,
    });
  });

  it("detects desktop viewport (width >= 1024px)", () => {
    window.innerWidth = 1280;
    window.innerHeight = 800;

    const { result } = renderHook(() => useViewport());

    expect(result.current.isDesktop).toBe(true);
    expect(result.current.isTablet).toBe(false);
    expect(result.current.isMobile).toBe(false);
    expect(result.current.width).toBe(1280);
    expect(result.current.height).toBe(800);
  });

  it("detects tablet viewport (640px <= width < 1024px)", () => {
    window.innerWidth = 768;
    window.innerHeight = 1024;

    const { result } = renderHook(() => useViewport());

    expect(result.current.isDesktop).toBe(false);
    expect(result.current.isTablet).toBe(true);
    expect(result.current.isMobile).toBe(false);
    expect(result.current.width).toBe(768);
  });

  it("detects mobile viewport (width < 640px)", () => {
    window.innerWidth = 375;
    window.innerHeight = 667;

    const { result } = renderHook(() => useViewport());

    expect(result.current.isDesktop).toBe(false);
    expect(result.current.isTablet).toBe(false);
    expect(result.current.isMobile).toBe(true);
    expect(result.current.width).toBe(375);
  });

  it("updates state on window resize event", () => {
    window.innerWidth = 1280;
    const { result } = renderHook(() => useViewport());
    expect(result.current.isDesktop).toBe(true);

    act(() => {
      window.innerWidth = 390;
      window.dispatchEvent(new Event("resize"));
    });

    expect(result.current.isMobile).toBe(true);
    expect(result.current.isDesktop).toBe(false);
    expect(result.current.width).toBe(390);
  });

  it("detects virtual keyboard opening when visualViewport height shrinks", () => {
    window.innerWidth = 390;
    window.innerHeight = 844;

    const listeners: Record<string, (() => void)[]> = {};
    const mockVisualViewport = {
      height: 844,
      width: 390,
      addEventListener: vi.fn((event: string, cb: () => void) => {
        listeners[event] = listeners[event] || [];
        listeners[event].push(cb);
      }),
      removeEventListener: vi.fn(),
    };

    Object.defineProperty(window, "visualViewport", {
      writable: true,
      configurable: true,
      value: mockVisualViewport,
    });

    const { result } = renderHook(() => useViewport());

    expect(result.current.isKeyboardOpen).toBe(false);
    expect(result.current.keyboardHeight).toBe(0);

    // Simulate virtual keyboard appearing (taking 300px)
    act(() => {
      mockVisualViewport.height = 544;
      listeners["resize"]?.forEach((cb) => cb());
    });

    expect(result.current.isKeyboardOpen).toBe(true);
    expect(result.current.keyboardHeight).toBe(300);
    expect(result.current.visualHeight).toBe(544);
  });
});
