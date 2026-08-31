import * as React from "react";
import { useReducedMotion } from "motion/react";

import { cn } from "@/lib/utils";

export interface FlipCardProps {
  flipped: boolean;
  front: React.ReactNode;
  back: React.ReactNode;
  className?: string;
  onClick?: (() => void) | undefined;
  /** Announced to a screen reader as the card's purpose. */
  label?: string | undefined;
}

/**
 * A card with two faces that turns over.
 *
 * There was no flip before this. Both flashcards swapped their contents with a
 * fade and left `transition-all` on the container, so the box animated its own
 * height while the text underneath changed instantly — which reads as a stutter
 * rather than as a card turning.
 *
 * Both faces are always in the DOM, stacked, and the container rotates: that is
 * what makes the motion continuous instead of a cross-fade between two heights.
 * `backfaceVisibility: hidden` is what hides the face pointing away, and the
 * back is pre-rotated so it lands upright when the container is at 180°.
 *
 * The height is the taller of the two faces, because both are laid out. That is
 * deliberate — a card that resizes mid-turn is the jank this replaces.
 */
export const FlipCard: React.FC<FlipCardProps> = ({
  flipped,
  front,
  back,
  className,
  onClick,
  label,
}) => {
  // A card that spins is exactly the motion someone with a vestibular disorder
  // has asked the OS not to show them. The card still turns — the two faces are
  // how it works — it simply arrives instead of travelling.
  const reduceMotion = useReducedMotion();

  const face =
    "col-start-1 row-start-1 w-full [backface-visibility:hidden] [-webkit-backface-visibility:hidden]";

  return (
    <div
      className={cn("[perspective:1400px]", className)}
      onClick={onClick}
      {...(onClick && {
        role: "button",
        tabIndex: 0,
        "aria-label": label,
        "aria-pressed": flipped,
        onKeyDown: (event: React.KeyboardEvent) => {
          // Enter and Space are what a button answers to. Space is also the
          // runner's global flip key, so the handler there sees it either way.
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            onClick();
          }
        },
      })}
    >
      {/*
        The rotation is a CSS transform, not a motion value.

        `motion.div` with `animate={{ rotateY: 180 }}` took ownership of the
        element — it wrote the inline style — and resolved the rotation to
        `transform: none`, so the card never turned. Measured in a browser, not
        assumed. A transform with a transition is what a 3D flip is; it
        composites on the GPU, interrupts cleanly mid-turn, and cannot be
        undone by a library's opinion about what my keyframe meant.

        `motion` stays installed and stays registered: it was chosen for the
        gamification work, and this component is not the reason to keep or drop
        it. What it still provides here is `useReducedMotion`, which is the one
        piece worth not hand-rolling.
      */}
      <div
        className={cn(
          "grid [transform-style:preserve-3d] will-change-transform",
          // 500ms with an ease-out back: long enough to read as a turn, short
          // enough not to sit between the question and the answer.
          reduceMotion
            ? "transition-none"
            : "transition-transform duration-500 [transition-timing-function:cubic-bezier(0.2,0.8,0.2,1)]",
        )}
        style={{ transform: flipped ? "rotateY(180deg)" : "rotateY(0deg)" }}
      >
        <div className={face} aria-hidden={flipped}>
          {front}
        </div>
        <div
          className={cn(face, "[transform:rotateY(180deg)]")}
          aria-hidden={!flipped}
        >
          {back}
        </div>
      </div>
    </div>
  );
};
