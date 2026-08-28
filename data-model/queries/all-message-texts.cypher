// List every Message's key and embeddable text. See all-issue-texts.cypher
// for why this re-embeds everything rather than tracking deltas.
//
// IMPORTANT: one file = one statement (see link-explicit.cypher).
//
// No parameters.

MATCH (m:Message)
RETURN m.channelId AS channelId, m.ts AS ts, m.text AS text;
