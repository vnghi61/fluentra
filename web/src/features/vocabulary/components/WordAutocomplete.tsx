import React, { useEffect, useRef, useState } from "react";
import { BookOpen, Check, Loader2, Plus, Search, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { useSearchVocabulary } from "../api/searchApi";
import type { WordSummary } from "../api/searchApi";

export interface WordAutocompleteProps {
  onSelectWord?: (word: WordSummary) => void;
  onCustomSubmit?: (term: string) => void;
  placeholder?: string;
  className?: string;
}

export const WordAutocomplete: React.FC<WordAutocompleteProps> = ({
  onSelectWord,
  onCustomSubmit,
  placeholder,
  className,
}) => {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const [isOpen, setIsOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(-1);
  const [selectedWord, setSelectedWord] = useState<WordSummary | null>(null);

  const containerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const { data, isLoading } = useSearchVocabulary(query, isOpen);
  const results = data?.results ?? [];

  // Close dropdown on click outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value;
    setQuery(val);
    setSelectedWord(null);
    setIsOpen(val.trim().length > 0);
    setActiveIndex(-1);
  };

  const handleSelect = (word: WordSummary) => {
    setSelectedWord(word);
    setQuery(word.lemma);
    setIsOpen(false);
    onSelectWord?.(word);
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (!isOpen && e.key === "ArrowDown") {
      setIsOpen(true);
      return;
    }

    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIndex((prev) =>
        prev < results.length - 1 ? prev + 1 : 0,
      );
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIndex((prev) =>
        prev > 0 ? prev - 1 : results.length - 1,
      );
    } else if (e.key === "Enter") {
      e.preventDefault();
      const activeWord = activeIndex >= 0 ? results[activeIndex] : undefined;
      if (activeWord) {
        handleSelect(activeWord);
      } else if (query.trim()) {
        const first = results[0];
        if (first && first.lemma.toLowerCase() === query.trim().toLowerCase()) {
          handleSelect(first);
        } else {
          setIsOpen(false);
          onCustomSubmit?.(query.trim());
        }
      }
    } else if (e.key === "Escape") {
      setIsOpen(false);
    }
  };

  const handleClear = () => {
    setQuery("");
    setSelectedWord(null);
    setIsOpen(false);
    inputRef.current?.focus();
  };

  return (
    <div ref={containerRef} className={cn("relative w-full", className)}>
      <div className="relative">
        <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-text-muted">
          <Search className="h-4 w-4" aria-hidden="true" />
        </div>

        <input
          ref={inputRef}
          type="text"
          role="combobox"
          aria-expanded={isOpen}
          aria-autocomplete="list"
          aria-controls="word-autocomplete-list"
          aria-activedescendant={
            activeIndex >= 0 ? `word-option-${activeIndex}` : undefined
          }
          value={query}
          onChange={handleInputChange}
          onFocus={() => {
            if (query.trim().length > 0) setIsOpen(true);
          }}
          onKeyDown={handleKeyDown}
          placeholder={
            placeholder ??
            t("vocabulary.searchPlaceholder", "Search dictionary or type a word...")
          }
          className={cn(
            "w-full rounded-xl border border-border bg-surface-card py-2.5 pl-10 pr-10",
            "text-base md:text-sm text-text placeholder:text-text-muted",
            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary shadow-xs transition-colors",
          )}
        />

        <div className="absolute inset-y-0 right-0 flex items-center pr-2.5 gap-1">
          {isLoading && (
            <Loader2 className="h-4 w-4 animate-spin text-primary" aria-hidden="true" />
          )}
          {query.length > 0 && (
            <button
              type="button"
              onClick={handleClear}
              className="rounded-md p-1 text-text-muted hover:bg-surface-raised hover:text-text transition-colors"
              aria-label={t("common.clear", "Clear search")}
            >
              <X className="h-3.5 w-3.5" aria-hidden="true" />
            </button>
          )}
        </div>
      </div>

      {/* Selected word confirmation badge */}
      {selectedWord && (
        <div className="mt-2 flex items-center gap-2 rounded-lg border border-success/30 bg-success/10 px-3 py-1.5 text-xs text-success-accent">
          <Check className="h-4 w-4 shrink-0" aria-hidden="true" />
          <span className="font-semibold">{selectedWord.lemma}</span>
          {selectedWord.ipa && <span className="font-mono text-text-muted">{selectedWord.ipa}</span>}
          <Badge variant="outline" className="text-[10px] uppercase font-bold ml-auto border-success/40">
            {selectedWord.cefr_level}
          </Badge>
        </div>
      )}

      {/* Autocomplete Dropdown List */}
      {isOpen && (
        <div
          id="word-autocomplete-list"
          role="listbox"
          className="absolute z-50 mt-1 max-h-64 w-full overflow-auto rounded-xl border border-border bg-surface-card p-1 shadow-lg animate-in fade-in zoom-in-95 duration-100"
        >
          {results.length > 0 ? (
            results.map((word, idx) => {
              const isSelected = idx === activeIndex;
              return (
                <div
                  key={word.id}
                  id={`word-option-${idx}`}
                  role="option"
                  aria-selected={isSelected}
                  onMouseDown={(e) => {
                    e.preventDefault();
                    handleSelect(word);
                  }}
                  onMouseEnter={() => setActiveIndex(idx)}
                  className={cn(
                    "flex cursor-pointer items-center justify-between rounded-lg px-3 py-2 text-xs transition-colors",
                    isSelected
                      ? "bg-primary text-primary-contrast"
                      : "text-text hover:bg-surface-raised",
                  )}
                >
                  <div className="flex items-center gap-2 min-w-0">
                    <BookOpen
                      className={cn("h-3.5 w-3.5 shrink-0", isSelected ? "text-primary-contrast" : "text-primary")}
                      aria-hidden="true"
                    />
                    <span className="font-bold truncate">{word.lemma}</span>
                    {word.ipa && (
                      <span
                        className={cn(
                          "font-mono text-[11px] truncate",
                          isSelected ? "text-primary-contrast/80" : "text-text-muted",
                        )}
                      >
                        {word.ipa}
                      </span>
                    )}
                    {word.pos && (
                      <span
                        className={cn(
                          "italic text-[11px]",
                          isSelected ? "text-primary-contrast/80" : "text-text-muted",
                        )}
                      >
                        ({word.pos})
                      </span>
                    )}
                  </div>

                  <Badge
                    variant="outline"
                    className={cn(
                      "text-[10px] font-bold uppercase shrink-0",
                      isSelected ? "border-primary-contrast/40 text-primary-contrast" : "border-border",
                    )}
                  >
                    {word.cefr_level}
                  </Badge>
                </div>
              );
            })
          ) : !isLoading && query.trim().length > 0 ? (
            <div className="p-3 text-center">
              <p className="text-xs text-text-muted mb-2">
                {t(
                  "vocabulary.notFoundInDict",
                  "\"{{query}}\" is not in the dictionary yet.",
                  { query },
                )}
              </p>
              {onCustomSubmit && (
                <Button
                  size="sm"
                  variant="outline"
                  className="w-full gap-1.5 text-xs font-semibold"
                  onMouseDown={(e) => {
                    e.preventDefault();
                    setIsOpen(false);
                    onCustomSubmit(query.trim());
                  }}
                >
                  <Plus className="h-3.5 w-3.5 text-primary" aria-hidden="true" />
                  {t("vocabulary.addCustomWord", "Add as new custom word")}
                </Button>
              )}
            </div>
          ) : null}
        </div>
      )}
    </div>
  );
};
