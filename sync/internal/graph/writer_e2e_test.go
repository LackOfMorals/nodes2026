package graph

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LackOfMorals/nodes2026/sync/internal/linear"
	syncslack "github.com/LackOfMorals/nodes2026/sync/internal/slack"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// TestUpsertWrites_EndToEnd (PLAN.md task 1.5) feeds synthetic
// Linear/Slack-shaped payloads through the real writer against a live
// local Neo4j, proving upsert-issue.cypher, upsert-thread.cypher, and
// link-explicit.cypher all execute under the one-file-one-statement
// contract (whole file = one tx.Run).
//
// Skipped unless WRITER_E2E=1, because it needs a running database and
// it writes rows. Override the target with WRITER_E2E_URI (default
// bolt://localhost:7687) and WRITER_E2E_QUERIES (default
// <repo>/data-model/queries).
//
// The synthetic data is deliberately NEW (NODES-7, proj_sync_test,
// kim@example.com, thread ts 1778745600.000100) except for the channel
// and one person, which exercise the merge-into-existing path. Remove
// the rows with the Phase-2 reset one-liner if you don't want them.
func TestUpsertWrites_EndToEnd(t *testing.T) {
	if os.Getenv("WRITER_E2E") != "1" {
		t.Skip("set WRITER_E2E=1 to run against a live local Neo4j")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	uri := os.Getenv("WRITER_E2E_URI")
	if uri == "" {
		uri = "bolt://localhost:7687"
	}
	queriesDir := os.Getenv("WRITER_E2E_QUERIES")
	if queriesDir == "" {
		queriesDir, _ = filepath.Abs(filepath.Join("..", "..", "..", "data-model", "queries"))
	}

	w, err := NewWriter(ctx, Config{
		URI:          uri,
		Username:     "neo4j",
		Password:     "password",
		DatabaseName: "neo4j",
		QueriesDir:   queriesDir,
	})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close(ctx)

	// 1. Issue upsert, run twice (idempotency). No assignee, so the
	//    $assigneeEmail IS NULL branch is exercised.
	issue := linear.Issue{
		ID:         "issue_nodes_7",
		Identifier: "NODES-7",
		Title:      "Add retry budget to Linear subgraph client",
		State:      "In Progress",
		Priority:   2,
		URL:        "https://linear.app/example/issue/NODES-7",
		CreatedAt:  time.Date(2026, 5, 14, 8, 30, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC),
		ProjectID:   "proj_sync_test",
		ProjectName: "Sync Writer Test Project",

		CreatorEmail:    "kim@example.com",
		CreatorLinearID: "user_kim_linear",
		CreatorName:     "Kim Osei",
	}
	if err := w.UpsertIssue(ctx, issue); err != nil {
		t.Fatalf("UpsertIssue: %v", err)
	}
	if err := w.UpsertIssue(ctx, issue); err != nil {
		t.Fatalf("UpsertIssue (2nd run, idempotency): %v", err)
	}

	// 2. Thread upsert in the pre-existing channel; jess is an existing
	//    person (merge-into-existing), kim is new. Run twice.
	channel := syncslack.Channel{
		ID:      "C09ABCDEF12",
		Name:    "nodes-demo-eng",
		Purpose: "Engineering discussion for the NODES demo",
	}
	thread := syncslack.Thread{
		ChannelID: "C09ABCDEF12",
		TS:        "1778745600.000100",
		Permalink: "https://example.slack.com/archives/C09ABCDEF12/p1778745600000100",
		Messages: []syncslack.Message{
			{
				TS:          "1778745600.000100",
				UserID:      "U09KIMID007",
				Text:        "Following up on NODES-7 — the retry budget change shipped, monitoring for flakiness.",
				ThreadTS:    "1778745600.000100",
				Permalink:   "https://example.slack.com/archives/C09ABCDEF12/p1778745600000100",
				AuthorEmail:   "kim@example.com",
				AuthorName:    "Kim Osei",
				AuthorSlackID: "U09KIMID007",
			},
			{
				TS:          "1778745900.000200",
				UserID:      "U09JESSID006",
				Text:        "Thanks for the heads up. Will watch the dashboards today.",
				ThreadTS:    "1778745600.000100",
				Permalink:   "https://example.slack.com/archives/C09ABCDEF12/p1778745900000200",
				AuthorEmail:   "jess@example.com",
				AuthorName:    "Jess Morgan",
				AuthorSlackID: "U09JESSID006",
			},
		},
	}
	if err := w.UpsertThread(ctx, channel, thread); err != nil {
		t.Fatalf("UpsertThread: %v", err)
	}
	if err := w.UpsertThread(ctx, channel, thread); err != nil {
		t.Fatalf("UpsertThread (2nd run, idempotency): %v", err)
	}

	// 3. Explicit linking pass — must create the NODES-7 edge.
	if err := w.RunExplicitLinking(ctx); err != nil {
		t.Fatalf("RunExplicitLinking: %v", err)
	}

	// 4. Verify graph state through the writer's own driver.
	session := w.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: "neo4j",
		AccessMode:   neo4j.AccessModeRead,
	})
	defer session.Close(ctx)

	// count runs a single-column count query and returns the value.
	count := func(t *testing.T, cypher string) int {
		t.Helper()
		got, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			res, err := tx.Run(ctx, cypher, nil)
			if err != nil {
				return nil, err
			}
			return res.Collect(ctx)
		})
		if err != nil {
			t.Fatalf("verify query: %v", err)
		}
		recs, ok := got.([]*neo4j.Record)
		if !ok {
			t.Fatalf("verify query: unexpected result type %T", got)
		}
		if len(recs) != 1 || len(recs[0].Values) != 1 {
			t.Fatalf("verify query: expected one count row, got %d records", len(recs))
		}
		v, ok := recs[0].Values[0].(int64)
		if !ok {
			t.Fatalf("verify query: expected int64 count, got %T", recs[0].Values[0])
		}
		return int(v)
	}

	if n := count(t, "MATCH (p:Project {id: 'proj_sync_test'})-[:HAS_ISSUE]->(i:Issue {id: 'issue_nodes_7'}) RETURN count(p)"); n != 1 {
		t.Errorf("NODES-7 project link: got %d, want 1", n)
	}
	if n := count(t, "MATCH (i:Issue {identifier: 'NODES-7'})-[d:DISCUSSED_IN]->(t:Thread {ts: '1778745600.000100'}) RETURN count(d)"); n != 1 {
		t.Errorf("NODES-7 -> new thread DISCUSSED_IN edge (created by linking pass): got %d, want 1", n)
	}
	if n := count(t, "MATCH (p:Person {email: 'kim@example.com'}) WHERE p.linearId IS NOT NULL AND p.slackId IS NOT NULL RETURN count(p)"); n != 1 {
		t.Errorf("Person unification (kim has both linearId and slackId): got %d, want 1", n)
	}
	if n := count(t, "MATCH (j:Person {email: 'jess@example.com'})-[:AUTHORED]->(m:Message {ts: '1778745900.000200'}) RETURN count(m)"); n != 1 {
		t.Errorf("jess AUTHORED edge on merged-into-existing person: got %d, want 1", n)
	}
	if n := count(t, "MATCH (t:Thread {ts: '1778745600.000100'})-[:HAS_MESSAGE]->(m) RETURN count(m)"); n != 2 {
		t.Errorf("new thread message count: got %d, want 2", n)
	}
}
