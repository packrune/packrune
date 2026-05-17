// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import { motion } from "motion/react";
import { Crown, Mail, Palette, UserCircle2 } from "lucide-react";

import { AuroraBackground } from "../components/AuroraBackground";
import { Glass } from "../components/Glass";
import { Shell } from "../components/Shell";
import { useTheme } from "../themes/ThemeProvider";
import { SUPPORTED_LANGUAGES, changeLanguage } from "../i18n";
import { useTranslation as useTranslationOriginal } from "react-i18next";
import { useMe, useVersion } from "../lib/api";

export function ProfilePage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const me = useMe();
  const version = useVersion();
  const { theme, setTheme, available } = useTheme();
  const { i18n } = useTranslationOriginal();

  useEffect(() => {
    if (me.isError) navigate({ to: "/login" });
  }, [me.isError, navigate]);

  if (!me.data) return null;

  return (
    <>
      <AuroraBackground />
      <Shell>
        <motion.div
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, ease: [0.32, 0.72, 0, 1] }}
          className="max-w-2xl"
        >
          <div className="mb-6">
            <h1 className="text-3xl font-semibold tracking-tight">{t("profile.title")}</h1>
            <p className="mt-1 text-sm text-[color:var(--fg-muted)]">{t("profile.subtitle")}</p>
          </div>

          <Glass elevation={2} className="mb-4 p-6">
            <div className="flex items-center gap-3">
              <UserCircle2 size={32} className="text-[color:var(--accent)]" />
              <div>
                <div className="flex items-center gap-2 text-lg font-semibold tracking-tight">
                  {me.data.display_name || me.data.username}
                  {me.data.is_admin && <Crown size={14} className="text-[color:var(--accent)]" />}
                </div>
                <div className="flex items-center gap-1 text-xs text-[color:var(--fg-muted)]">
                  <Mail size={12} />
                  {me.data.email}
                </div>
              </div>
            </div>
          </Glass>

          <Glass elevation={2} className="mb-4 p-6">
            <h2 className="mb-3 flex items-center gap-2 text-lg font-semibold tracking-tight">
              <Palette size={16} className="text-[color:var(--accent)]" />
              {t("profile.appearance")}
            </h2>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <label className="flex flex-col gap-1">
                <span className="text-xs uppercase tracking-wider text-[color:var(--fg-subtle)]">
                  {t("theme.label")}
                </span>
                <div className="grid grid-cols-5 gap-2">
                  {available.map((th) => (
                    <button
                      key={th.slug}
                      type="button"
                      onClick={() => setTheme(th.slug)}
                      className={`flex flex-col items-center gap-1 rounded-lg border p-2 transition-colors ${
                        th.slug === theme.slug
                          ? "border-[color:var(--accent)] bg-[color:var(--bg-elev-2)]"
                          : "border-[color:var(--border-glass)] bg-[color:var(--bg-elev-1)] hover:bg-[color:var(--bg-elev-2)]"
                      }`}
                      title={th.label}
                    >
                      <div className="flex h-4 w-full overflow-hidden rounded">
                        {th.swatch.map((color, i) => (
                          <div key={i} style={{ background: color, flex: 1 }} />
                        ))}
                      </div>
                      <span className="text-[10px] uppercase tracking-wider">{th.label}</span>
                    </button>
                  ))}
                </div>
              </label>
              <label className="flex flex-col gap-1">
                <span className="text-xs uppercase tracking-wider text-[color:var(--fg-subtle)]">
                  {t("language.label")}
                </span>
                <select
                  value={i18n.language}
                  onChange={(e) => changeLanguage(e.target.value)}
                  className="rounded-xl border border-[color:var(--border-glass)] bg-[color:var(--bg-elev-1)] px-3 py-2 text-sm focus:outline-none"
                >
                  {SUPPORTED_LANGUAGES.map((l) => (
                    <option key={l.code} value={l.code}>
                      {l.label}
                    </option>
                  ))}
                </select>
              </label>
            </div>
          </Glass>

          {version.data && (
            <Glass elevation={2} className="p-6 font-mono text-xs text-[color:var(--fg-muted)]">
              packrune {version.data.version}
              {version.data.commit !== "none" && ` · ${version.data.commit}`}
              {` · ${version.data.go}`}
            </Glass>
          )}
        </motion.div>
      </Shell>
    </>
  );
}
