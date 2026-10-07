import * as React from "react";
import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { cn } from "@/lib/utils";

export const TooltipProvider = TooltipPrimitive.Provider;
export const Tooltip = TooltipPrimitive.Root;
export const TooltipTrigger = TooltipPrimitive.Trigger;
export const TooltipContent = React.forwardRef<
  React.ElementRef<typeof TooltipPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof TooltipPrimitive.Content>
>(({ className, sideOffset = 6, ...props }, ref) => (
  <TooltipPrimitive.Portal>
    <TooltipPrimitive.Content
      ref={ref}
      sideOffset={sideOffset}
      className={cn(
        "z-[100] max-w-xs rounded-md bg-popover px-3 py-1.5 text-xs text-popover-foreground shadow-md",
        className,
      )}
      {...props}
    />
  </TooltipPrimitive.Portal>
));
TooltipContent.displayName = TooltipPrimitive.Content.displayName;

export const SidebarCollapsedContext = React.createContext(false);
export function useSidebarCollapsed(): boolean {
  return React.useContext(SidebarCollapsedContext);
}
export function SidebarCollapsedProvider({
  value,
  children,
}: {
  value: boolean;
  children: React.ReactNode;
}): React.JSX.Element {
  return (
    <SidebarCollapsedContext.Provider value={value}>
      {children}
    </SidebarCollapsedContext.Provider>
  );
}
export function NavTooltip({
  label,
  children,
  enabled: enabledProp,
  className,
}: {
  label: string;
  children: React.ReactNode;
  enabled?: boolean;
  className?: string;
}): React.JSX.Element {
  const collapsed = useSidebarCollapsed();
  const enabled = enabledProp ?? collapsed;
  const child = React.Children.only(children) as React.ReactElement<{
    className?: string;
  }>;
  const trigger = className
    ? React.cloneElement(child, {
        className: cn(className, child.props.className),
      })
    : child;
  return (
    <Tooltip>
      <TooltipTrigger asChild>{trigger}</TooltipTrigger>
      {enabled && <TooltipContent side="right">{label}</TooltipContent>}
    </Tooltip>
  );
}
