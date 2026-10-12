import * as React from "react";
import * as DialogPrimitive from "@radix-ui/react-dialog";

type ReturnFocusProps = Pick<
  React.ComponentPropsWithoutRef<typeof DialogPrimitive.Content>,
  "onOpenAutoFocus" | "onCloseAutoFocus"
>;

export function useReturnFocus({
  onOpenAutoFocus,
  onCloseAutoFocus,
}: ReturnFocusProps): ReturnFocusProps {
  const openerRef = React.useRef<HTMLElement | null>(null);

  return {
    onOpenAutoFocus: (event) => {
      const activeElement = document.activeElement;
      // A menu item is removed when it opens a dialog. Return to the menu's
      // trigger instead, which Radix identifies through aria-labelledby.
      const menu =
        activeElement instanceof HTMLElement
          ? activeElement.closest('[role="menu"]')
          : null;
      const triggerId = menu?.getAttribute("aria-labelledby");
      const trigger = triggerId ? document.getElementById(triggerId) : null;
      openerRef.current =
        trigger ??
        (activeElement instanceof HTMLElement ? activeElement : null);
      onOpenAutoFocus?.(event);
    },
    onCloseAutoFocus: (event) => {
      onCloseAutoFocus?.(event);
      const opener = openerRef.current;
      if (!event.defaultPrevented && opener?.isConnected) {
        event.preventDefault();
        opener.focus();
      }
    },
  };
}
