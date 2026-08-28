# NODES 2026 talk — working reference

A murder of MCP servers: taming AI tool sprawl with GraphQL federation and a knowledge graph.

This document is the persistent record of the talk's thinking — the argument,
the architecture, the design choices, the demo plan, and the build steps. It's
written for the speaker (me) coming back to this project months later with
half the context gone.

Last updated: 2026-08-27.

---

## Quick reference

| Field | Value |
|-------|-------|
| Conference | NODES 2026 |
| Date | November 12, 2026 |
| Format | Full session (30–45 minutes) |
| Track | AI Engineering: Generative AI, RAG, and Agents |
| Title | A murder of MCP servers: taming AI tool sprawl with GraphQL federation and a knowledge graph |
| Speaker | Jonathan Giffard (JG), Lead PM at Neo4j |
| Submission status | Drafting |

CFP submission fields are in `../cfp-submission/nodes-2026-cfp-submission.md`.

---


## Talk description

MCP solved the AI integration problem. Every SaaS vendor ships a server now. Wire
ten together and you get a different problem: auth sprawl, disconnected tool namespaces,
and response shapes each agent handles. The constraint in production AI
isn't connecting to systems. It's managing the surface area.

GraphQL federation resolved this for REST — independent services behind a single
unified schema. The same pattern applies to MCP server sprawl. Platform vendors are
already shipping production gateways that prove it.

This session shows how, live. From three real systems — Neo4j Aura, Linear, and Slack
— we build a federation gateway exposing all three as a single MCP endpoint. We cover
subgraph configuration, persisted operations that govern what an agent can call, and
the graph data model that connects the systems. The context window impact is visible:
20+ discrete tool definitions collapse to a handful of named operations.

The second half shows what a knowledge graph adds. Neo4j isn't a third data source
here — it's the layer that holds the relationships between the other two. We model
Linear issues and Slack threads as nodes, connect them with typed edges, and write
the Cypher that makes cross-system reasoning possible. The closing demo traverses all
three as a connected graph: one auth token, one log entry, one answer none of them
could produce alone.

A federated endpoint is manageable. A knowledge graph makes it intelligent.
Code and data model on GitHub before the conference.


## The argument

The talk makes one structural claim with two supporting layers.

**Structural claim**: MCP server sprawl is the same problem REST API sprawl
was, and GraphQL federation is the same solution. You compose independent
upstream sources behind a single unified schema, expose one MCP endpoint to
agents, and let persisted operations govern what they can call.

**Layer 1 (federation handles the plumbing)**: one endpoint, one auth surface,
collapsed tool list, governance via persisted operations, schema introspection
as the LLM's map.

**Layer 2 (the knowledge graph handles the intelligence)**: a federated
endpoint is manageable; the graph makes it reasoning-capable. Linear issues
connect to Slack threads connect to people via typed edges. The agent
traverses the connections rather than inferring them from disconnected tool
calls. This is the part that makes it a NODES talk rather than a GraphQLConf
talk.

The closing line: federation is the mechanism. The graph is the point.

## Honest tensions to acknowledge on stage

Three places where the pattern has rough edges. Address them directly rather
than letting Q&A find them.

**Action-heavy agents vs. retrieval-heavy ones.** Federation handles queries
(reads) cleanly. Mutations (writes — sending Slack messages, creating Linear
issues) work too, but the governance story is harder. Persisted operations
mitigate this; a slide acknowledging the distinction lands better than
pretending it isn't there.

**The federation gateway is itself a component.** You're trading MCP server
sprawl for one more piece of operational surface area. The trade is worth it
above roughly 4 MCP servers. Say that explicitly.

**Threshold tuning for vector linking.** Iteration 2 landed (2026-08-28)
and this tension turned out to be real, not hypothetical: 0.78 (tuned
against the 6-issue canonical seed graph, where it's the only value in a
0.0024-wide window that keeps the intended link and excludes four false
positives) still lets seven *further* false positives through once
run against the real 8-issue workspace. Mention this on stage rather than
letting a live run surprise anyone: "We picked this threshold by testing
against our own seed data, not by A/B testing in production — and at this
scale, short same-project text clusters tightly enough that no single
number cleanly separates 'genuinely related' from 'same topic, different
issue.' A production version of this needs a smarter scoring approach,
not just a bigger threshold."

## Why this belongs at NODES specifically

Multiple federation talks happen at GraphQLConf. The differentiated NODES
angle is the knowledge graph as the reasoning layer — not just a third data
source, but the layer that holds the *relationships between* the other
sources. That framing only works at a graph-database conference, because
the audience already knows why connected data matters.

