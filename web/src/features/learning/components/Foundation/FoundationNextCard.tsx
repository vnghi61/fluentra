import React from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ArrowRight, Lock, Map } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { useFoundationNext } from "../../api/foundation";

/**
 * The dashboard's door to the Foundation path (WO 21 Stage F).
 *
 * `/me/foundation/next` is the server's answer to "what should I learn next":
 * the first unmastered node across the strands, in prerequisite order. A 404
 * means there are no published topics yet, and a card advertising a path that
 * does not exist would be the same lie the generated-drills card refuses to
 * tell — so it renders nothing.
 */
export function FoundationNextCard(): React.JSX.Element | null {
  const { t } = useTranslation();
  const next = useFoundationNext();

  if (next.isLoading || next.isError || !next.data) return null;
  const node = next.data;

  return (
    <Card className="bg-surface-card border border-border-subtle rounded-2xl shadow-[0_2px_8px_rgba(15,23,42,0.04)] p-4 md:p-5 transition-all duration-150 hover:-translate-y-0.5 hover:shadow-md motion-reduce:transform-none">
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-2 min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="text-xs font-semibold uppercase tracking-wider text-text-muted">
              {t("foundation.next.eyebrow", "Learning path")}
            </span>
          </div>
          <div>
            <h3 className="text-base md:text-lg font-bold text-text">
              {t("foundation.next.title", "Next in your path")}
            </h3>
            <div className="mt-1 flex flex-wrap items-center gap-2">
              <span className="font-medium text-text text-sm md:text-base">
                {node.label || node.code}
              </span>
              {node.cefr_level && (
                <Badge variant="outline">{node.cefr_level}</Badge>
              )}
              {node.attempts > 0 && (
                <Badge variant="secondary">
                  {t("foundation.next.attempts", {
                    count: node.attempts,
                    defaultValue: `${node.attempts} attempts`,
                  })}
                </Badge>
              )}
            </div>
            <p className="mt-1.5 text-xs md:text-sm text-text-muted">
              {t(
                "foundation.next.desc",
                "The first topic you have not mastered yet, in prerequisite order.",
              )}
            </p>
          </div>
          <div className="pt-1">
            <Link
              to="/foundation/topics/$code"
              params={{ code: node.code }}
              className="inline-flex items-center justify-center font-medium rounded-lg min-h-[44px] px-4 text-sm gap-2 bg-primary text-primary-fg hover:bg-primary-hover active:bg-primary-hover shadow-sm transition-[transform,background-color,box-shadow,color] duration-150 ease-out active:scale-[0.97] hover:-translate-y-px motion-reduce:transition-none motion-reduce:active:scale-100 motion-reduce:hover:translate-y-0 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 select-none cursor-pointer"
            >
              {node.attempts > 0
                ? t("foundation.next.revisitBtn", "Revisit this topic")
                : t("foundation.next.startBtn", "Start this topic")}
              <ArrowRight className="h-4 w-4" aria-hidden="true" />
            </Link>
          </div>
        </div>

        {/* Desktop icon */}
        <div
          className="hidden md:flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary-accent"
          aria-hidden="true"
        >
          <Map className="h-5 w-5" />
        </div>
      </div>
    </Card>
  );
}

/**
 * A path node's state, derived from the two facts the API carries: mastered,
 * and next. A node that is neither, and comes after the next one, is locked
 * behind the prerequisite before it — the path is topologically ordered, so
 * "after next" is what locked means here.
 */
export function pathNodeState(
  node: { mastered: boolean; next: boolean },
  nextSeen: boolean,
): "mastered" | "next" | "locked" | "open" {
  if (node.mastered) return "mastered";
  if (node.next) return "next";
  return nextSeen ? "locked" : "open";
}

/** The lock badge a locked node carries, or null. */
export function LockedBadge(): React.JSX.Element {
  const { t } = useTranslation();
  return (
    <Badge variant="secondary" className="gap-1 text-xs">
      <Lock className="h-3 w-3" aria-hidden="true" />
      {t("foundation.locked", "Locked")}
    </Badge>
  );
}
