// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

import type { Theme } from "./types";

export const themes: readonly Theme[] = [
  {
    slug: "aurora",
    label: "Aurora",
    scheme: "dark",
    swatch: ["#7c5cff", "#3cc8ff", "#ff82dc"],
  },
  {
    slug: "midnight",
    label: "Midnight",
    scheme: "dark",
    swatch: ["#1e3cc8", "#64c8ff", "#3c328c"],
  },
  {
    slug: "daybreak",
    label: "Daybreak",
    scheme: "light",
    swatch: ["#6c4dff", "#6edcff", "#ffaae6"],
  },
  {
    slug: "terminal",
    label: "Terminal",
    scheme: "dark",
    swatch: ["#7dff7d", "#ffc83c", "#00783c"],
  },
  {
    slug: "mono",
    label: "Mono",
    scheme: "dark",
    swatch: ["#d4d4d4", "#8c8c8c", "#505050"],
  },
] as const;

export const DEFAULT_THEME = "aurora" as const;

export function themeBySlug(slug: string): Theme | undefined {
  return themes.find((t) => t.slug === slug);
}
