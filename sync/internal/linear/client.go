// Package linear is a thin GraphQL client for the Linear API.
//
// We deliberately don't depend on a generated client like genqlient — for
// this sync pipeline we touch maybe four queries, and a hand-rolled client
// is easier to read and maintain at this scale.
package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultEndpoint = "https://api.linear.app/graphql"

// Client talks to the Linear GraphQL API.
type Client struct {
	endpoint   string
	apiKey     string
	httpClient *http.Client
}

// NewClient builds a Linear client. The API key is a personal API key from
// Linear Settings → API → Personal API Keys.
func NewClient(apiKey string) *Client {
	return &Client{
		endpoint:   defaultEndpoint,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Issue is the subgraph-facing representation of a Linear issue, populated
// with everything the upsert query needs.
type Issue struct {
	ID         string
	Identifier string
	Title      string
	State      string
	Priority   int
	URL        string
	CreatedAt  time.Time
	UpdatedAt  time.Time

	ProjectID   string
	ProjectName string

	CreatorEmail    string
	CreatorLinearID string
	CreatorName     string

	AssigneeEmail    *string
	AssigneeLinearID *string
	AssigneeName     *string
}

// IssuesInProject fetches all issues in a Linear project, paginating
// transparently. Returns at most 250 issues — the free-tier cap — which is
// more than enough for the demo.
func (c *Client) IssuesInProject(ctx context.Context, projectID string) ([]Issue, error) {
	const query = `
		query IssuesInProject($projectId: String!, $after: String) {
			project(id: $projectId) {
				id
				name
				issues(first: 50, after: $after) {
					pageInfo {
						hasNextPage
						endCursor
					}
					nodes {
						id
						identifier
						title
						priority
						url
						createdAt
						updatedAt
						state { name }
						creator { id name email }
						assignee { id name email }
					}
				}
			}
		}`

	var all []Issue
	var cursor *string

	for {
		variables := map[string]any{
			"projectId": projectID,
		}
		if cursor != nil {
			variables["after"] = *cursor
		}

		var resp struct {
			Project struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Issues struct {
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []struct {
						ID         string `json:"id"`
						Identifier string `json:"identifier"`
						Title      string `json:"title"`
						Priority   int    `json:"priority"`
						URL        string `json:"url"`
						CreatedAt  string `json:"createdAt"`
						UpdatedAt  string `json:"updatedAt"`
						State      struct {
							Name string `json:"name"`
						} `json:"state"`
						Creator *struct {
							ID    string `json:"id"`
							Name  string `json:"name"`
							Email string `json:"email"`
						} `json:"creator"`
						Assignee *struct {
							ID    string `json:"id"`
							Name  string `json:"name"`
							Email string `json:"email"`
						} `json:"assignee"`
					} `json:"nodes"`
				} `json:"issues"`
			} `json:"project"`
		}

		if err := c.do(ctx, query, variables, &resp); err != nil {
			return nil, fmt.Errorf("fetch issues: %w", err)
		}

		for _, node := range resp.Project.Issues.Nodes {
			createdAt, _ := time.Parse(time.RFC3339, node.CreatedAt)
			updatedAt, _ := time.Parse(time.RFC3339, node.UpdatedAt)

			issue := Issue{
				ID:          node.ID,
				Identifier:  node.Identifier,
				Title:       node.Title,
				State:       node.State.Name,
				Priority:    node.Priority,
				URL:         node.URL,
				CreatedAt:   createdAt,
				UpdatedAt:   updatedAt,
				ProjectID:   resp.Project.ID,
				ProjectName: resp.Project.Name,
			}

			// Creator should always be present on a real Linear issue.
			// Treat missing creator as a soft error and skip the issue.
			if node.Creator == nil {
				continue
			}
			issue.CreatorEmail = node.Creator.Email
			issue.CreatorLinearID = node.Creator.ID
			issue.CreatorName = node.Creator.Name

			// Assignee is optional.
			if node.Assignee != nil {
				issue.AssigneeEmail = &node.Assignee.Email
				issue.AssigneeLinearID = &node.Assignee.ID
				issue.AssigneeName = &node.Assignee.Name
			}

			all = append(all, issue)
		}

		if !resp.Project.Issues.PageInfo.HasNextPage {
			break
		}
		cursor = &resp.Project.Issues.PageInfo.EndCursor
	}

	return all, nil
}

// do executes a GraphQL query against the Linear endpoint and decodes the
// data field of the response into out.
func (c *Client) do(ctx context.Context, query string, variables map[string]any, out any) error {
	body, err := json.Marshal(map[string]any{
		"query":     query,
		"variables": variables,
	})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("linear api returned %d: %s", resp.StatusCode, respBody)
	}

	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}

	if len(envelope.Errors) > 0 {
		return fmt.Errorf("graphql errors: %s", envelope.Errors[0].Message)
	}

	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("unmarshal data: %w", err)
	}

	return nil
}
