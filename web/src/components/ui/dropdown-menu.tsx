import * as React from "react";
import * as RadixDropdownMenu from "@radix-ui/react-dropdown-menu";

import { cn } from "@/lib/utils";

/**
 * The dropdown this design system did not have.
 *
 * Its absence is why the account controls sat exposed in the header: Sign out
 * had to stay a visible button, because rolling a menu by hand means rolling
 * focus trapping, roving tabindex, Escape, outside-click and `aria-expanded` by
 * hand too, and getting one of them wrong is a keyboard user locked in a menu.
 * Radix owns those, and its parts are unstyled, so the styling below is ours.
 *
 * Only the parts in use are re-exported. A wrapper around every Radix part
 * "for completeness" is API surface nobody called and nobody tested.
 */
export const DropdownMenu = RadixDropdownMenu.Root;
export const DropdownMenuTrigger = RadixDropdownMenu.Trigger;

export const DropdownMenuContent = React.forwardRef<
  React.ComponentRef<typeof RadixDropdownMenu.Content>,
  React.ComponentPropsWithoutRef<typeof RadixDropdownMenu.Content>
>(({ className, sideOffset = 6, ...props }, ref) => (
  <RadixDropdownMenu.Portal>
    <RadixDropdownMenu.Content
      ref={ref}
      sideOffset={sideOffset}
      className={cn(
        "z-50 min-w-[12rem] overflow-hidden rounded-xl border border-border-subtle",
        "bg-surface-card p-1 shadow-lg",
        "data-[state=open]:animate-in data-[state=closed]:animate-out",
        "data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0",
        className,
      )}
      {...props}
    />
  </RadixDropdownMenu.Portal>
));
DropdownMenuContent.displayName = "DropdownMenuContent";

export const DropdownMenuItem = React.forwardRef<
  React.ComponentRef<typeof RadixDropdownMenu.Item>,
  React.ComponentPropsWithoutRef<typeof RadixDropdownMenu.Item> & {
    destructive?: boolean;
  }
>(({ className, destructive = false, ...props }, ref) => (
  <RadixDropdownMenu.Item
    ref={ref}
    className={cn(
      // min-h-11 rather than a padding that happens to add up: ADR-0024's
      // 44px floor applies to menu rows on a phone like any other target.
      "relative flex min-h-11 cursor-pointer select-none items-center gap-2.5",
      "rounded-lg px-3 text-sm outline-none transition-colors",
      "focus:bg-surface-muted data-[highlighted]:bg-surface-muted",
      "data-[disabled]:pointer-events-none data-[disabled]:opacity-50",
      destructive
        ? "text-danger-accent focus:bg-danger/10 data-[highlighted]:bg-danger/10"
        : "text-text",
      className,
    )}
    {...props}
  />
));
DropdownMenuItem.displayName = "DropdownMenuItem";

export const DropdownMenuSeparator = React.forwardRef<
  React.ComponentRef<typeof RadixDropdownMenu.Separator>,
  React.ComponentPropsWithoutRef<typeof RadixDropdownMenu.Separator>
>(({ className, ...props }, ref) => (
  <RadixDropdownMenu.Separator
    ref={ref}
    className={cn("-mx-1 my-1 h-px bg-border-subtle", className)}
    {...props}
  />
));
DropdownMenuSeparator.displayName = "DropdownMenuSeparator";

export const DropdownMenuLabel = React.forwardRef<
  React.ComponentRef<typeof RadixDropdownMenu.Label>,
  React.ComponentPropsWithoutRef<typeof RadixDropdownMenu.Label>
>(({ className, ...props }, ref) => (
  <RadixDropdownMenu.Label
    ref={ref}
    className={cn("px-3 py-2 text-xs font-semibold text-text-muted", className)}
    {...props}
  />
));
DropdownMenuLabel.displayName = "DropdownMenuLabel";
