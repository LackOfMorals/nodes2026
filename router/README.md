# router — local Cosmo Router project (NODES 2026 demo)

The local Cosmo Router for the talk demo. Everything runs on the speaker's
laptop — no Cosmo Cloud control plane, no graph token, no plugin-registry
publish. The router loads subgraph **plugins** (Cosmo Connect) in-process and
exposes one GraphQL endpoint at `http://localhost:3010/graphql` (playground
at `http://localhost:3010`).

Subgraphs registered (Phase 4.1, verified live 2026-08-28):

- `plugins/slack` — Slack Web API wrapped as a Cosmo Connect plugin
  (see `plugins/slack/README.md`).
- `linear` — Linear's native GraphQL API, registered directly as a raw
  HTTP subgraph (`introspection.raw: true`). The Phase 4 experiment
  (Known issue 3 in `../PLAN.md`) **succeeded**: the router composes
  plain-GraphQL subgraphs, no wrapper plugin needed.
- `neo4j` — the local `../neo4jGraphQLSrv` node server
  (`@neo4j/graphql` 7.6.2 + Yoga, `127.0.0.1:4000/graphql`), registered
  from a generated plain-SDL file (see Decisions below). The old
  `../neo4j-api` Go service (:4400) is kept as an unwired fallback.

MCP Gateway (Phase 4.2–4.4, verified live 2026-08-28): the router also
serves a Model Context Protocol endpoint at `http://localhost:5025/mcp`
(streamable HTTP) exposing only the four persisted operations in
`operations/` — `get_issue_discussion_context`, `search_messages`,
`linear_project_issues`, `slack_thread` — plus the built-in
`get_operation_info`. An MCP client never sees the 60+ raw supergraph
fields. Runbook: `../DEMO-ENV.md` → "Router (Phase 4 …)".

## Layout

```
router/
├── README.md            # this file
├── config.yaml          # router config (listen addr, dev mode, plugin dir,
│                        #   outbound header rules — ${ENV} expansion works)
├── graph.yaml.template  # committed subgraph registry (__LINEAR_API_KEY__ placeholder)
├── graph.yaml           # rendered from the template (gitignored — holds the key)
├── config.json          # generated supergraph execution config (gitignored)
├── operations/          # MCP persisted operations (one named op per .graphql file)
├── release/router       # downloaded Cosmo Router binary (gitignored)
├── Makefile             # render-graph / compose / build / start / neo4j-srv /
│                        #   gen-neo4j-schema (compose depends on it)
└── plugins/
    └── slack/           # the Slack Cosmo Connect plugin (Go)
```

## Prerequisites

- Go 1.25+ (built with 1.26.5)
- Node 26 (for `npx -y wgc@0.130.1`)
- **protoc 29.x — strictly 29-series.** `wgc router plugin generate` rejects
  any other major (verified: brew's `protobuf` formula ships protoc 36.0 and
  is refused with `found 36.0, required ^29.3`). This project is
  self-contained: `tools/protoc-29.3/` (gitignored) holds a pinned protoc,
  `make install-tools` downloads it from the protobuf release page if
  missing, and **both Makefiles put it first on PATH automatically** — no
  manual export needed. One-time host setup (still required):

  ```bash
  go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.34.2
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1   # picked up from $HOME/go/bin
  ```

  Why the Makefile does this (the "wgc toolchain-check regression",
  resolved 2026-08-27): the toolchain check runs in whatever shell starts
  `make`. On this laptop a bare shell only sees Homebrew's protoc 36.0,
  so the check fails, wgc prints `Install required toolchain? (y/N)` and
  exits non-zero — and on non-TTY stdin that prompt hangs/dies with
  `unsettled top-level await`. Pinning the tool in-tree and exporting PATH
  per-recipe removes the failure mode entirely. (Symptom if it ever
  recurs: `wgc router plugin build|test` fails without running the tests,
  output mentions `protoc version mismatch on host: found 36.0`.)
