import * as React from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";
import { X } from "lucide-react";

import { cn } from "@/lib/utils";

export interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: React.ReactNode;
  icon?: React.ReactNode;
  size?: "md" | "lg";
  /** Body and footer. Wrap them in a `<form className={MODAL_FORM_CLASS}>` when needed. */
  children: React.ReactNode;
  /** Blocks backdrop click and Escape (e.g. while a mutation is pending). */
  closeDisabled?: boolean;
}

/** Class for a `<form>` that wraps ModalBody and ModalFooter. */
export const MODAL_FORM_CLASS = "flex min-h-0 flex-1 flex-col";

const FOCUSABLE_SELECTOR =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"]):not([disabled])';

function getFocusableElements(container: HTMLElement | null): HTMLElement[] {
  if (!container) return [];
  return Array.from(
    container.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR),
  ).filter(
    (el) =>
      !(el as HTMLButtonElement).disabled &&
      el.getAttribute("aria-hidden") !== "true" &&
      !el.hasAttribute("hidden") &&
      el.style.display !== "none" &&
      el.style.visibility !== "hidden",
  );
}

// Shared across instances so stacked modals restore the page overflow in order.
let scrollLockCount = 0;
let savedOverflow = "";

function lockScroll(): () => void {
  const root = document.documentElement;
  if (scrollLockCount === 0) {
    savedOverflow = root.style.overflow;
    root.style.overflow = "hidden";
  }
  scrollLockCount += 1;
  let released = false;
  return () => {
    if (released) return;
    released = true;
    scrollLockCount -= 1;
    if (scrollLockCount === 0) root.style.overflow = savedOverflow;
  };
}

export function ModalBody({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>): React.JSX.Element {
  return (
    <div
      className={cn("min-h-0 flex-1 space-y-4 overflow-y-auto p-5", className)}
      {...props}
    />
  );
}

export function ModalFooter({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>): React.JSX.Element {
  return (
    <div
      className={cn(
        "flex shrink-0 justify-end gap-3 border-t border-border-subtle p-4",
        className,
      )}
      {...props}
    />
  );
}

/**
 * Portaled dialog with a bounded panel: fixed header and footer, scrolling body.
 * Plain CSS only; no animation library, so it is safe in the entry chunk.
 */
export function Modal({
  open,
  onClose,
  title,
  icon,
  size = "lg",
  children,
  closeDisabled = false,
}: ModalProps): React.JSX.Element | null {
  const { t } = useTranslation();
  const panelRef = React.useRef<HTMLDivElement>(null);
  const titleId = React.useId();
  const onCloseRef = React.useRef(onClose);
  const closeDisabledRef = React.useRef(closeDisabled);
  React.useEffect(() => {
    onCloseRef.current = onClose;
    closeDisabledRef.current = closeDisabled;
  });

  React.useEffect(() => {
    if (!open) return;
    return lockScroll();
  }, [open]);

  React.useEffect(() => {
    if (!open) return;
    const trigger = document.activeElement as HTMLElement | null;
    const timer = setTimeout(() => {
      const panel = panelRef.current;
      if (!panel) return;
      if (panel.contains(document.activeElement)) return;
      const first = getFocusableElements(panel)[0];
      (first ?? panel).focus();
    }, 0);
    return () => {
      clearTimeout(timer);
      trigger?.focus?.();
    };
  }, [open]);

  React.useEffect(() => {
    if (!open) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        if (event.defaultPrevented) return;
        // An open combobox listbox handles its own Escape.
        if (panelRef.current?.querySelector('[aria-expanded="true"]')) return;
        event.preventDefault();
        if (!closeDisabledRef.current) onCloseRef.current();
        return;
      }
      if (event.key !== "Tab") return;
      const panel = panelRef.current;
      if (!panel) return;
      const focusables = getFocusableElements(panel);
      if (focusables.length === 0) {
        event.preventDefault();
        panel.focus();
        return;
      }
      const first = focusables[0];
      const last = focusables[focusables.length - 1];
      const active = document.activeElement;
      if (event.shiftKey) {
        if (active === first || !panel.contains(active)) {
          event.preventDefault();
          last?.focus();
        }
      } else if (active === last || !panel.contains(active)) {
        event.preventDefault();
        first?.focus();
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [open]);

  if (!open || typeof document === "undefined") return null;

  return createPortal(
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby={titleId}
      data-scroll-lock="managed"
      className="fixed inset-0 z-[60] flex items-center justify-center bg-overlay/50 p-4 pt-[max(1rem,env(safe-area-inset-top))] pb-[max(1rem,env(safe-area-inset-bottom))] animate-in fade-in duration-200 motion-reduce:animate-none"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget && !closeDisabled) onClose();
      }}
    >
      <div
        ref={panelRef}
        tabIndex={-1}
        className={cn(
          "flex max-h-[calc(100dvh-2rem)] w-full flex-col rounded-2xl border border-border-subtle bg-surface-card shadow-xl outline-none",
          size === "md" ? "max-w-md" : "max-w-lg",
        )}
      >
        <div className="flex shrink-0 items-center justify-between gap-3 border-b border-border-subtle px-5 py-3">
          <div className="flex min-w-0 items-center gap-2">
            {icon}
            <h2 id={titleId} className="text-lg font-bold text-text">
              {title}
            </h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            disabled={closeDisabled}
            aria-label={t("app.close", "Close")}
            className="flex h-11 w-11 min-h-[44px] min-w-[44px] shrink-0 items-center justify-center rounded-lg text-text-muted transition-colors hover:bg-surface-muted hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-50"
          >
            <X className="h-5 w-5" aria-hidden="true" />
          </button>
        </div>
        {children}
      </div>
    </div>,
    document.body,
  );
}
