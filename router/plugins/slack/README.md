# Slack subgraph — Cosmo Connect plugin

A Cosmo Connect plugin that exposes a slice of Slack as a GraphQL subgraph
inside the Cosmo Router. This is one of the three subgraphs in the NODES 2026
demo:

- **Linear** — native GraphQL API (Phase 4: direct registration or thin plugin)
- **Neo4j** — the agent-context query over bolt (Phase 4)
- **Slack** (this plugin) — REST-only Web API wrapped for the router via
  Cosmo Connect

The plugin runs **inside the Cosmo Router process** as a HashiCorp go-plugin:
the router loads `bin/darwin_arm64`, performs the plugin handshake over gRPC,
and routes queries that touch the Slack types to this plugin.

## What this plugin does

The schema (`src/schema.graphql`) declares four queries:

- `slackChannel(id)` — channel metadata
- `slackMessages(channelId, limit = 50)` — recent messages
- `slackThread(channelId, threadTs)` — full thread reply chain
- `slackUser(id)` — user lookup

Scoping is **not** enforced in this plugin — it is enforced at the gateway
via persisted operations (Phase 4 MCP Gateway). The plugin happily queries
whatever the bot token can see; the gateway controls what the LLM can ask.

## Layout

```
plugins/slack/
├── README.md
├── go.mod / go.sum            # module github.com/wundergraph/cosmo/plugin
├── Makefile                   # generate / build / test / publish (wgc)
├── Dockerfile                 # image build (unused by the local demo)
├── src/
│   ├── schema.graphql         # source of truth for the API
│   ├── main.go                # entry point: loads SLACK_BOT_TOKEN, serves plugin
│   ├── service.go             # gRPC resolvers → slackclient.Client
│   ├── main_test.go           # bufconn gRPC tests with a mock client
│   └── slackclient/
│       └── client.go          # slack-go wrapper + Client interface (mockable)
├── generated/                 # wgc-generated protobuf + gRPC (do not hand-edit)
└── bin/                       # built plugin binary (gitignored)
```

## Bootstrap — what actually worked (wgc 0.130.1, verified 2026-08-27)

This directory was scaffolded with:

```bash
wgc router plugin init slack -p demo-router --language go
```

Notes from doing it for real:

- `wgc router plugin init` scaffolds a **complete router project**
  (`demo-router/`: config.yaml, graph.yaml, Makefile, `plugins/slack`), not
  just a plugin. No auth/login needed for local scaffolding.
- `wgc router plugin generate .` (run **from this plugin directory**) reads
  `src/schema.graphql`, emits `generated/service.proto`, and runs protoc.
  **protoc must be 29.x** — the check is a strict series match
  (`found 36.0, required ^29.3` rejects brew's protobuf formula). The
  router project keeps a pinned protoc in `../../tools/protoc-29.3/`
  (auto-downloaded by `make install-tools` if missing) and the Makefiles
  put it first on PATH for every `wgc` invocation — **no manual export**.
  - protoc 29.3: `https://github.com/protocolbuffers/protobuf/releases/download/v29.3/protoc-29.3-osx-aarch_64.zip`
  - `protoc-gen-go` v1.34.2, `protoc-gen-go-grpc` v1.5.1 (`go install …@version`, picked up from `$HOME/go/bin`)
  - wgc itself: global `wgc` (0.130.1) or `npx -y wgc@0.130.1`
- Generated optional args/fields come back as **protobuf wrappers**
  (`*wrapperspb.Int32Value` for `limit`, and for all nullable output fields).
  `src/service.go` treats a nil wrapper as "absent" and applies the schema
  default; empty domain values are converted to nil wrappers so GraphQL
  renders `null`, not `""`.
- Response field names follow the query names: `SlackChannel`,
  `SlackMessages`, `SlackThread`, `SlackUser`.
- The scaffold's `go.mod` had a fake `wundergraph/cosmo/router-plugin
  v0.0.0`; the scaffold's real pinned version
  `v0.0.0-20250824152218-8eebc34c4995 // v0.4.1` is what resolves.
  `slack-go/slack v0.15.0` was added for the client wrapper.
- The default Go module path `github.com/wundergraph/cosmo/plugin` was kept;
  the generated package imports as `generated` under that module.
- **Plain GraphQL schema, no federation directives** (`@key`,
  `extend schema @link` were dropped from the original schema). Connect
  plugins serve plain GraphQL over the in-process gRPC contract; the router
  does not run federation introspection against them.

Workflow in this directory:

```bash
make generate   # regenerate protobuf/gRPC from src/schema.graphql
make build      # wgc router plugin build . --debug → bin/darwin_arm64
make test       # wgc router plugin test . (go test, bufconn + mock client)
go test ./...   # same tests directly
```

## Run

The plugin does not run standalone — it is loaded by the router. From the
router project root (one command):

```bash
cd .. && make       # downloads router, builds plugin, composes, starts on :3010
```

Requires `SLACK_BOT_TOKEN=xoxb-...` in the environment (the plugin logs
`fatal` without it). Then query `http://localhost:3010/graphql`.

## Scopes

Bot scopes required, per query:

| Query | Scope |
|-------|-------|
| `slackChannel` | `channels:read` (+ `groups:read` for private channels) |
| `slackMessages` | `channels:history` (+ `groups:history`) |
| `slackThread` | `channels:history` (+ `groups:history`) |
| `slackUser` | `users:read` (+ `users:read.email` for the email field) |

Demo-app status (verified live 2026-08-27 against the "Nodes 2026 Demo"
workspace, re-verified 2026-08-28): **all four queries work with the bot
token**. A fifth query, `searchSlackMessages`, existed until 2026-08-28 and
was dropped: Slack's `search.messages` requires the legacy `search:read`
scope, whose only supported token type is **User** (the granular
`search:read.public` successor supports Bot tokens but only enables the
Assistant API) — so a bot token can never call it. Full analysis and the
talk upside are in PLAN.md, Known issue 4 (RESOLVED).

## Pinned versions

| Component | Version |
|-----------|---------|
| wgc CLI | 0.130.1 |
| Cosmo Router | 0.343.1 |
| router-plugin runtime | v0.4.1 (`v0.0.0-20250824152218-8eebc34c4995`) |
| slack-go | v0.15.0 |
| Go | module target 1.25.1; built with 1.26.5 |
| protoc / protoc-gen-go / protoc-gen-go-grpc | 29.3 / v1.34.2 / v1.5.1 |

See `../../README.md` for the full router-side picture and the pinned
protoc toolchain in `../../tools/`.
