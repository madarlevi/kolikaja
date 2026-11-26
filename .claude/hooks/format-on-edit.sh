#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only
# Copyright (C) 2026 madarlevi
# PostToolUse hook: format the file that was just edited, if the formatter exists.
# Requires jq to read the hook payload. If jq is missing, the hook does nothing.
set -uo pipefail

command -v jq >/dev/null 2>&1 || exit 0

payload="$(cat)"
file="$(printf '%s' "$payload" | jq -r '.tool_input.file_path // empty')"
[ -n "$file" ] && [ -f "$file" ] || exit 0

case "$file" in
  *.go)
    command -v gofmt >/dev/null 2>&1 && gofmt -w "$file" ;;
  *.ts|*.tsx|*.js|*.jsx|*.json|*.css)
    if [ -x "$CLAUDE_PROJECT_DIR/apps/web/node_modules/.bin/prettier" ]; then
      (cd "$CLAUDE_PROJECT_DIR/apps/web" && ./node_modules/.bin/prettier --write "$file" >/dev/null 2>&1)
    fi ;;
  *.kt|*.kts)
    : ;; # ktlint format is run by the Gradle task, not per file
  *.swift)
    command -v swift-format >/dev/null 2>&1 && swift-format format -i "$file" ;;
esac
exit 0
