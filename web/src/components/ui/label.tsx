import * as React from "react";
import { cn } from "@/lib/utils";

export interface LabelProps extends React.LabelHTMLAttributes<HTMLLabelElement> {
  required?: boolean | undefined;
}

export const Label = React.forwardRef<HTMLLabelElement, LabelProps>(
  ({ className, required, children, ...props }, ref) => {
    return (
      <label
        ref={ref}
        className={cn(
          "text-sm font-medium text-slate-200 leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70 flex items-center gap-1",
          className,
        )}
        {...props}
      >
        {children}
        {required && <span className="text-rose-400 text-xs">*</span>}
      </label>
    );
  },
);

Label.displayName = "Label";
