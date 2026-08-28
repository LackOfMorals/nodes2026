// Schema for the NODES 2026 demo knowledge graph.
//
// Iteration 1: graph-only model. No vector indexes yet — those land in
// iteration 2 with embedding support.
//
// Run this once against a fresh Neo4j Aura database before any data ingest.
// All constraints and indexes are idempotent (IF NOT EXISTS).

// ----- Uniqueness constraints -----
//
// Constraints double as indexes — every node lookup by these properties
// goes through the constraint index automatically.

CREATE CONSTRAINT project_id_unique IF NOT EXISTS
FOR (p:Project) REQUIRE p.id IS UNIQUE;

CREATE CONSTRAINT issue_id_unique IF NOT EXISTS
FOR (i:Issue) REQUIRE i.id IS UNIQUE;

CREATE CONSTRAINT issue_identifier_unique IF NOT EXISTS
FOR (i:Issue) REQUIRE i.identifier IS UNIQUE;

CREATE CONSTRAINT channel_id_unique IF NOT EXISTS
FOR (c:Channel) REQUIRE c.id IS UNIQUE;

CREATE CONSTRAINT person_id_unique IF NOT EXISTS
FOR (p:Person) REQUIRE p.id IS UNIQUE;

// Person.email is the join key between Linear and Slack identities.
// Enforce uniqueness so we don't accidentally create two Person nodes
// for the same human.
CREATE CONSTRAINT person_email_unique IF NOT EXISTS
FOR (p:Person) REQUIRE p.email IS UNIQUE;

// Thread is uniquely identified by (channelId, ts). Slack message
// timestamps are globally unique within a channel.
CREATE CONSTRAINT thread_key IF NOT EXISTS
FOR (t:Thread) REQUIRE (t.channelId, t.ts) IS NODE KEY;

// Same composite uniqueness for Message.
CREATE CONSTRAINT message_key IF NOT EXISTS
FOR (m:Message) REQUIRE (m.channelId, m.ts) IS NODE KEY;

// ----- Lookup indexes -----
//
// Indexes for properties we filter or look up but don't make unique.

// Person.linearId — used during ingest to find a Person from a Linear API response
CREATE INDEX person_linear_id IF NOT EXISTS
FOR (p:Person) ON (p.linearId);

// Person.slackId — used during ingest to find a Person from a Slack API response
CREATE INDEX person_slack_id IF NOT EXISTS
FOR (p:Person) ON (p.slackId);

// Issue.state — for filtering open vs done issues
CREATE INDEX issue_state IF NOT EXISTS
FOR (i:Issue) ON (i.state);

// Issue.updatedAt — for "recently active" queries
CREATE INDEX issue_updated_at IF NOT EXISTS
FOR (i:Issue) ON (i.updatedAt);

// Message.postedAt — for chronological ordering and recency filters
CREATE INDEX message_posted_at IF NOT EXISTS
FOR (m:Message) ON (m.postedAt);

// Thread.startedAt — for chronological ordering of threads
CREATE INDEX thread_started_at IF NOT EXISTS
FOR (t:Thread) ON (t.startedAt);

// Channel.name — for human-readable channel lookups during search
CREATE INDEX channel_name IF NOT EXISTS
FOR (c:Channel) ON (c.name);

// ----- Relationship indexes -----
//
// :DISCUSSED_IN carries confidence and evidence properties. Index on
// confidence lets us filter "high-confidence links only" efficiently.
CREATE INDEX discussed_in_confidence IF NOT EXISTS
FOR ()-[r:DISCUSSED_IN]-() ON (r.confidence);

// ----- Vector indexes (iteration 2) -----
//
// Dimension 768 matches nomic-embed-text-v1.5, the local LM Studio model
// this demo embeds with. Changing embedding models means changing this
// dimension and recreating both indexes — the embeddingModel property
// written alongside every embedding is what lets you detect that drift
// with a WHERE clause instead of it silently corrupting similarity scores.
CREATE VECTOR INDEX issue_embedding IF NOT EXISTS
FOR (i:Issue) ON (i.embedding)
OPTIONS {indexConfig: {`vector.dimensions`: 768, `vector.similarity_function`: 'cosine'}};

CREATE VECTOR INDEX message_embedding IF NOT EXISTS
FOR (m:Message) ON (m.embedding)
OPTIONS {indexConfig: {`vector.dimensions`: 768, `vector.similarity_function`: 'cosine'}};
