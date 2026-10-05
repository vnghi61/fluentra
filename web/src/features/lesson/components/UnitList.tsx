import React, { useMemo, useState } from "react";
import { ChevronDown, FolderCheck, Search } from "lucide-react";
import { useTranslation } from "react-i18next";

import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { cn } from "@/lib/utils";
import type { CourseUnit } from "../api/lessonApi";
import { LessonRow } from "./LessonRow";

export interface UnitListProps {
  units: CourseUnit[];
  nextLessonId?: string | null;
  onStartLesson?: (lessonId: string) => void;
}

export function normalizeForSearch(text: string): string {
  return text
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .replace(/đ/g, "d")
    .replace(/Đ/g, "d")
    .toLowerCase()
    .trim();
}

export function lessonMatchesQuery(title: string, query: string): boolean {
  const normalizedQuery = normalizeForSearch(query);
  if (!normalizedQuery) return true;
  return normalizeForSearch(title).includes(normalizedQuery);
}

export function findCurrentUnitId(units: CourseUnit[]): string | null {
  for (const unit of units) {
    const hasUnlockedIncomplete = unit.lessons?.some(
      (lesson) => !lesson.locked && !lesson.completed,
    );
    if (hasUnlockedIncomplete) {
      return unit.id;
    }
  }
  return units[0]?.id ?? null;
}

