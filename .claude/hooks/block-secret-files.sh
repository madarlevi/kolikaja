#!/usr/bin/env bash
# PreToolUse hook: refuse writes to secret-bearing files.
# Exit code 2 blocks the tool call and feeds the message back to the agent.
set -uo pipefail

command -v jq >/dev/null 2>&1 || exit 0

file="$(jq -r '.tool_input.file_path // empty')"
[ -n "$file" ] || exit 0

base="$(basename "$file")"
case "$base" in
  .env|.env.local|.env.production|.env.*.local)
    echo "Blocked: $base may contain secrets. Write placeholders to .env.example instead." >&2
    exit 2 ;;
  *.pem|*.key|*.p12|*.pfx|*.jks|*.keystore|credentials.json|*.mobileprovision)
    echo "Blocked: $base looks like key or credential material. Never commit it." >&2
    exit 2 ;;
esac
exit 0
