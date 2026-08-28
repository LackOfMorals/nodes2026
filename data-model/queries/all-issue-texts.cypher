// List every Issue's id and embeddable text (title). Used by the
// embed-and-link-semantic sync stage to (re-)embed every issue on each
// run — demo-scale data (a handful of issues) makes re-embedding
// everything every run simpler and safer than tracking what changed.
//
// IMPORTANT: one file = one statement (see link-explicit.cypher).
//
// No parameters.

MATCH (i:Issue)
RETURN i.id AS id, i.identifier AS identifier, i.title AS text;
