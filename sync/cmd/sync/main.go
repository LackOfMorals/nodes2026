// Command sync pulls Linear issues and Slack threads, writes them to
// Neo4j, and runs the explicit linking pass that creates :DISCUSSED_IN
// edges where messages reference issue identifiers.
//
// Usage:
//
//	sync \
//	  --linear-project <project-id> \
//	  --slack-channel <channel-id>
//
// Environment variables:
//
//	LINEAR_API_KEY   Personal API key from Linear Settings → API
//	SLACK_BOT_TOKEN  xoxb-... bot user OAuth token
//	NEO4J_URI        e.g. neo4j+s://xxxxx.databases.neo4j.io
//	NEO4J_USER       typically "neo4j"
//	NEO4J_PASSWORD   the Aura instance password
//	NEO4J_DATABASE   optional, defaults to "neo4j"
//	QUERIES_DIR      optional, defaults to "../../data-model/queries"
//
// The orchestration is procedural and stops on first error. Every stage
// is idempotent — re-running after a failure replays cleanly.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LackOfMorals/nodes2026/sync/internal/graph"
	"github.com/LackOfMorals/nodes2026/sync/internal/linear"
	"github.com/LackOfMorals/nodes2026/sync/internal/orchestrator"
	"github.com/LackOfMorals/nodes2026/sync/internal/slack"
)

func main() {
	linearProjectID := flag.String("linear-project", "", "Linear project ID to sync")
	slackChannelID := flag.String("slack-channel", "", "Slack channel ID to sync")
	slackSinceDays := flag.Int("slack-since-days", 30,
		"How many days of Slack history to pull (0 = all visible)")

	flag.Parse()

	if *linearProjectID == "" || *slackChannelID == "" {
		flag.Usage()
		os.Exit(2)
	}

	if err := run(*linearProjectID, *slackChannelID, *slackSinceDays); err != nil {
		log.Fatalf("sync failed: %v", err)
	}
}

func run(linearProjectID, slackChannelID string, slackSinceDays int) error {
	// SIGINT/SIGTERM cancel the context so in-flight work can clean up.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// ----- Required configuration from environment -----

	linearKey := os.Getenv("LINEAR_API_KEY")
	if linearKey == "" {
		return fmt.Errorf("LINEAR_API_KEY environment variable is required")
	}

	slackToken := os.Getenv("SLACK_BOT_TOKEN")
	if slackToken == "" {
		return fmt.Errorf("SLACK_BOT_TOKEN environment variable is required")
	}

	neo4jURI := os.Getenv("NEO4J_URI")
	if neo4jURI == "" {
		return fmt.Errorf("NEO4J_URI environment variable is required")
	}

	neo4jUser := os.Getenv("NEO4J_USER")
	if neo4jUser == "" {
		neo4jUser = "neo4j"
	}

	neo4jPassword := os.Getenv("NEO4J_PASSWORD")
	if neo4jPassword == "" {
		return fmt.Errorf("NEO4J_PASSWORD environment variable is required")
	}

	neo4jDatabase := os.Getenv("NEO4J_DATABASE")
	if neo4jDatabase == "" {
		neo4jDatabase = "neo4j"
	}

	queriesDir := os.Getenv("QUERIES_DIR")
	if queriesDir == "" {
		queriesDir = "../../data-model/queries"
	}

	// ----- Build clients -----

	linearClient := linear.NewClient(linearKey)
	slackClient := slack.NewClient(slackToken)

	writer, err := graph.NewWriter(ctx, graph.Config{
		URI:          neo4jURI,
		Username:     neo4jUser,
		Password:     neo4jPassword,
		DatabaseName: neo4jDatabase,
		QueriesDir:   queriesDir,
	})
	if err != nil {
		return fmt.Errorf("init writer: %w", err)
	}
	defer func() {
		if err := writer.Close(ctx); err != nil {
			log.Printf("writer close: %v", err)
		}
	}()

	// ----- Run the pipeline -----

	var slackSince time.Time
	if slackSinceDays > 0 {
		slackSince = time.Now().AddDate(0, 0, -slackSinceDays)
	}

	return orchestrator.Run(ctx, orchestrator.Config{
		LinearProjectID: linearProjectID,
		SlackChannelID:  slackChannelID,
		SlackSince:      slackSince,
	}, linearClient, slackClient, writer)
}
