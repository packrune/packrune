#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
#
# scripts/check-headers.sh — verify every source file carries the SPDX header.
#
# Why: AGPL + Commons Clause requires the license notice to be preserved with
# the source. We enforce it in CI so we never ship a file that loses the
# notice in a refactor.
#
# Usage: scripts/check-headers.sh
# Exit:  0 if every tracked source file has the header; 1 otherwise.

set -euo pipefail

EXPECTED="SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause"

# File extensions we enforce on. Keep this list explicit — no surprise
# coverage.
EXTS=(go ts tsx js jsx mjs cjs css scss sh sql)

# Paths we never check (third-party, generated, configuration).
IGNORE_REGEX='^(LICENSE|NOTICE|COMMONS-CLAUSE|CLA\.md|CODE_OF_CONDUCT\.md|README\.md|CONTRIBUTING\.md|PHASES\.md|\.gitignore|\.editorconfig|go\.mod|go\.sum|Makefile|.*\.md|web/node_modules/.*|web/dist/.*|.*\.lock|.*\.json|.*\.yml|.*\.yaml|.*\.toml|docs/.*)$'

missing=()

# Use git to find tracked files; if outside git (CI tarball), fall back to find.
# Use a portable read loop instead of `mapfile` so the script works on bash 3.2
# (the default on macOS) as well as bash 4+.
list_files() {
  if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git ls-files
  else
    find . -type f \
      -not -path './.git/*' \
      -not -path './web/node_modules/*' \
      -not -path './web/dist/*' \
      | sed 's|^\./||'
  fi
}

while IFS= read -r f; do
  [ -z "$f" ] && continue
  # Skip ignored paths
  if [[ "$f" =~ $IGNORE_REGEX ]]; then
    continue
  fi

  # Only check extensions we care about
  ext="${f##*.}"
  found=0
  for want in "${EXTS[@]}"; do
    if [[ "$ext" == "$want" ]]; then
      found=1
      break
    fi
  done
  [[ "$found" -eq 1 ]] || continue

  # Look in first 20 lines (shebangs / package decls may push it down)
  if ! head -n 20 "$f" | grep -qF "$EXPECTED"; then
    missing+=("$f")
  fi
done < <(list_files)

if [ ${#missing[@]} -gt 0 ]; then
  echo "error: files missing SPDX header (\"$EXPECTED\"):" >&2
  printf '  %s\n' "${missing[@]}" >&2
  echo >&2
  echo "Add the header as a comment near the top of each file." >&2
  exit 1
fi

echo "ok: SPDX header present in all checked source files."
