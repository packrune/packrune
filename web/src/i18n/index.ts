// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

import i18n from "i18next";
import { initReactI18next } from "react-i18next";

import enCommon from "./en/common.json";
import trCommon from "./tr/common.json";

export const SUPPORTED_LANGUAGES = [
  { code: "en", label: "English" },
  { code: "tr", label: "Türkçe" },
] as const;

const STORAGE_KEY = "packrune.lang";

function detectInitialLang(): string {
  if (typeof window === "undefined") return "en";
  const stored = window.localStorage.getItem(STORAGE_KEY);
  if (stored && SUPPORTED_LANGUAGES.some((l) => l.code === stored)) return stored;
  const nav = window.navigator.language.slice(0, 2);
  if (SUPPORTED_LANGUAGES.some((l) => l.code === nav)) return nav;
  return "en";
}

void i18n.use(initReactI18next).init({
  resources: {
    en: { common: enCommon },
    tr: { common: trCommon },
  },
  lng: detectInitialLang(),
  fallbackLng: "en",
  defaultNS: "common",
  interpolation: { escapeValue: false },
  returnNull: false,
});

export function changeLanguage(code: string): void {
  if (!SUPPORTED_LANGUAGES.some((l) => l.code === code)) return;
  void i18n.changeLanguage(code);
  if (typeof window !== "undefined") window.localStorage.setItem(STORAGE_KEY, code);
}

export default i18n;
