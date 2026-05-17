// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "@tanstack/react-router";
import {
  BookOpen,
  Boxes,
  History,
  Key,
  LayoutDashboard,
  LogOut,
  Search,
  Users,
} from "lucide-react";

import { Glass } from "./Glass";
import { useTheme } from "../themes/ThemeProvider";
import { SUPPORTED_LANGUAGES, changeLanguage } from "../i18n";
import { useTranslation as useTranslationOriginal } from "react-i18next";
import { useLogout, useMe } from "../lib/api";

/**
 * Shell wraps every authenticated page: aurora background, sidebar, topbar,
 * theme + language switchers, sign-out, and the page slot.
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
  const me = useMe();
  const baseItems = [
    { icon: LayoutDashboard, label: t("nav.dashboard"), to: "/dashboard" },
    { icon: Boxes, label: t("nav.repositories"), to: "/repositories" },
    { icon: Key, label: t("nav.tokens"), to: "/tokens" },
  ];
  const adminItems = [
    { icon: Users, label: t("nav.users"), to: "/users" },
    { icon: History, label: t("nav.audit"), to: "/audit" },
  ];
  const items = me.data?.is_admin ? [...baseItems, ...adminItems] : baseItems;
  return (
    <aside className="w-64 shrink-0 p-4">
      <Glass elevation={2} className="flex h-full flex-col gap-2 p-4">
        <div className="mb-4 flex items-center gap-2 px-2">
          <BookOpen size={20} className="text-[color:var(--accent)]" />
          <span className="text-lg font-semibold tracking-tight">Packrune</span>
        </div>
        <nav className="flex flex-col gap-1">
          {items.map((it) => (
            <Link
              key={it.to}
              to={it.to}
              className="flex w-full items-center gap-3 rounded-xl px-3 py-2 text-left text-sm transition-colors hover:bg-[color:var(--bg-elev-2)]"
              activeProps={{ className: "bg-[color:var(--bg-elev-2)]" }}
            >
              <it.icon size={16} className="text-[color:var(--fg-muted)]" />
              <span>{it.label}</span>
            </Link>
          ))}
        </nav>
        <div className="mt-auto pt-4">
          <UserBadge />
        </div>
      </Glass>
    </aside>
  );
}

function UserBadge() {
  const { t } = useTranslation();
  const me = useMe();
  const logout = useLogout();
  const navigate = useNavigate();
  if (!me.data) return null;
  return (
    <div className="flex items-center justify-between gap-2 rounded-xl bg-[color:var(--bg-elev-1)] px-3 py-2">
      <div className="min-w-0">
        <div className="truncate text-sm font-medium">
          {me.data.display_name || me.data.username}
        </div>
        <div className="truncate text-xs text-[color:var(--fg-muted)]">{me.data.email}</div>
      </div>
      <button
        type="button"
        onClick={async () => {
          await logout.mutateAsync();
          navigate({ to: "/login" });
        }}
        className="rounded-md p-2 text-[color:var(--fg-muted)] transition-colors hover:bg-[color:var(--bg-elev-2)]"
        aria-label={t("actions.signOut")}
        title={t("actions.signOut")}
      >
        <LogOut size={14} />
      </button>
    </div>
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
