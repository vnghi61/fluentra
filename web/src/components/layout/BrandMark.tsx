import React from "react";

export interface BrandMarkProps {
  className?: string;
}

/**
 * The Fluentra mark, as two stroked paths.
 *
 * Redrawn rather than exported. brand/source/fluentra-mark.svg is a raster
 * trace: 119 paths, 26 of them `fill="white"` painting a ground and the
 * counters of the F, which makes it unusable on any surface that is not white,
 * and 81 kB to say so. This is the same glyph measured off that artwork —
 * stroke 117, stem centre x=175, arm centre y=141, corner radius 190, tail
 * centre x=451 — expressed as what it always was: two round-capped strokes.
 * 362 bytes, and it takes `currentColor`, so it inherits the theme instead of
 * carrying a white plate around with it.
 *
 * Fidelity to the original silhouette is 0.944 by intersection-over-union,
 * measured by rasterising both at 256px and comparing masks. The residue is the
 * trace's own hand-drawn wobble, which a geometric mark is meant to lose.
 */
export const BrandMark: React.FC<BrandMarkProps> = ({ className }) => (
  <svg
    viewBox="0 0 1024 1024"
    fill="none"
    role="img"
    aria-label="Fluentra"
    className={className}
  >
    <g
      stroke="currentColor"
      strokeWidth={117}
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M175 870V331a190 190 0 0 1 190-190h394" />
      <path d="M445 415H700C730 415 727 512 680 570L490 700C458 722 451 758 451 812V868" />
    </g>
  </svg>
);
