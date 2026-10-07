import * as React from "react";
import * as DialogPrimitive from "@radix-ui/react-dialog";

type ReturnFocusProps = Pick<
  React.ComponentPropsWithoutRef<
    typeof DialogPrimitive.Content
  >,
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
      openerRef.current =
        activeElement instanceof HTMLElement ? activeElement : null;
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
