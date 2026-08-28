// Package graph provides the Neo4j writer for the sync pipeline.
//
// It loads parameterised Cypher from disk (the data-model/queries/*.cypher
// files) and runs them as idempotent upserts. We deliberately read the
// Cypher from files rather than hardcoding it in Go — the data-model is
// the source of truth, and the Go code shouldn't duplicate it.
package graph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/LackOfMorals/nodes2026/sync/internal/linear"
	syncslack "github.com/LackOfMorals/nodes2026/sync/internal/slack"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Writer runs idempotent upserts against Neo4j.
type Writer struct {
	driver        neo4j.DriverWithContext
	queriesDir    string
	databaseName  string
	loadedQueries map[string]string
}

// Config configures the Writer.
type Config struct {
	URI          string // e.g. neo4j+s://xxx.databases.neo4j.io
	Username     string
	Password     string
	DatabaseName string // typically "neo4j" for Aura

	// QueriesDir is the path to data-model/queries. The Writer loads
	// upsert-issue.cypher, upsert-thread.cypher, and link-explicit.cypher
	// from this directory.
	QueriesDir string
}

// NewWriter connects to Neo4j and loads the Cypher files into memory.
func NewWriter(ctx context.Context, cfg Config) (*Writer, error) {
	driver, err := neo4j.NewDriverWithContext(
		cfg.URI,
		neo4j.BasicAuth(cfg.Username, cfg.Password, ""),
	)
	if err != nil {
		return nil, fmt.Errorf("create driver: %w", err)
	}

	if err := driver.VerifyConnectivity(ctx); err != nil {
		return nil, fmt.Errorf("verify connectivity: %w", err)
	}

	w := &Writer{
		driver:        driver,
		queriesDir:    cfg.QueriesDir,
		databaseName:  cfg.DatabaseName,
		loadedQueries: make(map[string]string),
	}

	// Pre-load every Cypher file we'll need.
	for _, name := range []string{
		"upsert-issue.cypher",
		"upsert-thread.cypher",
		"link-explicit.cypher",
	} {
		path := filepath.Join(cfg.QueriesDir, name)
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", name, err)
		}
		w.loadedQueries[name] = string(body)
	}

	return w, nil
}

// Close releases the Neo4j driver.
func (w *Writer) Close(ctx context.Context) error {
	return w.driver.Close(ctx)
}

// UpsertIssue runs upsert-issue.cypher against a single Linear issue.
func (w *Writer) UpsertIssue(ctx context.Context, issue linear.Issue) error {
	params := map[string]any{
		"id":              issue.ID,
		"identifier":      issue.Identifier,
		"title":           issue.Title,
		"state":           issue.State,
		"priority":        issue.Priority,
		"linearUrl":       issue.URL,
		"createdAt":       issue.CreatedAt.Format(time.RFC3339),
		"updatedAt":       issue.UpdatedAt.Format(time.RFC3339),
		"projectId":       issue.ProjectID,
		"projectName":     issue.ProjectName,
		"creatorEmail":    issue.CreatorEmail,
		"creatorLinearId": issue.CreatorLinearID,
		"creatorName":     issue.CreatorName,
	}

	// Nil-coalesce assignee fields so the Cypher conditional works.
	if issue.AssigneeEmail != nil {
		params["assigneeEmail"] = *issue.AssigneeEmail
		params["assigneeLinearId"] = *issue.AssigneeLinearID
		params["assigneeName"] = *issue.AssigneeName
	} else {
		params["assigneeEmail"] = nil
		params["assigneeLinearId"] = nil
		params["assigneeName"] = nil
	}

	return w.execute(ctx, w.loadedQueries["upsert-issue.cypher"], params)
}

// UpsertThread runs upsert-thread.cypher against a Slack thread + channel.
//
// `channelName` and `channelPurpose` are passed alongside because the Cypher
// upserts the channel as part of the same idempotent operation.
func (w *Writer) UpsertThread(ctx context.Context, channel syncslack.Channel, thread syncslack.Thread) error {
	messages := make([]map[string]any, 0, len(thread.Messages))
	for _, m := range thread.Messages {
		messages = append(messages, map[string]any{
			"ts":             m.TS,
			"userId":         m.UserID,
			"text":           m.Text,
			"threadTs":       m.ThreadTS,
			"permalink":      m.Permalink,
			"authorEmail":    m.AuthorEmail,
			"authorName":     m.AuthorName,
			"authorSlackId":  m.AuthorSlackID,
		})
	}

	params := map[string]any{
		"channelId":       channel.ID,
		"channelName":     channel.Name,
		"channelPurpose":  channel.Purpose,
		"threadTs":        thread.TS,
		"threadPermalink": thread.Permalink,
		"messages":        messages,
	}

	return w.execute(ctx, w.loadedQueries["upsert-thread.cypher"], params)
}

// RunExplicitLinking executes the linking pass that creates :DISCUSSED_IN
// edges where messages explicitly mention issue identifiers. The pass is
// pure Cypher and takes no parameters — it links against every Issue
// already in the graph.
func (w *Writer) RunExplicitLinking(ctx context.Context) error {
	return w.execute(ctx, w.loadedQueries["link-explicit.cypher"], nil)
}

// execute runs a Cypher statement in a write transaction.
func (w *Writer) execute(ctx context.Context, cypher string, params map[string]any) error {
	session := w.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: w.databaseName,
		AccessMode:   neo4j.AccessModeWrite,
	})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, cypher, params)
		return nil, err
	})
	if err != nil {
		return fmt.Errorf("execute cypher: %w", err)
	}

	return nil
}
