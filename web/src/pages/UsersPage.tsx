// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { motion } from "motion/react";
import { Crown, Plus, Trash2, User as UserIcon } from "lucide-react";

import { AuroraBackground } from "../components/AuroraBackground";
import { Glass } from "../components/Glass";
import { Shell } from "../components/Shell";
import { useMe } from "../lib/api";

interface AdminUser {
  id: string;
  email: string;
  username: string;
  display_name: string;
  is_admin: boolean;
  is_active: boolean;
  created_at: string;
}

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
    } catch {
      // ignore
    }
    throw new Error(msg);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export function UsersPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const me = useMe();

  const users = useQuery({
    queryKey: ["users"],
    queryFn: () => callJSON<{ items: AdminUser[] }>("/api/users"),
    enabled: !!me.data?.is_admin,
  });

  const qc = useQueryClient();
  const create = useMutation({
    mutationFn: (input: { email: string; username: string; password: string; is_admin: boolean }) =>
      callJSON<AdminUser>("/api/users", { method: "POST", body: JSON.stringify(input) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["users"] }),
  });
  const deactivate = useMutation({
    mutationFn: (id: string) => callJSON<void>(`/api/users/${id}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["users"] }),
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
            {t("users.adminOnly")}
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
            <h1 className="text-3xl font-semibold tracking-tight">{t("users.title")}</h1>
            <p className="mt-1 text-sm text-[color:var(--fg-muted)]">{t("users.subtitle")}</p>
          </div>

          <NewUserForm
            onCreate={(input) => create.mutateAsync(input)}
            pending={create.isPending}
            error={(create.error as Error | null)?.message}
          />

          <div className="mt-6">
            {users.data && users.data.items && users.data.items.length > 0 ? (
              <Glass elevation={2} className="overflow-hidden">
                <ul className="divide-y divide-[color:var(--border-glass)]">
                  {users.data.items.map((u) => (
                    <li key={u.id} className="flex items-center gap-3 px-4 py-3">
                      <UserIcon size={16} className="text-[color:var(--fg-muted)]" />
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2 text-sm font-medium">
                          <span className="truncate">{u.username}</span>
                          {u.is_admin && (
                            <Crown size={12} className="text-[color:var(--accent)]" aria-label="admin" />
                          )}
                          {!u.is_active && (
                            <span className="rounded-full bg-red-500/20 px-2 py-0.5 text-[10px] text-red-200">
                              inactive
                            </span>
                          )}
                        </div>
                        <div className="truncate text-xs text-[color:var(--fg-muted)]">{u.email}</div>
                      </div>
                      <div className="text-xs text-[color:var(--fg-subtle)]">
                        {new Date(u.created_at).toLocaleDateString()}
                      </div>
                      {u.id !== me.data.id && u.is_active && (
                        <button
                          type="button"
                          onClick={() => deactivate.mutate(u.id)}
                          className="rounded-md border border-red-500/30 bg-red-500/10 p-2 text-red-200 hover:bg-red-500/20"
                          aria-label={t("users.deactivate")}
                          title={t("users.deactivate")}
                        >
                          <Trash2 size={14} />
                        </button>
                      )}
                    </li>
                  ))}
                </ul>
              </Glass>
            ) : (
              <Glass elevation={1} className="p-6 text-center text-sm text-[color:var(--fg-muted)]">
                {t("users.empty")}
              </Glass>
            )}
          </div>
        </motion.div>
      </Shell>
    </>
  );
}

function NewUserForm({
  onCreate,
  pending,
  error,
}: {
  onCreate: (input: { email: string; username: string; password: string; is_admin: boolean }) => Promise<AdminUser>;
  pending: boolean;
  error?: string;
}) {
  const { t } = useTranslation();
  const [email, setEmail] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [isAdmin, setIsAdmin] = useState(false);

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    await onCreate({ email, username, password, is_admin: isAdmin });
    setEmail("");
    setUsername("");
    setPassword("");
    setIsAdmin(false);
  };

  return (
    <Glass elevation={2} className="p-5">
      <h2 className="mb-3 text-lg font-semibold tracking-tight">{t("users.newTitle")}</h2>
      <form onSubmit={onSubmit} className="grid grid-cols-1 gap-3 md:grid-cols-5">
        <input
          required
          type="email"
          placeholder={t("users.emailPlaceholder")}
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          className="rounded-xl border border-[color:var(--border-glass)] bg-[color:var(--bg-elev-1)] px-3 py-2 text-sm focus:outline-none md:col-span-2"
        />
        <input
          required
          placeholder={t("users.usernamePlaceholder")}
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          className="rounded-xl border border-[color:var(--border-glass)] bg-[color:var(--bg-elev-1)] px-3 py-2 text-sm focus:outline-none"
        />
        <input
          required
          type="password"
          placeholder={t("users.passwordPlaceholder")}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          className="rounded-xl border border-[color:var(--border-glass)] bg-[color:var(--bg-elev-1)] px-3 py-2 text-sm focus:outline-none"
        />
        <label className="flex items-center gap-2 rounded-xl bg-[color:var(--bg-elev-1)] px-3 py-2 text-sm">
          <input type="checkbox" checked={isAdmin} onChange={(e) => setIsAdmin(e.target.checked)} />
          {t("users.adminLabel")}
        </label>
        <button
          type="submit"
          disabled={pending}
          className="flex items-center justify-center gap-2 rounded-full bg-[color:var(--accent)] px-4 py-2 text-sm font-medium text-[color:var(--accent-fg)] hover:scale-[1.01] disabled:opacity-60 md:col-span-5"
        >
          <Plus size={14} />
          {pending ? t("users.creating") : t("users.create")}
        </button>
        {error && (
          <div className="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-xs text-red-200 md:col-span-5">
            {error}
          </div>
        )}
      </form>
    </Glass>
  );
}
