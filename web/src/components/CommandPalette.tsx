// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import { AnimatePresence, motion } from "motion/react";
import {
  Boxes,
  History,
  Key,
  LayoutDashboard,
  LogOut,
  Search,
  Users,
} from "lucide-react";

import { useLogout, useMe, useRepositories } from "../lib/api";

/**
 * CommandPalette — Cmd+K (or Ctrl+K) anywhere in the app opens a quick
 * launcher with fuzzy substring matching over: navigation targets, every
 * repository, and a sign-out action. Escape or click-outside closes.
 *
 * Inspired by Linear and Raycast; intentionally small (no fuzzy library,
 * no third-party dialog).
 */
export function CommandPalette() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const me = useMe();
  const logout = useLogout();
  const repos = useRepositories();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [highlight, setHighlight] = useState(0);
  const inputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      const isCmdK = (e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k";
      if (isCmdK) {
        e.preventDefault();
        setOpen((o) => !o);
        return;
      }
      if (e.key === "Escape") setOpen(false);
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  useEffect(() => {
    if (open) {
      setQuery("");
      setHighlight(0);
      requestAnimationFrame(() => inputRef.current?.focus());
    }
  }, [open]);

  type Item = { id: string; label: string; kind: string; action: () => void; icon: typeof Search };
  const items = useMemo<Item[]>(() => {
    if (!me.data) return [];
    const go = (to: string) => () => {
      setOpen(false);
      navigate({ to });
    };
    const nav: Item[] = [
      { id: "nav:dashboard", label: t("nav.dashboard"), kind: t("palette.kindPage"), icon: LayoutDashboard, action: go("/dashboard") },
      { id: "nav:repos", label: t("nav.repositories"), kind: t("palette.kindPage"), icon: Boxes, action: go("/repositories") },
      { id: "nav:tokens", label: t("nav.tokens"), kind: t("palette.kindPage"), icon: Key, action: go("/tokens") },
    ];
    if (me.data.is_admin) {
      nav.push(
        { id: "nav:users", label: t("nav.users"), kind: t("palette.kindPage"), icon: Users, action: go("/users") },
        { id: "nav:audit", label: t("nav.audit"), kind: t("palette.kindPage"), icon: History, action: go("/audit") },
      );
    }
    const repoItems: Item[] =
      repos.data?.items.map((r) => ({
        id: `repo:${r.id}`,
        label: r.name,
        kind: `${r.format} · ${r.kind}`,
        icon: Boxes,
        action: () => {
          setOpen(false);
          navigate({ to: "/repositories/$format/$name", params: { format: r.format, name: r.name } });
        },
      })) ?? [];
    const actions: Item[] = [
      {
        id: "act:logout",
        label: t("actions.signOut"),
        kind: t("palette.kindAction"),
        icon: LogOut,
        action: async () => {
          setOpen(false);
          await logout.mutateAsync();
          navigate({ to: "/login" });
        },
      },
    ];
    return [...nav, ...repoItems, ...actions];
  }, [me.data, repos.data, t, navigate, logout]);

  const filtered = useMemo(() => {
    if (!query.trim()) return items;
    const q = query.toLowerCase();
    return items.filter((i) => i.label.toLowerCase().includes(q) || i.kind.toLowerCase().includes(q));
  }, [items, query]);

  useEffect(() => {
    if (highlight >= filtered.length) setHighlight(0);
  }, [filtered.length, highlight]);

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.15 }}
          className="fixed inset-0 z-50 flex items-start justify-center bg-black/40 px-4 pt-[20vh] backdrop-blur-sm"
          onClick={() => setOpen(false)}
        >
          <motion.div
            initial={{ opacity: 0, scale: 0.96, y: -8 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.96, y: -8 }}
            transition={{ duration: 0.18, ease: [0.32, 0.72, 0, 1] }}
            className="w-full max-w-xl overflow-hidden rounded-2xl border border-[color:var(--border-strong)] bg-[color:var(--bg-elev-3)] shadow-2xl backdrop-blur-xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center gap-2 border-b border-[color:var(--border-glass)] px-4 py-3">
              <Search size={16} className="text-[color:var(--fg-muted)]" />
              <input
                ref={inputRef}
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "ArrowDown") {
                    e.preventDefault();
                    setHighlight((h) => Math.min(h + 1, filtered.length - 1));
                  } else if (e.key === "ArrowUp") {
                    e.preventDefault();
                    setHighlight((h) => Math.max(h - 1, 0));
                  } else if (e.key === "Enter") {
                    e.preventDefault();
                    const it = filtered[highlight];
                    if (it) it.action();
                  }
                }}
                placeholder={t("palette.placeholder")}
                className="flex-1 bg-transparent text-base placeholder:text-[color:var(--fg-subtle)] focus:outline-none"
              />
              <kbd className="hidden rounded border border-[color:var(--border-glass)] bg-[color:var(--bg-elev-1)] px-1.5 py-0.5 font-mono text-[10px] text-[color:var(--fg-muted)] sm:inline">
                ESC
              </kbd>
            </div>

            <ul className="max-h-80 overflow-y-auto">
              {filtered.length === 0 ? (
                <li className="px-4 py-6 text-center text-sm text-[color:var(--fg-muted)]">
                  {t("palette.empty")}
                </li>
              ) : (
                filtered.map((it, idx) => (
                  <li key={it.id}>
                    <button
                      type="button"
                      onClick={it.action}
                      onMouseEnter={() => setHighlight(idx)}
                      className={`flex w-full items-center gap-3 px-4 py-2.5 text-left text-sm transition-colors ${
                        idx === highlight ? "bg-[color:var(--bg-elev-2)]" : ""
                      }`}
                    >
                      <it.icon size={14} className="text-[color:var(--fg-muted)]" />
                      <span className="flex-1 truncate">{it.label}</span>
                      <span className="ml-auto truncate text-[10px] uppercase tracking-wider text-[color:var(--fg-subtle)]">
                        {it.kind}
                      </span>
                    </button>
                  </li>
                ))
              )}
            </ul>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
