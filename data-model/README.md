# data-model

The Neo4j knowledge graph for the NODES 2026 demo.

This directory holds the Cypher that defines the graph schema, the
parameter-driven upsert queries the sync pipeline calls during ingest,
the explicit linking pass, and the agent-facing query that powers the
closing demo.

This is **iteration 1**: graph only, no vectors. Iteration 2 adds
embedding properties, vector indexes, and a semantic linking pass.

## The model at a glance

```
                  ┌─────────┐
                  │ Project │
                  └────┬────┘
                       │ :HAS_ISSUE
                       │
                       ▼
   ┌───────────────────────────────────────┐
   │              Issue                    │
   │  identifier ("NODES-1"), title, state │
   └─┬──────────────────┬────────────────┬─┘
     │                  │                │
  :CREATED        :ASSIGNED_TO       :DISCUSSED_IN
     │                  │                │ {confidence, evidence}
     ▼                  ▼                ▼
   ┌────────┐      ┌────────┐      ┌─────────┐
   │ Person │      │ Person │      │ Thread  │
   └────────┘      └────────┘      └────┬────┘
       ▲                                │
       │ :AUTHORED              :HAS_MESSAGE
       │                                │
       │                                ▼
       │                          ┌──────────┐
       └──────────────────────────│ Message  │
                                  └──────────┘
                                       ▲
                                       │ :HOSTS_THREAD
                                       │
                                  ┌────┴────┐
                                  │ Channel │
                                  └─────────┘
```

## Node types

| Label | Purpose | Identity |
|-------|---------|----------|
| `Project` | A Linear project. Scoping anchor for the demo. | `id` |
| `Issue` | A Linear issue. | `id` (Linear UUID), `identifier` (NODES-123) |
| `Channel` | A Slack channel. | `id` (Slack channel ID) |
| `Thread` | A Slack thread — parent message plus replies. | composite `(channelId, ts)` |
| `Message` | A single Slack message. | composite `(channelId, ts)` |
| `Person` | A human, unified across Linear and Slack. | `id` (internal UUID), `email` (join key) |

## Relationship types

| Type | Direction | Properties | Notes |
|------|-----------|-----------|-------|
| `:HAS_ISSUE` | `Project → Issue` | none | Scoping |
| `:CREATED` | `Person → Issue` | none | Linear creator |
| `:ASSIGNED_TO` | `Person → Issue` | none | Current assignee |
| `:HOSTS_THREAD` | `Channel → Thread` | none | Slack channel containment |
| `:HAS_MESSAGE` | `Thread → Message` | none | Includes the parent message |
| `:AUTHORED` | `Person → Message` | none | Slack message author |
| `:DISCUSSED_IN` | `Issue → Thread` | `confidence` (Float), `evidence` (String), `createdAt` (DateTime) | The cross-system link |

## Design decisions worth knowing

### Person is unified

The single most important modelling choice. A real human has a Linear identity and a Slack identity, but in the graph they're one node. The join key is `email`. The `linearId` and `slackId` properties carry the platform-specific identifiers for back-references.

This is what lets the agent ask "who participated both in the Linear discussion and the Slack thread" as a single graph traversal, rather than joining across two separate types.

### Thread and Message are separate

Threads are decision units. Messages are utterances. Keeping them separate lets the agent reason at either level. The parent message of a thread is itself a Message with `threadTs == ts`, which makes the parent equivalent to a reply for query purposes while still being distinguishable.

### `:DISCUSSED_IN` carries evidence

The `evidence` property is honest about how the link was created:

- `explicit_mention` — message text contained the issue identifier (iteration 1)
- `semantic_match` — vector similarity above threshold (iteration 2)
- `manual` — a human created the link (always supported)

The `confidence` property is 1.0 for explicit and manual links, and the cosine similarity score for semantic matches. This lets the agent filter "high-confidence links only" vs "anything plausibly related" against the same graph.

### Why issue → thread, not thread → issue

The dominant query direction is "given an issue, find discussions." Modelling the relationship in that direction lets Cypher read naturally. Reverse traversal still works — Cypher relationships are bi-directional in query — but the data flow makes more sense this way.

## Files

| File | What it does | When you run it |
|------|--------------|-----------------|
| `schema.cypher` | Creates constraints and indexes | Once, against a fresh database |
| `sample-data.cypher` | Seeds a complete demo graph | Once after schema, or to reset between dev runs |
| `queries/upsert-issue.cypher` | Idempotent issue upsert | Once per issue, during sync |
| `queries/upsert-thread.cypher` | Idempotent thread + messages upsert | Once per thread, during sync |
| `queries/link-explicit.cypher` | Regex-based linking pass | After each batch of new messages |
| `queries/agent-context.cypher` | The closing demo query | Resolved by a persisted operation in the gateway |
| `queries/embed-issue.cypher` | Write an embedding vector onto one Issue | Once per issue, during the embed-and-link-semantic sync stage |
| `queries/embed-message.cypher` | Write an embedding vector onto one Message | Once per message, same stage |
| `queries/all-issue-texts.cypher` | List every Issue's id + embeddable text | Read before embedding, same stage |
| `queries/all-message-texts.cypher` | List every Message's key + embeddable text | Read before embedding, same stage |
| `queries/link-semantic.cypher` | Vector-similarity linking pass | After embedding, following the explicit pass |

## Running it

Against the local podman Neo4j container (the demo environment — see
`../DEMO-ENV.md` for the full runbook and the web-console workflow):

