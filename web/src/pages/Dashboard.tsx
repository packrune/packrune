// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import { motion } from "motion/react";
import { Activity, Boxes, Package, Users } from "lucide-react";

import { AuroraBackground } from "../components/AuroraBackground";
import { Glass } from "../components/Glass";
import { Shell } from "../components/Shell";
import { useMe, useRepositories, useStats } from "../lib/api";

export function Dashboard() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const me = useMe();
  const stats = useStats();
  const repos = useRepositories();

  useEffect(() => {
    if (me.isError) navigate({ to: "/login" });
  }, [me.isError, navigate]);

  if (me.isLoading) {
    return (
      <main className="grid min-h-screen place-items-center">
        <AuroraBackground />
        <div className="text-sm text-[color:var(--fg-muted)]">{t("common.loading")}</div>
      </main>
    );
  }
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
          <div className="mb-6">
            <h1 className="text-3xl font-semibold tracking-tight">
              {t("dashboard.greeting", { name: me.data.display_name || me.data.username })}
            </h1>
            <p className="mt-1 text-sm text-[color:var(--fg-muted)]">{t("dashboard.subtitle")}</p>
          </div>

          <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
            <StatCard
              icon={Boxes}
              label={t("dashboard.statRepos")}
              value={stats.data?.repository_count ?? "—"}
            />
            <StatCard
              icon={Package}
              label={t("dashboard.statArtifacts")}
              value={stats.data?.artifact_count ?? "—"}
            />
            <StatCard
              icon={Activity}
              label={t("dashboard.statFormats")}
              value={Object.keys(stats.data?.per_format ?? {}).length}
            />
          </div>

          <div className="mt-6 grid grid-cols-1 gap-4 md:grid-cols-3">
            <Glass elevation={2} className="md:col-span-2 p-6">
              <div className="mb-4 flex items-center justify-between">
                <h2 className="text-lg font-semibold tracking-tight">
                  {t("dashboard.repositoriesTitle")}
                </h2>
                <button
                  type="button"
                  onClick={() => navigate({ to: "/repositories" })}
                  className="text-xs text-[color:var(--accent)] hover:underline"
                >
                  {t("dashboard.viewAll")}
                </button>
              </div>
              {repos.data?.items.length ? (
                <ul className="flex flex-col gap-2">
                  {repos.data.items.slice(0, 6).map((r) => (
                    <li
                      key={r.id}
                      className="flex items-center justify-between rounded-xl bg-[color:var(--bg-elev-1)] px-4 py-2.5"
                    >
                      <div>
                        <div className="text-sm font-medium">{r.name}</div>
                        <div className="text-xs text-[color:var(--fg-muted)]">
                          {r.format} · {r.kind}
                        </div>
                      </div>
                      <div className="text-xs text-[color:var(--fg-muted)]">
                        {t("dashboard.artifactCount", { count: r.artifact_count })}
                      </div>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-sm text-[color:var(--fg-muted)]">{t("dashboard.empty")}</p>
              )}
            </Glass>

            <Glass elevation={2} className="p-6">
              <h2 className="mb-4 flex items-center gap-2 text-lg font-semibold tracking-tight">
                <Users size={16} className="text-[color:var(--accent)]" />
                {t("dashboard.youTitle")}
              </h2>
              <dl className="space-y-2 text-sm">
                <DlRow term={t("dashboard.username")} value={me.data.username} />
                <DlRow term={t("dashboard.email")} value={me.data.email} />
                <DlRow term={t("dashboard.role")} value={me.data.is_admin ? "admin" : "user"} />
              </dl>
            </Glass>
          </div>

          {stats.data && (
            <Glass elevation={2} className="mt-4 p-6">
              <h2 className="mb-3 text-lg font-semibold tracking-tight">
                {t("dashboard.perFormat")}
              </h2>
              <div className="flex flex-wrap gap-2">
                {Object.entries(stats.data.per_format).length === 0 ? (
                  <span className="text-sm text-[color:var(--fg-muted)]">
                    {t("dashboard.empty")}
                  </span>
                ) : (
                  Object.entries(stats.data.per_format).map(([fmt, n]) => (
                    <span
                      key={fmt}
                      className="rounded-full bg-[color:var(--bg-elev-2)] px-3 py-1 text-xs"
                    >
                      <span className="font-mono">{fmt}</span>{" "}
                      <span className="text-[color:var(--fg-muted)]">{n}</span>
                    </span>
                  ))
                )}
              </div>
            </Glass>
          )}
        </motion.div>
      </Shell>
    </>
  );
}

function StatCard({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof Activity;
  label: string;
  value: number | string;
}) {
  return (
    <Glass elevation={2} className="p-5">
      <div className="flex items-center gap-2 text-xs uppercase tracking-wider text-[color:var(--fg-muted)]">
        <Icon size={14} />
        {label}
      </div>
      <div className="mt-2 text-3xl font-semibold tracking-tight">{value}</div>
    </Glass>
  );
}

function DlRow({ term, value }: { term: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <dt className="text-xs uppercase tracking-wider text-[color:var(--fg-subtle)]">{term}</dt>
      <dd className="font-mono text-xs">{value}</dd>
    </div>
  );
}