export const UnitList: React.FC<UnitListProps> = ({
  units,
  nextLessonId,
  onStartLesson,
}) => {
  const { t } = useTranslation();
  const [searchQuery, setSearchQuery] = useState("");
  const [searchToggledIds, setSearchToggledIds] = useState<Set<string>>(
    () => new Set(),
  );

  const [expandedUnitIds, setExpandedUnitIds] = useState<Set<string>>(() => {
    const currentId = findCurrentUnitId(units);
    return currentId ? new Set([currentId]) : new Set();
  });

  const effectiveNextLessonId = useMemo(() => {
    if (nextLessonId !== undefined) return nextLessonId;
    for (const unit of units) {
      for (const lesson of unit.lessons ?? []) {
        if (!lesson.locked && !lesson.completed) {
          return lesson.id;
        }
      }
    }
    return null;
  }, [nextLessonId, units]);

  const isSearching = searchQuery.trim().length > 0;

  const totalMatches = useMemo(() => {
    if (!isSearching) return 0;
    let count = 0;
    for (const unit of units) {
      for (const lesson of unit.lessons ?? []) {
        if (lessonMatchesQuery(lesson.title, searchQuery)) {
          count++;
        }
      }
    }
    return count;
  }, [units, searchQuery, isSearching]);

  const toggleUnit = (unitId: string) => {
    if (isSearching) {
      setSearchToggledIds((prev) => {
        const next = new Set(prev);
        if (next.has(unitId)) next.delete(unitId);
        else next.add(unitId);
        return next;
      });
    } else {
      setExpandedUnitIds((prev) => {
        const next = new Set(prev);
        if (next.has(unitId)) next.delete(unitId);
        else next.add(unitId);
        return next;
      });
    }
  };

  if (units.length === 0) {
    return (
      <Card className="text-center py-12">
        <CardHeader>
          <div className="flex justify-center mb-2">
            <FolderCheck
              className="h-10 w-10 text-text-muted"
              aria-hidden="true"
            />
          </div>
          <CardTitle className="text-lg font-bold">
            {t("learn.noUnitsTitle", "Curriculum in Preparation")}
          </CardTitle>
          <CardDescription>
            {t(
              "learn.noUnitsDesc",
              "Units and lessons for this course are currently being authored. Please check back soon.",
            )}
          </CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      {/* Search Input */}
      <div className="space-y-2">
        <label
          htmlFor="lesson-search"
          className="block text-sm font-medium text-text"
        >
          {t("learn.searchLabel", "Search lessons")}
        </label>
        <div className="relative">
          <Search
            className="absolute left-3.5 top-1/2 -translate-y-1/2 h-4 w-4 text-text-muted pointer-events-none"
            aria-hidden="true"
          />
          <input
            id="lesson-search"
            type="search"
            value={searchQuery}
            onChange={(e) => {
              setSearchQuery(e.target.value);
              setSearchToggledIds(new Set());
            }}
            placeholder={t(
              "learn.searchPlaceholder",
              "Search by lesson title…",
            )}
            className="w-full pl-10 pr-4 py-2.5 rounded-xl border border-border-subtle bg-surface-card text-base text-text placeholder:text-text-muted focus:outline-none focus:ring-2 focus:ring-primary min-h-[44px]"
          />
        </div>
        <div
          role="status"
          aria-live="polite"
          className="text-xs text-text-muted"
        >
          {isSearching && totalMatches > 0
            ? totalMatches === 1
              ? t("learn.searchResultSingle", "1 lesson found")
              : t("learn.searchResultsCount", "{{count}} lessons found", {
                  count: totalMatches,
                })
            : null}
        </div>
      </div>

      {/* No search results state */}
      {isSearching && totalMatches === 0 ? (
        <div
          role="status"
          aria-live="polite"
          className="text-center py-12 rounded-xl border border-dashed border-border-subtle bg-surface-card/40"
        >
          <p className="text-base font-semibold text-text">
            {t("learn.noSearchResults", "No lessons found")}
          </p>
          <p className="text-sm text-text-muted mt-1">
            {t(
              "learn.noSearchResultsDesc",
              "Try searching with different keywords.",
            )}
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          {units.map((unit) => {
            const totalLessons = unit.lessons?.length ?? 0;
            const completedLessons =
              unit.lessons?.filter((l) => l.completed).length ?? 0;

            const matchingLessons = isSearching
              ? (unit.lessons ?? []).filter((lesson) =>
                  lessonMatchesQuery(lesson.title, searchQuery),
                )
              : (unit.lessons ?? []);

            const unitHasMatches = isSearching
              ? matchingLessons.length > 0
              : false;

            const isExpanded = isSearching
              ? searchToggledIds.has(unit.id)
                ? !unitHasMatches
                : unitHasMatches
              : expandedUnitIds.has(unit.id);

            return (
              <section
                key={unit.id}
                aria-labelledby={`unit-header-${unit.id}`}
                className="rounded-2xl border border-border-subtle bg-surface-card overflow-hidden transition-colors"
              >
                <h2 className="m-0 p-0 text-base font-semibold">
                  <button
                    type="button"
                    id={`unit-header-${unit.id}`}
                    aria-expanded={isExpanded}
                    aria-controls={`unit-content-${unit.id}`}
                    onClick={() => toggleUnit(unit.id)}
                    className="w-full min-h-[44px] px-4 py-3 md:px-5 flex items-center justify-between gap-3 text-left transition-colors hover:bg-surface-muted/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                  >
                    <div className="flex flex-wrap items-center gap-x-2 gap-y-1 min-w-0 flex-1">
                      <span className="text-xs font-bold text-primary-accent uppercase tracking-wider">
                        {t("learn.unitPrefix", "Unit")} {unit.position} ·
                      </span>{" "}
                      <span className="text-base md:text-lg font-bold text-text truncate">
                        {unit.title}
                      </span>
                    </div>

                    <div className="flex items-center gap-3 shrink-0">
                      <span className="text-xs font-medium text-text-muted">
                        {t("learn.unitProgress", {
                          completed: completedLessons,
                          total: totalLessons,
                          defaultValue: `${completedLessons}/${totalLessons} completed`,
                        })}
                      </span>
                      <ChevronDown
                        className={cn(
                          "h-4 w-4 text-text-muted transition-transform duration-200",
                          isExpanded && "rotate-180",
                        )}
                        aria-hidden="true"
                      />
                    </div>
                  </button>
                </h2>

                {isExpanded && (
                  <div
                    id={`unit-content-${unit.id}`}
                    role="region"
                    aria-labelledby={`unit-header-${unit.id}`}
                    className="p-4 pt-0 md:p-5 md:pt-0 space-y-3"
                  >
                    {unit.description && (
                      <p className="text-sm text-text-muted pb-1 pt-2">
                        {unit.description}
                      </p>
                    )}

                    {/* Lesson Rows */}
                    <div className="space-y-3 pt-2">
                      {matchingLessons.length > 0 ? (
                        matchingLessons.map((lesson) => {
                          const isCompleted = lesson.completed;
                          const isNext = lesson.id === effectiveNextLessonId;

                          return (
                            <LessonRow
                              key={lesson.id}
                              lesson={lesson}
                              isCompleted={isCompleted}
                              isNext={isNext}
                              {...(onStartLesson ? { onStartLesson } : {})}
                            />
                          );
                        })
                      ) : (
                        <p className="text-xs text-text-muted italic py-2">
                          {isSearching
                            ? t(
                                "learn.noMatchesInUnit",
                                "No matching lessons in this unit.",
                              )
                            : t(
                                "learn.emptyUnit",
                                "No lessons in this unit yet.",
                              )}
                        </p>
                      )}
                    </div>
                  </div>
                )}
              </section>
            );
          })}
        </div>
      )}
    </div>
  );
};
