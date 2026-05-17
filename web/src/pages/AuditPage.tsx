// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import { motion } from "motion/react";
import { useQuery } from "@tanstack/react-query";
import { CheckCircle2, ShieldAlert, ShieldOff, ShieldQuestion } from "lucide-react";

import { AuroraBackground } from "../components/AuroraBackground";
import { Glass } from "../components/Glass";
import { Shell } from "../components/Shell";
import { useMe } from "../lib/api";

interface AuditRecord {
  id: string;
  user_id?: string;
  action: string;
  target_type?: string;
  target_id?: string;
  result: "allow" | "deny" | "ok" | "error";
  metadata: string;
  remote_addr?: string;
  created_at: string;
}

export function AuditPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const me = useMe();

  const records = useQuery({
    queryKey: ["audit"],
    queryFn: async (): Promise<{ items: AuditRecord[] }> => {
      const r = await fetch("/api/audit", { credentials: "same-origin" });
      if (!r.ok) throw new Error(`audit: HTTP ${r.status}`);
      return r.json();
    },
    enabled: !!me.data?.is_admin,
    refetchInterval: 15_000,
  });

  useEffect(() => {
    if (me.isError) navigate({ to: "/login" });
  }, [me.isError, navigate]);

  if (!me.data) return null;

  if (!me.data.is_admin) {
    return (
      <>
        <AuroraBackground />
        <Shell>
          <Glass elevation={2} className="p-6 text-sm text-[color:var(--fg-muted)]">
            {t("audit.adminOnly")}
          </Glass>
        </Shell>
      </>
    );
  }

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
            <h1 className="text-3xl font-semibold tracking-tight">{t("audit.title")}</h1>
            <p className="mt-1 text-sm text-[color:var(--fg-muted)]">{t("audit.subtitle")}</p>
          </div>

          {records.data && records.data.items.length > 0 ? (
            <Glass elevation={2} className="overflow-hidden">
              <ul className="divide-y divide-[color:var(--border-glass)]">
                {records.data.items.map((rec) => (
                  <li key={rec.id} className="flex items-center gap-4 px-4 py-3">
                    <ResultIcon result={rec.result} />
                    <div className="min-w-0 flex-1">
                      <div className="font-mono text-sm">{rec.action}</div>
                      {(rec.target_type || rec.target_id) && (
                        <div className="font-mono text-xs text-[color:var(--fg-muted)]">
                          {rec.target_type}
                          {rec.target_type && rec.target_id ? " · " : ""}
                          {rec.target_id}
                        </div>
                      )}
                    </div>
                    <div className="hidden text-xs text-[color:var(--fg-muted)] sm:block">
                      {rec.remote_addr || "—"}
                    </div>
                    <div className="text-xs text-[color:var(--fg-subtle)]">
                      {new Date(rec.created_at).toLocaleString()}
                    </div>
                  </li>
                ))}
              </ul>
            </Glass>
          ) : (
            <Glass elevation={1} className="p-6 text-center text-sm text-[color:var(--fg-muted)]">
              {t("audit.empty")}
            </Glass>
          )}
        </motion.div>
      </Shell>
    </>
  );
}

function ResultIcon({ result }: { result: AuditRecord["result"] }) {
  switch (result) {
    case "allow":
    case "ok":
      return <CheckCircle2 size={16} className="text-emerald-300" />;
    case "deny":
      return <ShieldOff size={16} className="text-red-300" />;
    case "error":
      return <ShieldAlert size={16} className="text-amber-300" />;
    default:
      return <ShieldQuestion size={16} className="text-[color:var(--fg-muted)]" />;
  }
}
