// Write an embedding vector onto a single Issue node.
//
// Run once per issue during the embed-and-link-semantic sync stage, after
// the explicit linking pass. Overwrites any prior embedding — safe to
// re-run every sync tick, which is how the live-watch demo picks up
// title edits without any change-tracking logic.
//
// IMPORTANT: the sync writer (sync/internal/graph/writer.go) executes this
// entire file as ONE Cypher statement (one tx.Run call), so it must stay
// a single statement: no intermediate semicolons.
//
// Parameters:
//   $id             — Linear UUID for the issue (matches upsert-issue.cypher)
//   $vector         — the embedding, as a list of floats
//   $embeddingModel — the model name that produced $vector, for drift detection

MATCH (i:Issue {id: $id})
CALL db.create.setNodeVectorProperty(i, 'embedding', $vector)
SET i.embeddingModel = $embeddingModel
RETURN i.identifier AS identifier;