```bash
# Apply schema, then sample data, then the linking pass — in the Neo4j
# web console at http://localhost:7474 (no cypher-shell needed; every
# statement is self-contained, so they can be pasted individually):
#   data-model/schema.cypher             (16 statements)
#   data-model/sample-data.cypher        (21 statements)
#   data-model/queries/link-explicit.cypher   (expect 5 distinct rows)

# Test the agent query — paste queries/agent-context.cypher in the web
# console with identifier = "NODES-1". Expect 2 discussions, Thread 4 absent.

# Reset to the canonical state anytime:
#   MATCH (n) DETACH DELETE n;
#   then re-run the three files above.
```

## What the demo data shows

The seed graph models a one-week slice of a team working on the federation
demo. It includes:

- 6 issues spanning open, in-progress, and done states
- 4 Slack threads with 12 messages across them
- 6 people (5 with both Linear and Slack identities, 1 Slack-only)
- 5 explicit `:DISCUSSED_IN` edges from the regex pass
- 1 orphaned thread that references the same problem as another thread but
  doesn't name the issue explicitly — this is iteration 2's vector linking
  candidate

The orphaned thread (Thread 4 in the seed data) is deliberate. During the
talk you can point to it and say:

> The graph already shows what the team explicitly told it. There's also
> a discussion the regex pass missed — same topic, different words. The
> next iteration uses vector similarity to catch that one.

That sets up iteration 2 as a natural next step rather than an afterthought.

## What changes for iteration 2

Iteration 2 adds embeddings. The model itself doesn't change shape — the
graph schema is stable; vectors are an enhancement layer on top of it.

### Files that change

- `schema.cypher` gains `CREATE VECTOR INDEX` statements (one for Issue, one
  for Message)
- `sample-data.cypher` doesn't change — the new linking pass produces the
  new edges automatically
- A new `queries/link-semantic.cypher` joins the queries directory
- `queries/agent-context.cypher` doesn't change — new `:DISCUSSED_IN` edges
  flow through the same query, they just carry lower confidence scores

### Where embeddings come from

Three options for getting vectors into Neo4j. Pick based on where your
embedding service runs relative to Neo4j.

**Option 1: embed in the sync pipeline, write to Neo4j as a property.**

Your sync code calls a local embedding service (LM Studio, Ollama, or a
hosted provider you wrap yourself), gets a list of floats back, and writes
it to Neo4j via a parameterised Cypher `SET`. The Neo4j GenAI plugin isn't
involved.

This is what the demo uses. LM Studio runs on the same machine as the sync
pipeline, the embeddings never leave that machine, and Neo4j just receives
the resulting vectors as data.

**Option 2: use the GenAI plugin pointed at an OpenAI-compatible endpoint.**

Neo4j ships `ai.text.embed()` and `ai.text.embedBatch()` Cypher functions
that call a supported provider directly. The `genai.openai.baseurl`
configuration setting lets you point the OpenAI provider at any
OpenAI-compatible endpoint, which includes LM Studio and Ollama.

This works when Neo4j and the embedding service share a network — both
self-hosted on a VPC, both in a Docker network, both on the same on-prem
box. It doesn't work for the demo because Aura runs in AWS and LM Studio
runs on a laptop. Aura can't reach `localhost:1234`.

**Option 3: use a supported cloud provider.**

The plugin supports OpenAI, Azure OpenAI, Google Vertex AI, and Amazon
Bedrock Titan models out of the box. Conventional, low-friction, but moves
the embedding outside your control plane.

### Why Option 1 for this demo

The talk's framing benefits from a clean sovereignty story: no external
embedding API in the loop, embeddings produced on the same machine that
runs the sync pipeline. The graph stores them as data — Neo4j doesn't
need to know they came from LM Studio.

For production deployments where Neo4j and the embedding service share a
network, Option 2 is genuinely nice: Cypher reads cleanly and there's no
glue code to maintain. That's a future discussion, not a demo constraint.

### What to watch out for with externally-generated embeddings

**Dimension lock.** The vector index commits to a specific dimension count
when you create it. Switching from Nomic Embed (768 dim) to BGE-M3 (1024 dim)
mid-stream breaks the index. The `embeddingModel` property on Issue and
Message (added in iteration 2) lets you detect mid-stream model changes
and find re-embed candidates with a simple `WHERE` clause.

**Use `db.create.setNodeVectorProperty` for storage.** Neo4j's docs note
this procedure stores the list with a more space-efficient representation
than a plain `SET m.embedding = $vec`. The cost is one extra line of
Cypher; the benefit is meaningful at scale:

```cypher
MATCH (m:Message {channelId: $channelId, ts: $ts})
CALL db.create.setNodeVectorProperty(m, 'embedding', $vec)
```

**You manage batching.** `ai.text.embedBatch` orchestrates batching for you.
Without it, your sync code batches calls to the embedding service (16-32
messages per batch is typical) and writes them in a transaction. About
30 lines of Go.

**You manage retries.** Local embedding services are reliable, but model
loads, OOMs, and timeouts happen. Standard exponential backoff over 3-5
attempts is enough for the demo and most production cases.

**The vector index is provenance-agnostic.** `db.index.vector.queryNodes`
doesn't care who generated the floats. This is what makes Option 1 work
at all — the index treats LM Studio-generated vectors identically to
vectors from any other source.

### What this means for the talk

The line to land on stage:

> The graph stores embeddings as a property type. Neo4j ships convenience
> functions for supported cloud providers, but they're optional. Any
> embedding model that produces a list of floats can populate the property,
> and the vector index doesn't care where the floats came from. We use a
> local model running on the same box as the sync pipeline. No embedding
> service in the loop.

That's a stronger sovereignty claim than "we used OpenAI but you don't
have to." It's "we don't use any cloud provider for this." For a
graph-and-AI audience, that lands harder than the federation gateway
story alone.
