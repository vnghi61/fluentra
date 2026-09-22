import React from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ArrowRight, GraduationCap, Lock } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
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
    <Card>
      <CardHeader>
        <div className="mb-1 flex items-center gap-2 text-text-muted">
          <GraduationCap className="h-5 w-5" aria-hidden="true" />
          <span className="text-xs font-semibold uppercase tracking-wider">
            {t("foundation.next.label", "Foundation path")}
          </span>
        </div>
        <CardTitle className="text-base font-semibold">
          {t("foundation.next.title", "Next in your path")}
        </CardTitle>
        <CardDescription className="space-y-2">
          <span className="flex flex-wrap items-center gap-2">
            <span className="text-text">{node.label || node.code}</span>
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
          </span>
          {t(
            "foundation.next.desc",
            "The first topic you have not mastered yet, in prerequisite order.",
          )}
        </CardDescription>
      </CardHeader>
      <CardFooter className="pt-0">
        <Link to="/foundation/topics/$code" params={{ code: node.code }}>
          <Button variant="secondary" className="gap-2">
            {node.attempts > 0
              ? t("foundation.next.revisitBtn", "Revisit this topic")
              : t("foundation.next.startBtn", "Start this topic")}
            <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Button>
        </Link>
      </CardFooter>
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
