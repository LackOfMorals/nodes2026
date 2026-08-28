# NODES 2026 — slide outline

Slide-by-slide outline for *A murder of MCP servers: taming AI tool
sprawl with GraphQL federation and a knowledge graph*. Built from
`TALK.md`'s working outline (argument, structure, timings) plus what's
actually running today — the local federation stack in `router/` and the
live embedding demo in `sync/` (see `DEMO-ENV.md`).

This is a content outline, not final design: one slide per numbered
block below, titles and bullets as speaker material, not copy-paste
deck text. Build the real deck (Keynote/PowerPoint) from this.

**Style rules (from `TALK.md`'s editorial notes):** sentence case
headings. No "leverage," no "delve." Plain, direct language. No
corporate openers ("Today's engineers face...").

**Runtime:** ~38–40 min against a 30–45 min slot, leaving 5–10 min for
Q&A. The live-embedding segment (new since the original outline) adds
~4–5 min over the original plan — trimmed section 2 to make room. Time
this for real in a dry run before trusting it; see the note at the end.

---

## Section 1 — The problem statement (~3 min)

**Slide 1 — Title**
- *A murder of MCP servers*
- Taming AI tool sprawl with GraphQL federation and a knowledge graph
- Jonathan Giffard · Lead PM, Neo4j · NODES 2026

**Slide 2 — The visual: a murder of MCP servers**
- A cluttered grid/flock of MCP server logos and names — Linear, Slack,
  GitHub, Notion, Jira, Salesforce, Google Drive, Figma, Zendesk... as
  many as fit, deliberately overwhelming
- One line under it: *"Every SaaS vendor ships one now."*
- Visual gag: "murder" = a flock of crows — lean into it if the room
  reads as receptive; drop it if it feels like a stretch for the venue

**Slide 3 — The constraint has moved**
- MCP solved the AI *integration* problem — connecting to a system is a
  solved problem now
- Wire ten servers together and you get a different problem: auth
  sprawl, disconnected tool namespaces, inconsistent response shapes
- **Quote (verbatim, land it deliberately):** "The constraint in
  production AI isn't connecting to systems anymore. It's managing the
  surface area."
- Hook line to close the section: "We've been here before."

---

## Section 2 — The precedent (~2 min, trimmed from 3)

**Slide 4 — REST sprawl, before federation**
- Same shape of problem, one layer down the stack: dozens of REST
  services, each with its own auth, its own shape, its own client code
- GraphQL federation's answer: independent services behind one unified
  schema, one endpoint

**Slide 5 — Multiple vendors are already doing this for MCP**
- Reference (unnamed per the editorial note — "vendor convergence stays
  unnamed, credibility signal without being a pitch") that production
  federation gateways already ship an MCP surface on top of the composed
  schema
- This isn't speculative architecture — it's the same pattern, one layer
  up, and it's already shipping

---

## Section 3 — The pattern applied to MCP (~5 min)

**Slide 6 — What the agent sees**
- One MCP endpoint. One auth surface.
- Authenticated via OAuth in production (Cognito-backed); a bearer token
  or nothing at all for a local dev/demo router
- Discovers a handful of **named, persisted operations** — not the raw
  schema

**Slide 7 — The collapse, in real numbers**
- Three real, different upstream protocols behind one endpoint: Linear
  (native GraphQL, registered directly), Neo4j (a small GraphQL library
  server), Slack (REST, wrapped as a router plugin)
- Without federation: a stock Slack MCP server (13 tools) + Linear's
  hosted MCP + a bare Neo4j read-only canary — three servers, dozens of
  tools, and the agent has to pick among them itself
- With federation: **5 tools.** One endpoint.
- This number is live-verified, not a slideware estimate — say so

**Slide 8 — Persisted operations as the governance layer**
- The MCP surface exposes *only* four named operations (plus a
  built-in `get_operation_info`) — not 60+ raw federated GraphQL fields
- `get_issue_discussion_context`, `search_messages`,
  `linear_project_issues`, `slack_thread` — each one a deliberate,
  reviewed contract, not "whatever the schema happens to expose"
- This is the enforcement point: an agent can't call what isn't named
  here, no matter what the underlying subgraphs can do

---

## Section 4 — The honest edges (~2 min)

**Slide 9 — Three tensions, said out loud before Q&A finds them**
1. **Action-heavy vs. retrieval-heavy agents.** Federation handles reads
   cleanly. Writes (sending Slack messages, creating Linear issues) work
   too, but the governance story is harder — persisted operations help,
   this isn't fully solved.
2. **The gateway is itself a component.** Trading MCP-server sprawl for
   one more piece of operational surface area. The trade is worth it
   above roughly 4 MCP servers — say that number explicitly.
3. **Threshold tuning for vector linking is a real, live-tested
   tension, not a hypothetical one** — full story in Section 7, flagged
   here as a preview so it doesn't feel bolted on later.

---

## Section 5 — What federation doesn't give you (~3 min)

**Slide 10 — Manageable is not the same as intelligent**
- A federated endpoint collapses the tool surface. It does not, by
  itself, let the agent reason *across* the three systems.
- The knowledge graph is not a fourth data source sitting next to the
  other three — it's the layer that holds the *relationships between*
  them.
- Six node types, seven relationship types (see `data-model/README.md`)
  — the interesting one is `:DISCUSSED_IN`, which carries `confidence`
  and `evidence`, not just a bare edge
- **Closing line for this slide, quotable:** "A federated endpoint is
  manageable. A knowledge graph makes it intelligent."

---

## Section 6 — Live demo (~13–14 min — the core of the talk)

Two connected movements: first the federated-vs-not contrast on the
canonical seeded graph, then the live embedding segment on real data.
Both run from the same laptop, same router, same MCP endpoint —
different databases underneath (`neo4j` for the seeded story,
`livedemo` for the real-data segment — see `DEMO-ENV.md`).

**Slide 11 — Architecture, 60 seconds**
- One diagram: laptop, podman-run Neo4j, the local Cosmo Router (Slack
  plugin in-process, Linear registered directly, Neo4j via a small
  GraphQL library server), MCP Gateway on top
- Say explicitly: this is local-first for the demo; the "where this
  runs in production" slide (AWS Fargate + Aura + a real control plane)
  comes later as one slide, not the live setup

**Slide 12 — The graph, 90 seconds**
- Show the seeded graph in Neo4j Browser: 6 issues, 4 threads, 12
  messages, 6 people, 5 explicit `:DISCUSSED_IN` edges
- Point at the one deliberately orphaned thread — discusses "the schema
  problem" without naming the issue. The regex/explicit pass can't catch
  it. Set up: "there's a second story in this graph, and we'll come
  back to it."

**Slide 13 — Without federation (contrast first)**
- Agent has three separate MCP servers open: Linear, Slack, Neo4j
- Ask it: *"What did the team decide about the schema validation issue
  in NODES-1?"*
- Watch it make several tool calls across servers, get scattered
  context, and produce a weaker answer — and note live that the Slack
  server here has **no message-search tool at all** (a bot token cannot
  call `search.messages` — Slack platform constraint, not a federation
  argument, worth being precise about on stage)

**Slide 14 — With federation**
- Same agent, same question, pointed at the router's MCP endpoint now
- **One tool call:** `get_issue_discussion_context(identifier: "NODES-1")`
- Structured response, synthesized answer: *"The team decided to use
  explicit `@shareable` annotations across all three subgraphs to
  preserve nullability during composition. Sarah identified the root
  cause; Alex made the decision and confirmed the fix landed."*

**Slide 15 — The Cypher behind the answer, 90 seconds**
- Click into the resolved Cypher (Neo4j GraphQL library debug logging —
  literally what ran)
- One `MATCH` traversing `:DISCUSSED_IN`, crossing what started as
  Linear data and Slack data as one connected graph
- The agent never knew it was talking to a graph — to it, this was just
  a GraphQL response

**Slide 16 — The orphaned thread, the pivot into the live segment**
- Back to the thread from slide 12: "the team has another discussion
  about the same problem — they just didn't name the issue, so the
  regex linker missed it."
- "Watch what happens when we let the graph find that connection itself,
  on real data, live, right now."

**Slide 17 — LIVE: post something and watch it land**
- Switch to the real workspace (`livedemo` database, separate from the
  seeded story on purpose — see the callout on slide 19)
- `sync --watch` already running in a visible terminal, polling on an
  interval
- Post a new Slack message that discusses an existing real issue
  *without naming its identifier* — the same shape as the seeded
  orphan
- Talk through one or two polling intervals while it sits there
- Point at the terminal: the next tick embeds it and creates the
  `:DISCUSSED_IN` edge live — `evidence: "semantic_match"`, not
  `explicit_mention`, because nothing was told to look for this; the
  embedding found it
- Show the new edge directly against the graph (not through the MCP
  tool — that points at a different database right now, see slide 19)

---

## Section 7 — What building this live actually taught us (~4 min)

This replaces the original "iteration 2 preview" slide. Vectors are no
longer a future slide — they ran live, against real data, and the
result is a better, more honest story than "here's what's coming next."

**Slide 18 — This isn't a preview anymore**
- Real model, running locally: nomic-embed-text-v1.5, LM Studio, no
  cloud embedding API in the loop
- The line to land: "The graph stores embeddings as a property type.
  Any model that produces a list of floats can populate it — the vector
  index doesn't care where the floats came from."

**Slide 19 — Two things we found the hard way**
1. **Real data and seeded data can't share a database.** The real
   workspace mirrors the demo story closely enough that their
   embeddings collided — cross-links between the fictional and real
   versions of the same issues. Fixed with a second, isolated database.
   Not a hypothetical caveat — this broke on the first real run.
2. **A single similarity threshold doesn't generalize.** Tuned against
   six issues: 0.78 is the only value in a 0.0024-wide window that
   keeps the one intended link and excludes four false ones. Run
   against eight real issues: the intended link still forms — but seven
   *more* false positives get through at the same threshold. Short,
   same-project technical text just clusters too tightly for one
   number to cleanly separate signal from noise at this scale.

**Slide 20 — What production needs instead**
- Not a bigger threshold — a better scoring signal: reranking, a
  margin-based "best match" instead of "everything above X," more
  context per embedded chunk, maybe a hybrid lexical+semantic score
- Say plainly: this wasn't A/B tested, it was tuned against one seed
  set in one evening, and the numbers above are exactly why that
  distinction matters

---

## Section 8 — The takeaway (~3 min)

**Slide 21 — Three layers, three jobs**
- **Federation** is the mechanism — one endpoint, one auth surface, a
  collapsed tool list
- **The knowledge graph** is the meaning — the relationships *between*
  systems, not just another system
- **Vectors** are discovery — finding what nobody explicitly named, with
  an honest accounting of where that breaks down at small scale

**Slide 22 — Closing slide**
- Quotable close: *"A federated endpoint is manageable. A knowledge
  graph makes it intelligent. And when you actually build the vector
  layer instead of just proposing it, you find out exactly where the
  easy version breaks — which is a better talk than pretending it
  doesn't."*
- Contact / repo link (code and data model are public per the original
  CFP promise — confirm this is still true before the talk; the repo
  currently sits at whatever visibility state it's in on GitHub)

**Q&A (5–10 min)**
- Anticipated questions worth a prepared one-liner: production
  hardening (OTel, retries, caching — one slide's worth if asked, not
  presented proactively), whether this needs Cosmo/Apollo specifically
  (no — the pattern, not the vendor), what happens at real scale with
  the embedding noise problem from slide 19

---

## Notes for building the real deck

- **Section 6 and 7 are the parts that changed most** since the
  original `TALK.md` outline — the original plan hedged iteration 2 as
  "live if stable, slides-only if not, decide end of September." It's
  done, it's live, and the honest-failure story it produced (slide 19)
  is more interesting than the safe version would have been. Don't
  undersell that in the deck — it's the most technically credible part
  of the talk for this specific audience.
- **Timing risk:** this outline runs longer than `TALK.md`'s original
  35-minute plan. Section 6 alone is ~13–14 min against an original
  ~10 min live-demo budget. Time a real dry run before trusting the
  numbers above — if it's tight, the first candidate to cut is slide 5
  (vendor-convergence context), not anything in the demo or the honest
  tensions.
- **Recorded backup video:** per `TALK.md`'s existing plan, same demo,
  recorded, substitutable at any point if something breaks live. That
  plan now needs to cover the live-embedding segment too, not just the
  federation contrast — re-record once the segment is rehearsed.
- **Slide 3's quote and slide 10's closing line are the two lines most
  worth memorizing** verbatim — they're the ones doing the most
  argumentative work per word.
