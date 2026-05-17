// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import {
  BookOpen,
  Boxes,
  History,
  Key,
  LayoutDashboard,
  Package,
  Search,
  Settings,
  Users,
} from "lucide-react";

import { Glass } from "./Glass";
import { useTheme } from "../themes/ThemeProvider";
import { SUPPORTED_LANGUAGES, changeLanguage } from "../i18n";
import { useTranslation as useTranslationOriginal } from "react-i18next";

/**
 * Shell wraps every authenticated page: aurora background, sidebar, topbar,
 * theme + language switchers, and the page slot. Pages just render their
 * content and Shell handles chrome.
 *
 * Today only the demo Landing page is wired; once Faz 7 lands, this Shell
 * will host the real routes.
 */
export function Shell({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-screen w-screen text-[color:var(--fg)]">
      <div className="flex h-screen overflow-hidden">
        <Sidebar />
        <main className="flex-1 overflow-y-auto">
          <Topbar />
          <div className="px-8 py-6">{children}</div>
        </main>
      </div>
    </div>
  );
}

function Sidebar() {
  const { t } = useTranslation();
  const items = [
    { icon: LayoutDashboard, label: t("nav.dashboard") },
    { icon: Boxes, label: t("nav.repositories") },
    { icon: Package, label: t("nav.packages") },
    { icon: Users, label: t("nav.users") },
    { icon: Key, label: t("nav.tokens") },
    { icon: History, label: t("nav.audit") },
    { icon: Settings, label: t("nav.settings") },
  ];
  return (
    <aside className="w-64 shrink-0 p-4">
      <Glass elevation={2} className="flex h-full flex-col gap-2 p-4">
        <div className="mb-4 flex items-center gap-2 px-2">
          <BookOpen size={20} className="text-[color:var(--accent)]" />
          <span className="text-lg font-semibold tracking-tight">Packrune</span>
        </div>
        <nav className="flex flex-col gap-1">
          {items.map((it) => (
            <button
              key={it.label}
              type="button"
              className="flex w-full items-center gap-3 rounded-xl px-3 py-2 text-left text-sm transition-colors hover:bg-[color:var(--bg-elev-2)]"
            >
              <it.icon size={16} className="text-[color:var(--fg-muted)]" />
              <span>{it.label}</span>
            </button>
          ))}
        </nav>
      </Glass>
    </aside>
  );
}

function Topbar() {
  const { t } = useTranslation();
  const { theme, setTheme, available } = useTheme();
  const { i18n } = useTranslationOriginal();
  return (
    <div className="sticky top-0 z-10 px-4 py-3">
      <Glass elevation={2} className="flex items-center gap-3 px-4 py-2">
        <Search size={16} className="text-[color:var(--fg-muted)]" />
        <input
          type="text"
          placeholder={`${t("actions.search")} (Cmd+K)`}
          className="flex-1 bg-transparent text-sm placeholder:text-[color:var(--fg-subtle)] focus:outline-none"
        />
        <select
          value={theme.slug}
          onChange={(e) => setTheme(e.target.value)}
          className="rounded-md bg-[color:var(--bg-elev-2)] px-2 py-1 text-xs"
          aria-label={t("theme.label")}
        >
          {available.map((th) => (
            <option key={th.slug} value={th.slug}>
              {th.label}
            </option>
          ))}
        </select>
        <select
          value={i18n.language}
          onChange={(e) => changeLanguage(e.target.value)}
          className="rounded-md bg-[color:var(--bg-elev-2)] px-2 py-1 text-xs"
          aria-label={t("language.label")}
        >
          {SUPPORTED_LANGUAGES.map((l) => (
            <option key={l.code} value={l.code}>
              {l.label}
            </option>
          ))}
        </select>
      </Glass>
    </div>
  );
}
