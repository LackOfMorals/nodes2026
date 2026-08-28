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

## Live embedding demo (Phase 6)

Shows entries getting incorporated into the graph *live*, with vector
embedding — not the canonical seeded graph, the real sync pipeline
running in the background while you talk. See `sync/README.md` →
"Embedding and semantic linking" / "Live demo mode" for the full design;
this is the on-stage recipe.

**Runs against a dedicated `livedemo` database, never the default
`neo4j` database that holds the canonical seed graph.** This isn't
cosmetic — verified live on 2026-08-28 that it's load-bearing: the real
Linear/Slack workspace mirrors the same demo story as the canonical seed
data (NEO-10 ≈ NODES-1, etc.), so if both coexist in one database, their
near-duplicate phrasing floods the semantic pass with cross-links
between the fictional and real versions of the same issues. Isolating
them in separate databases removes that specific failure mode entirely.
Creating it once:

```bash
curl -s -u neo4j:password http://localhost:7474/db/system/tx/commit \
  -H 'Content-Type: application/json' \
  -d '{"statements":[{"statement":"CREATE DATABASE livedemo IF NOT EXISTS"}]}'

# apply schema.cypher's constraints + vector indexes to it (once) —
# paste data-model/schema.cypher's 18 statements into the web console
# with the target database switched to "livedemo" (top-left selector),
# or POST them individually to /db/livedemo/tx/commit.
```

**Prerequisites:**

