// Upsert a single Linear issue along with its project, creator, and assignee.
//
// Run once per issue during sync. Idempotent — running this twice with the
// same parameters produces the same graph state.
//
// IMPORTANT: the sync writer (sync/internal/graph/writer.go) executes this
// entire file as ONE Cypher statement (one tx.Run call). The Neo4j server
// rejects more than one statement per query, and Cypher does not share
// variables between statements, so this file must stay a single
// statement: clauses joined with WITH, no intermediate semicolons.
//
// Parameters:
//   $id              — Linear UUID for the issue
//   $identifier      — human-readable identifier (e.g. "NODES-123")
//   $title           — issue title
//   $state           — Linear state name (Triage, Backlog, In Progress, Done, Cancelled)
//   $priority        — 0-4 (0 = no priority, 4 = urgent)
//   $linearUrl       — link to the issue in Linear
//   $createdAt       — ISO 8601 timestamp
//   $updatedAt       — ISO 8601 timestamp
//
//   $projectId       — Linear project UUID
//   $projectName     — Linear project name
//
//   $creatorEmail    — email of the issue creator (Linear "createdBy")
//   $creatorLinearId — Linear user UUID of the creator
//   $creatorName     — display name of the creator
//
//   $assigneeEmail   — email of the assignee, or null if unassigned
//   $assigneeLinearId — Linear user UUID of the assignee
//   $assigneeName    — display name of the assignee

// Upsert the issue
MERGE (i:Issue {id: $id})
SET i.identifier = $identifier,
    i.title = $title,
    i.state = $state,
    i.priority = $priority,
    i.linearUrl = $linearUrl,
    i.createdAt = datetime($createdAt),
    i.updatedAt = datetime($updatedAt)

// Upsert the project and connect it
MERGE (p:Project {id: $projectId})
SET p.name = $projectName
MERGE (p)-[:HAS_ISSUE]->(i)

// Upsert the creator and connect them
MERGE (creator:Person {email: $creatorEmail})
ON CREATE SET creator.id = randomUUID()
SET creator.linearId = $creatorLinearId,
    creator.name = coalesce(creator.name, $creatorName)
MERGE (creator)-[:CREATED]->(i)

// Conditionally upsert the assignee (skipped when unassigned)
WITH i
WHERE $assigneeEmail IS NOT NULL
MERGE (assignee:Person {email: $assigneeEmail})
ON CREATE SET assignee.id = randomUUID()
SET assignee.linearId = $assigneeLinearId,
    assignee.name = coalesce(assignee.name, $assigneeName)
MERGE (assignee)-[:ASSIGNED_TO]->(i)

RETURN i.identifier AS identifier;