- The repo-root `.env` holds every secret (`SLACK_BOT_TOKEN`,
  `LINEAR_API_KEY`, `NEO4J_URI/USER/PASSWORD/DATABASE`). `make start`
  and `make neo4j-srv` source it automatically; for manual runs use
  `set -a; . ../.env; set +a`. Fresh checkout only: the neo4j subgraph
  server needs `cd neo4jGraphQLSrv && npm install` once.

## Build and run (two terminals)

```bash
# terminal 1 — the Neo4j GraphQL library server (POST :4000/graphql)
cd router
make neo4j-srv

# terminal 2 — the router on :3010
cd router
make start   # = download + build (Slack plugin) + compose (rendered graph), foreground
```

`make start` downloads the pinned router binary if missing, builds the
Slack plugin (`bin/darwin_arm64`), renders `graph.yaml` from the committed
template (inlining `LINEAR_API_KEY` from `.env`), composes the supergraph
into `config.json`, and runs the router on `localhost:3010`. Individual
steps: `make download | build | render-graph | compose`.

The router serves the three subgraphs in one supergraph; Linear calls go
out to `api.linear.app` (auth via the `headers` block in `config.yaml`,
which expands `${LINEAR_API_KEY}` from the environment — verified working
in router 0.343.1), Slack calls go out to Slack's API, and neo4j calls go
locally to `127.0.0.1:4000`.

The Slack plugin's own workflow lives in `plugins/slack/`
(`make generate | build | test` there), and its README documents the
bootstrap that actually worked, including the version pins below.

## Pinned versions (verified 2026-08-27)

| Component | Version | Where pinned |
|-----------|---------|--------------|
| wgc CLI | 0.130.1 | `npx -y wgc@0.130.1` (Makefiles use `wgc` if installed) |
| Cosmo Router | 0.343.1 | downloaded by `make download` (release tag `router@0.343.1`) |
| router-plugin runtime | v0.4.1 (`v0.0.0-20250824152218-8eebc34c4995`) | `plugins/slack/go.mod` |
| slack-go | v0.15.0 | `plugins/slack/go.mod` |
| Go module target | 1.25.1 | `plugins/slack/go.mod` |
| protoc | 29.3 | PATH (see Prerequisites) |
| protoc-gen-go | v1.34.2 | PATH (`~/go/bin`) |
| protoc-gen-go-grpc | v1.5.1 | PATH (`~/go/bin`) |
| neo4jGraphQLSrv Node | 26.5.0 | host Node |
| @neo4j/graphql | 7.6.2 | `neo4jGraphQLSrv/package.json` |
| graphql-yoga | 5.22.0 | `neo4jGraphQLSrv/package.json` |
| neo4j-driver | 5.28.3 | `neo4jGraphQLSrv/package.json` |

go.sum in `plugins/slack/` locks the rest.

## Verified live

**Phase 4.1 (2026-08-28):** with all three subgraphs composed, through
`http://localhost:3010/graphql` —

- **Slack** (verified 2026-08-27, re-verified 2026-08-28): `slackChannel`,
  `slackMessages`, `slackThread`, `slackUser` return real data from the
  "Nodes 2026 Demo" workspace (channel `C0BSX7Q9M0E`, thread
  `1787845145.093149` with 4 messages, user `U0BSZ59TY66` with email).
  A fifth query (`searchSlackMessages`) existed until 2026-08-28 and was
  dropped: Slack's `search.messages` only accepts user tokens
  (`search:read` is a legacy, User-only scope), so a bot-token plugin can
  never serve it — see `plugins/slack/README.md` → Scopes and PLAN.md
  Known issue 4.
- **Linear**: `projects(first: 2)` returns the real "NODES 2026 Demo"
  project (`1b3763ee-…`). Auth via the `config.yaml` `headers` rule —
  `${LINEAR_API_KEY}` env expansion works in router 0.343.1, and the key
  appears nowhere in the composed `config.json`.
