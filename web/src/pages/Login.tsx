// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import { motion } from "motion/react";
import { ArrowRight, Lock, User as UserIcon } from "lucide-react";

import { AuroraBackground } from "../components/AuroraBackground";
import { Glass } from "../components/Glass";
import { useLogin } from "../lib/api";

export function Login() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const login = useLogin();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    try {
      await login.mutateAsync({ username, password });
      navigate({ to: "/dashboard" });
    } catch {
      // The mutation surfaces the error via login.error below; nothing else to do.
    }
  };

  return (
    <main className="relative grid min-h-screen place-items-center px-6">
      <AuroraBackground />
      <motion.div
        initial={{ opacity: 0, y: 16 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5, ease: [0.32, 0.72, 0, 1] }}
        className="w-full max-w-md"
      >
        <Glass elevation={2} className="p-8">
          <h1 className="text-2xl font-semibold tracking-tight">{t("app.name")}</h1>
          <p className="mt-1 text-sm text-[color:var(--fg-muted)]">{t("login.subtitle")}</p>

          <form className="mt-6 flex flex-col gap-3" onSubmit={onSubmit}>
            <label className="flex flex-col gap-1">
              <span className="text-xs uppercase tracking-wider text-[color:var(--fg-subtle)]">
                {t("login.username")}
              </span>
              <div className="flex items-center gap-2 rounded-xl border border-[color:var(--border-glass)] bg-[color:var(--bg-elev-1)] px-3 py-2">
                <UserIcon size={14} className="text-[color:var(--fg-muted)]" />
                <input
                  autoFocus
                  required
                  autoComplete="username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="flex-1 bg-transparent text-sm focus:outline-none"
                />
              </div>
            </label>

            <label className="flex flex-col gap-1">
              <span className="text-xs uppercase tracking-wider text-[color:var(--fg-subtle)]">
                {t("login.password")}
              </span>
              <div className="flex items-center gap-2 rounded-xl border border-[color:var(--border-glass)] bg-[color:var(--bg-elev-1)] px-3 py-2">
                <Lock size={14} className="text-[color:var(--fg-muted)]" />
                <input
                  type="password"
                  required
                  autoComplete="current-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="flex-1 bg-transparent text-sm focus:outline-none"
                />
              </div>
            </label>

            {login.isError && (
              <div className="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-xs text-red-200">
                {(login.error as Error).message || t("login.failed")}
              </div>
            )}

            <button
              type="submit"
              disabled={login.isPending}
              className="mt-2 flex items-center justify-center gap-2 rounded-full bg-[color:var(--accent)] px-4 py-2.5 text-sm font-medium text-[color:var(--accent-fg)] transition-transform hover:scale-[1.01] disabled:opacity-60"
            >
              {login.isPending ? t("login.signingIn") : t("actions.signIn")}
              <ArrowRight size={16} />
            </button>
          </form>
        </Glass>
        <p className="mt-4 text-center text-xs text-[color:var(--fg-subtle)]">
          {t("login.helpHint")}{" "}
          <code className="rounded bg-[color:var(--bg-elev-2)] px-1.5 py-0.5">
            packrune users add ...
          </code>
        </p>
      </motion.div>
    </main>
  );
}
