// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { motion } from "motion/react";
import { Database, FileText, HardDrive, Lock, Server } from "lucide-react";

import { AuroraBackground } from "../components/AuroraBackground";
import { Glass } from "../components/Glass";
import { Shell } from "../components/Shell";
import { useMe } from "../lib/api";

interface SystemConfig {
  server: { addr: string; external_url: string; read_timeout: number; write_timeout: number };
  database: { driver: string; dsn: string };
  storage: {
    backend: string;
    fs: { root: string };
    s3: { endpoint: string; bucket: string; region: string };
  };
  auth: { session_ttl: number; allow_signup: boolean };
  log: { level: string; format: string };
}

export function SettingsPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const me = useMe();

  const cfg = useQuery({
    queryKey: ["system-config"],
    queryFn: async (): Promise<SystemConfig> => {
      const r = await fetch("/api/system/config", { credentials: "same-origin" });
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return r.json();
    },
    enabled: !!me.data?.is_admin,
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
            {t("settings.adminOnly")}
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
          className="max-w-3xl"
        >
          <div className="mb-6">
            <h1 className="text-3xl font-semibold tracking-tight">{t("settings.title")}</h1>
            <p className="mt-1 text-sm text-[color:var(--fg-muted)]">{t("settings.subtitle")}</p>
          </div>

          {cfg.data && (
            <div className="flex flex-col gap-4">
              <ConfigBlock icon={Server} title={t("settings.server")}>
                <KV k={t("settings.addr")} v={cfg.data.server.addr} />
                <KV k={t("settings.externalUrl")} v={cfg.data.server.external_url} />
              </ConfigBlock>

              <ConfigBlock icon={Database} title={t("settings.database")}>
                <KV k={t("settings.driver")} v={cfg.data.database.driver} />
                <KV k="DSN" v={cfg.data.database.dsn} mono />
              </ConfigBlock>

              <ConfigBlock icon={HardDrive} title={t("settings.storage")}>
                <KV k={t("settings.backend")} v={cfg.data.storage.backend} />
                {cfg.data.storage.backend === "fs" && (
                  <KV k={t("settings.root")} v={cfg.data.storage.fs.root} mono />
                )}
                {cfg.data.storage.backend === "s3" && (
                  <>
                    <KV k="Endpoint" v={cfg.data.storage.s3.endpoint} mono />
                    <KV k="Bucket" v={cfg.data.storage.s3.bucket} />
                  </>
                )}
              </ConfigBlock>

              <ConfigBlock icon={Lock} title={t("settings.auth")}>
                <KV k={t("settings.sessionTTL")} v={`${cfg.data.auth.session_ttl}h`} />
                <KV k={t("settings.allowSignup")} v={cfg.data.auth.allow_signup ? "yes" : "no"} />
              </ConfigBlock>

              <ConfigBlock icon={FileText} title={t("settings.log")}>
                <KV k={t("settings.level")} v={cfg.data.log.level} />
                <KV k={t("settings.format")} v={cfg.data.log.format} />
              </ConfigBlock>

              <p className="text-center text-xs text-[color:var(--fg-subtle)]">
                {t("settings.editHint")}
              </p>
            </div>
          )}
        </motion.div>
      </Shell>
    </>
  );
}

function ConfigBlock({
  icon: Icon,
  title,
  children,
}: {
  icon: typeof Server;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <Glass elevation={2} className="p-5">
      <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold uppercase tracking-wider text-[color:var(--fg-muted)]">
        <Icon size={14} className="text-[color:var(--accent)]" />
        {title}
      </h2>
      <dl className="space-y-1.5">{children}</dl>
    </Glass>
  );
}

function KV({ k, v, mono }: { k: string; v: string; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between gap-3 text-sm">
      <dt className="text-xs uppercase tracking-wider text-[color:var(--fg-subtle)]">{k}</dt>
      <dd className={mono ? "font-mono text-xs" : ""}>{v}</dd>
    </div>
  );
}
