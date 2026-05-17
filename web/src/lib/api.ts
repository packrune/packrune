// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

/**
 * Typed JSON API client. Errors map to ApiError; auth state is kept by
 * cookies so most calls just need to provide the path and body.
 */

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

export interface User {
  id: string;
  email: string;
  username: string;
  display_name: string;
  is_admin: boolean;
  is_active: boolean;
  created_at: string;
}

export interface Repository {
  id: string;
  name: string;
  format: string;
  kind: string;
  created_at: string;
  updated_at: string;
  artifact_count: number;
}

export interface SystemStats {
  repository_count: number;
  artifact_count: number;
  per_format: Record<string, number>;
}

export interface VersionInfo {
  version: string;
  commit: string;
  date: string;
  go: string;
}

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.status = status;
    this.name = "ApiError";
  }
}

async function call<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: {
      Accept: "application/json",
      "Content-Type": init.body ? "application/json" : "application/json",
      ...(init.headers ?? {}),
    },
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
    throw new ApiError(msg, res.status);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const api = {
  version: () => call<VersionInfo>("/api/system/version"),
  stats: () => call<SystemStats>("/api/system/stats"),
  me: () => call<User>("/api/me"),
  login: (username: string, password: string) =>
    call<User>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),
  logout: () => call<void>("/api/auth/logout", { method: "POST" }),
  repositories: () => call<{ items: Repository[] }>("/api/repositories"),
  formats: () => call<{ items: { name: string; display_name: string }[] }>("/api/formats"),
};

// React Query hooks ---------------------------------------------------------

export function useMe() {
  return useQuery({
    queryKey: ["me"],
    queryFn: api.me,
    retry: false,
    staleTime: 60_000,
  });
}

export function useRepositories() {
  return useQuery({
    queryKey: ["repositories"],
    queryFn: api.repositories,
  });
}

export function useStats() {
  return useQuery({
    queryKey: ["stats"],
    queryFn: api.stats,
    refetchInterval: 30_000,
  });
}

export function useVersion() {
  return useQuery({
    queryKey: ["version"],
    queryFn: api.version,
    staleTime: Number.POSITIVE_INFINITY,
  });
}

export function useFormats() {
  return useQuery({
    queryKey: ["formats"],
    queryFn: api.formats,
    staleTime: Number.POSITIVE_INFINITY,
  });
}

export function useLogin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ username, password }: { username: string; password: string }) =>
      api.login(username, password),
    onSuccess: (user) => {
      qc.setQueryData(["me"], user);
    },
  });
}

export function useLogout() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: api.logout,
    onSettled: () => {
      qc.removeQueries({ queryKey: ["me"] });
      qc.removeQueries({ queryKey: ["repositories"] });
      qc.removeQueries({ queryKey: ["stats"] });
    },
  });
}

// Back-compat with the earlier scaffold's named helper.
export const getVersion = api.version;
