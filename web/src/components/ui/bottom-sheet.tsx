import * as React from "react";
import { createPortal } from "react-dom";
import {
  AnimatePresence,
  motion,
  useReducedMotion,
  type PanInfo,
} from "motion/react";
import { useTranslation } from "react-i18next";
import { X } from "lucide-react";

import { cn } from "@/lib/utils";

export interface BottomSheetProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: React.ReactNode;
  description?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}

const FOCUSABLE_SELECTOR =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"]):not([disabled])';

function getFocusableElements(container: HTMLElement | null): HTMLElement[] {
  if (!container) return [];
  const nodes = Array.from(
    container.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR),
  );
  return nodes.filter((el) => {
    if (el.hasAttribute("disabled") || (el as HTMLButtonElement).disabled) {
      return false;
    }
    if (el.getAttribute("aria-hidden") === "true") {
      return false;
    }
    if (el.hasAttribute("hidden")) {
      return false;
    }
    if (el.style.display === "none" || el.style.visibility === "hidden") {
      return false;
    }
    return true;
  });
}

/**
 * A bottom sheet modal primitive with a drag handle, spring slide-up, fading backdrop,
 * safe-area padding, and hand-written focus trapping.
 *
 * NOTE: This component uses the full `motion.div` from motion/react and must only
 * ever be loaded from lazily loaded modules (e.g. MoreSheet, ThemeLanguageControls).
 * Static imports from entry-chunk modules such as AppShell or router would violate
 * the lazy-loading convention; the initial download remains budgeted in
 * scripts/check-bundle.mjs (500 kB gzip, user-approved).
 */
export const BottomSheet: React.FC<BottomSheetProps> = ({
  open,
  onOpenChange,
  title,
  description,
  children,
  className,
}) => {
  const { t } = useTranslation();
  const reduceMotion = useReducedMotion();
  const shouldReduceMotion = Boolean(reduceMotion);

  const panelRef = React.useRef<HTMLDivElement>(null);
  const triggerRef = React.useRef<HTMLElement | null>(null);

  const id = React.useId();
  const titleId = `${id}-title`;
  const descriptionId = `${id}-desc`;

  // Focus trap: save trigger, focus first focusable on open, return focus on close
  React.useEffect(() => {
    if (!open) return;
    triggerRef.current = document.activeElement as HTMLElement | null;

    const focusFirst = () => {
      if (!panelRef.current) return;
      const focusables = getFocusableElements(panelRef.current);
      if (focusables.length > 0) {
        focusables[0]?.focus();
      } else {
        panelRef.current.focus();
      }
    };

    const timer = setTimeout(focusFirst, 0);

    return () => {
      clearTimeout(timer);
      triggerRef.current?.focus?.();
    };
  }, [open]);

  // Lock body scroll while open
  React.useEffect(() => {
    if (!open) return;
    const originalOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = originalOverflow;
    };
  }, [open]);

  // Keyboard trap: Tab / Shift+Tab cycling, Escape to close
  React.useEffect(() => {
    if (!open) return;

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        onOpenChange(false);
        return;
      }

      if (event.key === "Tab") {
        if (!panelRef.current) return;
        const focusables = getFocusableElements(panelRef.current);
        if (focusables.length === 0) {
          event.preventDefault();
          panelRef.current.focus();
          return;
        }

        const first = focusables[0];
        const last = focusables[focusables.length - 1];
        const active = document.activeElement;

        if (event.shiftKey) {
          if (active === first || !panelRef.current.contains(active)) {
            event.preventDefault();
            last?.focus();
          }
        } else {
          if (active === last || !panelRef.current.contains(active)) {
            event.preventDefault();
            first?.focus();
          }
        }
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [open, onOpenChange]);

  if (typeof document === "undefined") {
    return null;
  }

  const panelAnimation = shouldReduceMotion
    ? {
        initial: { opacity: 0 },
        animate: { opacity: 1 },
        exit: { opacity: 0 },
        transition: { duration: 0.15 },
      }
    : {
        initial: { y: "100%" },
        animate: { y: 0 },
        exit: { y: "100%" },
        transition: { type: "spring" as const, damping: 28, stiffness: 300 },
        drag: "y" as const,
        dragConstraints: { top: 0 },
        dragElastic: { top: 0, bottom: 0.6 },
        dragSnapToOrigin: true,
        onDragEnd: (
          _event: MouseEvent | TouchEvent | PointerEvent,
          info: PanInfo,
        ) => {
          if (info.offset.y > 100 || info.velocity.y > 500) {
            onOpenChange(false);
          }
        },
      };

  return createPortal(
    <AnimatePresence>
      {open ? (
        <div
          key="bottom-sheet-root"
          className="fixed inset-0 z-50 flex flex-col justify-end"
          onClick={(e) => {
            if (e.target === e.currentTarget) {
              onOpenChange(false);
            }
          }}
        >
          <motion.div
            key="bottom-sheet-backdrop"
            className="fixed inset-0 bg-overlay/50"
            onClick={() => onOpenChange(false)}
            aria-hidden="true"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: shouldReduceMotion ? 0 : 0.15 }}
          />
          <motion.div
            ref={panelRef}
            key="bottom-sheet-panel"
            role="dialog"
            aria-modal="true"
            aria-labelledby={titleId}
            {...(description ? { "aria-describedby": descriptionId } : {})}
            tabIndex={-1}
            className={cn(
              "relative z-10 flex max-h-[85dvh] w-full flex-col overflow-x-hidden rounded-t-3xl border-t border-border-subtle bg-surface-card pb-[max(1rem,env(safe-area-inset-bottom))] shadow-xl outline-none",
              className,
            )}
            {...panelAnimation}
          >
            <div
              className="flex w-full items-center justify-center pt-3 pb-1"
              aria-hidden="true"
            >
              <div
                className="h-1.5 w-10 rounded-full bg-border-subtle"
                aria-hidden="true"
              />
            </div>
            <div className="flex items-start justify-between gap-4 px-6 pt-2 pb-4">
              <div className="min-w-0 flex-1">
                <h2
                  id={titleId}
                  className="text-lg font-semibold leading-tight text-text"
                >
                  {title}
                </h2>
                {description && (
                  <div
                    id={descriptionId}
                    className="mt-1 text-sm text-text-muted"
                  >
                    {description}
                  </div>
                )}
              </div>
              <button
                type="button"
                onClick={() => onOpenChange(false)}
                aria-label={t("common.close", "Close")}
                className="flex h-11 w-11 min-h-[44px] min-w-[44px] shrink-0 items-center justify-center rounded-lg text-text-muted transition-colors hover:bg-surface-muted hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
              >
                <X className="h-5 w-5" aria-hidden="true" />
              </button>
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto px-6">
              {children}
            </div>
          </motion.div>
        </div>
      ) : null}
    </AnimatePresence>,
    document.body,
  );
};

BottomSheet.displayName = "BottomSheet";
