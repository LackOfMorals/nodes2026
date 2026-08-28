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

// Embedder produces embedding vectors for a batch of texts, in order, and
// reports the model name that produced them. Implemented by
// sync/internal/embed.Client; declared as an interface here so the
// orchestrator doesn't need to import the embedding client's HTTP details.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float64, error)
	Model() string
}

// EmbedConfig carries the parameters for the embed-and-link-semantic
// stage. Ignored when Run is called with a nil Embedder.
type EmbedConfig struct {
	Threshold float64 // minimum cosine similarity to create a :DISCUSSED_IN edge
	TopK      int     // how many nearest messages to consider per issue
}

// Run executes the full sync pipeline: pull, upsert, explicit link, and
// (when embedder is non-nil) embed + semantic link. A nil embedder skips
// the embedding stage entirely — sync remains fully usable without an
// embedding service configured, matching iteration 1 behaviour.
func Run(ctx context.Context, cfg Config, linearClient *linear.Client, slackClient *slack.Client, writer *graph.Writer, embedder Embedder, embedCfg EmbedConfig) error {
	if err := syncLinear(ctx, cfg, linearClient, writer); err != nil {
		return fmt.Errorf("sync linear: %w", err)
	}

	if err := syncSlack(ctx, cfg, slackClient, writer); err != nil {
		return fmt.Errorf("sync slack: %w", err)
	}

	if err := linkExplicit(ctx, cfg, writer); err != nil {
		return fmt.Errorf("link explicit: %w", err)
	}

	if embedder != nil {
		if err := embedAndLinkSemantic(ctx, writer, embedder, embedCfg); err != nil {
			return fmt.Errorf("embed and link semantic: %w", err)
		}
	} else {
		log.Println("no embedder configured — skipping embedding and semantic linking")
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

// embedAndLinkSemantic (re-)embeds every Issue and Message in the graph
// and runs the semantic linking pass. It re-reads text from the graph
// rather than reusing this tick's pulled data, so it works the same way
// whether the graph was just synced or has been sitting untouched — and
// re-embedding everything every tick is simpler and safer than tracking
// what changed at this demo's scale (a handful of issues and messages).
func embedAndLinkSemantic(ctx context.Context, writer *graph.Writer, embedder Embedder, cfg EmbedConfig) error {
	issues, err := writer.AllIssueTexts(ctx)
	if err != nil {
		return fmt.Errorf("list issues: %w", err)
	}
	if len(issues) > 0 {
		texts := make([]string, len(issues))
		for i, issue := range issues {
			texts[i] = issue.Text
		}
		vectors, err := embedder.Embed(ctx, texts)
		if err != nil {
			return fmt.Errorf("embed issues: %w", err)
		}
		for i, issue := range issues {
			if err := writer.EmbedIssue(ctx, issue.ID, vectors[i], embedder.Model()); err != nil {
				return fmt.Errorf("write embedding for %s: %w", issue.Identifier, err)
			}
		}
		log.Printf("embedded %d issues", len(issues))
	}

	messages, err := writer.AllMessageTexts(ctx)
	if err != nil {
		return fmt.Errorf("list messages: %w", err)
	}
	if len(messages) > 0 {
		texts := make([]string, len(messages))
		for i, m := range messages {
			texts[i] = m.Text
		}
		vectors, err := embedder.Embed(ctx, texts)
		if err != nil {
			return fmt.Errorf("embed messages: %w", err)
		}
		for i, m := range messages {
			if err := writer.EmbedMessage(ctx, m.ChannelID, m.TS, vectors[i], embedder.Model()); err != nil {
				return fmt.Errorf("write embedding for message %s: %w", m.TS, err)
			}
		}
		log.Printf("embedded %d messages", len(messages))
	}

	if err := writer.RunSemanticLinking(ctx, cfg.Threshold, cfg.TopK); err != nil {
		return fmt.Errorf("semantic linking: %w", err)
	}
	log.Println("semantic linking done")

	return nil
}