- LM Studio running with an embedding model loaded and its local server
  started (`lms server start`, or the app's "Local Server" tab). This
  demo is built around **nomic-embed-text-v1.5** (768 dim, the `@q8_0`
  quantization verified live) — matches `schema.cypher`'s vector indexes.
  A different model needs a different index dimension — see PLAN.md
  Phase 6.1.
- The real Linear/Slack workspaces from [SaaS prerequisites](#saas-prerequisites--credentials),
  since this demo posts genuinely new content to them, not the canonical
  seed data.
- One real timed dry run of `--watch` at your chosen `--interval` before
  the talk, watching the log for `status 429` (PLAN.md Known issue 10 —
  Slack's rate limits for a non-Marketplace app are unverified against
  this demo's actual polling cadence).

**Known, accepted limitation — extra links may appear.** Verified live
2026-08-28 against the real workspace (8 issues, 4 threads, in the
isolated `livedemo` database): the intended moment works — a real
orphaned thread linked to its issue at confidence 0.780, evidence
`semantic_match` — but **7 other semantic links also appeared** among
unrelated issues (confidence 0.759–0.779). At this corpus's scale, short
same-project engineering text clusters too tightly for `0.78` (or any
single fixed threshold) to cleanly separate "genuinely related" from
"same-domain-different-topic" — this is the corpus-scale version of the
"0.75 is a heuristic, not A/B tested" tension `TALK.md` already plans to
name on stage. **Decision (2026-08-28): ship it as-is, accept the noise,
do not add reranking/top-1 logic before the talk.** If asked in Q&A why
other links appeared alongside the intended one, that's the honest
answer, not a bug to apologize for.

**Setup, before you go on stage:**

```bash
set -a; . ./.env; set +a
cd sync
go build -o bin/sync ./cmd/sync

export NEO4J_URI=bolt://localhost:7687
export NEO4J_USER=neo4j
export NEO4J_PASSWORD=password
export NEO4J_DATABASE=livedemo                                       # NOT the default "neo4j" db — see above
export EMBEDDING_BASE_URL=http://localhost:1234/v1
export EMBEDDING_MODEL="text-embedding-nomic-embed-text-v1.5@q8_0"    # match what `curl localhost:1234/v1/models` reports

./bin/sync --watch --interval 20s \
  --linear-project "$LINEAR_PROJECT_ID" --slack-channel "$SLACK_CHANNEL_ID"
```

Leave this running in a terminal the audience can see (or a second
screen). It logs every tick — pulled issues/threads, `embedded N issues`,
`embedded N messages`, `semantic linking done` — so the "it's working"
signal is visible without needing to query the graph directly.

**On stage:**

1. Post a new Slack message in `#nodes-demo-eng` that discusses an
   existing issue's topic **without naming its identifier** — the same
   shape as the orphaned Thread 4 in the canonical seed data. Or create a
   new Linear issue.
2. Talk for the length of one `--interval` (or two, to be safe) while it
   sits there.
3. Point at the terminal: the next tick picks it up, embeds it, and (if
   the similarity clears `SEMANTIC_LINK_THRESHOLD`) creates the
   `:DISCUSSED_IN` edge live — evidence `semantic_match`, not
   `explicit_mention`, because nothing in the graph was told to look for
   this connection; the embedding found it.
4. Show the new edge. **The router's neo4j subgraph (`neo4jGraphQLSrv`)
   points at the `neo4j` database, not `livedemo`** — the MCP agent tool
   (`get_issue_discussion_context`) won't see this segment's data unless
   you restart it against `livedemo` first (`NEO4J_DATABASE=livedemo
   node index.cjs`, then re-point the router or just query
   `:4000/graphql` directly), which also means the *main* demo segment's
   `get_issue_discussion_context` calls stop working until you restart it
   back. Simplest for this segment: skip the MCP tool and show the edge
   directly —
   ```bash
   curl -s -u neo4j:password http://localhost:7474/db/livedemo/tx/commit \
     -H 'Content-Type: application/json' \
     -d '{"statements":[{"statement":"MATCH (i:Issue)-[d:DISCUSSED_IN]->(t:Thread) WHERE d.evidence = \"semantic_match\" RETURN i.identifier, t.ts, d.confidence ORDER BY d.createdAt DESC LIMIT 5"}]}'
   ```
   or the Neo4j Browser pointed at `livedemo`, with the browser's graph
   view doing the visual work instead of a JSON blob.

**Recovery, if something doesn't land in front of the audience:** the
loop keeps ticking regardless — a failed or empty tick just means "try
again next interval," not a crash. If the whole segment is going badly,
fall back to the canonical seeded story (Thread 4 already demonstrates
the same "orphaned thread, semantic match" moment, just pre-baked) and
move on; this segment is additive to the main demo, not load-bearing for
it.

**After the talk:** stop the `--watch` process. The canonical `neo4j`
database was never touched by this segment (that's the point of
`livedemo` being separate), so there's nothing to restore there. If you
want `livedemo` itself back to a clean slate for the next rehearsal:
`MATCH (n) DETACH DELETE n` against `/db/livedemo/tx/commit` (or the web
console with the database selector set to `livedemo`), then re-run
`./bin/sync` once (without `--watch`) to repopulate it from the real
workspaces before your next dry run.

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
   names collide with Linear's types in the composed supergraph.
   `discussedInThreadsConnection` (auto-generated from the
   `DISCUSSED_IN` relationship's `properties: "DiscussedInProperties"`)
   carries the confidence/evidence for each link directly on the edge
   — load-bearing talk content, no `@cypher` needed. The one remaining
   `@cypher` field is root `searchMessagesCI` (case-insensitive message
   search; Neo4j 2026.x `CONTAINS` is case-sensitive). The router
   registers a **generated plain-SDL file** (`schema: {file:
   ../neo4jGraphQLSrv/neo4j-plain-schema.graphql}`) — a gitignored
   build artifact produced by `make gen-neo4j-schema` (plain SDL from
   the library's schema via `printSchema`, which strips the `@cypher`
   directive).

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

### MCP client config (Claude Desktop)

The verified working form (2026-08-28) is the `mcp-remote` stdio bridge
— the bare `url` form is not what is proven:

```json
{
  "mcpServers": {
    "nodes2026": {
      "command": "npx",
      "args": ["mcp-remote", "http://localhost:5025/mcp"]
    }
  }
}
```

Both talk profiles are committed under `mcp-profiles/` and installed by
`scripts/switch-mcp-profile.sh` — see "Phase 5 — Claude Desktop profiles
and the contrast demo" below.

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

---

## Phase 5 — Claude Desktop profiles and the contrast demo

Two Claude Desktop MCP profiles, both committed under `mcp-profiles/`:

| Profile | File | MCP servers | Used for |
|---------|------|-------------|----------|
| A — federated | `mcp-profiles/federated.json` | `nodes2026` (router MCP Gateway via `mcp-remote`) | Demo main segment: one endpoint, five tools, one persisted-operation call |
| B — no federation | `mcp-profiles/no-federation.json` | `linear` (official hosted MCP), `slack` (slack-mcp-server), `neo4j` (local canary binary) | Contrast: scattered servers, more and weaker tool calls |

### Switching profiles

```sh
scripts/switch-mcp-profile.sh federated        # install profile A
scripts/switch-mcp-profile.sh no-federation    # install profile B
scripts/switch-mcp-profile.sh restore          # pre-talk mcpServers
scripts/switch-mcp-profile.sh show [profile]   # print rendered JSON, change nothing
```

- The **first** switch backs up the whole live config to
  `~/Library/Application Support/Claude/claude_desktop_config.json.nodes2026.bak`
  and never overwrites an existing backup.
- Only the `mcpServers` key is replaced; everything else in the live
  config (`coworkUserFilesPath`, `preferences`, …) is preserved.
- Credentials are substituted from the repo `.env` at switch time; the
  committed profile files contain only `__PLACEHOLDERS__`.
- **Restart Claude Desktop after every switch.**

The live config's pre-talk `mcpServers` also contains `neo4j-gong` /
`neo4j-c360` (Aura canary entries) — profile A deliberately drops them so
the federated view shows exactly one server. `restore` brings them back.

### Contrast server inventory (all verified headless 2026-08-28)

| Server | Wiring | Verified | Notes for the talk |
|--------|--------|----------|--------------------|
| `linear` | `url: https://mcp.linear.app/mcp` + `Authorization: Bearer <key>` | initialize OK, serverInfo "Linear MCP" v1.0.0 | The MCP endpoint **accepts** the `Bearer` prefix (the raw Linear GraphQL API rejects it). Needs internet. |
| `slack` | `npx -y slack-mcp-server@1.3.0 --transport stdio --no-cache`, env `SLACK_MCP_XOXB_TOKEN` | initialize OK, 13 tools, `conversations_history` + `conversations_replies` returned real `#nodes-demo-eng` data incl. thread t1 | **No message-search tool** — bot tokens cannot call `search.messages` (Known issue 4); that is the contrast point, not a gap. The `--no-cache` flag is load-bearing: see PLAN.md Known issue 7. Use channel IDs, not names (no cache). |
| `neo4j` | local canary binary, `NEO4J_READ_ONLY=true` | initialize OK (v0.4.0), 4 tools, `read-cypher` returned NODES-1… | The tool is named **`read-cypher`**, not `cypher`. The agent must write its own Cypher — another "weaker without federation" point (no off-the-shelf `getIssueDiscussionContext`). |

The router's MCP endpoint is **stateless**: `initialize` returns no
`mcp-session-id` header and that is correct — per the MCP spec the client
then never sends a session header. `mcp-remote` handles it; no client-side
workaround needed.

### Prompt rehearsal (5.3)

**Prompt 1 (federated path)** —
> What did the team decide about the schema validation issue in NODES-1?

Expected: exactly **one** tool call, `get_issue_discussion_context`
(`identifier: "NODES-1"`). Verified response shape (2026-08-28, re-verified
2026-08-28 after switching to `discussedInThreadsConnection`): one issue
(In Progress, priority 4, created/assigned Sarah Chen) +
`discussedInThreadsConnection.edges` with 2 entries (4 + 3 messages,
`#nodes-demo-eng`), each edge's `properties` carrying `confidence: 1`,
`evidence: "explicit_mention"` directly. Expected synthesis (TALK.md):

> The team decided to use explicit `@shareable` annotations across all three
> subgraphs to preserve nullability during composition. Sarah identified the
> root cause; Alex made the decision and confirmed the fix landed.

(Decision message: Alex Rivera, "Going with the explicit @shareable annotation
approach … across all three subgraphs." in thread 1.)

**Prompt 2 (cliffhanger, only if time allows)** —
> Has anyone discussed the same problem recently without referencing the
> issue directly?

Iteration-1 answer: "I don't have visibility into discussions that don't
reference the issue." (Thread 4 is deliberately orphaned — no `DISCUSSED_IN`
edge. Vectors in Phase 6 find it by semantic similarity.)

On profile B the same Prompt 1 is expected to degrade visibly: the agent must
pick among separate servers, hand-write Cypher against `neo4j` (no composed
operation), and cannot search Slack at all.

### Morning-of checklist (both profiles)

1. `podman ps` — `neo4j-prod-01` up; if not, `podman start neo4j-prod-01`.
2. Start the two foreground services (see Router → Start above):
   `make neo4j-srv` (:4000) and `make start` (:3010/:5025).
3. `tools/list` sanity: the curl in Router → Verify must show exactly the
   five tools.
4. Pre-warm the npx caches so first use is instant:
   `npx -y mcp-remote --version` and
   `npx -y slack-mcp-server@1.3.0 --transport stdio --no-cache` (exit
   immediately — it just needs to be in the cache).
5. Confirm internet access for the hosted Linear MCP (profile B).
6. `scripts/switch-mcp-profile.sh federated` → restart Claude Desktop →
   confirm one MCP server with five tools → run Prompt 1 → screenshot the
   single tool call + answer.
7. `scripts/switch-mcp-profile.sh no-federation` → restart → confirm three
   servers → run Prompt 1 again → screenshot the weaker/longer tool trail.
8. `scripts/switch-mcp-profile.sh restore` → restart (post-talk reset).
