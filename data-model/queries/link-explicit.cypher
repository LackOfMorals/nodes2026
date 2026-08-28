// The explicit linking pass: find messages that mention Linear issue
// identifiers (e.g. "NODES-123") and create :DISCUSSED_IN edges between
// the issue and the thread the message belongs to.
//
// Run this after every batch of new messages is upserted.
//
// Pure Cypher — no APOC. (Neo4j 2025+/2026.x no longer ships
// apoc.text.regexGroups.) Each Issue's identifier is matched against
// every message with word-boundary checks: the character before the
// match must not continue an identifier, and the character after must
// not either, so "NODES-12" does not create a link for "NODES-1".
//
// IMPORTANT: the sync writer (sync/internal/graph/writer.go) executes
// this entire file as ONE Cypher statement (one tx.Run call), so it must
// stay a single statement: no intermediate semicolons.
//
// No parameters — the pass links against every Issue already in the
// graph.
//
// Confidence is 1.0 because the link is explicit — the message named the
// issue directly. Iteration 2 adds a semantic linking pass that creates
// :DISCUSSED_IN edges with confidence < 1.0 based on vector similarity.

// Find every message that mentions an issue identifier, with boundaries
MATCH (m:Message)
MATCH (i:Issue)
WITH m, i,
  [pos IN range(0, size(m.text) - size(i.identifier))
   WHERE substring(m.text, pos, size(i.identifier)) = i.identifier] AS hits
WITH m, i,
  [pos IN hits
   WHERE (pos = 0 OR NOT substring(m.text, pos - 1, 1) =~ '[A-Za-z0-9-]')
      AND (pos + size(i.identifier) = size(m.text)
           OR NOT substring(m.text, pos + size(i.identifier), 1) =~ '[A-Za-z0-9-]')
  ] AS mentions
WHERE size(mentions) > 0

// For each mention, find the thread the message belongs to
MATCH (t:Thread)-[:HAS_MESSAGE]->(m)

// Create the :DISCUSSED_IN edge if it doesn't exist
MERGE (i)-[d:DISCUSSED_IN]->(t)
ON CREATE SET d.confidence = 1.0,
              d.evidence = 'explicit_mention',
              d.createdAt = datetime()

// DISTINCT: an issue mentioned in several messages of the same thread
// yields one row per edge, not one per mention.
RETURN DISTINCT i.identifier AS issue, t.ts AS threadTs, d.confidence AS confidence;