The talk also extends 2025's NODES content. The 2025 MCP architectural
patterns talk argued for graph-backed tool registries within an individual
MCP server. This 2026 talk takes the same argument fleet-level: federation
gateway as the registry layer for a whole MCP collection.

---

## Architecture

### High-level

```
                     ┌──────────────────────────────┐
                     │   LLM agent (Claude Desktop  │
                     │   or claude.ai connector)    │
                     └──────────────┬───────────────┘
                                    │ MCP (Streamable HTTP, OAuth)
                                    │
            AWS account             │
        ┌───────────────────────────┼────────────────────────────┐
        │                           │                            │
        │           ┌───────────────▼────────────────┐           │
        │           │  Cosmo Router (ECS Fargate)    │           │
        │           │  + MCP Gateway feature         │           │
        │           │  + Slack plugin (Go)           │           │
        │           │  + persisted operations        │           │
        │           └──────┬──────────────┬──────────┘           │
        │                  │              │                       │
        │  ┌───────────────▼──┐    ┌──────▼────────────┐         │
        │  │ Neo4j AuraDB     │    │ Plugin: Slack     │         │
        │  │ (AWS region)     │    │ wraps REST API    │         │
        │  │ + GraphQL endpt  │    │ (in-process)      │         │
        │  └──────────────────┘    └──────┬────────────┘         │
        │                                  │                      │
        └──────────────────────────────────┼──────────────────────┘
                                           │ HTTPS
                            ┌──────────────▼──────────────┐
                            │  Slack Web API              │
                            │  (api.slack.com)            │
                            └─────────────────────────────┘
                            ┌─────────────────────────────┐
                            │  Linear GraphQL API         │
                            │  (api.linear.app/graphql)   │
                            └──────────────▲──────────────┘
                                           │
                              ┌────────────┴───────────┐
                              │ Cosmo Router fetches   │
                              │ directly as a subgraph │
                              └────────────────────────┘
```

### Why each piece is where it is

