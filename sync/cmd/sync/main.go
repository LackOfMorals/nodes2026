// Command sync pulls Linear issues and Slack threads, writes them to
// Neo4j, runs the explicit linking pass that creates :DISCUSSED_IN edges
// where messages reference issue identifiers, and — when an embedding
// service is configured — embeds every issue and message and runs the
// semantic linking pass on top.
//
// Usage:
//
//	sync \
//	  --linear-project <project-id> \
//	  --slack-channel <channel-id>
//
//	# Live demo mode: re-run on an interval instead of once.
//	sync --watch --interval 20s \
//	  --linear-project <project-id> \
//	  --slack-channel <channel-id>
//
// Environment variables:
//
//	LINEAR_API_KEY          Personal API key from Linear Settings → API
//	SLACK_BOT_TOKEN         xoxb-... bot user OAuth token
//	NEO4J_URI               e.g. neo4j+s://xxxxx.databases.neo4j.io
//	NEO4J_USER              typically "neo4j"
//	NEO4J_PASSWORD          the Aura instance password
//	NEO4J_DATABASE          optional, defaults to "neo4j"
//	QUERIES_DIR             optional, defaults to "../../data-model/queries"
//	EMBEDDING_BASE_URL      optional, e.g. http://localhost:1234/v1 (LM Studio).
//	                        Unset skips embedding and semantic linking entirely —
//	                        sync behaves exactly like iteration 1.
//	EMBEDDING_MODEL         required if EMBEDDING_BASE_URL is set, e.g.
//	                        text-embedding-nomic-embed-text-v1.5
//	SEMANTIC_LINK_THRESHOLD optional, defaults to 0.78 (cosine similarity —
//	                        tuned against real nomic-embed-text-v1.5
//	                        embeddings of the canonical seed data; the
//	                        margin is thin, re-verify if the seed text changes)
//	SEMANTIC_LINK_TOPK      optional, defaults to 5
//
// The orchestration is procedural and stops on first error. Every stage
// is idempotent — re-running after a failure replays cleanly. In --watch
// mode a failed tick is logged and the loop continues at the next
// interval rather than exiting, since it may run unattended during a demo.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/LackOfMorals/nodes2026/sync/internal/embed"
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
	watch := flag.Bool("watch", false,
		"keep re-running on --interval instead of running once (for a live demo)")
	interval := flag.Duration("interval", 20*time.Second,
		"how often to re-run in --watch mode. Slack's history/replies "+
			"endpoints are rate-limited more tightly than Linear's — verify "+
			"this interval against your app's actual limits before a live run")

	flag.Parse()

	if *linearProjectID == "" || *slackChannelID == "" {
		flag.Usage()
		os.Exit(2)
	}

	if err := run(*linearProjectID, *slackChannelID, *slackSinceDays, *watch, *interval); err != nil {
		log.Fatalf("sync failed: %v", err)
	}
}

func run(linearProjectID, slackChannelID string, slackSinceDays int, watch bool, interval time.Duration) error {
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

	// EMBEDDING_BASE_URL unset means "no embedding service configured" —
	// embedder stays nil and orchestrator.Run skips the embedding and
	// semantic-linking stage entirely (iteration 1 behaviour).
	var embedder orchestrator.Embedder
	embeddingBaseURL := os.Getenv("EMBEDDING_BASE_URL")
	if embeddingBaseURL != "" {
		embeddingModel := os.Getenv("EMBEDDING_MODEL")
		if embeddingModel == "" {
			return fmt.Errorf("EMBEDDING_MODEL environment variable is required when EMBEDDING_BASE_URL is set")
		}
		embedder = embed.New(embed.Config{BaseURL: embeddingBaseURL, Model: embeddingModel})
	}

	embedCfg := orchestrator.EmbedConfig{Threshold: 0.78, TopK: 5}
	if v := os.Getenv("SEMANTIC_LINK_THRESHOLD"); v != "" {
		threshold, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("SEMANTIC_LINK_THRESHOLD: %w", err)
		}
		embedCfg.Threshold = threshold
	}
	if v := os.Getenv("SEMANTIC_LINK_TOPK"); v != "" {
		topK, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("SEMANTIC_LINK_TOPK: %w", err)
		}
		embedCfg.TopK = topK
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

	runOnce := func() error {
		var slackSince time.Time
		if slackSinceDays > 0 {
			slackSince = time.Now().AddDate(0, 0, -slackSinceDays)
		}

		return orchestrator.Run(ctx, orchestrator.Config{
			LinearProjectID: linearProjectID,
			SlackChannelID:  slackChannelID,
			SlackSince:      slackSince,
		}, linearClient, slackClient, writer, embedder, embedCfg)
	}

	if !watch {
		return runOnce()
	}

	// Watch mode runs once immediately, then on every tick of interval.
	// A failed tick is logged, not fatal — a transient Slack rate limit
	// or embedding-service hiccup shouldn't end a live demo, it should
	// just be visible in the log and retried next tick.
	log.Printf("watch mode: re-running every %s (Ctrl-C to stop)", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	tick := 0
	for {
		tick++
		if err := runOnce(); err != nil {
			log.Printf("tick %d failed (will retry at tick %d): %v", tick, tick+1, err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
