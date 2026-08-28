// Upsert a Slack thread along with its channel, all its messages, and the
// people who authored them.
//
// Idempotent. Re-running with the same parameters produces the same graph.
//
// IMPORTANT: the sync writer (sync/internal/graph/writer.go) executes this
// entire file as ONE Cypher statement (one tx.Run call). The Neo4j server
// rejects more than one statement per query, and Cypher does not share
// variables between statements, so this file must stay a single
// statement: clauses joined with WITH, no intermediate semicolons.
//
// Parameters:
//   $channelId       — Slack channel ID
//   $channelName     — Slack channel name (without the leading #)
//   $channelPurpose  — channel purpose text, or empty string
//
//   $threadTs        — parent message timestamp (the thread identifier)
//   $threadPermalink — link to the thread in Slack
//
//   $messages        — list of {ts, userId, text, threadTs, permalink,
//                              authorEmail, authorName, authorSlackId}
//                      The parent message is included in this list with
//                      threadTs == ts.

// Upsert the channel
MERGE (c:Channel {id: $channelId})
SET c.name = $channelName,
    c.purpose = $channelPurpose

// Upsert the thread and connect it to the channel
MERGE (t:Thread {channelId: $channelId, ts: $threadTs})
ON CREATE SET t.startedAt = datetime({epochSeconds: toInteger(split($threadTs, '.')[0])}),
              t.permalink = $threadPermalink
SET t.messageCount = size($messages)
MERGE (c)-[:HOSTS_THREAD]->(t)

// Process each message in the thread
WITH t
UNWIND $messages AS msg
  MERGE (m:Message {channelId: $channelId, ts: msg.ts})
  SET m.text = msg.text,
      m.postedAt = datetime({epochSeconds: toInteger(split(msg.ts, '.')[0])}),
      m.threadTs = msg.threadTs,
      m.permalink = msg.permalink
  MERGE (t)-[:HAS_MESSAGE]->(m)

  // Upsert the author and link them to the message
  // authorEmail is the primary key; slackId may not always be available
  MERGE (author:Person {email: msg.authorEmail})
  ON CREATE SET author.id = randomUUID()
  SET author.slackId = coalesce(msg.authorSlackId, author.slackId),
      author.name = coalesce(author.name, msg.authorName)
  MERGE (author)-[:AUTHORED]->(m)

RETURN count(*) AS messagesProcessed;
