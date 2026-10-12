import * as React from "react";
import { cn } from "@/lib/utils";

export interface CardProps extends React.HTMLAttributes<HTMLDivElement> {
  interactive?: boolean;
  tilt?: boolean;
}

function canTilt(): boolean {
  if (
    typeof window === "undefined" ||
    typeof window.matchMedia !== "function"
  ) {
    return false;
  }
  const isFinePointer = window.matchMedia("(pointer: fine)").matches;
  const isReducedMotion = window.matchMedia(
    "(prefers-reduced-motion: reduce)",
  ).matches;
  return isFinePointer && !isReducedMotion;
}

export const Card = React.forwardRef<HTMLDivElement, CardProps>(
  (
    {
      className,
      interactive,
      tilt,
      onPointerMove,
      onPointerLeave,
      style,
      ...props
    },
    ref,
  ) => {
    const [tiltTransform, setTiltTransform] = React.useState<
      string | undefined
    >(undefined);

    const isInteractive = Boolean(interactive || tilt);

    const handlePointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
      onPointerMove?.(e);

      if (!tilt) return;
      if (e.pointerType !== "mouse") return;
      if (!canTilt()) return;

      const rect = e.currentTarget.getBoundingClientRect();
      const centerX = rect.width / 2;
      const centerY = rect.height / 2;
      if (centerX <= 0 || centerY <= 0) return;

      const x = e.clientX - rect.left;
      const y = e.clientY - rect.top;

      const normX = Math.max(-1, Math.min(1, (x - centerX) / centerX));
      const normY = Math.max(-1, Math.min(1, (y - centerY) / centerY));

      const maxTilt = 6;
      const rotateX = -normY * maxTilt;
      const rotateY = normX * maxTilt;

      setTiltTransform(
        `perspective(800px) rotateX(${rotateX.toFixed(2)}deg) rotateY(${rotateY.toFixed(2)}deg)`,
      );
    };

    const handlePointerLeave = (e: React.PointerEvent<HTMLDivElement>) => {
      onPointerLeave?.(e);

      if (!tilt) return;
      setTiltTransform(undefined);
    };

    return (
      <div
        ref={ref}
        onPointerMove={handlePointerMove}
        onPointerLeave={handlePointerLeave}
        style={{
          ...style,
          ...(tiltTransform
            ? {
                transform: style?.transform
                  ? `${style.transform} ${tiltTransform}`
                  : tiltTransform,
              }
            : {}),
        }}
        className={cn(
          "rounded-[var(--radius-card)] border border-border-subtle bg-surface-card text-text shadow-[var(--shadow-card)]",
          isInteractive &&
            "transition-[transform,box-shadow] duration-200 hover:-translate-y-0.5 hover:shadow-md motion-reduce:transition-none motion-reduce:hover:translate-y-0",
          className,
        )}
        {...props}
      />
    );
  },
);
Card.displayName = "Card";

export type CardHeaderProps = React.HTMLAttributes<HTMLDivElement>;

export const CardHeader = React.forwardRef<HTMLDivElement, CardHeaderProps>(
  ({ className, ...props }, ref) => (
    <div
      ref={ref}
      className={cn("flex flex-col space-y-1.5 p-6", className)}
      {...props}
    />
  ),
);
CardHeader.displayName = "CardHeader";

export type CardTitleProps = React.HTMLAttributes<HTMLHeadingElement>;

export const CardTitle = React.forwardRef<HTMLHeadingElement, CardTitleProps>(
  ({ className, children, ...props }, ref) => (
    <h3
      ref={ref}
      className={cn(
        "text-lg font-semibold leading-none tracking-tight text-text",
        className,
      )}
      {...props}
    >
      {children}
    </h3>
  ),
);
CardTitle.displayName = "CardTitle";

export type CardDescriptionProps = React.HTMLAttributes<HTMLParagraphElement>;

export const CardDescription = React.forwardRef<
  HTMLParagraphElement,
  CardDescriptionProps
>(({ className, ...props }, ref) => (
  <p
    ref={ref}
    className={cn("text-sm text-text-muted", className)}
    {...props}
  />
));
CardDescription.displayName = "CardDescription";

export type CardContentProps = React.HTMLAttributes<HTMLDivElement>;

export const CardContent = React.forwardRef<HTMLDivElement, CardContentProps>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn("p-6 pt-0", className)} {...props} />
  ),
);
CardContent.displayName = "CardContent";

export type CardFooterProps = React.HTMLAttributes<HTMLDivElement>;

export const CardFooter = React.forwardRef<HTMLDivElement, CardFooterProps>(
  ({ className, ...props }, ref) => (
    <div
      ref={ref}
      className={cn("flex items-center p-6 pt-0", className)}
      {...props}
    />
  ),
);
CardFooter.displayName = "CardFooter";
