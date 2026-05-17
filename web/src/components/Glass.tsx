// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

import type { CSSProperties, HTMLAttributes, ReactNode } from "react";

interface GlassProps extends HTMLAttributes<HTMLDivElement> {
  /** Visual depth — higher values feel closer to the viewer. */
  elevation?: 1 | 2 | 3;
  /** Override the base glass alpha (0..1). */
  alpha?: number;
  /** Override the backdrop blur radius (px). */
  blur?: number;
  /** Pill-shaped instead of rounded-rect. */
  pill?: boolean;
  children?: ReactNode;
}

/**
 * Glass — a translucent surface that sits over the aurora gradient. The base
 * styles come from CSS custom properties so themes can tune the entire
 * application without touching component code.
 */
export function Glass({
  elevation = 1,
  alpha,
  blur,
  pill,
  className,
  style,
  children,
  ...rest
}: GlassProps) {
  const baseAlpha = alpha ?? (elevation === 3 ? 0.7 : elevation === 2 ? 0.6 : 0.5);
  const blurPx = blur ?? 18 + elevation * 4;

  const inline: CSSProperties = {
    backgroundColor: `color-mix(in oklab, var(--bg-elev-${elevation}) 100%, transparent)`,
    backdropFilter: `blur(${blurPx}px) saturate(var(--glass-saturate))`,
    WebkitBackdropFilter: `blur(${blurPx}px) saturate(var(--glass-saturate))`,
    border: "1px solid var(--border-glass)",
    boxShadow: "var(--shadow-glass)",
    borderRadius: pill ? "var(--radius-pill)" : "var(--radius-glass)",
    opacity: baseAlpha < 1 ? undefined : 1,
    color: "var(--fg)",
    ...style,
  };

  return (
    <div className={className} style={inline} {...rest}>
      {children}
    </div>
  );
}
