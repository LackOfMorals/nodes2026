// Write an embedding vector onto a single Message node.
//
// See embed-issue.cypher for why this overwrites unconditionally.
//
// IMPORTANT: one file = one statement (see link-explicit.cypher).
//
// Parameters:
//   $channelId, $ts — identify the message (matches upsert-thread.cypher's key)
//   $vector         — the embedding, as a list of floats
//   $embeddingModel — the model name that produced $vector, for drift detection

MATCH (m:Message {channelId: $channelId, ts: $ts})
CALL db.create.setNodeVectorProperty(m, 'embedding', $vector)
SET m.embeddingModel = $embeddingModel
RETURN m.ts AS ts;
