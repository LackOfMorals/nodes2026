# nodes2026

Working code for the NODES 2026 talk: *A murder of MCP servers: taming AI tool sprawl with GraphQL federation and a knowledge graph*.

See [`TALK.md`](TALK.md) for the full talk reference — argument, architecture,
build status, timeline, demo plan, and speaker notes.

## What this is

A working federation gateway demo combining three data sources into a single MCP endpoint for an LLM agent:

| Subgraph | Native API | Wrapper needed |
|----------|-----------|----------------|
| Linear | GraphQL | None — registered directly |
| Neo4j Aura | GraphQL | None — registered directly |
| Slack | REST | Yes — Cosmo Connect plugin |

The federation router exposes the supergraph over a single MCP endpoint via Cosmo's MCP Gateway feature. The LLM agent sees one tool surface; the persisted operations layer governs what it can call. The knowledge graph in Neo4j Aura holds the relationships *between* Linear issues and Slack threads — the layer that turns a federated tool surface into something an agent can reason across.

## Repo layout

```
nodes2026/
├── README.md                          # you are here — the orientation map
├── TALK.md                            # the talk reference document
├── slack-subgraph/                    # Go plugin wrapping Slack REST API
├── data-model/                        # Neo4j graph schema, sample data, queries
└── sync/                              # Go pipeline pulling Linear+Slack into the graph
```

Each subdirectory has its own README with build instructions, design notes,
and prerequisites specific to that component.

## Where to start reading

| If you want to... | Start here |
|-------------------|-----------|
| Understand the argument | `TALK.md` |
| See the architecture diagram | `TALK.md` — Architecture section |
| Understand the graph model | `data-model/README.md` |
| Build the Slack subgraph plugin | `slack-subgraph/README.md` |
| Build the sync pipeline | `sync/README.md` |
| Submit the CFP | `cfp-submission/` (separate, not in this repo) |

## Component status

- [x] CFP description (1493/1500 characters)
- [x] Architecture decisions
- [x] Data model design — schema, sample data, ingest queries, agent query
- [x] Slack subgraph plugin (Go code complete, awaits wgc bootstrap)
- [x] Sync pipeline (Go code complete, awaits free-tier accounts to test against)
- [ ] Linear free-tier workspace setup with seeded issues
- [ ] Slack free-tier workspace setup with seeded messages
- [ ] Federation router configuration (Cosmo subgraph registration + persisted ops)
- [ ] Router deployment to ECS Fargate
- [ ] Agent test harness (Claude Desktop config + sample prompts)
- [ ] Iteration 2: vector embeddings via LM Studio, semantic linking
- [ ] Speaker slides
- [ ] Live demo dry runs

Detailed status, timeline, and risk register live in `TALK.md`.

## Quick start (end-to-end demo prep)

In order:

1. Set up free-tier accounts: Linear, Slack, Neo4j Aura, Cosmo Cloud
2. Seed Linear with 5-6 issues; seed Slack with messages that reference them
3. Load `data-model/schema.cypher` into Aura
4. Build and run the sync pipeline (`sync/README.md`)
5. Verify the graph populated by running the agent context query
6. Bootstrap the Slack subgraph plugin (`slack-subgraph/README.md`)
7. Register all three subgraphs with the federation router
8. Deploy the router to ECS
9. Point Claude Desktop at the MCP endpoint
10. Test with the demo prompts in `TALK.md`

Estimated time end-to-end (assuming no debugging): one focused day, maybe two.
