# NODES 2026 demo — completion plan

Plan to take the demo from its current state to a rehearsed, live-runnable
form for the talk on **November 12, 2026**. Last updated: 2026-08-28.

Read alongside `TALK.md` (the argument and narrative) — this document is
only about building and running the demo.

---

## Locked decisions

These were decided on 2026-08-27 and shape the whole plan:

1. **The talk is accepted.** No CFP uncertainty gating the build.
2. **Local-first demo environment.** Everything except Linear and Slack
   (which are SaaS) runs on the speaker's laptop:
   - Neo4j 5 in a podman container (not Aura).
   - Cosmo Router run locally via `wgc` (no Cosmo Cloud control plane, no
     plugin registry publish). The router's local/air-gapped mode is
     documented and supported; the MCP Gateway can load operations from a
     `file_system` storage provider in local mode.
   - Claude Desktop local, pointed at the router's local MCP endpoint.
3. **ECS Fargate / AWS deployment is cut.** It is not part of the demo.
   AWS (Fargate + Cosmo Cloud + Aura) appears in the talk only as a
   "where this runs in production" slide. The original August milestone
   ("router on ECS") is dropped, not deferred.
4. **Container tooling is podman**, not Docker — same CLI surface
   (`podman run`, `podman buildx`, etc.). Any future container step in
   this plan uses podman.
5. **Seed strategy:** `data-model/sample-data.cypher` is the canonical
   live-demo graph (it tells the story and is stable). The sync pipeline
   is run against real workspaces to prove it works end-to-end, but the
   live demo does not depend on it.
6. **Real workspaces are JG-only for now** (one Linear user, one Slack
   member). Use the same email in both (j.giffard@icloud.com) so
   Person-unification-by-email is demonstrated with real data.

## Current state (verified 2026-08-27)

