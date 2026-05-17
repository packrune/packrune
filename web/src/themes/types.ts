// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

/**
 * Theme registry. Each theme is referenced by its slug; the actual visual
 * variables are CSS custom properties bound via `:root[data-theme="..."]`
 * (see global.css). The TS Theme record carries metadata for the picker:
 * label, color scheme, and a swatch hint for the UI thumbnail.
 */
export interface Theme {
  /** kebab-case identifier, e.g. "aurora". Matches data-theme attribute. */
  readonly slug: string;
  /** human label, shown in the theme picker. */
  readonly label: string;
  /** "dark" | "light" — drives system surfaces (scrollbars, form controls). */
  readonly scheme: "dark" | "light";
  /** swatch colors used by the theme picker thumbnail. */
  readonly swatch: readonly [string, string, string];
}

export type ThemeSlug = Theme["slug"];
