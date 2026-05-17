// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import { AuroraBackground } from "./components/AuroraBackground";
import { ThemeProvider } from "./themes/ThemeProvider";
import { Landing } from "./pages/Landing";
import "./i18n";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      gcTime: 5 * 60_000,
      refetchOnWindowFocus: false,
    },
  },
});

/**
 * App root. Once Faz 7 lands and we have multiple routes, this becomes the
 * TanStack Router host; today it renders the Landing page directly so the
 * scaffold can be validated visually.
 */
export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <AuroraBackground />
        <Landing />
      </ThemeProvider>
    </QueryClientProvider>
  );
}
