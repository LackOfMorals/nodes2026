// Package orchestrator coordinates the sync pipeline:
//
//  1. Pull Linear issues for a project
//  2. Pull Slack threads for a channel
//  3. Upsert everything into Neo4j
//  4. Run the explicit linking pass to create :DISCUSSED_IN edges
//
// The orchestrator is small and procedural on purpose. Each stage prints
// progress; failures stop the pipeline rather than partial-completing.
// Idempotent operations let you re-run after fixing the failure.
package orchestrator

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/LackOfMorals/nodes2026/sync/internal/graph"
	"github.com/LackOfMorals/nodes2026/sync/internal/linear"
	"github.com/LackOfMorals/nodes2026/sync/internal/slack"
)

// Config carries the inputs the orchestrator needs.
type Config struct {
	LinearProjectID string
	SlackChannelID  string
	SlackSince      time.Time // pull Slack messages no older than this; zero = all
}

// Run executes the full sync pipeline.
func Run(ctx context.Context, cfg Config, linearClient *linear.Client, slackClient *slack.Client, writer *graph.Writer) error {
	if err := syncLinear(ctx, cfg, linearClient, writer); err != nil {
		return fmt.Errorf("sync linear: %w", err)
	}

	if err := syncSlack(ctx, cfg, slackClient, writer); err != nil {
		return fmt.Errorf("sync slack: %w", err)
	}

	if err := linkExplicit(ctx, cfg, writer); err != nil {
		return fmt.Errorf("link explicit: %w", err)
	}

	log.Println("sync complete")
	return nil
}

// syncLinear pulls every issue in the configured project and upserts each.
func syncLinear(ctx context.Context, cfg Config, client *linear.Client, writer *graph.Writer) error {
	log.Printf("pulling Linear issues for project %s", cfg.LinearProjectID)

	issues, err := client.IssuesInProject(ctx, cfg.LinearProjectID)
	if err != nil {
		return fmt.Errorf("pull issues: %w", err)
	}
	log.Printf("pulled %d issues", len(issues))

	for i, issue := range issues {
		if err := writer.UpsertIssue(ctx, issue); err != nil {
			return fmt.Errorf("upsert issue %s: %w", issue.Identifier, err)
		}
		log.Printf("  [%d/%d] upserted %s — %s", i+1, len(issues), issue.Identifier, issue.Title)
	}

	return nil
}

// syncSlack pulls every thread in the configured channel and upserts each
// alongside the channel itself.
func syncSlack(ctx context.Context, cfg Config, client *slack.Client, writer *graph.Writer) error {
	log.Printf("pulling Slack channel %s", cfg.SlackChannelID)

	channel, err := client.GetChannel(ctx, cfg.SlackChannelID)
	if err != nil {
		return fmt.Errorf("get channel: %w", err)
	}
	log.Printf("channel: #%s — %q", channel.Name, channel.Purpose)

	threads, err := client.ThreadsInChannel(ctx, cfg.SlackChannelID, cfg.SlackSince)
	if err != nil {
		return fmt.Errorf("pull threads: %w", err)
	}
	log.Printf("pulled %d threads", len(threads))

	for i, thread := range threads {
		if err := writer.UpsertThread(ctx, *channel, thread); err != nil {
			return fmt.Errorf("upsert thread %s: %w", thread.TS, err)
		}
		log.Printf("  [%d/%d] upserted thread %s (%d messages)", i+1, len(threads), thread.TS, len(thread.Messages))
	}

	return nil
}

// linkExplicit runs the explicit linking pass that connects messages
// naming an issue identifier to that issue's threads.
func linkExplicit(ctx context.Context, cfg Config, writer *graph.Writer) error {
	log.Printf("running explicit linking pass")

	if err := writer.RunExplicitLinking(ctx); err != nil {
		return fmt.Errorf("link: %w", err)
	}
	log.Println("explicit linking done")

	return nil
}
