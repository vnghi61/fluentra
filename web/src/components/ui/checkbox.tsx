import * as React from "react";
import { Check } from "lucide-react";
import { cn } from "@/lib/utils";

export interface CheckboxProps
  extends Omit<React.InputHTMLAttributes<HTMLInputElement>, "type" | "onChange"> {
  checked?: boolean | undefined;
  onCheckedChange?: ((checked: boolean) => void) | undefined;
  label?: React.ReactNode | undefined;
}

export const Checkbox = React.forwardRef<HTMLInputElement, CheckboxProps>(
  ({ className, checked = false, onCheckedChange, disabled, label, id, ...props }, ref) => {
    const generatedId = React.useId();
    const inputId = id || generatedId;

    return (
      <div className="inline-flex items-center gap-2.5 min-h-[44px] cursor-pointer select-none">
        <div className="relative flex items-center justify-center">
          <input
            type="checkbox"
            id={inputId}
            ref={ref}
            checked={checked}
            disabled={disabled}
            onChange={(e) => onCheckedChange?.(e.target.checked)}
            className="peer sr-only"
            {...props}
          />
          <div
            onClick={() => {
              if (!disabled) onCheckedChange?.(!checked);
            }}
            className={cn(
              "h-5 w-5 rounded-md border border-slate-700 bg-slate-900/80 transition-all peer-focus-visible:ring-2 peer-focus-visible:ring-indigo-500 peer-focus-visible:ring-offset-2 peer-focus-visible:ring-offset-slate-950 flex items-center justify-center cursor-pointer",
              checked && "bg-indigo-600 border-indigo-600 text-white",
              disabled && "opacity-50 cursor-not-allowed",
              className,
            )}
          >
            {checked && <Check className="h-3.5 w-3.5 stroke-[3]" />}
          </div>
        </div>
        {label && (
          <label
            htmlFor={inputId}
            className={cn(
              "text-sm font-medium text-slate-300 cursor-pointer select-none",
              disabled && "opacity-50 cursor-not-allowed",
            )}
          >
            {label}
          </label>
        )}
      </div>
    );
  },
);

Checkbox.displayName = "Checkbox";
