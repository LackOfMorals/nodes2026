# Neo4js GraphQL Library with Yoga GraphQL Server

## NODES 2026 demo usage

This folder is the router's registered neo4j subgraph (Phase 4.5). The
hand-written typeDefs in `schema.graphql` are the **single source of
truth** — the server loads them directly (introspection is off in
`index.cjs`), so there is no drift between what the server serves and
what the router composes.

- Install once on a fresh checkout: `npm install` (node 26.5.0;
  @neo4j/graphql 7.6.2, graphql-yoga 5.22.0, neo4j-driver 5.28.3).
- Run the server: `node index.cjs` → `http://127.0.0.1:4000/graphql`
  (env from `../.env`: `NEO4J_URI/USER/PASSWORD/DATABASE`; `PORT`
  overrides the port). Or from `router/`: `make neo4j-srv`.
- Generate the plain-SDL file the router composes:
  `node gen-plain-schema.cjs` (or `make gen-neo4j-schema` from
  `router/`) → **gitignored** `neo4j-plain-schema.graphql` (printed SDL
  via `printSchema`, which strips the `@cypher` directives). `router/
  graph.yaml.template` registers it via `schema: {file: …}`; `make
  compose` regenerates it first. After any change to `schema.graphql`:
  `make gen-neo4j-schema && make compose` + restart the router.

Schema notes (why it looks the way it does):

- `Issue`/`Project` are named **`GraphIssue`/`GraphProject`** (mapped
  back to the graph labels with `@node(labels:)`). The raw names
  collide with Linear's `Issue`/`Project` types, and wgc hard-fails to
  compose shared type names across subgraphs.
- `GraphIssue.discussedInThreads` declares `properties:
  "DiscussedInProperties"`, so `@neo4j/graphql` auto-generates
  `discussedInThreadsConnection { edges { properties { confidence
  evidence createdAt } node { ... } } } }` — the `DISCUSSED_IN`
  confidence/evidence (load-bearing talk content) comes straight off
  the edge in the same traversal, no `@cypher` field needed.
- The one remaining `@cypher` field is root `searchMessagesCI` — the
  demo's only message search. The library's `contains` filter compiles
  to plain `CONTAINS`, which is **case-sensitive** on Neo4j 2026.x, so
  the field uses `toLower(m.text) CONTAINS toLower($query)`; `LIMIT
  toInteger(coalesce($limit, 20))` works around the driver sending
  numbers as float (and null) params.
- Generated root queries are plural-only (`graphIssues`,
  `graphProjects`, …); "not found" is an empty list, not null.

See `../PLAN.md` (Phase 4.5, Known issue 6) and `../DEMO-ENV.md` →
"Router (Phase 4 …)".

---

## Install

```bash
npm init es6 --yes
npm i graphql-yoga @neo4j/graphql @neo4j/introspector neo4j-driver graphql
```


## Configuration

uses environmental variables for neo4j db configuration

```bash
export NEO4J_URI=bolt://localhost:7687
export NEO4J_USER=neo4j
export NEO4J_PASSWORD=password
export NEO4J_DATABASE=neo4j  
```

or store these in .env


On startup introspection is used to generate the graphql type defs.  If you don't want this, change the index.cjs file



## Running

