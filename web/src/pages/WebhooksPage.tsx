// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { motion } from "motion/react";
import { Plus, Radio, Trash2 } from "lucide-react";

import { AuroraBackground } from "../components/AuroraBackground";
import { Glass } from "../components/Glass";
import { Shell } from "../components/Shell";
import { useMe, type Webhook } from "../lib/api";

async function callJSON<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: { Accept: "application/json", "Content-Type": "application/json", ...(init.headers ?? {}) },
    credentials: "same-origin",
  });
  if (!res.ok) {
    let msg = res.statusText;
    try {
      const body = (await res.json()) as { error?: string };
      if (body.error) msg = body.error;
    } catch {}
    throw new Error(msg);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export function WebhooksPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const me = useMe();

  const list = useQuery({
    queryKey: ["webhooks"],
    queryFn: () => callJSON<{ items: Webhook[] }>("/api/webhooks"),
    enabled: !!me.data?.is_admin,
  });

  const qc = useQueryClient();
  const create = useMutation({
    mutationFn: (input: { name: string; url: string; secret: string; events: string[] }) =>
      callJSON<Webhook>("/api/webhooks", { method: "POST", body: JSON.stringify(input) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["webhooks"] }),
  });
  const remove = useMutation({
    mutationFn: (id: string) => callJSON<void>(`/api/webhooks/${id}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["webhooks"] }),
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
            {t("webhooks.adminOnly")}
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
            <h1 className="text-3xl font-semibold tracking-tight">{t("webhooks.title")}</h1>
            <p className="mt-1 text-sm text-[color:var(--fg-muted)]">{t("webhooks.subtitle")}</p>
          </div>

          <NewWebhookForm
            pending={create.isPending}
            error={(create.error as Error | null)?.message}
            onCreate={async (input) => {
              await create.mutateAsync(input);
            }}
          />

          <div className="mt-6">
            {list.data && list.data.items.length > 0 ? (
              <ul className="flex flex-col gap-2">
                {list.data.items.map((wh) => (
                  <li key={wh.id}>
                    <Glass elevation={2} className="flex items-center justify-between gap-4 px-4 py-3">
                      <div className="flex min-w-0 flex-1 items-center gap-3">
                        <Radio
                          size={14}
                          className={wh.is_active ? "text-emerald-300" : "text-[color:var(--fg-muted)]"}
                        />
                        <div className="min-w-0 flex-1">
                          <div className="truncate text-sm font-medium">{wh.name}</div>
                          <div className="truncate font-mono text-xs text-[color:var(--fg-muted)]">
                            {wh.url}
                          </div>
                        </div>
                        <div className="hidden text-xs text-[color:var(--fg-muted)] sm:block">
                          {wh.events.length === 0 ? "*" : wh.events.join(", ")}
                        </div>
                      </div>
                      <button
                        type="button"
                        onClick={() => remove.mutate(wh.id)}
                        className="rounded-md border border-red-500/30 bg-red-500/10 p-2 text-red-200 hover:bg-red-500/20"
                        aria-label={t("webhooks.delete")}
                      >
                        <Trash2 size={14} />
                      </button>
                    </Glass>
                  </li>
                ))}
              </ul>
            ) : (
              <Glass elevation={1} className="p-6 text-center text-sm text-[color:var(--fg-muted)]">
                {t("webhooks.empty")}
              </Glass>
            )}
          </div>
        </motion.div>
      </Shell>
    </>
  );
}

function NewWebhookForm({
  onCreate,
  pending,
  error,
}: {
  onCreate: (input: { name: string; url: string; secret: string; events: string[] }) => Promise<void>;
  pending: boolean;
  error?: string;
}) {
  const { t } = useTranslation();
  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const [secret, setSecret] = useState("");
  const [events, setEvents] = useState("artifact.created");

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    await onCreate({
      name,
      url,
      secret,
      events: events.split(",").map((s) => s.trim()).filter(Boolean),
    });
    setName("");
    setUrl("");
    setSecret("");
  };

  return (
    <Glass elevation={2} className="p-5">
      <h2 className="mb-3 text-lg font-semibold tracking-tight">{t("webhooks.newTitle")}</h2>
      <form onSubmit={onSubmit} className="grid grid-cols-1 gap-3 md:grid-cols-2">
        <input
          required
          placeholder={t("webhooks.namePlaceholder")}
          value={name}
          onChange={(e) => setName(e.target.value)}
          className="rounded-xl border border-[color:var(--border-glass)] bg-[color:var(--bg-elev-1)] px-3 py-2 text-sm focus:outline-none"
        />
        <input
          required
          type="url"
          placeholder={t("webhooks.urlPlaceholder")}
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          className="rounded-xl border border-[color:var(--border-glass)] bg-[color:var(--bg-elev-1)] px-3 py-2 font-mono text-xs focus:outline-none"
        />
        <input
          placeholder={t("webhooks.secretPlaceholder")}
          value={secret}
          onChange={(e) => setSecret(e.target.value)}
          className="rounded-xl border border-[color:var(--border-glass)] bg-[color:var(--bg-elev-1)] px-3 py-2 font-mono text-xs focus:outline-none"
        />
        <input
          placeholder={t("webhooks.eventsPlaceholder")}
          value={events}
          onChange={(e) => setEvents(e.target.value)}
          className="rounded-xl border border-[color:var(--border-glass)] bg-[color:var(--bg-elev-1)] px-3 py-2 font-mono text-xs focus:outline-none"
        />
        <button
          type="submit"
          disabled={pending}
          className="flex items-center justify-center gap-2 rounded-full bg-[color:var(--accent)] px-4 py-2 text-sm font-medium text-[color:var(--accent-fg)] hover:scale-[1.01] disabled:opacity-60 md:col-span-2"
        >
          <Plus size={14} />
          {pending ? t("webhooks.creating") : t("webhooks.create")}
        </button>
        {error && (
          <div className="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-xs text-red-200 md:col-span-2">
            {error}
          </div>
        )}
      </form>
    </Glass>
  );
}
