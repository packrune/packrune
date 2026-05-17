// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import { motion } from "motion/react";
import { Boxes } from "lucide-react";

import { AuroraBackground } from "../components/AuroraBackground";
import { Glass } from "../components/Glass";
import { Shell } from "../components/Shell";
import { useMe, useRepositories } from "../lib/api";

const formatBadgeClass: Record<string, string> = {
  docker: "bg-blue-500/20 text-blue-200 border-blue-400/30",
  npm: "bg-red-500/20 text-red-200 border-red-400/30",
  helm: "bg-cyan-500/20 text-cyan-200 border-cyan-400/30",
  gomod: "bg-emerald-500/20 text-emerald-200 border-emerald-400/30",
  pypi: "bg-yellow-500/20 text-yellow-200 border-yellow-400/30",
  maven: "bg-orange-500/20 text-orange-200 border-orange-400/30",
};

export function RepositoriesPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const me = useMe();
  const repos = useRepositories();

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
        >
          <div className="mb-6 flex items-center justify-between">
            <div>
              <h1 className="text-3xl font-semibold tracking-tight">
                {t("repositories.title")}
              </h1>
              <p className="mt-1 text-sm text-[color:var(--fg-muted)]">
                {t("repositories.subtitle")}
              </p>
            </div>
          </div>

          {repos.isLoading && (
            <p className="text-sm text-[color:var(--fg-muted)]">{t("common.loading")}</p>
          )}

          {repos.data && (
            <motion.ul
              initial="hidden"
              animate="visible"
              variants={{
                hidden: {},
                visible: { transition: { staggerChildren: 0.04 } },
              }}
              className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3"
            >
              {repos.data.items.map((r) => (
                <motion.li
                  key={r.id}
                  variants={{
                    hidden: { opacity: 0, y: 12 },
                    visible: { opacity: 1, y: 0, transition: { duration: 0.4 } },
                  }}
                >
                  <Glass elevation={2} className="flex h-full flex-col gap-3 p-5">
                    <div className="flex items-start justify-between gap-2">
                      <div className="flex items-center gap-2">
                        <Boxes size={16} className="text-[color:var(--accent)]" />
                        <h3 className="text-base font-semibold tracking-tight">{r.name}</h3>
                      </div>
                      <span
                        className={`rounded-full border px-2 py-0.5 text-[10px] uppercase tracking-wider ${
                          formatBadgeClass[r.format] ??
                          "border-[color:var(--border-glass)] bg-[color:var(--bg-elev-2)]"
                        }`}
                      >
                        {r.format}
                      </span>
                    </div>
                    <div className="flex items-center justify-between text-xs text-[color:var(--fg-muted)]">
                      <span className="capitalize">{r.kind}</span>
                      <span>{t("repositories.artifactCount", { count: r.artifact_count })}</span>
                    </div>
                    <div className="text-[10px] uppercase tracking-wider text-[color:var(--fg-subtle)]">
                      {new Date(r.created_at).toLocaleDateString()}
                    </div>
                  </Glass>
                </motion.li>
              ))}
            </motion.ul>
          )}
        </motion.div>
      </Shell>
    </>
  );
}
