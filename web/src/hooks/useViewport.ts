import { useState, useEffect } from "react";

export interface ViewportState {
  width: number;
  height: number;
  visualHeight: number;
  isMobile: boolean; // < 640px
  isTablet: boolean; // 640px - 1023px
  isDesktop: boolean; // >= 1024px
  isKeyboardOpen: boolean;
  keyboardHeight: number;
}

function getViewportDimensions(): ViewportState {
  if (typeof window === "undefined") {
    return {
      width: 1280,
      height: 800,
      visualHeight: 800,
      isMobile: false,
      isTablet: false,
      isDesktop: true,
      isKeyboardOpen: false,
      keyboardHeight: 0,
    };
  }

  const width = window.innerWidth;
  const height = window.innerHeight;
  const visualHeight = window.visualViewport?.height ?? height;
  const keyboardHeight = Math.max(0, height - visualHeight);
  // Virtual keyboard is considered active if visual viewport shrunk by > 120px
  const isKeyboardOpen = keyboardHeight > 120;

  return {
    width,
    height,
    visualHeight,
    isMobile: width < 640,
    isTablet: width >= 640 && width < 1024,
    isDesktop: width >= 1024,
    isKeyboardOpen,
    keyboardHeight,
  };
}

export function useViewport(): ViewportState {
  const [viewport, setViewport] = useState<ViewportState>(
    getViewportDimensions,
  );

  useEffect(() => {
    if (typeof window === "undefined") return;

    const handleResize = () => {
      setViewport(getViewportDimensions());
    };

    window.addEventListener("resize", handleResize);
    window.visualViewport?.addEventListener("resize", handleResize);
    window.visualViewport?.addEventListener("scroll", handleResize);

    return () => {
      window.removeEventListener("resize", handleResize);
      window.visualViewport?.removeEventListener("resize", handleResize);
      window.visualViewport?.removeEventListener("scroll", handleResize);
    };
  }, []);

  return viewport;
}
