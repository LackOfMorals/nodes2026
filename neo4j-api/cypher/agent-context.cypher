// Embedded copy for neo4j-api (go:embed). The canonical version is
// data-model/queries/agent-context.cypher — keep the two in sync.

// The closing demo query — get the full discussion context for an issue.
//
// This is what the persisted operation `getIssueDiscussionContext` resolves to.
// One Cypher query, one round trip, full cross-system context.
//
// The returned shape gives the LLM agent everything it needs to summarise:
//   - The issue itself (title, state, priority, who created and owns it)
//   - The threads where it was discussed (with confidence scores)
//   - Every message in those threads, in order, with author names
//   - The Slack channel each thread lives in
//
// The agent doesn't have to make follow-up calls to Linear or Slack —
// the graph already has everything connected.
//
// Parameters:
//   $identifier — the Linear issue identifier (e.g. "NODES-123")

MATCH (i:Issue {identifier: $identifier})

// People connected to the issue (creator, assignee)
OPTIONAL MATCH (creator:Person)-[:CREATED]->(i)
OPTIONAL MATCH (assignee:Person)-[:ASSIGNED_TO]->(i)

// Threads where this issue was discussed, with the channel they live in
OPTIONAL MATCH (i)-[d:DISCUSSED_IN]->(t:Thread)<-[:HOSTS_THREAD]-(c:Channel)

// Messages in each thread, ordered, with their authors
OPTIONAL MATCH (t)-[:HAS_MESSAGE]->(m:Message)<-[:AUTHORED]-(author:Person)

WITH i, creator, assignee, t, d, c,
     collect({
       ts: m.ts,
       postedAt: m.postedAt,
       text: m.text,
       authorName: author.name,
       authorEmail: author.email,
       permalink: m.permalink
     }) AS messages

WITH i, creator, assignee,
     collect({
       channel: c.name,
       threadTs: t.ts,
       startedAt: t.startedAt,
       permalink: t.permalink,
       confidence: d.confidence,
       evidence: d.evidence,
       messages: [msg IN messages WHERE msg.ts IS NOT NULL | msg]
     }) AS discussions

RETURN
  i.identifier AS identifier,
  i.title AS title,
  i.state AS state,
  i.priority AS priority,
  i.linearUrl AS linearUrl,
  i.createdAt AS createdAt,
  i.updatedAt AS updatedAt,
  {name: creator.name, email: creator.email} AS createdBy,
  CASE WHEN assignee IS NOT NULL
    THEN {name: assignee.name, email: assignee.email}
    ELSE NULL
  END AS assignedTo,
  [d IN discussions WHERE d.channel IS NOT NULL | d] AS discussions;
