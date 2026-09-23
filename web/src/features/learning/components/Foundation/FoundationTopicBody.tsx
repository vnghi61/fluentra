import React from "react";
import { AlertTriangle, Lightbulb, ListChecks } from "lucide-react";
import { useTranslation } from "react-i18next";

/**
 * A foundation topic's published body: objective, explanation, examples and
 * common mistakes.
 *
 * Shared, deliberately: the topic page and the lesson runner render a topic
 * with the same component (WO 22 Stage F). A second renderer would drift, and
 * the drift would show a learner two different explanations of one topic.
 */

interface TopicExample {
  text?: string;
  note?: string;
}

interface TopicMistake {
  wrong?: string;
  right?: string;
  why?: string;
}

export interface FoundationTopicBodyShape {
  objective?: string;
  explanation?: { en?: string; vi?: string } | string;
  examples?: TopicExample[];
  common_mistakes?: TopicMistake[];
}

function explanationFor(
  explanation: FoundationTopicBodyShape["explanation"],
  language: string,
): string {
  if (typeof explanation === "string") return explanation;
  if (!explanation) return "";
  const preferVi = language.startsWith("vi");
  const primary = preferVi ? explanation.vi : explanation.en;
  const fallback = preferVi ? explanation.en : explanation.vi;
  return (primary || fallback || "").trim();
}

export function FoundationTopicBody({
  body,
}: {
  body: FoundationTopicBodyShape;
}): React.JSX.Element {
  const { t, i18n } = useTranslation();
  const explanation = explanationFor(body.explanation, i18n.language);
  const examples = (body.examples ?? []).filter(
    (example) => (example.text ?? "").trim() !== "",
  );
  const mistakes = body.common_mistakes ?? [];

  return (
    <div className="space-y-6">
      {body.objective && (
        <div className="rounded-xl border border-primary/20 bg-primary/5 p-4">
          <p className="text-[11px] font-bold uppercase tracking-wider text-primary-accent">
            {t("foundation.objective", "Learning objective")}
          </p>
          <p className="mt-1 text-sm text-text">{body.objective}</p>
        </div>
      )}

      {explanation && (
        <div className="space-y-2">
          <div className="flex items-center gap-2 text-text-muted">
            <Lightbulb className="h-4 w-4" aria-hidden="true" />
            <span className="text-[11px] font-bold uppercase tracking-wider">
              {t("foundation.explanation", "Explanation")}
            </span>
          </div>
          <p className="whitespace-pre-wrap text-sm leading-relaxed text-text">
            {explanation}
          </p>
        </div>
      )}

      {examples.length > 0 && (
        <div className="space-y-2">
          <div className="flex items-center gap-2 text-text-muted">
            <ListChecks className="h-4 w-4" aria-hidden="true" />
            <span className="text-[11px] font-bold uppercase tracking-wider">
              {t("foundation.examples", "Examples")}
            </span>
          </div>
          <ul className="space-y-2">
            {examples.map((example, index) => (
              <li
                key={index}
                className="rounded-lg border border-border-subtle bg-surface-muted/40 p-3"
              >
                <p className="text-sm font-medium text-text">{example.text}</p>
                {example.note && (
                  <p className="mt-1 text-xs text-text-muted">{example.note}</p>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}

      {mistakes.length > 0 && (
        <div className="space-y-2">
          <div className="flex items-center gap-2 text-text-muted">
            <AlertTriangle className="h-4 w-4" aria-hidden="true" />
            <span className="text-[11px] font-bold uppercase tracking-wider">
              {t("foundation.commonMistakes", "Common mistakes")}
            </span>
          </div>
          <ul className="space-y-2">
            {mistakes.map((mistake, index) => (
              <li
                key={index}
                className="rounded-lg border border-border-subtle p-3 text-sm"
              >
                <p className="text-danger-accent line-through">{mistake.wrong}</p>
                <p className="text-text">{mistake.right}</p>
                {mistake.why && (
                  <p className="mt-1 text-xs text-text-muted">{mistake.why}</p>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
