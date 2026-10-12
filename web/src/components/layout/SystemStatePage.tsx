import * as React from "react";
import { useTranslation } from "react-i18next";

import { OrbitBrandMark } from "./OrbitBrandMark";

export type SystemStateVariant =
  "404" | "server-unavailable" | "500" | "maintenance";

export const systemStateActionClass =
  "inline-flex min-h-[44px] w-full sm:w-auto items-center justify-center rounded-lg px-5 py-2.5 text-sm font-semibold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2";

export interface SystemStatePageProps {
  variant: SystemStateVariant;
  actions?: React.ReactNode;
  /** Quiet supporting detail under the actions, e.g. a request id. */
  children?: React.ReactNode;
}

interface StateCopy {
  titleKey: string;
  title: string;
  descriptionKey: string;
  description: string;
}

const stateCopy: Record<SystemStateVariant, StateCopy> = {
  "404": {
    titleKey: "notFound.title",
    title: "Page not found",
    descriptionKey: "notFound.description",
    description:
      "The page you are looking for does not exist, or has been moved, deleted or is temporarily unavailable.",
  },
  "server-unavailable": {
    titleKey: "server.unavailableTitle",
    title: "Cannot connect to the server",
    descriptionKey: "server.unavailableDescription",
    description:
      "Fluentra cannot reach the server right now. Check your connection or try again in a few minutes.",
  },
  "500": {
    titleKey: "server.errorTitle",
    title: "Something went wrong",
    descriptionKey: "server.errorDescription",
    description:
      "The server ran into a problem handling the request. Please try again later.",
  },
  maintenance: {
    titleKey: "server.maintenanceTitle",
    title: "We are tuning things up",
    descriptionKey: "server.maintenanceDescription",
    description: "Fluentra will be back shortly. Please try again soon.",
  },
};

export function SystemStatePage({
  variant,
  actions,
  children,
}: SystemStatePageProps): React.JSX.Element {
  const { t } = useTranslation();
  const copy = stateCopy[variant];

  return (
    <div className="flex min-h-full flex-1 items-center justify-center overflow-x-hidden px-4 py-12 text-center sm:px-5">
      <div className="flex w-full max-w-[720px] flex-col items-center">
        <OrbitBrandMark size="lg" animated={false} />
        {variant === "404" && (
          <div
            className="mt-4 text-7xl font-extrabold tracking-tight text-primary-accent"
            aria-hidden="true"
          >
            404
          </div>
        )}
        <h1 className="mt-5 text-2xl font-bold leading-tight tracking-tight text-text md:text-3xl">
          {t(copy.titleKey, copy.title)}
        </h1>
        <p className="mt-3 max-w-xl text-base text-text-muted">
          {t(copy.descriptionKey, copy.description)}
        </p>
        {actions && (
          <div className="mt-8 flex w-full flex-col gap-3 sm:w-auto sm:flex-row">
            {actions}
          </div>
        )}
        {children}
      </div>
    </div>
  );
}
