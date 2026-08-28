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

[[ -f "$REPO_ROOT/.env" ]] || { echo "missing $REPO_ROOT/.env" >&2; exit 1; }
env_val() { grep -E "^$1=" "$REPO_ROOT/.env" | head -1 | cut -d= -f2-; }
LINEAR_API_KEY=$(env_val LINEAR_API_KEY)
SLACK_BOT_TOKEN=$(env_val SLACK_BOT_TOKEN)
NEO4J_PASSWORD=$(env_val NEO4J_PASSWORD)

render() { # render <federated|no-federation> → stdout
  local f="$REPO_ROOT/mcp-profiles/$1.json"
  sed -e "s|__LINEAR_API_KEY__|$LINEAR_API_KEY|g" \
      -e "s|__SLACK_BOT_TOKEN__|$SLACK_BOT_TOKEN|g" \
      -e "s|__NEO4J_PASSWORD__|$NEO4J_PASSWORD|g" "$f"
}

if [[ "$cmd" == "show" ]]; then
  render "${2:-federated}"
  exit 0
fi

[[ -f "$CONFIG" ]] || { echo "live config not found: $CONFIG" >&2; exit 1; }

if [[ "$cmd" == "restore" ]]; then
  [[ -f "$BACKUP" ]] || { echo "no backup at $BACKUP — nothing to restore" >&2; exit 1; }
  node -e '
    const fs = require("fs");
    const [configPath, backupPath] = process.argv.slice(1);
    const live = JSON.parse(fs.readFileSync(configPath, "utf8"));
    const backup = JSON.parse(fs.readFileSync(backupPath, "utf8"));
    live.mcpServers = backup.mcpServers ?? {};
    fs.writeFileSync(configPath, JSON.stringify(live, null, 2) + "\n");
  ' "$CONFIG" "$BACKUP"
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

node -e '
  const fs = require("fs");
  const [configPath, profilePath] = process.argv.slice(1);
  const live = JSON.parse(fs.readFileSync(configPath, "utf8"));
  const profile = JSON.parse(fs.readFileSync(profilePath, "utf8"));
  live.mcpServers = profile.mcpServers;
  fs.writeFileSync(configPath, JSON.stringify(live, null, 2) + "\n");
' "$CONFIG" "$RENDERED"

echo "Installed '$cmd' profile — mcpServers replaced, other settings preserved."
echo "Restart Claude Desktop for it to take effect."
