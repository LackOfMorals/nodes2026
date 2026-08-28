# DEMO-ENV — local demo environment runbook

The exact command sequence to bring the demo environment up from cold,
reset it, and verify it — so venue dry runs and demo sessions are one
documented procedure. See `PLAN.md` for *what* we're building and `TALK.md`
for *why*; this file is only about *how to run it on the laptop*.

The demo is local-first: Neo4j and the Cosmo Router run on the laptop
(podman / `wgc`); only Linear and Slack are SaaS (see
[SaaS prerequisites](#saas-prerequisites--credentials) below).

## Neo4j (podman container)

The currently running container is `neo4j-prod-01`
(image `neo4j:2026.07.1-enterprise`), auth `neo4j` / `password`,
ports 7474 (web console + HTTP API) and 7687 (bolt).

To start a fresh one from scratch:

```bash
podman run -d --name nodes-demo-neo4j \
  -p 7687:7687 -p 7474:7474 \
  -e NEO4J_AUTH=neo4j/password \
  neo4j:2026.07.1-enterprise

podman start nodes-demo-neo4j        # later sessions
```

Notes:

- 2026.x is the version this data model is written against. There is no
  cypher-shell installed; the web console at
  <http://localhost:7474/browser> is the documented demo-data path.
- Nothing in the data model needs APOC on 2026.x — `link-explicit.cypher`
  is pure Cypher (the `apoc.text` family is gone from the product).

## Loading schema + demo data (web console)

1. Open <http://localhost:7474/browser>, sign in `neo4j` / `password`.
2. Run `data-model/schema.cypher` (16 statements).
3. Run `data-model/sample-data.cypher` (21 statements) — this is the
   canonical live-demo graph.
4. Run `data-model/queries/link-explicit.cypher` — expect **5** distinct
   rows back (one per `:DISCUSSED_IN` edge).

Every statement in these files is **self-contained** (each re-`MERGE`s its
nodes by key; one file = one statement is the contract the sync writer
enforces too). You can paste statements one at a time in the console if a
large paste is awkward — order within a file doesn't matter for
`sample-data.cypher`, but run schema before data.

### Verify

Run `data-model/queries/agent-context.cypher` with `identifier = "NODES-1"`.
Expected: issue + creator/assignee (Sarah Chen) + **exactly 2 discussions**
(threads 1 and 2) with all messages populated and May 2026 timestamps.
Thread 4 (`ts = 1778659200.000100`) must **not** appear — it's the
deliberately orphaned thread for iteration 2.

Sanity counts (optional):

```cypher
MATCH (n) RETURN labels(n) AS labels, count(*) AS n;
// 1 Project, 6 Issue, 1 Channel, 4 Thread, 12 Message, 6 Person
MATCH ()-[r]->() RETURN type(r) AS rel, count(*) AS n;
// HAS_ISSUE 6, HOSTS_THREAD 4, HAS_MESSAGE 12, CREATED 6,
// DISCUSSED_IN 5, ASSIGNED_TO (n), AUTHORED 12
```

## Reset to canonical state

The canonical demo state is the seeded graph above — nothing else. Reset
any time (stale state, e2e-test residue, experiments):

```cypher
MATCH (n) DETACH DELETE n;
```

…then re-run the four files in the load order above (schema →
sample-data → link-explicit). Total time: under a minute.

The e2e writer test (below) leaves synthetic rows behind (issue NODES-7,
project `proj_sync_test`, person kim@example.com, thread
`1778745600.000100`). Reset the same way if you don't want them.

## Sync writer e2e test (regression)

Proves `upsert-issue.cypher`, `upsert-thread.cypher`, and
`link-explicit.cypher` still execute under the one-file-one-statement
contract against the live local database:

```bash
cd sync
WRITER_E2E=1 go test ./internal/graph/ -run TestUpsertWrites_EndToEnd -v
# optional overrides:
#   WRITER_E2E_URI=bolt://localhost:7687
#   WRITER_E2E_QUERIES=../data-model/queries
```

Skipped (no-op) unless `WRITER_E2E=1`, so plain `go test ./...` is safe.
Remember it **writes rows** — reset afterwards if you want the canonical
graph back.

## Credentials (.env convention)

Values stay in a local env file, never committed. Example
(`.env.example` shape for `sync/`):

```bash
# Linear — personal API key, Settings → API (lin_api_...)
LINEAR_API_KEY=
# Slack — bot user OAuth token (xoxb-...)
SLACK_BOT_TOKEN=
# Neo4j — local container, not Aura
NEO4J_URI=bolt://localhost:7687
NEO4J_USER=neo4j
NEO4J_PASSWORD=
# optional
NEO4J_DATABASE=neo4j
QUERIES_DIR=../data-model/queries
```

## SaaS prerequisites & credentials

Everything the demo needs from the two SaaS systems, in one place. Both
use **j.giffard@icloud.com** on purpose — Person-unification-by-email is
part of the demo story, and using the same real email proves it.

### Linear

What's needed for the demo (Phase 2):

- Free workspace → create a **team** → create a **project**
  "NODES 2026 Demo".
- **Done (2026-08-27, via API):** team `NEO`, project "NODES 2026 Demo"
  (UUID `1b3763ee-c856-4727-bc80-bf1ae5d0794b`), six issues
  **NEO-10..NEO-15** mirroring the demo story — NEO-10 is the NODES-1
  equivalent (In Progress, assigned to j.giffard); NEO-12 Todo; NEO-14
  Done.
- **The real identifier prefix is `NEO`** — the workspace counter was
  already at 9 (Linear never reuses numbers), so the story issues are
  NEO-10..15, not NEO-1..6. Slack messages must name the *real*
  identifiers for the explicit linking pass to find them: **NEO-10**
  (threads t1+t2), **NEO-12** (t3), **NEO-14** + **NEO-15** (t2); the
  orphan thread names none. Demo prompts quote the same identifiers.
- **Access: a personal API key.** Settings → API → Personal API Keys →
  Create. Key looks like `lin_api_...`. This is the `LINEAR_API_KEY`
  value. The sync pipeline's only Linear credential.
- **In this agent environment a Linear MCP server is connected** with
  the same key (verified 2026-08-27: sees the workspace, project, and
  issue states). Prefer its tools (`get_project`, `get_issue`,
  `save_issue`, `list_teams`, …) for Linear work over hand-rolled
  GraphQL scripts. The sync pipeline itself still uses the raw API key
  — MCP is a dev-time convenience, not part of the demo.
- **Also note the project UUID** (Settings on the project, or from the
  list-projects API) — it's the `--linear-project` CLI value.
- Verify once: `curl` (or any GraphQL client) one `projects` query at
  `https://api.linear.app/graphql` with the key **raw** in the
  `Authorization` header — Linear **rejects the `Bearer` prefix**
  ("Remove the Bearer prefix"): `Authorization: lin_api_...`, the key
  itself with no prefix. The sync client
  (`sync/internal/linear/client.go`) already sends it this way.
  **Never paste the key into docs** — it lives only in `.env`.
- **Profile name quirk (cosmetic):** Linear's profile name defaults to
  the account email, and the API returns it as `name` — so a synced
  `:Person` gets `name = j.giffard@icloud.com` unless a display name is
  set (Linear → Settings → Profile). `upsert-issue.cypher` keeps the
  first name seen (`coalesce(name, …)`), so set the profile name
  *before* the first sync into that DB, or wipe + re-sync afterwards.


### Slack

What's needed for the demo (Phases 2 + 3):

- Free, owned (personal) workspace → create channel
  `#nodes-demo-eng`.
- Create an app at <https://api.slack.com/apps> (from scratch).
- **All seven bot scopes** (Oauth Token Scopes):
  - `channels:read`, `channels:history` — public channel reads
  - `groups:read`, `groups:history` — private channel reads
  - `users:read` — resolve member IDs to names
  - `users:read.email` — **required**: Person unification joins on email
  - `search:read` — search-based reads
- Install to the workspace, then **invite the bot to `#nodes-demo-eng`**
  (`/invite @your-app`). A bot not in the channel can't read it.
- Seed 3–4 threads (Phase 2.2): at least one naming an issue identifier
  explicitly, one discussing the same topic *without* naming it (the
  orphan, for iteration 2).
- **Access: the bot user OAuth token** (`xoxb-...`), shown after
  installation. This is the `SLACK_BOT_TOKEN` value, shared by both the
  sync pipeline and the slack-subgraph plugin.
- **Also note the channel ID** (`C...` — from the channel's ⋯ → About,
  or the `auth.test`/conversations API) — it's the `--slack-channel`
  CLI value.
- Verify once: `curl -H "Authorization: Bearer xoxb-..." https://slack.com/api/auth.test`
  → `{"ok":true,...}`.
- **Done (2026-08-27):** workspace "Nodes 2026 Demo"
  (nodes2026demo.slack.com), `#nodes-demo-eng` = **`C0BSX7Q9M0E`**, bot
  `nodes2026demo` is a member, token + channel ID in `.env`. Verified
  against every endpoint the sync client calls: `conversations.info` ✓,
  `conversations.history` ✓, `users.info` ✓ (returns **email** → Person
  unification by email works through the client's exact path).
  JG's Slack user ID: **`U0BSZ59TY66`** (owner). `groups:*`/
  `search:read` are not used by the current client.
- **Seeding — done (2026-08-27), seed the threads as the human user in
  the Slack UI, not via the bot.** Messages must be authored by
  j.giffard@icloud.com for the Person node to unify with the Linear
  side; a bot-authored thread would carry the bot's user ID (no email)
  and never unify. Four threads posted by JG, all authored by
  j.giffard@icloud.com:
  - t1 `1787845145.093149` — federation/NEO-10 nullability; names
    **NEO-10** (3 replies)
  - t2 `1787845177.141579` — pre-mortem; names **NEO-10, NEO-14,
    NEO-15** (2 replies)
  - t3 `1787845201.547149` — token refresh; names **NEO-12** (1 reply)
  - t4 `1787845220.346049` — schema onboarding; names **nothing** (the
    orphan for iteration 2; 2 replies)
  Total 12 messages. The channel also carries 3 system events
  (channel join/rename) — the sync client skips any message with a
  non-empty `subtype`.

## Real-data sync run (Phase 2 recipe)

The canonical demo state is the seeded graph. This recipe re-verifies
the pipeline against the real workspaces (or powers a live-sync bit).
Order matters: real data goes into a wiped DB, then restore the
canonical graph (see [Reset to canonical state](#reset-to-canonical-state)).

```bash
# from the repo root
set -a; . ./.env; set +a
cd sync
go build -o bin/sync ./cmd/sync
./bin/sync --linear-project "$LINEAR_PROJECT_ID" --slack-channel "$SLACK_CHANNEL_ID"
```

Before running, wipe the DB: `MATCH (n) DETACH DELETE n;`

Expected log (2026-08-27 actual run): `pulled 6 issues` → six
`upserted NEO-1x — …` lines → `channel: #nodes-demo-eng …` →
`pulled 4 threads` → four `upserted thread … (N messages)` lines
(4+3+2+3 = 12) → `explicit linking done` → `sync complete`.

Verify:

```cypher
// 6 issues with real states
MATCH (i:Issue) RETURN i.identifier AS id, i.state AS state ORDER BY id;
// NEO-10 In Progress, NEO-11/13/15 Backlog, NEO-12 Todo, NEO-14 Done

// exactly 5 explicit links, confidence 1.0
MATCH (i:Issue)-[d:DISCUSSED_IN]->(t:Thread)
RETURN i.identifier AS issue, t.ts AS thread ORDER BY issue, thread;
// NEO-10 → 1787845145.093149 (t1) and 1787845177.141579 (t2)
// NEO-12 → 1787845201.547149 (t3)
// NEO-14, NEO-15 → 1787845177.141579 (t2)
// thread 1787845220.346049 (t4) must NOT appear — the orphan
```

- `agent-context.cypher` with `identifier=NEO-10` → exactly 2
  discussions (t1, t2), all messages with author name/email +
  permalinks, channel `nodes-demo-eng`, real `linearUrl`/dates.
- One `:Person` node with both `linearId` (`9765f250-eb4f-476d-a27b-`
  `d0ed5a90597f`) and `slackId` (`U0BSZ59TY66`) — email unification.
- Thread message counts: t1 = 4, t2 = 3, t3 = 2, t4 = 3.

Then restore the canonical graph (wipe → schema → sample-data →
link-explicit) and re-verify `agent-context NODES-1`.

Notes:

- Re-runs are idempotent (upserts), but the pipeline **never
  tombstones**: if a Slack message is deleted and re-posted, the old
  copy stays in the graph. **Edit** the message in Slack instead.
- If Linear issue states change, a re-run updates `state`/`updatedAt`
  in place — but a real-data run should still start from a wipe so the
  graph is exactly "what the sources look like now".

## Router (Phase 4 — three subgraphs + MCP gateway)

The local Cosmo Router project lives in `router/` (router 0.343.1,
wgc 0.130.1). Three subgraphs are registered in `router/graph.yaml`
(rendered from the committed `graph.yaml.template` by `make render-graph`):

1. **Slack** — Connect plugin (`router/plugins/slack`), real workspace,
   `SLACK_BOT_TOKEN` from the environment. Four queries: `slackChannel`,
   `slackMessages`, `slackThread`, `slackUser`. (No search query: Slack's
   `search.messages` only accepts user tokens — Known issue 4 in
   `PLAN.md`; the graph's `searchMessages` is the demo's only search
   surface.)
2. **Linear** — `https://api.linear.app/graphql` with
   `introspection: {raw: true}` (treat the introspection response as
   plain schema; no federation unwrap). Auth two ways, both from
   `LINEAR_API_KEY`: inlined into the gitignored `graph.yaml` at compose
   time (introspection), and at runtime via `${LINEAR_API_KEY}` env
   expansion in `config.yaml` → `headers.subgraphs.linear.request`
   (verified working in 0.343.1 — the key appears nowhere in the
   composed `config.json`).
3. **Neo4j** — the local `neo4jGraphQLSrv/` node server
   (`@neo4j/graphql` 7.6.2 + Yoga, `node index.cjs`) on
   `127.0.0.1:4000/graphql`. The hand-written typeDefs in
   `neo4jGraphQLSrv/schema.graphql` are the single source of truth;
   `@node(labels:)` maps them to the graph labels — the demo types are
   named `GraphIssue`/`GraphProject` because raw `Issue`/`Project`
   names collide with Linear's types in the composed supergraph. Two
   `@cypher` fields cover what the generated schema can't express:
   `GraphIssue.discussionDetails` (threadTs/permalink + the
   `DISCUSSED_IN` confidence/evidence — load-bearing talk content) and
   root `searchMessagesCI` (case-insensitive message search; Neo4j
   2026.x `CONTAINS` is case-sensitive). The router registers a
   **generated plain-SDL file** (`schema: {file:
   ../neo4jGraphQLSrv/neo4j-plain-schema.graphql}`) — a gitignored
   build artifact produced by `make gen-neo4j-schema` (plain SDL from
   the library's schema via `printSchema`, which strips the `@cypher`
   directives). The old Go service `neo4j-api/` (:4400) is kept as an
   unwired fallback.

### Start (two terminals, from the repo root)

```bash
# Terminal 1 — Neo4j GraphQL library server (:4000), foreground
(cd router && make neo4j-srv)

# Terminal 2 — router (:3010) + MCP gateway (:5025), foreground
(cd router && make start)
```

`make start` = `download` (router binary, skipped if present) + `build`
(Slack plugin) + `compose` (render graph.yaml + `wgc router compose`) +
run. It sources `../.env` itself, so the only env requirement is that
the repo-root `.env` exists with the values in the
[Credentials](#credentials--env-convention) section. Fresh checkout
only: the neo4j subgraph server needs `cd neo4jGraphQLSrv && npm
install` once (node 26.5.0; installed deps @neo4j/graphql 7.6.2,
graphql-yoga 5.22.0, neo4j-driver 5.28.3 — see
`neo4jGraphQLSrv/package.json`).

### Ports

| Port | What |
|------|------|
| `http://localhost:3010/graphql` | Federated GraphQL endpoint (playground at `/`) |
| `http://localhost:5025/mcp` | MCP gateway (streamable HTTP; the agent-facing endpoint) |
| `http://127.0.0.1:4000/graphql` | Neo4j GraphQL library subgraph (reached through the router, not used directly) |
| `http://127.0.0.1:8088/metrics` | Router Prometheus metrics |

### MCP gateway tools

`config.yaml` enables the MCP server with
`mcp.storage.provider_id: mcp` → the `file_system` storage provider
(`path: operations`, relative to `router/`). One named operation per
`.graphql` file in `router/operations/`; each file's `"""description"""`
becomes the tool description and variable descriptions become
input-schema property descriptions. `enable_arbitrary_operations: false`
+ `expose_schema: false` mean the MCP surface exposes **only** the
persisted operations — an MCP client discovers the table below, not the
60+ raw federated fields.

| Tool | Subgraph | Arguments |
|------|----------|-----------|
| `get_issue_discussion_context` | graph | `identifier` (NODES-1 … NODES-6) |
| `search_messages` | graph | `query`, `limit` (default 20) |
| `linear_project_issues` | live Linear | none (demo project hardcoded in the operation) |
| `slack_thread` | live Slack | `channelId` (`C0BSX7Q9M0E`), `threadTs` (see the seeded list in the operation's description) |
| `get_operation_info` | built-in | `operationName` — how to HTTP-execute any operation outside MCP |

### Verify

```bash
# GraphQL side — one request touching all three subgraphs:
curl -s http://localhost:3010/graphql \
  -H 'Content-Type: application/json' \
  -d '{"query":"{ projects(first: 1) { nodes { name } } slackChannel(id: \"C0BSX7Q9M0E\") { name } graphIssues(where: {identifier: {eq: \"NODES-1\"}}, limit: 1) { identifier discussedInThreads { ts } } }"}'

# MCP side — tools/list works stateless (no initialize/session dance):
curl -s http://localhost:5025/mcp \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
# → SSE body; the tools array must contain exactly the five tools above
```

`tools/call` for `get_issue_discussion_context` with
`{"identifier":"NODES-1"}` returns the issue + 2 discussions + all
messages in one call against the local graph, with zero external network
calls (the talk's main segment).

### MCP client config (Claude Desktop — Phase 5.1)

`claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "nodes2026": {
      "url": "http://localhost:5025/mcp"
    }
  }
}
```

The contrast profile (Phase 5.2) instead registers three separate MCP
servers (Linear, Slack, Neo4j) in a second config.

### Stop / regenerate

- Stop: find the PID (`lsof -tiTCP:3010 -sTCP:LISTEN`), kill by PID.
  Same for the neo4j subgraph server (`lsof -tiTCP:4000 -sTCP:LISTEN`).
  Avoid `pkill -f` patterns that can match the invoking shell.
- After changing an operation file: restart the router (no file
  watching — static execution config).
- After changing the Slack plugin schema/code: `make build` in
  `router/plugins/slack`, then `make compose` + restart. The pinned
  protoc 29.3 lives in `router/tools/` and the Makefiles apply it
  automatically (see `router/README.md`).
- After changing `neo4jGraphQLSrv/` code: kill the port-4000 PID and
  restart with `make neo4j-srv`. If `neo4jGraphQLSrv/schema.graphql`
  changed, additionally `make gen-neo4j-schema && make compose` +
  restart the router (the composed config embeds the generated
  plain SDL).

Verified live 2026-08-28: all four Slack plugin queries against the
real workspace; Linear `projects`/`project` with real data; both graph
queries against the local graph; a single combined request touching all
three subgraphs; and the MCP `tools/list` + `tools/call` round-trip for
all four persisted operations (see `router/README.md`).