- **Neo4j** (re-verified 2026-08-28 after the switch to the
  `@neo4j/graphql` server): `graphIssues(where: {identifier: {eq:
  "NODES-1"}}, limit: 1)` returns the issue with `discussedInThreads`
  (channel, ts) and `discussionDetails` (the `DISCUSSED_IN`
  confidence/evidence per thread, correlated by `threadTs`);
  `searchMessagesCI` returns case-insensitive hits with
  author/channel/thread/permalink.
- **Combined**: a single request touching all three subgraphs
  (Linear `projects` + `slackChannel` + `graphIssues` +
  `searchMessagesCI`) returns everything in one response.

**Phase 4.2–4.4 (2026-08-28):** MCP Gateway on `localhost:5025` —

- `tools/list` (stateless; works with a bare curl, no
  initialize/session dance) returns exactly the four operations +
  built-in `get_operation_info`.
- `tools/call` round-trips all four: `get_issue_discussion_context`
  (`NODES-1`) → issue + 2 discussions + all messages, graph-only (zero
  external calls); `search_messages` (`cache`) → 3 hits with
  channel/thread/permalink; `linear_project_issues` (zero-arg) → 6 real
  issues NEO-10..15 with live states; `slack_thread` (t1
  `1787845145.093149`) → 4 live messages from the real workspace.
- Operation-file gotcha recorded: the router marks non-null variables
  `required` in the tool input schema even when they have a default —
  so `LinearProjectIssues` hardcodes the demo project ID instead of
  taking a defaulted variable.

## Decisions (documented per PLAN.md Phase 3)

- **Directory renamed `staging/demo-router` → `router/`.** The scaffold
  (`wgc router plugin init slack -p demo-router`) produces a complete router
  project, not just a plugin; Phase 4 adds more subgraphs here. The old
  hand-written `slack-subgraph/` layout was replaced — its client code was
  ported to `plugins/slack/src/slackclient/`, its service logic rewritten
  for the generated types, and its design rationale folded into
  `plugins/slack/README.md`.
- **Plain GraphQL schema in the plugin** (no `@key`/`extend schema @link`
  federation directives). Cosmo Connect plugins serve plain GraphQL over
  the in-process gRPC contract; the router does not run federation
  introspection against them. Federation only becomes a question for raw
  HTTP subgraphs in Phase 4.
- **Default wgc module path kept** (`github.com/wundergraph/cosmo/plugin`);
  the scaffold's fake `router-plugin v0.0.0` dependency was replaced by the
  real `v0.4.1` pseudo-version.
- **Neo4j subgraph switched from the Go `neo4j-api/` service to the
  `@neo4j/graphql` (Yoga) library server (2026-08-28).** The
  hand-written typeDefs in `neo4jGraphQLSrv/schema.graphql` are the
  single source of truth; `Issue`/`Project` were renamed
  `GraphIssue`/`GraphProject` (`@node(labels:)` maps them back to the
  graph labels) because the raw names collide with Linear's types when
  wgc composes the supergraph — shared type names across subgraphs
  hard-fail the composition. Two `@cypher` fields cover what the
  generated schema can't express: `GraphIssue.discussionDetails` (the
  `DISCUSSED_IN` confidence/evidence — load-bearing talk content) and
  root `searchMessagesCI` (case-insensitive search; Neo4j 2026.x
  `CONTAINS` is case-sensitive). `make gen-neo4j-schema` (via
  `neo4jGraphQLSrv/gen-plain-schema.cjs`, `printSchema`) writes the
  library's schema as plain SDL to the gitignored
  `neo4jGraphQLSrv/neo4j-plain-schema.graphql` (printSchema strips the
  `@cypher` directives), which `graph.yaml.template` registers via
  `schema: {file: …}`; `compose` depends on `gen-neo4j-schema`. The Go
  service (`neo4j-api/`, :4400) is kept as an unwired fallback. Both
  graph operations (`get-issue-discussion-context`, `search-messages`)
  were rewritten against the new field names; 31/31 MCP checks pass.
