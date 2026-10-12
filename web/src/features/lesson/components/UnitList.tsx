import React, { useMemo, useRef, useState } from "react";
import { ChevronDown, FolderCheck, Search } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { cn } from "@/lib/utils";
import type { CourseUnit } from "../api/lessonApi";
import { LessonRow } from "./LessonRow";
import { NativeSelect } from "@/components/ui/native-select";

export interface UnitListProps {
  units: CourseUnit[];
  nextLessonId?: string | null | undefined;
  onStartLesson?: ((lessonId: string) => void) | undefined;
}

export type StatusFilter =
  "all" | "next" | "not_started" | "completed" | "locked";

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
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [searchToggledIds, setSearchToggledIds] = useState<Set<string>>(
    () => new Set(),
  );

  const searchInputRef = useRef<HTMLInputElement>(null);

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

  // Derive available filter options from the dataset
  const hasNext = Boolean(effectiveNextLessonId);
  const hasNotStarted = useMemo(
    () =>
      units.some((u) =>
        u.lessons?.some(
          (l) => !l.locked && !l.completed && l.id !== effectiveNextLessonId,
        ),
      ),
    [units, effectiveNextLessonId],
  );
  const hasCompleted = useMemo(
    () => units.some((u) => u.lessons?.some((l) => l.completed)),
    [units],
  );
  const hasLocked = useMemo(
    () => units.some((u) => u.lessons?.some((l) => l.locked)),
    [units],
  );

  const isSearching = searchQuery.trim().length > 0;
  const isFiltering = statusFilter !== "all";
  const isConstrained = isSearching || isFiltering;

  // Filter lessons based on both search query and status filter
  const unitMatches = useMemo(() => {
    const matchesMap = new Map<string, CourseUnit["lessons"]>();
    for (const unit of units) {
      const filtered = (unit.lessons ?? []).filter((lesson) => {
        const matchesQuery = isSearching
          ? lessonMatchesQuery(lesson.title, searchQuery)
          : true;
        if (!matchesQuery) return false;

        if (statusFilter === "next") {
          return lesson.id === effectiveNextLessonId;
        }
        if (statusFilter === "not_started") {
          return (
            !lesson.locked &&
            !lesson.completed &&
            lesson.id !== effectiveNextLessonId
          );
        }
        if (statusFilter === "completed") {
          return lesson.completed;
        }
        if (statusFilter === "locked") {
          return lesson.locked;
        }
        return true;
      });
      matchesMap.set(unit.id, filtered);
    }
    return matchesMap;
  }, [units, isSearching, searchQuery, statusFilter, effectiveNextLessonId]);

  const totalMatches = useMemo(() => {
    let count = 0;
    for (const [, list] of unitMatches) {
      count += list?.length ?? 0;
    }
    return count;
  }, [unitMatches]);

  const totalLessonsAll = useMemo(() => {
    return units.reduce((acc, u) => acc + (u.lessons?.length ?? 0), 0);
  }, [units]);

  const queryMatchesCount = useMemo(() => {
    if (!isSearching) return totalLessonsAll;
    let count = 0;
    for (const unit of units) {
      for (const lesson of unit.lessons ?? []) {
        if (lessonMatchesQuery(lesson.title, searchQuery)) {
          count++;
        }
      }
    }
    return count;
  }, [isSearching, units, searchQuery, totalLessonsAll]);

  const toggleUnit = (unitId: string) => {
    if (isConstrained) {
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

  if (units.length === 0 || totalLessonsAll === 0) {
    return (
      <Card className="text-center py-12">
        <CardHeader>
          <div className="flex justify-center mb-2">
            <FolderCheck
              className="h-10 w-10 text-text-muted"
              aria-hidden="true"
            />
          </div>
          <CardTitle className="leading-tight">
            {t("learn.noLessonsTitle", "No lessons")}
          </CardTitle>
          <CardDescription className="text-base leading-relaxed">
            {t(
              "learn.noLessonsDesc",
              "Lessons for this course are currently being authored. Please check back soon.",
            )}
          </CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return (
    <div className="space-y-3">
      {/* Search & Status Filter Row */}
      <div className="flex flex-col gap-2 sm:flex-row sm:gap-3">
        {/* Search Input */}
        <div className="min-w-0 flex-1">
          <label htmlFor="lesson-search" className="sr-only">
            {t("learn.searchLabel", "Search lessons")}
          </label>
          <div className="relative">
            <Search
              className="absolute left-3.5 top-1/2 -translate-y-1/2 h-4 w-4 text-text-muted pointer-events-none"
              aria-hidden="true"
            />
            <input
              ref={searchInputRef}
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
              className="w-full min-h-[44px] rounded-xl border border-border-subtle bg-surface-card py-2.5 pl-10 pr-4 text-base text-text shadow-xs transition-colors placeholder:text-text-muted hover:border-border focus:border-primary/50 focus:outline-none focus:ring-2 focus:ring-primary/30 motion-reduce:transition-none"
            />
          </div>
        </div>

        {/* Status Select Filter */}
        <div className="relative sm:w-[220px]">
          <label htmlFor="status-filter" className="sr-only">
            {t("learn.filterLabel", "Filter by status")}
          </label>
          <NativeSelect
            id="status-filter"
            aria-label={t("learn.filterLabel", "Filter by status")}
            value={statusFilter}
            onChange={(e) => {
              setStatusFilter(e.target.value as StatusFilter);
              setSearchToggledIds(new Set());
            }}
            className="rounded-xl font-medium"
          >
            <option value="all">{t("learn.filterAll", "All lessons")}</option>
            {hasNext && (
              <option value="next">
                {t("learn.filterNext", "Next lesson")}
              </option>
            )}
            {hasNotStarted && (
              <option value="not_started">
                {t("learn.filterNotStarted", "Not started lessons")}
              </option>
            )}
            {hasCompleted && (
              <option value="completed">
                {t("learn.filterCompleted", "Completed lessons")}
              </option>
            )}
            {hasLocked && (
              <option value="locked">
                {t("learn.filterLocked", "Locked lessons")}
              </option>
            )}
          </NativeSelect>
        </div>
      </div>

      {/* Live status announcement region - stays permanently in DOM */}
      <div
        role="status"
        aria-live="polite"
        className="text-sm text-text-muted empty:hidden"
      >
        {isSearching && totalMatches > 0
          ? totalMatches === 1
            ? t("learn.searchResultSingle", "1 lesson found")
            : t("learn.searchResultsCount", "{{count}} lessons found", {
                count: totalMatches,
              })
          : null}
      </div>

      {/* Empty States */}
      {isFiltering &&
      totalMatches === 0 &&
      (queryMatchesCount > 0 || !isSearching) ? (
        <div className="text-center py-12 rounded-xl border border-dashed border-border-subtle bg-surface-card/40 space-y-3">
          <p className="text-base leading-relaxed text-text-muted">
            {t("learn.filterEmpty", "No lessons match the selected filter.")}
          </p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => {
              setStatusFilter("all");
              setSearchToggledIds(new Set());
            }}
          >
            {t("learn.showAllLessons", "Show all lessons")}
          </Button>
        </div>
      ) : isSearching && totalMatches === 0 ? (
        <div
          role="status"
          aria-live="polite"
          className="text-center py-12 rounded-xl border border-dashed border-border-subtle bg-surface-card/40 space-y-3"
        >
          <p className="text-base font-semibold text-text">
            {t("learn.noSearchResults", "No lessons found")}
          </p>
          <p className="break-words text-base leading-relaxed text-text-muted">
            {t("learn.noSearchResultsQuery", {
              query: searchQuery,
              defaultValue: `No lessons match "${searchQuery}".`,
            })}
          </p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => {
              setSearchQuery("");
              setSearchToggledIds(new Set());
              searchInputRef.current?.focus();
            }}
          >
            {t("learn.clearSearch", "Clear search")}
          </Button>
        </div>
      ) : (
        /* Unit & Divided Lesson List */
        <div className="space-y-4">
          {units.map((unit) => {
            const totalLessons = unit.lessons?.length ?? 0;
            const completedLessons =
              unit.lessons?.filter((l) => l.completed).length ?? 0;

            const matchingLessons = unitMatches.get(unit.id) ?? [];
            const unitHasMatches = matchingLessons.length > 0;

            const isExpanded = isConstrained
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
                    className="w-full min-h-[44px] px-4 py-3 md:px-5 flex items-center justify-between gap-3 text-left transition-colors hover:bg-surface-muted/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary cursor-pointer"
                  >
                    <div className="flex flex-wrap items-center gap-x-2 gap-y-1 min-w-0 flex-1">
                      <span className="text-xs font-semibold text-primary-accent uppercase tracking-wider">
                        {t("learn.unitPrefix", "Unit")} {unit.position} ·
                      </span>{" "}
                      <span className="min-w-0 line-clamp-2 break-words text-base md:text-lg font-semibold leading-snug text-text">
                        {unit.title}
                      </span>
                    </div>

                    <div className="flex items-center gap-3 shrink-0">
                      <span className="text-sm font-medium tabular-nums text-text-muted">
                        {t("learn.unitProgress", {
                          completed: completedLessons,
                          total: totalLessons,
                          defaultValue: `${completedLessons}/${totalLessons} completed`,
                        })}
                      </span>
                      <ChevronDown
                        className={cn(
                          "h-5 w-5 text-text-muted transition-transform duration-200",
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
                    className="border-t border-border-subtle"
                  >
                    {unit.description && (
                      <p className="text-sm leading-relaxed text-text-muted px-4 py-2.5 md:px-5 bg-surface-muted/20 border-b border-border-subtle">
                        {unit.description}
                      </p>
                    )}

                    {/* Compact divided lesson rows */}
                    <div className="divide-y divide-border-subtle">
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
                        <p className="text-sm text-text-muted italic px-4 py-3 md:px-5">
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