| Component | State |
|-----------|-------|
| `data-model/` | **Run and verified against the live local container (Neo4j 2026.07.1):** wipe → schema 16/16 → sample-data 21/21 → link-explicit 5 edges. All statements self-contained; `link-explicit` is pure Cypher; epochs are May 2026. Agent-context for NODES-1 returns the expected structure, Thread 4 orphaned. |
| `sync/` | Complete, builds and vets clean. Writer **verified end-to-end against the live container** via `internal/graph/writer_e2e_test.go` (`WRITER_E2E=1`). **Ran against the real workspaces on 2026-08-27 (Phase 2.3) — clean first run:** 6 issues (NEO-10..15, real states/priorities/URLs), 4 threads / 12 messages (t1 in with all 4 — the `include_all_metadata` fix proven live), system events skipped (the `SubType` filter proven live), 5 `DISCUSSED_IN` edges, t4 orphaned, one unified `:Person`. Linear client matches the live API exactly (`$projectId: String!`, raw-key auth header). |
| Linear workspace | Seeded 2026-08-27: project "NODES 2026 Demo" (`1b3763ee-c856-4727-bc80-bf1ae5d0794b`, team `NEO`) + 6 issues **NEO-10..NEO-15** mirroring the story graph, with states/assignee set (NEO-10 In Progress + j.giffard; NEO-12 Todo; NEO-14 Done). See Phase 2.1. |
| Slack workspace | Ready + seeded 2026-08-27: "Nodes 2026 Demo" workspace, `#nodes-demo-eng` (`C0BSX7Q9M0E`), bot token verified against all 5 sync-client endpoints, `users.info` returns email (unification OK). JG's Slack user `U0BSZ59TY66`. **Thread seeding done (Phase 2.2):** four threads posted by JG in the UI (not the bot), all authored by j.giffard@icloud.com — t1 `1787845145.093149` (names NEO-10; 3 replies), t2 `1787845177.141579` (names NEO-10/14/15; 2 replies), t3 `1787845201.547149` (names NEO-12; 1 reply), t4 `1787845220.346049` (names none — the orphan; 2 replies). Channel also carries 3 system events (filtered by the sync client's `SubType` check). |
| `neo4j-api/` | **Done 2026-08-28 (Phase 4.1):** small Go plain-GraphQL HTTP service on `127.0.0.1:4400` (graphql-go + neo4j-go-driver v5.27.0). Root fields `getIssueDiscussionContext` + `searchMessages`, both verified live against the local graph (unknown identifier → null; search returns thread/channel/permalink hits). `schema.graphql` doubles as the router's static subgraph SDL. `//go:embed` workaround: `loadAsset` reads files from CWD/exe-dir at startup (Go 1.26.5 rejects embed on this machine). Superseded as the registered subgraph 2026-08-28 (Phase 4.5); kept as an unwired fallback until then. **Removed 2026-08-28, see Known issue 9** — unreachable from any Make target and already drifting from `neo4jGraphQLSrv/`'s schema; git history retains it. |
| `neo4jGraphQLSrv/` | **Done 2026-08-28 (Phase 4.5):** the router's registered neo4j subgraph — `@neo4j/graphql` 7.6.2 + Yoga on `127.0.0.1:4000/graphql` (`node index.cjs`). Hand-written typeDefs in `schema.graphql` are the single source of truth: `Issue`/`Project` renamed `GraphIssue`/`GraphProject` via `@node(labels:)` (name collision with Linear's types hard-fails composition), plus a `@cypher` field for root `searchMessagesCI` (case-insensitive; see Known issue 6). `gen-plain-schema.cjs` (`printSchema`) → gitignored `neo4j-plain-schema.graphql`, registered via `schema: {file: …}`; Make targets `make gen-neo4j-schema` / `make neo4j-srv`. Verified 31/31 MCP checks + combined 3-subgraph request. **Updated 2026-08-28, see Known issue 9:** the `GraphIssue.discussionDetails` `@cypher` field (confidence/evidence, correlated by `threadTs`) was removed in favor of the auto-generated `discussedInThreadsConnection`, which already exposes the same `DISCUSSED_IN` properties on the edge from one traversal; `debug` was also reverted `false → true` so the demo's live-Cypher beat still works. |
| `router/` (was `staging/demo-router/`) | **Phase 4.1 done 2026-08-28:** router 0.343.1 composes all three subgraphs — Slack Connect plugin (**all 4 queries verified live** against the real workspace; the 5th, `searchSlackMessages`, was dropped 2026-08-28 — Slack's `search:read` is user-token-only, Known issue 4 RESOLVED), Linear (`raw: true` introspection, full SDL composed), and neo4j (generated plain-SDL from the Neo4j GraphQL library server, Phase 4.5). A single request touching all three subgraphs verified live. `graph.yaml` rendered from committed template by `make render-graph` (key from `.env`); runtime Linear auth via config.yaml `${LINEAR_API_KEY}` env expansion (verified working in 0.343.1). Start: `make start` (foreground) + `make neo4j-srv` (second terminal). |
| Router config / persisted ops / MCP | **Done 2026-08-28 (Phase 4.2–4.4):** MCP Gateway on `localhost:5025` exposes exactly the four persisted operations + built-in `get_operation_info` (not the 60+ raw fields). All four `tools/call` round-trips verified live via JSON-RPC: `get_issue_discussion_context(NODES-1)` → issue + 2 discussions + all messages from the local graph (zero external calls); `search_messages(cache)` → 3 hits; `linear_project_issues` (zero-arg) → 6 real issues NEO-10..15; `slack_thread` → 4 live messages (t1). Runbook in `DEMO-ENV.md` → "Router (Phase 4 …)". |
| Agent harness (Claude Desktop) | **Done headless 2026-08-28 (Phase 5.1–5.2):** both profiles committed (`mcp-profiles/`), switch script `scripts/switch-mcp-profile.sh` (back up → replace `mcpServers` only → restart). All four MCP paths verified headlessly this session via the exact stdio/JSON-RPC path Claude Desktop uses: router MCP (5 tools, `tools/call` round-trips), Linear hosted MCP (`Bearer` accepted), slack-mcp-server 1.3.0 with bot token + `--no-cache` (13 tools, real channel data, no search tool — see Known issue 7), Neo4j canary (`read-cypher`, `NEO4J_READ_ONLY=true`). Remaining: GUI runs + screenshots + timed dry run (5.3/5.4) — user-driven. |
| Iteration 2 (vectors) | **Built and verified end-to-end with real embeddings, 2026-08-28 (Phase 6).** nomic-embed-text-v1.5@q8_0 running in LM Studio; embedding client, writer methods, `schema.cypher` vector indexes, and `link-semantic.cypher` all live-verified against real Linear/Slack data in an isolated `livedemo` database (real orphaned thread → its issue at confidence 0.780) plus a permanent e2e test with synthetic vectors. `sync --watch` runs the whole pipeline (embed + semantic-link included) on a timer for the live demo. **Known, accepted tradeoff:** at this corpus's scale a fixed similarity threshold also surfaces a handful of extra links alongside the intended one — see Known issue 9 for the database-isolation fix and the full threshold finding. See PLAN.md Phase 6. |
| Slides, dry runs, backup video | Not started. |

Local toolchain: Go 1.26.5, Node 26.5.0, `wgc` 0.130.1 (via npx), podman.
No cypher-shell (optional to install: `brew tap neo4j/tap && brew install
neo4j/tap/cypher-shell`).

## Known issues to fix as part of the build

1. **Cypher cross-statement scope. RESOLVED (2026-08-27, verified on the
   live container — Neo4j 2026.07.1 Enterprise).** The contract now in
   force: one file = one statement. `sync/internal/graph/writer.go`
   `execute()` runs each query file as a single `tx.Run`, and every
   statement in `sample-data.cypher` is self-contained (21 statements,
   each re-`MERGE`s its nodes by key). Fixed along the way: `''` string
   escapes are rejected on 2026.x (message texts use U+2019 `’`), the
   `apoc.text` family is gone (so `link-explicit.cypher` was rewritten as
   pure Cypher with word-boundary matching — no APOC anywhere in the
   data model), and the sample data's May-2025 epochs were shifted
   +31536000s to May 2026 to match the issue dates (talk is Nov 2026).
   Verified: wipe → schema 16/16 → sample-data 21/21 → link-explicit
   5 distinct rows; agent-context for NODES-1 returns the expected
   2-discussion structure with Thread 4 orphaned. Regression test:
   `sync/internal/graph/writer_e2e_test.go` (gated by `WRITER_E2E=1`).
2. **wgc plugin bootstrap mismatch. RESOLVED (2026-08-27, Phase 3).**
   Confirmed exactly as predicted: `wgc router plugin init slack -p
   demo-router --language go` (0.130.1) scaffolds a **complete router
   project**; the fake `router-plugin v0.0.0` placeholder is not a real
   module (the scaffold's own go.mod pins the real v0.4.1
   pseudo-version); the default module path
   `github.com/wundergraph/cosmo/plugin` was kept. Executed: scaffold in
   `staging/`, ported schema (plain GraphQL, federation directives
   dropped) + client, rewrote the service for the generated types
   (protobuf wrappers for optional args/fields), moved the finished
   project to `router/` (renamed — it hosts all subgraphs + the Phase 4
   MCP config), deleted the old `slack-subgraph/`. Full bootstrap
   write-up in `router/README.md` + `router/plugins/slack/README.md`
   (incl. the protoc 29.x strict-series requirement).
3. **Federation introspection risk. RESOLVED (2026-08-28, Phase 4.1,
   verified live).** The Cosmo Router composes **plain-GraphQL
   subgraphs** fine — no Apollo Federation `_service`/`_entities`
   introspection required. Both non-plugin subgraphs registered and
   served through `localhost:3010`:
   - **Linear** — `graph.yaml` entry with
     `introspection: {url: https://api.linear.app/graphql, raw: true,
     headers: {Authorization: <raw key, no Bearer>}}`. `raw: true` is
     the flag that makes wgc treat the response as the schema (no
     federation unwrap). The composed `config.json` embeds Linear's
     full ~1.3 MB SDL.
   - **Neo4j** — `graph.yaml` entry with `routing_url:
     http://127.0.0.1:4400/graphql` + `schema: {file:
     ../neo4j-api/schema.graphql}` (static SDL, no live introspection;
     wgc resolves the path relative to graph.yaml). Verified in wgc
     0.130.1 source (`compose.js` handles `s.schema.file`).
   Runtime Linear auth comes from `config.yaml`
   `headers.subgraphs.linear.request` with `value: "${LINEAR_API_KEY}"` —
   **env expansion in config.yaml works in router 0.343.1** (verified
   live: Linear queries return data and the key appears nowhere in the
   composed config.json). Fallback 4.1b (wrapper plugins) is not needed.
   The talk's "registered directly" framing holds.
4. **Slack bot scope gap. RESOLVED by dropping the query (2026-08-28 —
   platform constraint, not a fixable gap).** `search.messages` with a
   bot token returns `not_allowed_token_type`, and the Slack docs confirm
   why: `search:read` is a **legacy scope, supported token type = User
   only** (docs.slack.dev/reference/scopes/search.read). The granular
   successor scopes (e.g. `search:read.public`) do accept a bot token but
   only enable the Assistant API (`assistant.search.context` / `.info`),
   which searches as a specific user — not a general search endpoint.
   **No bot token, under any bot-scope combination, can ever call
   `search.messages`.** Decision (user-approved): drop
   `searchSlackMessages` from the Slack subgraph. Executed 2026-08-28:
   query removed from schema/service/client/tests, proto regenerated,
   plugin rebuilt, router recomposed + restarted; all 4 remaining
   queries re-verified live against the real workspace. Talk upside: in
   the world-without-federation profile (Phase 5.2) the stock Slack MCP
   servers run on bot tokens and therefore have **no message search** —
   the knowledge graph's `searchMessages` is the only search surface in
   the demo, which sharpens the federation payoff.
5. **Doc drift.** `TALK.md` build status still lists the sync pipeline as
   "To do" (it is done and building); its architecture section and the
   root README's quick-start describe the cut AWS/Aura/ECS/Cosmo-Cloud
   path. Update both when Phase 4 lands (task 7.4).
6. **Neo4j 2026.x Cypher strictness (discovered 2026-08-28, Phase 4.5).**
   Two gotchas in raw Cypher and `@cypher` fields:
   - `CONTAINS` is **case-sensitive** (the `@neo4j/graphql` v7 `contains`
     filter compiles to plain `CONTAINS`). Case-insensitive matching must
     be written explicitly: `toLower(a) CONTAINS toLower(b)` (used by
     `searchMessagesCI`).
   - `LIMIT $param` **rejects driver float and null bindings** — the JS
     driver sends numbers as float (`'3.0' is not a valid value`) and an
     absent/null variable fails outright. Pattern: `LIMIT
     toInteger(coalesce($param, N))` (verified for limit=3/20/null).
   Affects the sync pipeline's raw Cypher and any future `@cypher` work.
7. **slack-mcp-server fatal boot with a bot token. RESOLVED via
   `--no-cache` (2026-08-28, Phase 5.2).** `npx slack-mcp-server@1.3.0`
   (korotovsky) authenticates fine with `SLACK_MCP_XOXB_TOKEN` but dies at
   boot: `Error booting provider — API returned zero channels and no
   existing cache is available`. Root cause (debug log): its
   `RefreshChannels` enumerates **all** channel types, and this bot lacks
   `im:read` / `mpim:read` — `conversations.list types=im` /
   `types=mpim` return `missing_scope`, and the failed enumeration aborts
   startup instead of keeping the 3 public channels it already fetched.
   Fix used: `--no-cache` skips the user/channel cache watchers, so boot
   succeeds; `conversations_history` / `conversations_replies` by channel
   **ID** work against the real demo channel (verified, incl. thread t1),
   the tool list is 13 tools with **no `conversations_search_messages`**
   (bot tokens can't call `search.messages` — Known issue 4, by design for
   the contrast). Trade-off: no `#channel-name` / `@username` lookup — the
   demo uses channel IDs, which is fine. Alternative not taken: adding
   `im:read` + `mpim:read` to the app and reinstalling (works too, but
   needs an app-settings step; `--no-cache` keeps the profile
   self-contained).
8. **Router MCP endpoint is stateless — no `mcp-session-id`. Observed,
   no action needed (2026-08-28, Phase 5.1).** `initialize` against
   `localhost:5025/mcp` returns an SSE envelope **without** an
   `mcp-session-id` response header. Per the MCP spec the client must not
   send a session header in that case, and `mcp-remote` (the exact bridge
   Claude Desktop uses) handles it — full initialize → tools/list →
   tools/call round-trips work. Do not "fix" this by adding session
   tracking to the router.
9. **Code-review follow-up, RESOLVED (2026-08-28).** A review of the
   Phase 4.5 diff found and fixed: (a) `neo4jGraphQLSrv/index.cjs`'s
   `debug` flag had been left `false`, which silently disabled the
   Cypher-console-logging the live demo's "show the Cypher" beat
   depends on — reverted to `true`; (b) `GraphIssue.discussionDetails`
   (the hand-rolled `@cypher` field) duplicated data already available
   via the auto-generated `discussedInThreadsConnection` (the
   relationship already declares `properties: "DiscussedInProperties"`)
   — removed `discussionDetails`/`DiscussionDetail` entirely;
   `get-issue-discussion-context.graphql` now reads confidence/evidence
   off `discussedInThreadsConnection.edges[].properties` in the same
   traversal instead of a second hand-correlated list. Re-verified live:
   31/31-equivalent MCP round-trip for `get_issue_discussion_context`
   (NODES-1) through the full router → MCP gateway path, unchanged
   response content. Also fixed: `scripts/switch-mcp-profile.sh` no
   longer aborts `restore`/`show` when a secret is missing from `.env`
   (previously a `pipefail` interaction made any subcommand die
   silently), secret substitution moved from unescaped `sed` to a
   literal Node string-replace (a secret containing `&`/`\`/`|` used to
   corrupt or crash the substitution), and the two divergent inline
   `mcpServers`-patch blocks were unified into one `patch_mcp_servers`
   function. The old Go `neo4j-api/` fallback service (superseded in
   Phase 4.5) was removed — it was unreachable from `make start`/`make
   compose` and had already begun drifting from `neo4jGraphQLSrv/`'s
   schema; git history retains it if ever needed.

---

## Phase 0 — Prerequisites (≈1 h, mostly account setup)

- [x] Linear: free workspace, team, project "NODES 2026 Demo", personal
      API key (Settings → API). Done 2026-08-27 via the API: project
      `1b3763ee-c856-4727-bc80-bf1ae5d0794b` in team `NEO`, key in `.env`
      (verified: list + update round-trip). Quirk: the key goes in the
      `Authorization` header **without a `Bearer` prefix** — Linear
      rejects `Bearer lin_api_...`.
- [x] Slack: free workspace (owned, personal), channel
      `#nodes-demo-eng`, Slack app at api.slack.com/apps with bot
      scopes, bot invited to the channel, token in `.env`. Done
      2026-08-27: workspace "Nodes 2026 Demo"
      (nodes2026demo.slack.com), `#nodes-demo-eng` = `C0BSX7Q9M0E`, bot
      `nodes2026demo` is a member. Token verified live against every
      endpoint the sync client calls — `conversations.info`,
      `conversations.history`, `users.info` (email **is** returned, so
      Person unification by email works through the client's exact
      path). `groups:*`/`search:read` (Known issue 4) are not used by
      the current client, so no gap.
- [x] Confirm podman works: `podman info`. (podman 6.0.2, darwin/arm64;
      `neo4j-prod-01` up on 7474/7687.)
- [x] Note credentials somewhere safe (env file, not committed). Root
      `.env` (covered by `.gitignore`) holds the real values.

Acceptance: both tokens can make one API call (Linear: list projects;
Slack: `auth.test`).

## Phase 1 — Local environment + graph validation + Cypher fixes (half day)

- [x] 1.1 Start Neo4j. Container was provided pre-running as
      `neo4j-prod-01` (image `neo4j:2026.07.1-enterprise`, auth
      `neo4j`/`password`, ports 7474/7687) rather than via the
      documented `podman run` command; the start command is in
      `DEMO-ENV.md`. APOC check dropped: on 2026.x the `apoc.text`
      family is gone and `link-explicit.cypher` is pure Cypher, so
      nothing in the data model needs APOC.
- [x] 1.2 Run `schema.cypher` (web console) — 16/16 statements OK.
- [x] 1.3 `sample-data.cypher` fixed and runs clean (21/21) — see
      Known issue 1 for the fixes.
- [x] 1.4 Verify with `queries/agent-context.cypher` for `NODES-1`:
      2 discussions (threads 1 and 2), messages populated,
      Thread 4 **not** linked. ✓
- [x] 1.5 Implemented as a permanent in-repo test rather than a
      throwaway script: `sync/internal/graph/writer_e2e_test.go`,
      skipped unless `WRITER_E2E=1` (overrides: `WRITER_E2E_URI`,
      `WRITER_E2E_QUERIES`). Passes against the local container:
      issue/thread upsert idempotency, Person unification (one node
      with both `linearId` and `slackId`), explicit linking edge
      creation.
- [x] 1.6 Recorded in `DEMO-ENV.md` — podman + web-console workflow,
      reset one-liner, e2e test invocation, SaaS prerequisites.

Acceptance: fresh container → schema → sample data → agent query returns
the expected NODES-1 structure with Thread 4 orphaned; the sync writer's
upserts + explicit linking reproduce the same edges from raw payloads.

## Phase 2 — Real data + sync end-to-end (a few hours, after Phase 0)

- [x] 2.1 Seed Linear: project "NODES 2026 Demo" + six issues mirroring
      the demo story (2026-08-27, via API): **NEO-10** federation schema
      validation (the NODES-1 equivalent — In Progress, assigned to
      j.giffard), **NEO-11** 401 under load, **NEO-12** OAuth refresh
      (Todo), **NEO-13** docs, **NEO-14** race condition (Done), **NEO-15**
      cache invalidation. Real prefix is **`NEO`** and the workspace
      counter was already at 9 (numbers aren't reused after deletions), so
      the story issues are NEO-10..15, not NEO-1..6. Slack threads must
      name the real identifiers: NEO-10 (t1+t2), NEO-12 (t3), NEO-14 and
      NEO-15 (t2); the orphan thread names none. (The sync CLI has no
      identifier-pattern flag — `link-explicit.cypher` matches against
      whatever Issue identifiers are already in the graph.)
- [x] 2.2 Seed Slack: 3–4 threads in `#nodes-demo-eng` — at least one
      naming an issue identifier explicitly, one discussing the same
      topic without naming it (the orphan, for iteration 2).
      Done 2026-08-27 — posted by JG in the Slack UI (not the bot), all
      authored by j.giffard@icloud.com: t1 `1787845145.093149`
      (federation/NEO-10 nullability, 3 replies, names NEO-10 twice),
      t2 `1787845177.141579` (pre-mortem, 2 replies, names NEO-10 +
      NEO-14 + NEO-15), t3 `1787845201.547149` (token refresh, 1 reply,
      names NEO-12), t4 `1787845220.346049` (schema onboarding, 2
      replies, names no identifier — the orphan). t3/t4 initially had a
      duplicate/missing reply; fixed in the UI before the 2.3 run and
      re-verified via API (4+3+2+3 = 12 messages). The sync pipeline
      upserts but never tombstones — edit Slack messages rather than
      delete-and-repost.
- [x] 2.3 `go build -o bin/sync ./cmd/sync`, then run with
      `NEO4J_URI=bolt://localhost:7687` (local Neo4j, not Aura).
      Ran 2026-08-27 against a wiped DB — clean first run: `pulled 6
      issues` (NEO-10..15 with real states/priorities/URLs), `pulled 4
      threads` (message counts 4+3+2+3 = 12; t1 in with all 4 messages
      — the `include_all_metadata` fix proven live), system events
      skipped (the `SubType` filter proven live), `explicit linking
      done`. Graph verified: 5 `DISCUSSED_IN` edges (NEO-10→t1,
      NEO-10/14/15→t2, NEO-12→t3), t4 orphaned. Full recipe +
      verification checklist in `DEMO-ENV.md` → "Real-data sync run".
- [x] 2.4 Verify `agent-context.cypher` against the real issue
      identifier; confirm your Person node carries both `linearId` and
      `slackId` (email unification worked).
      `identifier=NEO-10` returns the issue (real title/state/priority/
      linearUrl/dates) with **exactly 2 discussions** (t1, t2), all
      messages with author name/email + permalinks, channel
      `nodes-demo-eng`, confidence 1.0 / `explicit_mention`. One Person
      node: `linearId=9765f250-eb4f-476d-a27b-d0ed5a90597f` +
      `slackId=U0BSZ59TY66` — email unification worked. (Cosmetic:
      `Person.name` is `j.giffard@icloud.com` because Linear's profile
      name defaults to the email and the API returns it as `name`; set
      a display name in Linear Settings → Profile if it matters on
      stage — see the quirk note in `DEMO-ENV.md`.)
- [x] 2.5 Decide the live-demo graph final state: seeded story graph
      (canonical), optionally layered with a run of real sync. If
      layered, document how to reset between demo sessions
      (`MATCH (n) DETACH DELETE n` + re-seed, or a second database).
      Decision: **the canonical seeded graph is the final demo state**
      — `sample-data.cypher` tells the story; the real-data run is a
      build-time verification, not part of the show. DB was restored to
      canonical state on 2026-08-27 after the real-data run (wipe →
      schema 16/16 → sample-data 21/21 → link-explicit 5 rows;
      `agent-context NODES-1` re-verified: 2 discussions, Thread 4
      orphaned, Sarah Chen people). Reset between seeded/real states is
      the documented one-liner in `DEMO-ENV.md`; the full real-data
      recipe (wipe → run → verify → restore) is there too.

Acceptance: sync log matches the README's expected output shape; the
agent query returns populated discussions from **real** data; reset
between seeded/real states is a documented one-liner.

## Phase 3 — Slack plugin bootstrap (half day, after Phase 1) — done 2026-08-27 (search query pending a Slack scope, see 3.5)

- [x] 3.1 Scaffold: `wgc router plugin init slack -p demo-router
      --language go` in `staging/` (2026-08-27). Note: the scaffold
      produces a **complete router project** (config.yaml, graph.yaml,
      Makefile, `plugins/slack`), not just a plugin. No auth needed for
      local scaffolding.
- [x] 3.2 Ported into the scaffold: `src/schema.graphql` (plain GraphQL —
      the federation `@key`/`@link` block was dropped; the router does
      not run federation introspection against Connect plugins),
      `src/slackclient/client.go` (ported as-is from the old
      `internal/slack/client.go`, slack-go v0.15.0, mockable `Client`
      interface), `src/service.go` (rewritten for the generated types:
      optional args are protobuf wrappers — nil = absent → schema
      defaults 50/20; `ErrNotFound` → empty/nil result, not error; empty
      domain values → nil wrappers so GraphQL renders `null`),
      `src/main.go` (scaffold pattern: `routerplugin.NewRouterPlugin` +
      `WithTracing` + `pl.Serve()`; fatal without `SLACK_BOT_TOKEN`).
- [x] 3.3 `wgc router plugin build . --debug` → `bin/darwin_arm64`;
      `wgc router plugin test .` green; `go build` / `go vet` / `go test`
      green (12 bufconn tests with a mock client, no network). The fake
      `router-plugin v0.0.0` dependency is gone (real v0.4.1
      pseudo-version from the scaffold's go.mod).
- [x] 3.4 Versions pinned in `router/README.md` +
      `router/plugins/slack/README.md` and go.sum: wgc 0.130.1, router
      0.343.1, router-plugin v0.4.1 (`v0.0.0-20250824152218-8eebc34c4995`),
      slack-go v0.15.0, Go target 1.25.1 (built 1.26.5), protoc 29.3,
      protoc-gen-go v1.34.2, protoc-gen-go-grpc v1.5.1. protoc **must** be
      29-series — brew's 36.0 is rejected (`found 36.0, required ^29.3`);
      download URL + PATH export are documented in both READMEs.
- [x] 3.5 Live queries through `http://localhost:3010/graphql` with
      `SLACK_BOT_TOKEN` against the real workspace — **all 4 verified
      2026-08-27, re-verified 2026-08-28 after the search drop**:
      `slackChannel(C0BSX7Q9M0E)` (name/topic/purpose/members),
      `slackMessages(limit: 3)`, `slackThread(1787845145.093149)` (full
      4-message chain with permalinks), `slackUser(U0BSZ59TY66)` (email
      returned). The 5th query, `searchSlackMessages`, was dropped
      2026-08-28: `search.messages` is user-token-only in Slack (the
      `search:read` scope is legacy, supported token type = User), so a
      bot-token subgraph can never serve it — see Known issue 4.
- [x] 3.6 Finished project moved to **`router/`** (renamed from
      `staging/demo-router/` — it is the local *router* project; Phase 4
      adds the Linear/Neo4j subgraphs and MCP config here; decision
      documented in `router/README.md`). Old hand-written `slack-subgraph/`
      deleted (code ported; design rationale folded into the plugin
      README); `staging/` deleted; both READMEs rewritten to describe what
      actually worked.

Acceptance: `wgc router plugin test` green ✓; live queries return real
channel/thread/message data through the local router ✓ (all 4; the 5th
was dropped — Slack platform constraint, Known issue 4).

## Phase 4 — Router config + persisted operations + MCP Gateway (half–1 day)

The federation-registration question (Known issue 3) is answered here,
first.

- [x] 4.1 **Experiment A (do first, ~30 min).** DONE 2026-08-28 —
      **not rejected**: router 0.343.1 composes plain-GraphQL subgraphs
      (see Known issue 3 for the full recipe). Wiring as built:
      - `neo4j-api/` — small Go plain-GraphQL HTTP service on
        `127.0.0.1:4400` (user chose Go over the Node.js
        `@neo4j/graphql` option — **switched to the Node.js library
        server 2026-08-28, see 4.5**; the Go service is kept as an
        unwired fallback). `graphql-go` + `neo4j-go-driver
        v5.27.0`; root fields `getIssueDiscussionContext` +
        `searchMessages`, both verified live against the local graph.
        Notes: `MustParseSchema(..., graphql.UseFieldResolvers())` is
        required for struct-field resolution; nullable Go fields must
        be pointers (lists made `[X]!` in the SDL); driver decodes
        `datetime` to `time.Time` (formatted RFC3339); `//go:embed` is
        rejected by Go 1.26.5 on this machine — files load via
        `loadAsset` (CWD → exe dir → exe parent).
      - `router/graph.yaml.template` (committed, `__LINEAR_API_KEY__`
        placeholder) → `make render-graph` seds the key from `.env`
        into gitignored `router/graph.yaml`; `make compose` renders
        then runs `wgc router compose`.
      - `router/config.yaml` `headers.subgraphs.linear.request` sets
        `Authorization: "${LINEAR_API_KEY}"` (env expansion verified
        live in 0.343.1 — no `config.local.yaml` fallback needed).
      - `make start` sources `../.env` and runs the router in the
        foreground; `make neo4j-api` builds + runs the subgraph
        service in the foreground. Two terminals for the demo.
      - Verified live 2026-08-28 through `localhost:3010/graphql`: all
        4 Slack queries, Linear `projects(first:2)` (real project),
        neo4j `getIssueDiscussionContext("NODES-1")` (2 discussions),
        and a single combined request touching all three subgraphs.
- [x] 4.1b **Fallback: NOT NEEDED** (4.1 succeeded 2026-08-28 —
      documented in Known issue 3).
- [x] 4.2 Define persisted operations as files (`.graphql`).
      Done 2026-08-28: four operations in `router/operations/` —
      `GetIssueDiscussionContext` (the closing-demo query),
      `SearchMessages` (the demo's only message-search surface),
      `LinearProjectIssues` (live Linear, **zero-arg** — the demo
      project ID is hardcoded in the operation because router 0.343.1
      marks non-null variables `required` in the MCP input schema even
      when they carry a default, so a default-valued variable could not
      be called with no arguments), `SlackThread` (live Slack,
      channelId + threadTs — its description lists the four *real*
      seeded thread ts values, because the canonical graph carries
      fictional thread ts that do not exist in the real workspace).
      Wiring: `mcp.storage.provider_id: mcp` + top-level
      `storage_providers.file_system: [{id: mcp, path: operations}]`
      (all keys verified against the router config schema). Note on
      `security.block_non_persisted_operations`: deliberately **not**
      enabled globally — the "MCP surface exposes only the named
      operations" guarantee already comes from `mcp.storage` +
      `enable_arbitrary_operations: false`, and blocking non-persisted
      operations would also kill ad-hoc `:3010` playground queries used
      for live debugging. One-line config change if the stricter claim
      is wanted on stage.
- [x] 4.3 Enable the MCP Gateway in the router config; verify with an MCP
      client. Done 2026-08-28: `mcp.enabled: true`,
      `server.listen_addr: localhost:5025`, `exclude_mutations: true`,
      `enable_arbitrary_operations: false`, `expose_schema: false`,
      `omit_tool_name_prefix: true`, `graph_name: nodes2026`. Verified
      with raw JSON-RPC (streamable HTTP, SSE bodies): `tools/list`
      returns exactly the four operations + built-in `get_operation_info`
      — not 60+ raw fields — and `tools/call` round-trips all four:
      NODES-1 → issue + 2 discussions + all messages from the local
      graph (zero external calls); `search_messages(cache)` → 3 hits
      with channel/thread/permalink; `linear_project_issues` → 6 real
      issues (NEO-10..15, live states); `slack_thread` (t1) → 4 live
      messages. The endpoint is stateless — a bare curl `tools/list`
      works without the initialize/session dance (runbook one-liner in
      `DEMO-ENV.md`).
- [x] 4.4 Write `DEMO-ENV.md` router section. Done 2026-08-28:
      three-subgraph layout, two-terminal start (`make neo4j-api` +
      `make start`), port table (3010 GraphQL/playground, 5025 MCP,
      4400 neo4j-api, 8088 Prometheus), the five-tool table, curl smoke
      tests, the Claude Desktop MCP client snippet (Phase 5.1 input),
      and stop/regenerate procedures. (Updated for 4.5: `make
      neo4j-srv`, port 4000.)
- [x] 4.5 Switch the neo4j subgraph from `neo4j-api/` (Go, :4400) to
      `neo4jGraphQLSrv/` (`@neo4j/graphql` 7.6.2 + Yoga, :4000).
      Done 2026-08-28 (user-approved): the hand-written typeDefs in
      `neo4jGraphQLSrv/schema.graphql` are the single source of truth.
      **Rename:** `Issue`/`Project` are also Linear type names and wgc
      hard-fails composing shared type names across subgraphs, so the
      demo types became `GraphIssue`/`GraphProject` with `@node(labels:)`
      mapping them back to the graph labels (all 6 types got explicit
      `@node(labels:)`). **Two `@cypher` fields** (v7 signature
      `@cypher(statement: String!, columnName: String!)`, plain `type
      Query`, `RETURN {…} AS result` per row for object lists):
      `GraphIssue.discussionDetails` — the `DISCUSSED_IN`
      confidence/evidence/permalink (load-bearing talk content) kept
      separate from the relationship field `discussedInThreads` and
      correlated by `threadTs` — and root `searchMessagesCI`, the
      demo's only search surface, case-insensitive because the
      library's `contains` filter compiles to plain `CONTAINS`, which
      is case-sensitive on Neo4j 2026.x (Known issue 6); its
      `LIMIT toInteger(coalesce($limit, 20))` avoids the driver
      float/null param rejection. **Generated SDL:** new
      `gen-plain-schema.cjs` (`new Neo4jGraphQL({typeDefs, driver:
      null})` → `printSchema(await getSchema())`) writes the gitignored
      `neo4j-plain-schema.graphql` (printSchema strips the `@cypher`
      directives — clean plain SDL); `graph.yaml.template` registers it
      via `schema: {file: …}` and `compose` depends on `make
      gen-neo4j-schema`. Both graph operations were rewritten against
      the new plural root fields (not-found = empty list, not null).
      Verified 31/31 MCP JSON-RPC checks (5 tools + schemas, NODES-1
      structure with 2 correlated `discussionDetails`, NODES-99 empty,
      search hits, case-insensitivity, limit honored, Linear/Slack
      round-trips) plus a combined 3-subgraph request through :3010.
      `neo4j-api/` kept as an unwired fallback.

Acceptance: MCP client discovers only persisted operations; calling
`getIssueDecisionContext`/`getIssueDiscussionContext` for NODES-1 returns
issue + threads + messages in one call against the local graph, with
zero external network calls.

## Phase 5 — Agent harness + contrast demo (a few hours, after Phase 4)

- [x] 5.1 Config A — **federated**: single MCP server at the local router
      MCP endpoint. `mcp-profiles/federated.json` uses the verified
      `npx mcp-remote http://localhost:5025/mcp` stdio bridge (the form in
      the live config that works; bearer token not needed locally).
      **Verified headless 2026-08-28** through that exact bridge:
      initialize + `tools/list` (exactly 5 tools) + `tools/call` round-trip.
      GUI check (puzzle-piece icon shows one server / five tools) is part
      of the user-driven 5.3/5.4.
- [x] 5.2 Config B — **world without federation**: `mcp-profiles/no-federation.json`
      registers three separate MCP servers — Linear (official hosted MCP,
      `url` + `Authorization: Bearer` — the MCP endpoint accepts the Bearer
      prefix even though the raw GraphQL API rejects it), Slack
      (`slack-mcp-server@1.3.0` stdio, bot token, `--no-cache` — see Known
      issue 7), Neo4j (local canary binary, `NEO4J_READ_ONLY=true`, read
      tool named `read-cypher`). **All three verified headless
      2026-08-28** (initialize + tool lists + real-data `tools/call`s;
      Slack has 13 tools and no message-search tool — Known issue 4 in
      action, a feature for the contrast). Profile files carry
      `__PLACEHOLDERS__`; `scripts/switch-mcp-profile.sh` substitutes from
      `.env`, backs up the live config once, replaces only `mcpServers`,
      and `show`/`restore` subcommands are provided. Profile B needs
      internet for the Linear server. GUI check is user-driven (5.3/5.4).
- [ ] 5.3 Rehearse both demo prompts from `TALK.md` (Prompt 1 federated;
      Prompt 2 the orphaned-thread cliffhanger) and capture the tool-call
      traces — they're the exhibit for the "context window impact" slide
      (20+ tools → handful of operations). **Headless rehearsal done
      2026-08-28:** Prompt 1 through the bridge returns the full NODES-1
      context in one call (issue + 2 threads / 7 messages + 2
      `discussionDetails` confidence 1 / `explicit_mention`); expected
      synthesis + rehearsal steps documented in `DEMO-ENV.md` → "Phase 5 →
      Prompt rehearsal". Remaining: the GUI runs + screenshots (user).
- [ ] 5.4 First full end-to-end dry run, timed. Capture screenshots of
      both profiles as fallback material.

Acceptance: prompt 1 via the federated endpoint produces the expected
synthesis ("explicit @shareable annotations…") in one persisted-operation
call; the contrast run visibly makes more, weaker tool calls; the whole
segment is under 8 minutes.

## Phase 6 — Iteration 2: vectors (started 2026-08-28)

Follow the iteration-2 design in `data-model/README.md`
(Option 1: embed in the sync pipeline, LM Studio local, write vectors as
properties). **Decision (2026-08-28, ahead of the original end-of-September
target):** iteration 2 is in the live demo, via a background `--watch`
poll rather than a manual on-stage trigger — post to Slack/Linear during
the talk and the graph updates without anyone running a command. See
`sync/README.md` → "Live demo mode".

- [x] 6.1 Pick the embedding model in LM Studio and pin it. **Done
      2026-08-28: nomic-embed-text-v1.5@q8_0 (768 dim)**, downloaded via
      `lms` CLI and running (`lms status` confirms server ON at :1234,
      model loaded; `curl :1234/v1/embeddings` verified live — 768-dim
      vectors, matches `schema.cypher`'s indexes exactly). Exact model
      identifier per `curl :1234/v1/models`:
      `text-embedding-nomic-embed-text-v1.5@q8_0` — the `@q8_0` suffix is
      required in `EMBEDDING_MODEL`, not just the base name.
- [x] 6.2 Embedding client added: `sync/internal/embed/client.go` — batched
      (32 texts/request), 4 attempts exponential backoff, calls an
      OpenAI-compatible `/v1/embeddings` endpoint. Writer methods
      `EmbedIssue`/`EmbedMessage` write via
      `db.create.setNodeVectorProperty` (`embed-issue.cypher`,
      `embed-message.cypher`); `AllIssueTexts`/`AllMessageTexts` feed them
      from the graph itself (not the current tick's pulled data), so the
      stage is self-contained — same behaviour whether the graph was just
      synced or not. `orchestrator.Run` takes a nilable `Embedder`; nil
      skips the stage entirely (iteration 1 behaviour, unchanged, verified
      by the existing e2e test still passing).
- [x] 6.3 `schema.cypher`: both `CREATE VECTOR INDEX` statements added
      (`issue_embedding`, `message_embedding`, 768 dim, cosine). Verified
      live against the local container (Neo4j 2026.07.1) — index creation,
      `db.create.setNodeVectorProperty`, and `db.index.vector.queryNodes`
      all work with no gotchas beyond the dimension lock itself.
- [x] 6.4 `queries/link-semantic.cypher` added: `CALL (i) { ... }`
      variable-scoped subquery (works on 2026.07.1) over
      `db.index.vector.queryNodes('message_embedding', $topK, i.embedding)`,
      `WHERE score >= $threshold`, `MERGE ... ON CREATE SET` so an
      existing explicit_mention edge is never downgraded. topK defaults
      to 5 (`SEMANTIC_LINK_TOPK`), threshold to **0.78**
      (`SEMANTIC_LINK_THRESHOLD`, raised from an initial 0.75 — see 6.5).
      Verified with synthetic vectors: live against the canonical seeded
      graph (Thread 4 → NODES-1 at confidence ~0.9998; existing explicit
      edges on NODES-3/5/6 confirmed untouched) and in a permanent
      regression test, `TestEmbedAndSemanticLinking_EndToEnd` in
      `sync/internal/graph/writer_e2e_test.go` (gated by `WRITER_E2E=1`) —
      seeds an orphaned thread, embeds it near an issue's embedding,
      confirms the semantic edge forms, confirms a dissimilar thread does
      NOT get linked (proves the threshold actually filters, not just
      topK), and confirms re-running is idempotent. Passes twice in a row.
- [x] 6.5 **Run with real embeddings — done 2026-08-28, and it surfaced a
      real design problem, now resolved.** `sync` run live (real Linear +
      real Slack, `EMBEDDING_MODEL=text-embedding-nomic-embed-text-v1.5@q8_0`)
      against the same database as the canonical seed graph: the real
      workspace mirrors the seed story closely enough (NEO-10 ≈ NODES-1,
      etc.) that cross-links exploded between the fictional and real
      versions of the same issues — not a threshold problem, a corpus
      problem. **Fix: `livedemo`, a separate Neo4j database** (this
      container is Enterprise, multi-database works: `CREATE DATABASE
      livedemo`), created and schema-applied, so the live segment's real
      data never coexists with the canonical seed graph. Re-run against
      the isolated `livedemo` (8 real issues, 4 real threads): the
      intended moment held — a real orphaned thread linked to NEO-10 at
      confidence 0.780, evidence `semantic_match` — proving the exact
      mechanism the talk depends on works with real embeddings, not just
      synthetic ones. **But even isolated, 7 further spurious links
      appeared** (confidence 0.759–0.779) among unrelated real issues —
      at this corpus's scale, short same-project engineering text
      clusters too tightly for any single fixed threshold to cleanly
      separate signal from noise; 0.78 (tuned against the 6-issue
      canonical corpus in isolation) does not generalize to the 8-issue
      real corpus. **Decision (2026-08-28): ship as-is, accept the noise,
      do not build top-1/reranking logic before the talk** — user's call
      after being shown the exact numbers. If asked in Q&A why extra
      links appear alongside the intended one, that's the honest answer:
      it's the corpus-scale version of the "0.75 is a heuristic, not A/B
      tested" tension `TALK.md` already plans to name on stage.
- [x] 6.6 **Decision (2026-08-28, moved up from end of September):**
      live in the demo, via `--watch` background polling (see above),
      not a manual on-stage trigger and not slides-only.

Acceptance: Thread 4 linked by the semantic pass only; rerunning the
pass is idempotent (no duplicate edges, `MERGE` on the relationship). —
**Met, with real embeddings, in the isolated `livedemo` database** (see
6.5). Not met in the literal sense of "only" — extra links also appear at
real-workspace scale, accepted as a known tradeoff, not a defect.

### Known issue 9: real workspace data and canonical seed data must never share a database

Discovered 2026-08-28 running Phase 6.5: the real Linear/Slack workspace
mirrors the canonical seed story closely enough (NEO-10 ≈ NODES-1, etc.)
that embedding both into the same database floods the semantic pass with
cross-links between the fictional and real versions of the same issues.
**Fixed by giving the live-embedding demo its own database (`livedemo`,
created via `CREATE DATABASE livedemo`, Enterprise-only), never the
default `neo4j` database the canonical seed graph lives in.** See
`DEMO-ENV.md` → "Live embedding demo" for the full runbook, including the
integration wrinkle this creates: the router's neo4j subgraph
(`neo4jGraphQLSrv`) points at the `neo4j` database, so `livedemo`'s data
isn't visible through `get_issue_discussion_context` without restarting
that server against `livedemo` first (which then breaks the *main* demo
segment until restarted back) — query `livedemo` directly instead during
this segment.

### Known issue 10: Slack rate limits under `--watch`, unverified

Slack has tightened `conversations.history`/`conversations.replies`
limits for non-Marketplace apps in the past; this demo's Slack app is a
fresh single-workspace bot, not Marketplace-listed. Polling every
`--interval` (default 20s) during a live segment could hit a 429. A
failed tick logs and retries next interval rather than crashing the
`--watch` loop, but repeated 429s during the live segment would still
look bad even if nothing breaks outright. **Not yet verified against the
real Slack app's actual tier limits** — do a real timed `--watch` dry run
before the talk and watch the log for `status 429`; lengthen `--interval`
if it appears. Linear's limits are generous enough not to worry about.

## Phase 7 — Talk deliverables (through October)

- [ ] 7.1 Slides: 8 sections per the `TALK.md` working outline, sentence
      case headings, no "leverage". The AWS/Fargate/Cosmo-Cloud
      material is one "where this runs in production" slide — the demo
      environment slides show the local picture (one laptop: podman
      Neo4j + local router + plugin + three subgraphs, two of them SaaS).
- [ ] 7.2 Speaker notes with both demo prompts verbatim.
- [ ] 7.3 ≥3 full dry runs (one at the venue or an equivalent wifi
      environment), plus the backup video: same demo, recorded
      (authenticity over polish, per the open questions lean).
- [ ] 7.4 Doc cleanup: `TALK.md` build status (sync done; local-first
      architecture), architecture section, root README quick-start and
      component-status list, `slack-subgraph/README.md` bootstrap,
      sync README (Aura → local Neo4j URI).
- [ ] 7.5 Morning-of checklist: `podman start nodes-demo-neo4j`, router
      up, both Claude profiles tested with Prompt 1, backup video
      accessible, offline mode known (main segment needs no internet).

---

## Timeline (from 2026-08-27)

| Window | Goal |
|--------|------|
| Aug 31 – Sep 4 | Phase 0 + Phase 1. Local environment running, Cypher fixed and verified. |
| Sep 7 – 11 | Phase 2 (real sync) + Phase 3 (plugin). |
| Sep 14 – 18 | Phase 4. Federated MCP endpoint working locally end-to-end. |
| Sep 21 – 25 | Phase 5. Agent harness, contrast demo, first timed dry run. |
| Sep 28 – Oct 9 | Phase 6 (vectors) + live-vs-slides decision; slides drafting. |
| Oct 12 – 23 | Slides finished; dry runs 2–3; backup video recorded. |
| Oct 26 – Nov 8 | Venue-style rehearsal; final notes; morning-of checklist dry run. |
| Nov 12 | Talk. |

The critical path is Phase 4's federation-registration experiment — it
starts the moment Phase 1's local environment exists, so it costs no
calendar time on top.

## Risks (carried from TALK.md, updated)

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Cosmo router rejects non-federation subgraphs (Linear, plain-GraphQL Neo4j) | Medium | Medium | Fallback 4.1b wrapper plugins; design parallel to experiment A |
| Local-mode MCP + persisted-ops wiring differs from docs | Low | Low | Verify in 4.2/4.3; worst case: point the local router at a free Cosmo Cloud account as control plane only (the one cloud piece the local design can absorb without restructuring) |
| Live demo wifi failure | Low | High | Main segment makes zero external calls (graph-resolved); contrast segment has recorded fallback; recorded video substitutes at any point |
| Neo4j container state drifts across sessions | Low | Medium | Documented reset one-liner (Phase 2.5); demo data is small and re-seedable in seconds |
| wgc/Cosmo API changes before November | Medium | Medium | Pin wgc 0.130.1 + router version in the scaffold and README; build is reproducible |
| LM Studio crash mid-demo (iteration 2) | Medium | Medium | Embeddings pre-computed and stored; semantic pass already ran; decision point end of September |
| Claude Desktop / connector behaviour changes | Medium | Medium | Test morning-of; screenshots as fallback |

(Removed from the original register: Aura free-tier limits and pausing —
no longer in the demo path.)
