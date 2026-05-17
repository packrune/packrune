// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowRight, FileCode, Heart, Zap } from "lucide-react";
import { motion } from "motion/react";

import { Glass } from "../components/Glass";
import { useTheme } from "../themes/ThemeProvider";
import { SUPPORTED_LANGUAGES, changeLanguage } from "../i18n";
import { useTranslation as useTranslationOriginal } from "react-i18next";
import { getVersion, type VersionInfo } from "../lib/api";

/**
 * Landing — the public unauthenticated face of Packrune. This page is also
 * a vehicle for showing off every theme and language up front: a visitor
 * lands here and immediately can tell the project takes its UI seriously.
 */
export function Landing() {
  const { t } = useTranslation();
  const { theme, setTheme, available } = useTheme();
  const { i18n } = useTranslationOriginal();
  const [version, setVersion] = useState<VersionInfo | null>(null);

  useEffect(() => {
    getVersion()
      .then(setVersion)
      .catch(() => setVersion(null));
  }, []);

  return (
    <main className="relative flex min-h-screen flex-col items-center justify-start px-6 pb-24 pt-12 sm:pt-20">
      <div className="absolute right-4 top-4 z-20 flex gap-2">
        <select
          value={theme.slug}
          onChange={(e) => setTheme(e.target.value)}
          className="rounded-md bg-[color:var(--bg-elev-2)] px-2 py-1 text-xs text-[color:var(--fg)]"
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
          className="rounded-md bg-[color:var(--bg-elev-2)] px-2 py-1 text-xs text-[color:var(--fg)]"
          aria-label={t("language.label")}
        >
          {SUPPORTED_LANGUAGES.map((l) => (
            <option key={l.code} value={l.code}>
              {l.label}
            </option>
          ))}
        </select>
      </div>

      <motion.div
        initial={{ opacity: 0, y: 24 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.7, ease: [0.32, 0.72, 0, 1] }}
        className="flex w-full max-w-5xl flex-col items-center text-center"
      >
        <Glass pill className="mb-6 px-3 py-1 text-xs uppercase tracking-widest">
          <span className="text-[color:var(--accent)]">Pre-alpha</span>
          <span className="mx-2 opacity-30">·</span>
          <span className="text-[color:var(--fg-muted)]">AGPL-3.0 + Commons Clause</span>
        </Glass>

        <h1 className="bg-gradient-to-br from-[color:var(--fg-strong)] to-[color:var(--fg-muted)] bg-clip-text text-5xl font-semibold leading-[1.05] tracking-tight text-transparent sm:text-6xl md:text-7xl">
          {t("landing.headline")}
        </h1>
        <p className="mt-6 max-w-2xl text-base text-[color:var(--fg-muted)] sm:text-lg">
          {t("landing.subhead")}
        </p>

        <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
          <button
            type="button"
            className="group flex items-center gap-2 rounded-full bg-[color:var(--accent)] px-5 py-2.5 text-sm font-medium text-[color:var(--accent-fg)] transition-transform hover:scale-[1.02]"
          >
            {t("landing.ctaPrimary")}
            <ArrowRight size={16} className="transition-transform group-hover:translate-x-0.5" />
          </button>
          <a
            href="https://github.com/packrune/packrune/blob/main/PHASES.md"
            target="_blank"
            rel="noopener noreferrer"
            className="rounded-full border border-[color:var(--border-glass)] px-5 py-2.5 text-sm font-medium text-[color:var(--fg)] transition-colors hover:bg-[color:var(--bg-elev-2)]"
          >
            {t("landing.ctaSecondary")}
          </a>
        </div>
      </motion.div>

      <BentoFeatures />

      {version && (
        <div className="mt-10 font-mono text-xs text-[color:var(--fg-subtle)]">
          packrune {version.version} · {version.go}
        </div>
      )}
    </main>
  );
}

function BentoFeatures() {
  const { t } = useTranslation();
  const cards = [
    {
      icon: Heart,
      title: t("landing.featureFreeTitle"),
      body: t("landing.featureFreeBody"),
      span: "md:col-span-2",
    },
    {
      icon: Zap,
      title: t("landing.featureFastTitle"),
      body: t("landing.featureFastBody"),
      span: "",
    },
    {
      icon: FileCode,
      title: t("landing.featureUnifiedTitle"),
      body: t("landing.featureUnifiedBody"),
      span: "md:col-span-3",
    },
  ];

  return (
    <motion.div
      initial="hidden"
      animate="visible"
      variants={{
        hidden: {},
        visible: { transition: { staggerChildren: 0.08, delayChildren: 0.4 } },
      }}
      className="mt-16 grid w-full max-w-5xl grid-cols-1 gap-4 md:grid-cols-3"
    >
      {cards.map((c) => (
        <motion.div
          key={c.title}
          variants={{
            hidden: { opacity: 0, y: 20 },
            visible: { opacity: 1, y: 0, transition: { duration: 0.6 } },
          }}
          className={c.span}
        >
          <Glass elevation={2} className="flex h-full flex-col gap-3 p-6">
            <c.icon size={20} className="text-[color:var(--accent)]" />
            <h3 className="text-lg font-semibold tracking-tight">{c.title}</h3>
            <p className="text-sm leading-relaxed text-[color:var(--fg-muted)]">{c.body}</p>
          </Glass>
        </motion.div>
      ))}
    </motion.div>
  );
}
