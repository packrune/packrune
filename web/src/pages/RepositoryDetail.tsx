// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate, useParams } from "@tanstack/react-router";
import { motion } from "motion/react";
import { ArrowLeft, Boxes, Filter } from "lucide-react";

import { AuroraBackground } from "../components/AuroraBackground";
import { Glass } from "../components/Glass";
import { Shell } from "../components/Shell";
import { useMe, useRepository, useRepositoryArtifacts } from "../lib/api";

export function RepositoryDetail() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const me = useMe();
  const { name, format } = useParams({ from: "/repositories/$format/$name" });
  const repo = useRepository(name, format);
  const arts = useRepositoryArtifacts(name, format);
  const [filter, setFilter] = useState("");

  useEffect(() => {
    if (me.isError) navigate({ to: "/login" });
  }, [me.isError, navigate]);

  const filtered = useMemo(() => {
    if (!arts.data) return [];
    if (!filter) return arts.data.items;
    const f = filter.toLowerCase();
    return arts.data.items.filter((a) => a.path.toLowerCase().includes(f));
  }, [arts.data, filter]);

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
          <Link
            to="/repositories"
            className="mb-4 inline-flex items-center gap-2 text-xs text-[color:var(--fg-muted)] hover:text-[color:var(--fg)]"
          >
            <ArrowLeft size={12} />
            {t("repositoryDetail.backToRepos")}
          </Link>

          <div className="mb-6 flex items-start justify-between gap-3">
            <div>
              <div className="flex items-center gap-2">
                <Boxes size={20} className="text-[color:var(--accent)]" />
                <h1 className="text-3xl font-semibold tracking-tight">{repo.data?.name ?? name}</h1>
              </div>
              {repo.data && (
                <p className="mt-1 font-mono text-xs text-[color:var(--fg-muted)]">
                  {repo.data.format} · {repo.data.kind} · {t("repositoryDetail.artifactCount", { count: repo.data.artifact_count })}
                </p>
              )}
            </div>
          </div>

          <Glass elevation={2} className="mb-4 flex items-center gap-2 px-4 py-2">
            <Filter size={14} className="text-[color:var(--fg-muted)]" />
            <input
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder={t("repositoryDetail.filterPlaceholder")}
              className="flex-1 bg-transparent text-sm placeholder:text-[color:var(--fg-subtle)] focus:outline-none"
            />
            {arts.data && (
              <span className="text-xs text-[color:var(--fg-muted)]">
                {filtered.length}/{arts.data.total}
              </span>
            )}
          </Glass>

          {arts.isLoading && (
            <p className="text-sm text-[color:var(--fg-muted)]">{t("common.loading")}</p>
          )}

          {filtered.length === 0 && !arts.isLoading ? (
            <Glass elevation={1} className="p-6 text-center text-sm text-[color:var(--fg-muted)]">
              {t("repositoryDetail.empty")}
            </Glass>
          ) : (
            <Glass elevation={2} className="overflow-hidden">
              <ul className="divide-y divide-[color:var(--border-glass)]">
                {filtered.slice(0, 200).map((a) => (
                  <li key={a.id} className="flex items-center gap-3 px-4 py-2.5">
                    <div className="min-w-0 flex-1">
                      <div className="truncate font-mono text-xs">{a.path}</div>
                      <div className="truncate font-mono text-[10px] text-[color:var(--fg-subtle)]">
                        {a.digest}
                      </div>
                    </div>
                    <div className="hidden text-xs text-[color:var(--fg-muted)] sm:block">
                      {formatBytes(a.size)}
                    </div>
                    <div className="text-[10px] text-[color:var(--fg-subtle)]">
                      {new Date(a.created_at).toLocaleDateString()}
                    </div>
                  </li>
                ))}
              </ul>
              {filtered.length > 200 && (
                <div className="px-4 py-2 text-center text-[10px] text-[color:var(--fg-subtle)]">
                  {t("repositoryDetail.truncated", { shown: 200, total: filtered.length })}
                </div>
              )}
            </Glass>
          )}
        </motion.div>
      </Shell>
    </>
  );
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`;
  return `${(n / 1024 / 1024 / 1024).toFixed(2)} GB`;
}
