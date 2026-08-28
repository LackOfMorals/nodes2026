#!/bin/zsh
# scripts/switch-mcp-profile.sh — swap Claude Desktop's MCP servers between
# the NODES 2026 talk profiles (mcp-profiles/).
#
# Usage:
#   scripts/switch-mcp-profile.sh federated       # Config A: single router MCP server
#   scripts/switch-mcp-profile.sh no-federation   # Config B: Linear + Slack + Neo4j
#   scripts/switch-mcp-profile.sh restore         # back to the pre-talk mcpServers
#   scripts/switch-mcp-profile.sh show [profile]  # print rendered JSON; changes nothing
#
# Safety:
#   - The FIRST switch creates a one-time backup of the whole live config:
#         ~/Library/Application Support/Claude/claude_desktop_config.json.nodes2026.bak
#   - Only the "mcpServers" key is replaced — every other setting
#     (coworkUserFilesPath, …) is preserved. `restore` puts back the
#     pre-talk mcpServers from the backup.
#   - Credentials are read from the repo .env at switch time; the profile
#     files in git contain only __PLACEHOLDERS__.
#   - RESTART CLAUDE DESKTOP after every switch.

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)
CLAUDE_DIR="$HOME/Library/Application Support/Claude"
CONFIG="$CLAUDE_DIR/claude_desktop_config.json"
BACKUP="$CLAUDE_DIR/claude_desktop_config.json.nodes2026.bak"

cmd=${1:-}
case "$cmd" in
  federated|no-federation|restore|show) ;;
  *) echo "usage: $0 federated|no-federation|restore|show [profile]" >&2; exit 1 ;;
esac

# env_val <KEY> — read KEY from .env (if present), stripping one layer of
# surrounding quotes. Prints "" (not an error) when .env or the key is
# missing — commands that don't need secrets (restore, show federated)
# must not fail just because a key isn't set; render() below is what
# decides whether a missing value actually matters, and says so clearly.
env_val() {
  local line=""
  [[ -f "$REPO_ROOT/.env" ]] && line=$(grep -E "^$1=" "$REPO_ROOT/.env" | head -1 || true)
  local value=${line#*=}
  value=${value%\"}; value=${value#\"}
  value=${value%\'}; value=${value#\'}
  printf '%s' "$value"
}

# render <federated|no-federation> → stdout. Substitutes each __KEY__
# placeholder with its .env value via Node's literal string replace, not
# sed — secrets can contain sed metacharacters (&, \, the delimiter) that
# would otherwise corrupt the output or crash the substitution. Only
# placeholders actually present in the profile file are required; a
# missing value fails with a clear message naming the key, before
# anything is written.
render() {
  local f="$REPO_ROOT/mcp-profiles/$1.json"
  [[ -f "$f" ]] || { echo "no such profile: $1" >&2; exit 1; }
  LINEAR_API_KEY="$(env_val LINEAR_API_KEY)" \
  SLACK_BOT_TOKEN="$(env_val SLACK_BOT_TOKEN)" \
  NEO4J_PASSWORD="$(env_val NEO4J_PASSWORD)" \
  node -e '
    const fs = require("fs");
    const path = process.argv[1];
    let text = fs.readFileSync(path, "utf8");
    const KEYS = ["LINEAR_API_KEY", "SLACK_BOT_TOKEN", "NEO4J_PASSWORD"];
    const missing = [];
    for (const key of KEYS) {
      const placeholder = `__${key}__`;
      if (!text.includes(placeholder)) continue;
      const value = process.env[key];
      if (!value) { missing.push(key); continue; }
      text = text.split(placeholder).join(value);
    }
    if (missing.length) {
      console.error(`render: missing .env value(s) for: ${missing.join(", ")}`);
      process.exit(1);
    }
    process.stdout.write(text);
  ' "$f"
}

# patch_mcp_servers <configPath> <sourcePath> — replace configPath's
# "mcpServers" key with sourcePath's; every other top-level key in
# configPath is left untouched. Shared by both restore and install so
# there's one JSON-patch implementation instead of two that can drift.
patch_mcp_servers() {
  node -e '
    const fs = require("fs");
    const [configPath, sourcePath] = process.argv.slice(1);
    const live = JSON.parse(fs.readFileSync(configPath, "utf8"));
    const source = JSON.parse(fs.readFileSync(sourcePath, "utf8"));
    live.mcpServers = source.mcpServers ?? {};
    fs.writeFileSync(configPath, JSON.stringify(live, null, 2) + "\n");
  ' "$1" "$2"
}

if [[ "$cmd" == "show" ]]; then
  render "${2:-federated}"
  exit 0
fi

[[ -f "$CONFIG" ]] || { echo "live config not found: $CONFIG" >&2; exit 1; }

if [[ "$cmd" == "restore" ]]; then
  [[ -f "$BACKUP" ]] || { echo "no backup at $BACKUP — nothing to restore" >&2; exit 1; }
  patch_mcp_servers "$CONFIG" "$BACKUP"
  echo "Restored pre-talk mcpServers from $BACKUP."
  echo "Restart Claude Desktop."
  exit 0
fi

# cmd is federated or no-federation
if [[ ! -f "$BACKUP" ]]; then
  cp "$CONFIG" "$BACKUP"
  echo "Backed up live config → $BACKUP"
fi

RENDERED=$(mktemp)
trap 'rm -f "$RENDERED"' EXIT
render "$cmd" > "$RENDERED"

patch_mcp_servers "$CONFIG" "$RENDERED"

echo "Installed '$cmd' profile — mcpServers replaced, other settings preserved."
echo "Restart Claude Desktop for it to take effect."
