// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import { motion } from "motion/react";
import { Check, Copy, Key, Plus, Trash2 } from "lucide-react";

import { AuroraBackground } from "../components/AuroraBackground";
import { Glass } from "../components/Glass";
import { Shell } from "../components/Shell";
import { useCreateToken, useMe, useRevokeToken, useTokens, type TokenWithPlain } from "../lib/api";

export function TokensPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const me = useMe();
  const tokens = useTokens();
  const create = useCreateToken();
  const revoke = useRevokeToken();
  const [issued, setIssued] = useState<TokenWithPlain | null>(null);
  const [copied, setCopied] = useState(false);

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
          <div className="mb-6">
            <h1 className="text-3xl font-semibold tracking-tight">{t("tokens.title")}</h1>
            <p className="mt-1 text-sm text-[color:var(--fg-muted)]">{t("tokens.subtitle")}</p>
          </div>

          <NewTokenForm
            onIssued={(it) => {
              setIssued(it);
              setCopied(false);
            }}
            issuing={create.isPending}
            mutate={(input) => create.mutateAsync(input)}
          />

          {issued && (
            <motion.div
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              className="mt-4"
            >
              <Glass elevation={3} className="border border-[color:var(--accent)] p-5">
                <div className="mb-2 text-xs uppercase tracking-wider text-[color:var(--accent)]">
                  {t("tokens.copyOnce")}
                </div>
                <div className="flex items-center gap-3">
                  <code className="flex-1 truncate rounded-md bg-[color:var(--bg-elev-2)] px-3 py-2 font-mono text-xs">
                    {issued.plain}
                  </code>
                  <button
                    type="button"
                    onClick={async () => {
                      await navigator.clipboard.writeText(issued.plain);
                      setCopied(true);
                    }}
                    className="flex items-center gap-2 rounded-md bg-[color:var(--accent)] px-3 py-2 text-xs font-medium text-[color:var(--accent-fg)]"
                  >
                    {copied ? <Check size={14} /> : <Copy size={14} />}
                    {copied ? t("tokens.copied") : t("tokens.copy")}
                  </button>
                </div>
              </Glass>
            </motion.div>
          )}

          <div className="mt-6">
            {tokens.data && tokens.data.items.length > 0 ? (
              <ul className="flex flex-col gap-2">
                {tokens.data.items.map((tk) => (
                  <li key={tk.id}>
                    <Glass
                      elevation={2}
                      className="flex items-center justify-between gap-4 px-4 py-3"
                    >
                      <div className="flex min-w-0 flex-1 items-center gap-3">
                        <Key size={14} className="text-[color:var(--fg-muted)]" />
                        <div className="min-w-0 flex-1">
                          <div className="truncate text-sm font-medium">{tk.name}</div>
                          <div className="truncate font-mono text-xs text-[color:var(--fg-muted)]">
                            {tk.prefix}…
                          </div>
                        </div>
                        <div className="hidden text-xs text-[color:var(--fg-muted)] sm:block">
                          {tk.scopes.join(", ") || t("tokens.noScopes")}
                        </div>
                        <div className="hidden text-xs text-[color:var(--fg-subtle)] md:block">
                          {new Date(tk.created_at).toLocaleDateString()}
                        </div>
                      </div>
                      <button
                        type="button"
                        onClick={() => revoke.mutate(tk.id)}
                        className="rounded-md border border-red-500/30 bg-red-500/10 p-2 text-red-200 transition-colors hover:bg-red-500/20"
                        aria-label={t("tokens.revoke")}
                      >
                        <Trash2 size={14} />
                      </button>
                    </Glass>
                  </li>
                ))}
              </ul>
            ) : (
              <Glass elevation={1} className="p-6 text-center text-sm text-[color:var(--fg-muted)]">
                {t("tokens.empty")}
              </Glass>
            )}
          </div>
        </motion.div>
      </Shell>
    </>
  );
}

function NewTokenForm({
  onIssued,
  issuing,
  mutate,
}: {
  onIssued: (t: TokenWithPlain) => void;
  issuing: boolean;
  mutate: (input: { name: string; scopes: string[]; ttl_hours: number }) => Promise<TokenWithPlain>;
}) {
  const { t } = useTranslation();
  const [name, setName] = useState("");
  const [scopes, setScopes] = useState("repo:read,repo:write");
  const [ttl, setTtl] = useState("0");

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    const issued = await mutate({
      name: name.trim(),
      scopes: scopes
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean),
      ttl_hours: Number(ttl) || 0,
    });
    setName("");
    onIssued(issued);
  };

  return (
    <Glass elevation={2} className="p-5">
      <h2 className="mb-3 text-lg font-semibold tracking-tight">{t("tokens.newTitle")}</h2>
      <form onSubmit={onSubmit} className="grid grid-cols-1 gap-3 md:grid-cols-4">
        <input
          required
          placeholder={t("tokens.namePlaceholder")}
          value={name}
          onChange={(e) => setName(e.target.value)}
          className="rounded-xl border border-[color:var(--border-glass)] bg-[color:var(--bg-elev-1)] px-3 py-2 text-sm focus:outline-none"
        />
        <input
          placeholder={t("tokens.scopesPlaceholder")}
          value={scopes}
          onChange={(e) => setScopes(e.target.value)}
          className="rounded-xl border border-[color:var(--border-glass)] bg-[color:var(--bg-elev-1)] px-3 py-2 font-mono text-xs focus:outline-none"
        />
        <input
          type="number"
          min={0}
          placeholder={t("tokens.ttlPlaceholder")}
          value={ttl}
          onChange={(e) => setTtl(e.target.value)}
          className="rounded-xl border border-[color:var(--border-glass)] bg-[color:var(--bg-elev-1)] px-3 py-2 text-sm focus:outline-none"
        />
        <button
          type="submit"
          disabled={issuing}
          className="flex items-center justify-center gap-2 rounded-full bg-[color:var(--accent)] px-4 py-2 text-sm font-medium text-[color:var(--accent-fg)] hover:scale-[1.01] disabled:opacity-60"
        >
          <Plus size={14} />
          {issuing ? t("tokens.issuing") : t("tokens.issue")}
        </button>
      </form>
    </Glass>
  );
}
