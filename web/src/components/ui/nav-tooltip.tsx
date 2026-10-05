import * as React from "react";
import { createPortal } from "react-dom";

import { cn } from "@/lib/utils";

export const SidebarCollapsedContext = React.createContext<boolean>(false);

export function useSidebarCollapsed(): boolean {
  return React.useContext(SidebarCollapsedContext);
}

export interface SidebarCollapsedProviderProps {
  value: boolean;
  children: React.ReactNode;
}

export function SidebarCollapsedProvider({
  value,
  children,
}: SidebarCollapsedProviderProps): React.JSX.Element {
  return (
    <SidebarCollapsedContext.Provider value={value}>
      {children}
    </SidebarCollapsedContext.Provider>
  );
}

export interface NavTooltipProps {
  label: string;
  children: React.ReactNode;
  enabled?: boolean | undefined;
  className?: string | undefined;
}

export function NavTooltip({
  label,
  children,
  enabled: enabledProp,
  className,
}: NavTooltipProps): React.JSX.Element {
  const contextCollapsed = React.useContext(SidebarCollapsedContext);
  const enabled = enabledProp ?? contextCollapsed;

  const [isOpen, setIsOpen] = React.useState(false);
  const [coords, setCoords] = React.useState<{ top: number; left: number }>({
    top: 0,
    left: 0,
  });
  const wrapperRef = React.useRef<HTMLDivElement>(null);

  // Reset tooltip open state when disabled without triggering setState inside an effect
  const [prevEnabled, setPrevEnabled] = React.useState(enabled);
  if (prevEnabled !== enabled) {
    setPrevEnabled(enabled);
    if (!enabled && isOpen) {
      setIsOpen(false);
    }
  }

  const show = React.useCallback(() => {
    if (!enabled) return;
    if (wrapperRef.current) {
      const rect = wrapperRef.current.getBoundingClientRect();
      setCoords({
        top: rect.top + rect.height / 2,
        left: rect.right + 8,
      });
      setIsOpen(true);
    }
  }, [enabled]);

  const hide = React.useCallback(() => {
    setIsOpen(false);
  }, []);

  const visible = enabled && isOpen;

  React.useEffect(() => {
    if (!visible) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setIsOpen(false);
      }
    };

    const handleScroll = () => {
      setIsOpen(false);
    };

    document.addEventListener("keydown", handleKeyDown);
    window.addEventListener("scroll", handleScroll, true);

    return () => {
      document.removeEventListener("keydown", handleKeyDown);
      window.removeEventListener("scroll", handleScroll, true);
    };
  }, [visible]);

  const handleBlur = (e: React.FocusEvent<HTMLDivElement>) => {
    if (
      wrapperRef.current &&
      e.relatedTarget &&
      wrapperRef.current.contains(e.relatedTarget)
    ) {
      return;
    }
    hide();
  };

  return (
    <div
      ref={wrapperRef}
      className={className}
      onMouseEnter={show}
      onMouseLeave={hide}
      onFocus={show}
      onBlur={handleBlur}
    >
      {children}
      {visible && typeof document !== "undefined"
        ? createPortal(
            <>
              <style>{`
                @keyframes nav-tooltip-fade-in {
                  from { opacity: 0; }
                  to { opacity: 1; }
                }
                @media (prefers-reduced-motion: reduce) {
                  .nav-tooltip-animate {
                    animation: none !important;
                    opacity: 1 !important;
                  }
                }
              `}</style>
              <div
                role="tooltip"
                aria-hidden="true"
                style={{
                  position: "fixed",
                  top: coords.top,
                  left: coords.left,
                  transform: "translateY(-50%)",
                  animation: "nav-tooltip-fade-in 150ms ease-out forwards",
                }}
                className={cn(
                  "pointer-events-none z-[60] whitespace-nowrap rounded-lg px-2 py-1 text-xs font-medium shadow-md",
                  "bg-text text-surface-card nav-tooltip-animate motion-reduce:animate-none",
                )}
              >
                {label}
              </div>
            </>,
            document.body,
          )
        : null}
    </div>
  );
}