**Cosmo Router on ECS Fargate**: chosen over Apollo because it's Apache 2.0
licensed (Apollo's router is ELv2, which has commercial restrictions). Fargate
chosen over EC2 to avoid managing nodes. Single task, public ALB in front,
internal endpoints for management. Talks to Cosmo Cloud free tier for the
control plane (schema registry, plugin distribution, observability).

**Neo4j AuraDB in same AWS region as the router**: low latency between
router and graph subgraph. Aura ships with a built-in GraphQL endpoint
that the router registers as a federation subgraph directly — no adapter
code needed.

**Linear as a federation subgraph directly**: Linear's API is native
GraphQL, so the router connects to `api.linear.app/graphql` with no
wrapper. Personal API key in Secrets Manager.

**Slack as a Cosmo Connect plugin (Go)**: Slack is REST-only, so it needs
adapting. The plugin runs *inside the router process* via HashiCorp's
go-plugin library — not a separate service. About 300 lines of Go.

**Bot token for Slack**: a fresh free Slack workspace owned by JG, single
bot, simple bot token authentication. No OAuth dance needed since the
workspace is owner-controlled.

### What the agent sees

One MCP endpoint. Authenticated via OAuth (Cognito-backed in production, or
a simple bearer token for the demo). Discovers a handful of persisted
operations — not 60+ raw tools. Calls them with parameters; receives
structured GraphQL responses; reasons over them.

### Why this is the right shape for the talk

Three deliberate properties of this architecture make it demoable:

1. The three subgraphs have *visibly different* upstream protocols. GraphQL,
   REST-via-plugin, GraphQL. The federation surface absorbs that difference.
   That's a slide-friendly visual.
2. The plugin lives inside the router. There's no "deployment dance" with
   multiple containers and services for the audience to track. One router,
   one process, one network call from the audience's mental model.
3. The knowledge graph isn't a separate concern. It's one of the federated
   subgraphs. Its role is *not* "the cache" or "the index" — it's "the
   semantic layer that connects the other two."

---

## The data model

Full design lives in `../data-model/README.md`. Key things to remember for
the talk:

**Six node types**: Project, Issue, Channel, Thread, Message, Person.
Person is the unification point — one node per human, even though they
exist in both Linear and Slack. Join key is email.

**Seven relationship types**: `:HAS_ISSUE`, `:CREATED`, `:ASSIGNED_TO`,
`:HOSTS_THREAD`, `:HAS_MESSAGE`, `:AUTHORED`, `:DISCUSSED_IN`.

**The interesting edge is `:DISCUSSED_IN`**: carries `confidence` (0.0-1.0)
and `evidence` ("explicit_mention" | "semantic_match" | "manual"). This is
what makes the graph reasoning-capable rather than just storage.

**Why this matters for the talk**: most people show graphs that model
*entities*. The interesting bit here is modelling *cross-system connections*
with provenance. The `evidence` property is what lets the agent say
"the team discussed this with high confidence" vs "this is plausibly
related." Slide moment.

### Demo data shape

One project, one channel, six issues, four threads, twelve messages, six
people. Sized to fit on screen during a live walk-through.

One deliberately orphaned thread (Thread 4): discusses "the schema problem"
without naming `NODES-1` explicitly. The iteration 1 regex pass can't link
it. When iteration 2 lands, the semantic pass finds it. That's the
"see — the system found a connection that was invisible" moment.

---

## The closing demo

The structural narrative for the live demo segment:

1. **Show the world without federation.** Agent has access to 3 MCP servers
   (Linear, Slack, Neo4j separately). Ask it "what did the team decide
   about the schema validation issue?" Watch it make many tool calls, get
   scattered context, answer poorly.

2. **Show the federation gateway.** Same agent, now pointing at the Cosmo
   router's MCP endpoint. Same question. One tool call (`getIssueDiscussionContext`).
   Structured response. Better answer.

3. **Show the graph traversal**. Click into the Cypher that resolved that
   persisted operation. Show the `MATCH` with `:DISCUSSED_IN` traversal. Point
   out that this single query crossed Linear and Slack as one connected graph.

4. **Show the orphaned thread.** Point at Thread 4 in the graph. "The team
   has another discussion about the same problem — but they didn't name the
   issue, so the regex linker missed it. In iteration 2, vector similarity
   catches this."

The whole live segment is maybe 8 minutes. The rest is architecture talk and
pattern argument.

### The persisted operation the agent calls

```graphql
query getIssueDiscussionContext($identifier: String!) {
  issue(identifier: $identifier) {
    identifier
    title
    state
    priority
    createdBy { name email }
    assignedTo { name email }
    discussions {
      channel
      threadTs
      startedAt
      permalink
      confidence
      evidence
      messages {
        text
        authorName
        postedAt
        permalink
      }
    }
  }
}
```

This is the federation schema view. Behind it, the resolver runs
`queries/agent-context.cypher` against Aura. The agent never sees the Cypher
or even knows the response came from a graph — to the agent, it's just a
GraphQL query that returns connected data.

### The demo prompts

Two prompts to use during the live segment. Both should be in the speaker
notes; both should be tested the morning of the talk.

**Prompt 1 (federated path)**:
> What did the team decide about the schema validation issue in NODES-1?

Agent calls `getIssueDiscussionContext(identifier: "NODES-1")`. Gets back
issue + two threads + messages. Synthesises:
> The team decided to use explicit `@shareable` annotations across all three
> subgraphs to preserve nullability during composition. Sarah identified the
> root cause; Alex made the decision and confirmed the fix landed.

**Prompt 2 (the harder one — saved for if time allows)**:
> Has anyone discussed the same problem recently without referencing the
> issue directly?

For iteration 1, this returns "I don't have visibility into discussions that
don't reference the issue." For iteration 2, it finds Thread 4 via semantic
similarity. This is the cliffhanger that closes the demo if iteration 2 is
ready, or the natural setup for "here's what's coming next" if it isn't.

---

## Build status

### Done

- [x] CFP description (1493/1500 characters)
- [x] Architecture decisions
- [x] Data model design (`../data-model/`)
  - [x] Schema with constraints and indexes
  - [x] Idempotent upsert queries for Issue and Thread
  - [x] Explicit linking pass using APOC regex
  - [x] Agent context query
  - [x] Complete sample data (six issues, four threads, twelve messages)
- [x] Slack subgraph Go code (`../slack-subgraph/`)
  - [x] Schema definition
  - [x] Slack client wrapper
  - [x] gRPC service handler
  - [x] Plugin entry point
  - [x] README with bootstrap steps

### In progress

Nothing currently.

### To do

- [ ] Sync pipeline — the Go binary that pulls from Linear + Slack and
      feeds the upsert queries
- [ ] Federation router configuration — register Linear, Aura, and Slack
      subgraphs; declare persisted operations
- [ ] Router deployment to ECS Fargate (Task definition, ALB, secrets)
- [ ] Linear free-tier workspace setup with seeded issues
- [ ] Slack free-tier workspace setup with seeded messages
- [ ] Agent test harness (Claude Desktop config + sample prompts)
- [ ] Iteration 2: embedding integration with LM Studio, vector indexes,
      semantic linking pass
- [ ] Speaker slides (Keynote / PowerPoint)
- [ ] Live demo dry runs (at least three before the conference)

### Risk register

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Cosmo wgc API changes between now and Nov | Medium | Medium | Pin wgc version in plugin's go.mod; document the version |
| Live demo network failure | Low | High | Record a backup video; have a recorded path that can substitute |
| Aura free tier resource limits hit during demo | Low | Medium | Stay on small dataset; verify limits week-of |
| Slack rate limits during live demo | Low | Medium | All Slack data is in Neo4j by demo time; live calls aren't needed |
| LM Studio crash mid-demo (iteration 2) | Medium | Medium | Have embeddings pre-computed; semantic linking pass already ran |
| Claude Desktop / claude.ai connector behaviour changes | Medium | Medium | Test on the morning of; have screenshots as fallback |

---

## Timeline

| Window | Goal |
|--------|------|
| June 2026 | CFP submitted. Architecture locked. Data model done. Slack plugin written. |
| July 2026 | Sync pipeline running. End-to-end iteration 1 demo working locally. |
| August 2026 | Router on ECS. Real Linear + Slack data flowing. Full iteration 1 demo runnable from claude.ai. |
| September 2026 | Iteration 2 — vectors via LM Studio. Semantic linking pass. Slides drafted. |
| October 2026 | Slides finished. Demo dry runs. Speaker notes complete. |
| November 12 2026 | Talk delivered. |

The hardest milestone is August — moving from "works on my laptop" to
"works on AWS with real data." Plan to have something running end-to-end
by end of July so August is debugging, not building.

---

## Build instructions

Each component has its own README with its own setup steps. This section
captures the order they need to be set up in, with cross-references.

### Prerequisites

- A free-tier Linear workspace ( email j.giffard@icloud.com )
- A free-tier Slack workspace (owned, not work)  ( email j.giffard@icloud.com )
- A free-tier Neo4j AuraDB instance
- A free-tier Cosmo Cloud account (for the federation control plane) 
- An AWS account
- Go 1.23+
- Node.js (for `wgc` CLI)
- `cypher-shell` (for running data-model files against Aura)

### Setup order

**1. Free-tier accounts (1 hour total)**

- Create the Linear workspace. Add a team. Create a project (e.g.
  "NODES 2026 Demo"). Create 5-6 sample issues with identifiers
  starting at NODES-1. Generate a personal API key under
  Settings → API.
- Create the Slack workspace. Add a channel (e.g. `#nodes-demo-eng`).
  Create a Slack app at api.slack.com/apps. Add bot token scopes:
  `channels:read`, `channels:history`, `groups:read`, `groups:history`,
  `users:read`, `users:read.email`, `search:read`. Install to workspace.
  Copy the bot user OAuth token (xoxb-...).
- Create the Neo4j Aura free instance. Copy the connection URI and
  generated password.
- Create the Cosmo Cloud account. Note the API token for `wgc`.

**2. Load the data model into Aura (15 minutes)**

```bash
cd data-model
cypher-shell -a <bolt-uri> -u neo4j -p <password> -f schema.cypher
# Optional: load sample data to test before sync pipeline is ready
cypher-shell -a <bolt-uri> -u neo4j -p <password> -f sample-data.cypher
```

Verify with the agent context query against `NODES-1`. Should return a
populated structure.

**3. Build and test the Slack subgraph plugin (30 minutes)**

```bash
cd slack-subgraph
wgc router plugin init   # adopt wgc's scaffolding, then merge our files
wgc router plugin generate
go mod tidy
export SLACK_BOT_TOKEN=xoxb-...
wgc router plugin dev    # local test against a local router
```

Hit the plugin via the local router's GraphQL endpoint to confirm
slackChannel and slackMessages return real data.

**4. Build and run the sync pipeline (1 hour — once built)**

The sync pipeline doesn't exist yet. Building it is the next task. Once
built:

```bash
cd sync
go build -o bin/sync ./cmd/sync
LINEAR_API_KEY=lin_api_... \
SLACK_BOT_TOKEN=xoxb-... \
NEO4J_URI=neo4j+s://... \
NEO4J_USER=neo4j \
NEO4J_PASSWORD=... \
./bin/sync --project-id <linear-project-id> --channel-id <slack-channel-id>
```

**5. Configure the federation router (1-2 hours)**

Register three subgraphs with Cosmo Cloud:

- Linear: `https://api.linear.app/graphql` with `Authorization: Bearer <key>`
- Neo4j Aura: the Aura GraphQL endpoint URL
- Slack: published as a plugin via `wgc router plugin publish`

Compose the supergraph schema. Define persisted operations
(starting with `getIssueDiscussionContext`).

**6. Deploy the router to ECS Fargate (2 hours)**

Task definition references the Cosmo Cloud control plane via environment
variables. ALB in front. Secrets Manager for the Linear API key, Slack
bot token, Aura password.

**7. Connect the agent (15 minutes)**

Configure Claude Desktop (or claude.ai's MCP connector feature) to point at
the router's MCP endpoint. Test with the demo prompts.

---

## Talk structure (working outline)

This is a working outline, not the final slide deck. Roughly 8 sections in
35 minutes, leaving room for 5-10 minutes of Q&A.

1. **The problem statement** (3 min)
   - Murder of MCP servers — the visual
   - Quote: "the constraint isn't connecting to systems anymore"
   - Hook: "we've been here before"

2. **The precedent** (3 min)
   - REST sprawl, pre-federation
   - GraphQL federation's answer: unified schema, one endpoint
   - The two major federation vendors (unnamed, but reference their
     production gateways)

3. **The pattern applied to MCP** (5 min)
   - One MCP endpoint
   - One auth surface
   - Collapsed tool list (60+ → handful of named operations)
   - Persisted operations as the governance layer

4. **The honest edges** (2 min)
   - Action-heavy vs retrieval-heavy
   - The gateway is itself a component
   - Where the pattern works cleanly, where it doesn't

5. **What federation doesn't give you** (3 min)
   - Connected reasoning across systems
   - The bridge between "manageable" and "intelligent"
   - The knowledge graph as the missing layer

6. **Live demo** (10 min)
   - The architecture (60 sec)
   - The graph (90 sec)
   - The federated MCP endpoint (90 sec)
   - The federated query in action (3 min)
   - The Cypher behind the persisted operation (90 sec)
   - The orphaned thread (60 sec) — sets up iteration 2

7. **Iteration 2 preview** (3 min)
   - Vectors as the next layer
   - Local model, no cloud API
   - Semantic linking — finding what nobody named

8. **The takeaway** (3 min)
   - Federation: mechanism
   - Knowledge graph: meaning
   - Vectors: discovery
   - Closing slide / quotable line

Demo failure plan: if anything goes wrong during the live demo, switch to
the pre-recorded video at any point. Don't try to debug on stage.

---

## Editorial notes

- Use sentence case headings on slides (Neo4j brand standard)
- No corporate openers ("Today's engineers face...")
- The vendor convergence point in Act 1 stays unnamed — multiple federation
  platform vendors have shipped MCP gateways; that's a credibility signal
  without being a vendor pitch
- The local embedding model is named on stage (LM Studio); the federation
  router and control plane stay unnamed (Cosmo)
- Avoid the word "leverage." Avoid "delve." Use plain Go.

---

## Open questions

Things to resolve before October:

1. **Should the talk include a 60-second "production hardening" slide?**
   The risk register material is honest about what would change for production
   — OpenTelemetry, retries, caching. Could be useful for the audience that
   wants to implement this. Could also be a Q&A topic instead.

2. **Should iteration 2 (vectors) be in the live demo or just the slides?**
   Live is more impressive. Slides are safer. Decision point: end of August,
   based on how stable iteration 2 is at that point.

3. **Recorded backup video — same demo or different?**
   If same, lower production cost. If different (more polished, more scripted),
   stronger fallback. Lean toward same-but-recorded — authenticity matters.

4. **Speaker bio for the CFP form** — needs writing. Should reference the
   2025 NODES talk and the relevant production work at Neo4j.

---

## Reference material

External:

- WunderGraph Cosmo MCP Gateway docs:
  https://cosmo-docs.wundergraph.com/router/mcp
- Apollo MCP Server docs:
  https://www.apollographql.com/docs/apollo-mcp-server
- Neo4j GenAI plugin embeddings docs:
  https://neo4j.com/docs/genai/plugin/25/embeddings/
- GraphQLConf 2025: "LLMs + GraphQL + MCP: A Blueprint for Scalable AI Tooling"
  https://graphql.org/conf/2025/schedule/0edcd2dd0e8d11fb19db1974a0114df0/

Internal to this repo:

- CFP submission: `../cfp-submission/nodes-2026-cfp-submission.md`
- Source blog post the talk extends: `../graphql-mcp-ai-llm/`
  (the original blog post on GraphQL for MCP servers)
- Data model design: `../data-model/README.md`
- Slack subgraph implementation: `../slack-subgraph/README.md`
