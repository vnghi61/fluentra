import * as React from "react";
import { Link } from "@tanstack/react-router";

import { BrandMark } from "./BrandMark";

export interface MinimalHeaderProps {
  controls?: React.ReactNode;
}

export function MinimalHeader({
  controls,
}: MinimalHeaderProps): React.JSX.Element {
  return (
    <header className="flex min-h-[64px] items-center justify-between border-b border-border-subtle px-4 sm:px-6">
      <Link
        to="/"
        className="inline-flex min-h-[44px] items-center gap-2 text-text"
        aria-label="Fluentra"
      >
        <BrandMark decorative className="h-6 w-6 text-brand" />
        <span className="font-semibold">Fluentra</span>
      </Link>
      {controls && <div className="flex items-center gap-2">{controls}</div>}
    </header>
  );
}
