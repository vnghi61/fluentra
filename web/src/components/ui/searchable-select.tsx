import React, { useEffect, useMemo, useRef, useState } from "react";
import { Check, ChevronDown, Search } from "lucide-react";

import { cn } from "@/lib/utils";

export interface SearchableSelectOption {
  value: string;
  label: string;
  /** Extra words the search matches: aliases, codes, names in another language. */
  searchTerms?: string;
}

export interface SearchableSelectProps {
  /** The trigger's id, so a `<Label htmlFor>` names the combobox. */
  id: string;
  value: string;
  options: SearchableSelectOption[];
  onChange: (value: string) => void;
  searchPlaceholder: string;
  emptyText: string;
  /** Shown on the trigger when no option carries the current value. */
  placeholder?: string;
  disabled?: boolean;
  invalid?: boolean;
}

/**
 * Lower-cased, with Vietnamese diacritics folded, so "viet nam" finds "Việt Nam".
 */
function fold(text: string): string {
  return text
    .normalize("NFD")
    .replace(/\p{M}/gu, "")
    .replace(/đ/gi, "d")
    .toLowerCase();
}

/**
 * A select with a search box, for lists too long to scroll: ~250 countries,
 * ~400 time zones.
 */
export const SearchableSelect: React.FC<SearchableSelectProps> = ({
  id,
  value,
  options,
  onChange,
  searchPlaceholder,
  emptyText,
  placeholder = "",
  disabled = false,
  invalid = false,
}) => {
  const [isOpen, setIsOpen] = useState(false);
  const [query, setQuery] = useState("");
  const rootRef = useRef<HTMLDivElement>(null);
  const listboxId = `${id}-listbox`;

  const selected = options.find((option) => option.value === value);

  const filtered = useMemo(() => {
    const q = fold(query.trim());
    if (!q) return options;
    return options.filter((option) =>
      fold(
        `${option.label} ${option.value} ${option.searchTerms ?? ""}`,
      ).includes(q),
    );
  }, [options, query]);

  useEffect(() => {
    if (!isOpen) return;
    const handleClickOutside = (event: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") setIsOpen(false);
    };
    document.addEventListener("mousedown", handleClickOutside);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [isOpen]);

  const choose = (next: string) => {
    onChange(next);
    setIsOpen(false);
    setQuery("");
  };

  return (
    <div className="relative" ref={rootRef}>
      <button
        id={id}
        type="button"
        role="combobox"
        aria-haspopup="listbox"
        aria-expanded={isOpen}
        aria-controls={listboxId}
        aria-invalid={invalid}
        disabled={disabled}
        onClick={() => {
          setIsOpen((prev) => !prev);
          setQuery("");
        }}
        className={
          "flex h-11 min-h-[44px] w-full items-center justify-between rounded-lg border border-border-subtle " +
          "bg-surface-card px-3 text-base text-text focus:outline-none " +
          "focus:ring-2 focus:ring-primary disabled:cursor-not-allowed disabled:opacity-50"
        }
      >
        <span className={cn("truncate", !selected && "text-text-muted")}>
          {selected ? selected.label : placeholder}
        </span>
        <ChevronDown className="h-4 w-4 text-text-muted shrink-0 ml-2" />
      </button>

      {isOpen && (
        <div className="absolute z-50 mt-1 max-h-64 w-full overflow-hidden rounded-lg border border-border-subtle bg-surface-card shadow-xl flex flex-col">
          <div className="p-2 border-b border-border-subtle bg-surface-card">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-text-muted" />
              <input
                type="text"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={(e) => {
                  // Enter takes the first match, so typing "viet" + Enter is enough.
                  if (e.key === "Enter") {
                    e.preventDefault();
                    const first = filtered[0];
                    if (first) choose(first.value);
                  }
                }}
                placeholder={searchPlaceholder}
                aria-label={searchPlaceholder}
                className="w-full pl-9 pr-3 py-2 text-base rounded-md border border-border-subtle bg-surface-muted text-text focus:outline-none focus:ring-2 focus:ring-primary min-h-[44px]"
                autoFocus
              />
            </div>
          </div>
          <ul
            id={listboxId}
            role="listbox"
            className="overflow-y-auto max-h-48 p-1 divide-y divide-border-subtle/20"
          >
            {filtered.length === 0 ? (
              <li className="p-3 text-sm text-text-muted text-center">
                {emptyText}
              </li>
            ) : (
              filtered.map((option) => (
                <li
                  key={option.value || "__unset"}
                  role="option"
                  aria-selected={option.value === value}
                  onClick={() => choose(option.value)}
                  className={
                    "flex items-center justify-between px-3 py-2.5 min-h-[44px] text-base rounded-md cursor-pointer transition-colors hover:bg-surface-muted " +
                    (option.value === value
                      ? "bg-primary/10 text-primary-accent font-medium"
                      : "text-text")
                  }
                >
                  <span className="truncate">{option.label}</span>
                  {option.value === value && (
                    <Check className="h-4 w-4 text-primary-accent shrink-0 ml-2" />
                  )}
                </li>
              ))
            )}
          </ul>
        </div>
      )}
    </div>
  );
};
