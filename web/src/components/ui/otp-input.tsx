import * as React from "react";
import { cn } from "@/lib/utils";

export interface OtpInputProps {
  value: string;
  onChange: (value: string) => void;
  length?: number | undefined;
  disabled?: boolean | undefined;
  error?: boolean | undefined;
  onComplete?: ((code: string) => void) | undefined;
  className?: string | undefined;
}

export const OtpInput = React.forwardRef<HTMLDivElement, OtpInputProps>(
  (
    {
      value = "",
      onChange,
      length = 6,
      disabled = false,
      error = false,
      onComplete,
      className,
    },
    ref,
  ) => {
    const inputRefs = React.useRef<(HTMLInputElement | null)[]>([]);

    const digits = React.useMemo(() => {
      const arr = value.split("").slice(0, length);
      while (arr.length < length) {
        arr.push("");
      }
      return arr;
    }, [value, length]);

    const handleDigitChange = (index: number, digit: string) => {
      // Keep only numeric characters
      const cleanDigit = digit.replace(/\D/g, "");
      if (!cleanDigit && digit !== "") return;

      const newDigits = [...digits];
      newDigits[index] = cleanDigit.slice(-1);
      const newValue = newDigits.join("");
      onChange(newValue);

      if (cleanDigit && index < length - 1) {
        inputRefs.current[index + 1]?.focus();
        inputRefs.current[index + 1]?.select();
      }

      if (newValue.length === length && !newDigits.includes("")) {
        onComplete?.(newValue);
      }
    };

    const handleKeyDown = (
      index: number,
      e: React.KeyboardEvent<HTMLInputElement>,
    ) => {
      if (e.key === "Backspace") {
        if (!digits[index] && index > 0) {
          e.preventDefault();
          inputRefs.current[index - 1]?.focus();
          inputRefs.current[index - 1]?.select();
        }
      } else if (e.key === "ArrowLeft" && index > 0) {
        e.preventDefault();
        inputRefs.current[index - 1]?.focus();
        inputRefs.current[index - 1]?.select();
      } else if (e.key === "ArrowRight" && index < length - 1) {
        e.preventDefault();
        inputRefs.current[index + 1]?.focus();
        inputRefs.current[index + 1]?.select();
      }
    };

    const handlePaste = (e: React.ClipboardEvent<HTMLInputElement>) => {
      e.preventDefault();
      const pasteData = e.clipboardData.getData("text").replace(/\D/g, "");
      if (!pasteData) return;

      const pastedCode = pasteData.slice(0, length);
      onChange(pastedCode);

      const nextFocusIndex = Math.min(pastedCode.length, length - 1);
      inputRefs.current[nextFocusIndex]?.focus();

      if (pastedCode.length === length) {
        onComplete?.(pastedCode);
      }
    };

    return (
      <div
        ref={ref}
        role="group"
        aria-label="One-Time Password"
        className={cn(
          "flex items-center justify-center gap-2 sm:gap-3",
          className,
        )}
      >
        {digits.map((digit, index) => (
          <input
            key={index}
            ref={(el) => {
              inputRefs.current[index] = el;
            }}
            type="text"
            inputMode="numeric"
            pattern="[0-9]*"
            maxLength={1}
            // autocomplete="one-time-code" enables iOS and Android SMS/notification autofill
            autoComplete={index === 0 ? "one-time-code" : "off"}
            aria-label={`Digit ${index + 1} of ${length}`}
            value={digit}
            disabled={disabled}
            onChange={(e) => handleDigitChange(index, e.target.value)}
            onKeyDown={(e) => handleKeyDown(index, e)}
            onPaste={handlePaste}
            onFocus={(e) => e.target.select()}
            className={cn(
              // 44×44 px touch target, text-xl font-bold, visual feedback
              "h-12 w-11 sm:h-14 sm:w-12 rounded-lg border text-center text-xl font-semibold text-slate-100 bg-slate-900/80 transition-all select-none focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-slate-950 disabled:cursor-not-allowed disabled:opacity-50",
              error
                ? "border-rose-500 focus:border-rose-500 focus:ring-rose-500"
                : digit
                  ? "border-indigo-500/80 bg-slate-900 focus:border-indigo-500 focus:ring-indigo-500"
                  : "border-slate-700 focus:border-indigo-500 focus:ring-indigo-500",
            )}
          />
        ))}
      </div>
    );
  },
);

OtpInput.displayName = "OtpInput";
