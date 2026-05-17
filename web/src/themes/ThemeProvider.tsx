// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";

import { DEFAULT_THEME, themeBySlug, themes } from "./themes";
import type { Theme, ThemeSlug } from "./types";

interface ThemeContextValue {
  theme: Theme;
  setTheme: (slug: ThemeSlug) => void;
  available: readonly Theme[];
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

const STORAGE_KEY = "packrune.theme";

function readInitialSlug(): ThemeSlug {
  if (typeof document === "undefined") return DEFAULT_THEME;
  const stored = window.localStorage.getItem(STORAGE_KEY);
  if (stored && themeBySlug(stored)) return stored;
  const attr = document.documentElement.getAttribute("data-theme");
  if (attr && themeBySlug(attr)) return attr;
  return DEFAULT_THEME;
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [slug, setSlug] = useState<ThemeSlug>(readInitialSlug);

  useEffect(() => {
    const root = document.documentElement;
    root.setAttribute("data-theme", slug);
    const t = themeBySlug(slug);
    if (t) root.style.colorScheme = t.scheme;
    window.localStorage.setItem(STORAGE_KEY, slug);
  }, [slug]);

  const setTheme = useCallback((next: ThemeSlug) => {
    if (themeBySlug(next)) setSlug(next);
  }, []);

  const value = useMemo<ThemeContextValue>(
    () => ({
      theme: themeBySlug(slug) ?? themes[0]!,
      setTheme,
      available: themes,
    }),
    [slug, setTheme],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error("useTheme: must be used inside <ThemeProvider>");
  return ctx;
}
