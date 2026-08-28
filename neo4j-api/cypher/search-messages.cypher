// Message search across the discussion graph — the Cypher behind the
// neo4j-api `searchMessages` operation.
//
// Verified against the live container (Neo4j 2026.07.1) on 2026-08-27
// against the canonical seeded graph.
//
// Parameters:
//   $q     — substring to match, case-insensitive
//   $limit — maximum number of rows (GraphQL default 20)

MATCH (m:Message)
WHERE toLower(m.text) CONTAINS toLower($q)
OPTIONAL MATCH (t:Thread)-[:HAS_MESSAGE]->(m)
OPTIONAL MATCH (c:Channel)-[:HOSTS_THREAD]->(t)
OPTIONAL MATCH (a:Person)-[:AUTHORED]->(m)
RETURN
  m.ts AS ts,
  m.postedAt AS postedAt,
  m.text AS text,
  a.name AS authorName,
  a.email AS authorEmail,
  m.permalink AS permalink,
  t.ts AS threadTs,
  c.name AS channel
ORDER BY m.postedAt ASC
LIMIT $limit;
