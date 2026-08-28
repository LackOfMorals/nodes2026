# neo4j-api — local Neo4j subgraph service (NODES 2026 demo)

A small plain-GraphQL HTTP service over the demo knowledge graph. It
exposes exactly the operations the demo needs and is registered as a
subgraph of the local Cosmo Router (`router/graph.yaml.template` →
`schema.file`, static SDL — no live introspection).

Localhost only, no auth. Verified live 2026-08-28 (see `../PLAN.md`
Phase 4.1).

## What it serves

`POST /graphql` on `127.0.0.1:4400` (override with `NEO4J_API_ADDR`),
plus `GET /healthz`:

- `getIssueDiscussionContext(identifier: String!)` — the closing-demo
  query, resolved entirely from the graph: the issue, its people, and
  every Slack thread where it was discussed with all messages in order.
  Null when the identifier is not in the graph.
- `searchMessages(query: String!, limit: Int = 20)` — case-insensitive
  substring search across all stored messages; each hit carries its
  thread and channel. This is the demo's only message-search surface
  (Slack's `search.messages` is user-token-only — see PLAN.md Known
  issue 4).

## Run

```bash
cd router
make neo4j-api        # builds + runs in the foreground, env from ../.env
```

## Config (env)

| Var | Default |
|-----|---------|
| `NEO4J_URI` | `bolt://localhost:7687` |
| `NEO4J_USER` | `neo4j` |
| `NEO4J_PASSWORD` | `password` |
| `NEO4J_DATABASE` | `neo4j` |
| `NEO4J_API_ADDR` | `127.0.0.1:4400` |

All set in the repo-root `.env`.

## Files

```
neo4j-api/
├── main.go            # resolver + HTTP server (single file, ~500 lines)
├── schema.graphql     # single source of truth — the router's static SDL too
├── cypher/
│   ├── agent-context.cypher   # copy of ../data-model/queries/agent-context.cypher (keep in sync)
│   └── search-messages.cypher
└── bin/               # built binary (gitignored)
```

## Implementation notes (things that bit, 2026-08-28)

- **Go over Node** — the user chose the Go service over a Node.js
  `@neo4j/graphql` setup; router wiring is identical either way.
- **graphql-go**: `MustParseSchema(..., graphql.UseFieldResolvers())` is
  required or nested object fields don't resolve from struct fields.
  Nullable Go fields must be pointers; list fields are `[X]!` in the
  SDL (a nullable list would require a `*[]T` Go field).
- **neo4j-go-driver v5.27.0**: `datetime` properties decode to
  `time.Time` (formatted RFC3339 in `asString`); datetimes stay native
  in the graph so the web-console demo shows real datetime values.
  In-repo reference for the driver API: `../sync/internal/graph/writer.go`.
- **`//go:embed` is rejected by Go 1.26.5 on this machine** (standalone
  repro: spurious `"embed" imported and not used`). Assets load via
  `loadAsset` — working dir, then the executable's dir and its parent —
  so the binary runs from the module root or from `bin/`.
- Type names avoid the Linear subgraph's SDL (nothing named `Issue`) so
  the two compose cleanly in the supergraph.
