// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

/**
 * Tiny fetch wrapper for the Packrune JSON API. Real query hooks land in
 * Faz 7 alongside the pages; this file exists so types and helpers are in
 * place from day one.
 */

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

export async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: {
      Accept: "application/json",
      ...(init?.headers ?? {}),
    },
    credentials: "same-origin",
  });
  if (!res.ok) {
    let body = "";
    try {
      body = await res.text();
    } catch {
      // ignore
    }
    throw new ApiError(body || res.statusText, res.status);
  }
  return (await res.json()) as T;
}

export async function getVersion(): Promise<VersionInfo> {
  return fetchJSON<VersionInfo>("/version");
}
