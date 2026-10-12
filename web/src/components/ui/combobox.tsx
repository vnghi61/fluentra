import * as React from "react";
import * as PopoverPrimitive from "@radix-ui/react-popover";
import { Check, ChevronDown, Search } from "lucide-react";
import { cn } from "@/lib/utils";

export interface ComboboxOption {
  value: string;
  label: string;
  searchTerms?: string;
}
export function Combobox({
  id,
  value,
  options,
  onChange,
  searchPlaceholder,
  emptyText,
  placeholder = "",
  disabled = false,
  invalid = false,
}: {
  id: string;
  value: string;
  options: ComboboxOption[];
  onChange: (value: string) => void;
  searchPlaceholder: string;
  emptyText: string;
  placeholder?: string;
  disabled?: boolean;
  invalid?: boolean;
}): React.JSX.Element {
  const [open, setOpen] = React.useState(false);
  const [query, setQuery] = React.useState("");
  const searchRef = React.useRef<HTMLInputElement>(null);
  const fold = (text: string) =>
    text
      .normalize("NFD")
      .replace(/\p{M}/gu, "")
      .replace(/đ/gi, "d")
      .toLowerCase();
  const selected = options.find((option) => option.value === value);
  const visible = options.filter((option) =>
    fold(
      `${option.label} ${option.value} ${option.searchTerms ?? ""}`,
    ).includes(fold(query.trim())),
  );
  return (
    <PopoverPrimitive.Root open={open} onOpenChange={setOpen}>
      <PopoverPrimitive.Trigger asChild>
        <button
          id={id}
          type="button"
          role="combobox"
          aria-haspopup="listbox"
          aria-expanded={open}
          aria-invalid={invalid}
          disabled={disabled}
          className="flex h-11 min-h-[44px] w-full items-center justify-between rounded-lg border border-border bg-surface px-3.5 text-base text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary aria-[invalid=true]:border-danger disabled:cursor-not-allowed disabled:opacity-50 md:text-sm"
        >
          <span className={cn("truncate", !selected && "text-text-muted")}>
            {selected?.label ?? placeholder}
          </span>
          <ChevronDown
            className="ml-2 h-4 w-4 shrink-0 text-text-muted"
            aria-hidden="true"
          />
        </button>
      </PopoverPrimitive.Trigger>
      <PopoverPrimitive.Portal>
        <PopoverPrimitive.Content
          align="start"
          sideOffset={4}
          className="z-[70] w-[var(--radix-popover-trigger-width)] max-h-[var(--radix-popover-content-available-height)] overflow-y-auto rounded-lg border border-border-subtle bg-surface-card shadow-xl"
          onOpenAutoFocus={(event) => {
            event.preventDefault();
            searchRef.current?.focus();
          }}
        >
          <div className="relative border-b border-border-subtle p-2">
            <Search
              className="absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-text-muted"
              aria-hidden="true"
            />
            <input
              ref={searchRef}
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  if (visible[0]) {
                    onChange(visible[0].value);
                    setOpen(false);
                    setQuery("");
                  }
                }
              }}
              placeholder={searchPlaceholder}
              aria-label={searchPlaceholder}
              className="min-h-11 w-full rounded-md border border-border-subtle bg-surface-muted py-2 pl-9 pr-3 text-base focus:outline-none md:text-sm focus:ring-2 focus:ring-primary"
            />
          </div>
          <ul role="listbox" className="max-h-56 overflow-y-auto p-1">
            {visible.length ? (
              visible.map((option) => (
                <li key={option.value || "__unset"}>
                  <button
                    type="button"
                    role="option"
                    aria-selected={option.value === value}
                    onClick={() => {
                      onChange(option.value);
                      setOpen(false);
                      setQuery("");
                    }}
                    className="flex min-h-11 w-full items-center justify-between rounded-md px-3 text-left text-base hover:bg-surface-muted md:text-sm"
                  >
                    {option.label}
                    {option.value === value && (
                      <Check className="h-4 w-4" aria-hidden="true" />
                    )}
                  </button>
                </li>
              ))
            ) : (
              <li className="p-3 text-center text-sm text-text-muted">
                {emptyText}
              </li>
            )}
          </ul>
        </PopoverPrimitive.Content>
      </PopoverPrimitive.Portal>
    </PopoverPrimitive.Root>
  );
}
