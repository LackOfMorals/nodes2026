// The semantic linking pass: for every Issue with an embedding, find
// Messages whose embedding is cosine-similar above $threshold, and
// create a :DISCUSSED_IN edge to the thread that message belongs to.
//
// Run after the embed-issue/embed-message steps, following the explicit
// linking pass (link-explicit.cypher). An (issue, thread) pair already
// linked by either pass is never touched again — ON CREATE SET only
// fires for a genuinely new pair, so an existing explicit_mention edge
// always wins over a later semantic_match for the same pair; it is
// never downgraded or overwritten.
//
// IMPORTANT: the sync writer (sync/internal/graph/writer.go) executes
// this entire file as ONE Cypher statement (one tx.Run call), so it must
// stay a single statement: no intermediate semicolons.
//
// Confidence is the cosine similarity score itself — this is the
// heuristic the talk is honest about on stage. 0.78 is not a round-number
// guess: it's tuned against real nomic-embed-text-v1.5 embeddings of the
// canonical seed data, where it's the only value that keeps NODES-1's
// true link to the orphaned thread (0.7811) while excluding four
// spurious matches from other issues (0.7663-0.7788) — a margin of only
// 0.0024. Short, topically-similar engineering text embeds close
// together; re-verify this number if the seed text ever changes.
//
// Parameters:
//   $threshold — minimum cosine similarity to create a link (0.78 in the demo)
//   $topK      — how many nearest messages to consider per issue

MATCH (i:Issue)
WHERE i.embedding IS NOT NULL
CALL (i) {
  CALL db.index.vector.queryNodes('message_embedding', $topK, i.embedding)
  YIELD node AS m, score
  WHERE score >= $threshold
  MATCH (t:Thread)-[:HAS_MESSAGE]->(m)
  RETURN t, max(score) AS bestScore
}
MERGE (i)-[d:DISCUSSED_IN]->(t)
ON CREATE SET d.confidence = bestScore,
              d.evidence = 'semantic_match',
              d.createdAt = datetime()
RETURN DISTINCT i.identifier AS issue, t.ts AS threadTs, d.confidence AS confidence, d.evidence AS evidence;
