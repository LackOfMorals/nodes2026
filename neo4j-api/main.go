// neo4j-api is a small plain-GraphQL HTTP service over the NODES 2026
// demo knowledge graph. It exposes exactly the operations the demo
// needs — getIssueDiscussionContext (the closing-demo query, resolved
// entirely from the graph) and searchMessages — and is registered as a
// subgraph of the local Cosmo Router (router/graph.yaml, static schema
// file, no live introspection).
//
// Localhost only, no auth. Configuration via environment (the repo-root
// .env has all of these): NEO4J_URI, NEO4J_USER, NEO4J_PASSWORD,
// NEO4J_DATABASE, NEO4J_API_ADDR (default 127.0.0.1:4400).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/graph-gophers/graphql-go"
	neo4j "github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// The schema and Cypher files are loaded from disk at startup instead
// of via //go:embed: Go 1.26.5 on this machine rejects embed with a
// spurious "embed imported and not used" compile error (standalone
// repro, 2026-08-28). loadAsset looks in the working directory, then
// the executable's directory and its parent, so the binary runs from
// the module root, from bin/, or anywhere the assets sit next to it.
var (
	schemaSDL            = loadAsset("schema.graphql")
	agentContextCypher   = loadAsset("cypher/agent-context.cypher")
	searchMessagesCypher = loadAsset("cypher/search-messages.cypher")
)

func loadAsset(name string) string {
	candidates := []string{"."}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates, dir, filepath.Dir(dir))
	}
	for _, dir := range candidates {
		if b, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			return string(b)
		}
	}
	log.Fatalf("neo4j-api: cannot load %s (looked in %v)", name, candidates)
	return ""
}

// ----- GraphQL model (mirrors schema.graphql; Go type names match the
// GraphQL type names, case-insensitively, as the library requires) -----

type Resolver struct {
	driver   neo4j.DriverWithContext
	database string
}

type person struct {
	Name  *string `json:"name,omitempty"`
	Email *string `json:"email,omitempty"`
}

type issueDiscussionContext struct {
	Identifier string             `json:"identifier"`
	Title      string             `json:"title"`
	State      *string            `json:"state,omitempty"`
	Priority   *int32             `json:"priority,omitempty"`
	LinearUrl  *string            `json:"linearUrl,omitempty"`
	CreatedAt  *string            `json:"createdAt,omitempty"`
	UpdatedAt  *string            `json:"updatedAt,omitempty"`
	CreatedBy  *person            `json:"createdBy,omitempty"`
	AssignedTo *person            `json:"assignedTo,omitempty"`
	Discussions []*discussion     `json:"discussions,omitempty"`
}

type discussion struct {
	Channel    *string             `json:"channel,omitempty"`
	ThreadTs   *string             `json:"threadTs,omitempty"`
	StartedAt  *string             `json:"startedAt,omitempty"`
	Permalink  *string             `json:"permalink,omitempty"`
	Confidence *float64            `json:"confidence,omitempty"`
	Evidence   *string             `json:"evidence,omitempty"`
	Messages   []*discussionMessage `json:"messages,omitempty"`
}

type discussionMessage struct {
	Ts          *string `json:"ts,omitempty"`
	PostedAt    *string `json:"postedAt,omitempty"`
	Text        *string `json:"text,omitempty"`
	AuthorName  *string `json:"authorName,omitempty"`
	AuthorEmail *string `json:"authorEmail,omitempty"`
	Permalink   *string `json:"permalink,omitempty"`
}

type messageSearchHit struct {
	Ts          *string `json:"ts,omitempty"`
	PostedAt    *string `json:"postedAt,omitempty"`
	Text        *string `json:"text,omitempty"`
	AuthorName  *string `json:"authorName,omitempty"`
	AuthorEmail *string `json:"authorEmail,omitempty"`
	Permalink   *string `json:"permalink,omitempty"`
	ThreadTs    *string `json:"threadTs,omitempty"`
	Channel     *string `json:"channel,omitempty"`
}

// ----- Resolvers -----

func (r *Resolver) GetIssueDiscussionContext(ctx context.Context, args struct{ Identifier string }) (*issueDiscussionContext, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode:   neo4j.AccessModeRead,
		DatabaseName: r.database,
	})
	defer session.Close(ctx)

	res, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		rs, err := tx.Run(ctx, agentContextCypher, map[string]any{"identifier": args.Identifier})
		if err != nil {
			return nil, err
		}
		recs, err := rs.Collect(ctx)
		if err != nil {
			return nil, err
		}
		if len(recs) == 0 {
			return nil, nil // identifier not in the graph -> GraphQL null
		}
		return mapDiscussionContext(recs[0])
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	out, ok := res.(*issueDiscussionContext)
	if !ok {
		return nil, fmt.Errorf("internal: unexpected result type %T", res)
	}
	return out, nil
}

func (r *Resolver) SearchMessages(ctx context.Context, args struct {
	Query string
	Limit int32
}) ([]*messageSearchHit, error) {
	if args.Limit <= 0 {
		args.Limit = 20
	}
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode:   neo4j.AccessModeRead,
		DatabaseName: r.database,
	})
	defer session.Close(ctx)

	res, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		rs, err := tx.Run(ctx, searchMessagesCypher, map[string]any{"q": args.Query, "limit": args.Limit})
		if err != nil {
			return nil, err
		}
		recs, err := rs.Collect(ctx)
		if err != nil {
			return nil, err
		}
		hits := make([]*messageSearchHit, 0, len(recs))
		for _, rec := range recs {
			h, err := mapSearchHit(rec)
			if err != nil {
				return nil, err
			}
			hits = append(hits, h)
		}
		return hits, nil
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return []*messageSearchHit{}, nil
	}
	return res.([]*messageSearchHit), nil
}

// ----- Row mapping (Neo4j driver values -> GraphQL structs) -----
//
// neo4j-go-driver v5 exposes records as *neo4j.Record (an alias of the
// plain db.Record struct: Keys/Values). AsMap() hands back a plain
// map[string]any, so the mapping below is ordinary map work.

func mapDiscussionContext(rec *neo4j.Record) (*issueDiscussionContext, error) {
	m := rec.AsMap()
	out := &issueDiscussionContext{}
	var err error
	if out.Identifier, err = reqStr(m, "identifier"); err != nil {
		return nil, err
	}
	if out.Title, err = reqStr(m, "title"); err != nil {
		return nil, err
	}
	if out.State, err = strFrom(m, "state"); err != nil {
		return nil, err
	}
	if out.Priority, err = intFrom(m, "priority"); err != nil {
		return nil, err
	}
	if out.LinearUrl, err = strFrom(m, "linearUrl"); err != nil {
		return nil, err
	}
	if out.CreatedAt, err = strFrom(m, "createdAt"); err != nil {
		return nil, err
	}
	if out.UpdatedAt, err = strFrom(m, "updatedAt"); err != nil {
		return nil, err
	}
	if out.CreatedBy, err = toPerson(m["createdBy"]); err != nil {
		return nil, err
	}
	if out.AssignedTo, err = toPerson(m["assignedTo"]); err != nil {
		return nil, err
	}
	if out.Discussions, err = toDiscussions(m["discussions"]); err != nil {
		return nil, err
	}
	return out, nil
}

func mapSearchHit(rec *neo4j.Record) (*messageSearchHit, error) {
	m := rec.AsMap()
	out := &messageSearchHit{}
	var err error
	if out.Ts, err = strFrom(m, "ts"); err != nil {
		return nil, err
	}
	if out.PostedAt, err = strFrom(m, "postedAt"); err != nil {
		return nil, err
	}
	if out.Text, err = strFrom(m, "text"); err != nil {
		return nil, err
	}
	if out.AuthorName, err = strFrom(m, "authorName"); err != nil {
		return nil, err
	}
	if out.AuthorEmail, err = strFrom(m, "authorEmail"); err != nil {
		return nil, err
	}
	if out.Permalink, err = strFrom(m, "permalink"); err != nil {
		return nil, err
	}
	if out.ThreadTs, err = strFrom(m, "threadTs"); err != nil {
		return nil, err
	}
	if out.Channel, err = strFrom(m, "channel"); err != nil {
		return nil, err
	}
	return out, nil
}

// reqStr is for required scalar fields: the Cypher query contract
// guarantees they exist and are strings, so an assertion failure is a
// real bug worth surfacing.
func reqStr(m map[string]any, field string) (string, error) {
	s, ok := m[field].(string)
	if !ok {
		return "", fmt.Errorf("%s: expected string, got %T", field, m[field])
	}
	return s, nil
}

// asString accepts the two value kinds the demo Cypher can return:
// plain strings and Neo4j datetimes (the driver decodes datetime
// properties to time.Time — rendered as RFC3339 here). Datetimes stay
// native in the graph so the web-console closing demo shows real
// datetime values.
func asString(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case time.Time:
		return x.Format(time.RFC3339), true
	}
	return "", false
}

func strFrom(m map[string]any, field string) (*string, error) {
	v := m[field]
	if v == nil {
		return nil, nil
	}
	s, ok := asString(v)
	if !ok {
		return nil, fmt.Errorf("%s: expected string, got %T", field, v)
	}
	return &s, nil
}

func intFrom(m map[string]any, field string) (*int32, error) {
	v := m[field]
	if v == nil {
		return nil, nil
	}
	n, ok := v.(int64)
	if !ok {
		return nil, fmt.Errorf("%s: expected integer, got %T", field, v)
	}
	x := int32(n)
	return &x, nil
}

func toPerson(v any) (*person, error) {
	if v == nil {
		return nil, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected map for person, got %T", v)
	}
	p := &person{}
	if s, ok := m["name"].(string); ok {
		p.Name = &s
	}
	if s, ok := m["email"].(string); ok {
		p.Email = &s
	}
	return p, nil
}

func toDiscussions(v any) ([]*discussion, error) {
	if v == nil {
		return []*discussion{}, nil
	}
	list, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("expected list for discussions, got %T", v)
	}
	out := make([]*discussion, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected map in discussions, got %T", item)
		}
		d := &discussion{}
		var s *string
		var err error
		if s, err = toStrPtr(m["channel"]); err != nil {
			return nil, err
		}
		d.Channel = s
		if s, err = toStrPtr(m["threadTs"]); err != nil {
			return nil, err
		}
		d.ThreadTs = s
		if s, err = toStrPtr(m["startedAt"]); err != nil {
			return nil, err
		}
		d.StartedAt = s
		if s, err = toStrPtr(m["permalink"]); err != nil {
			return nil, err
		}
		d.Permalink = s
		if f, err := toFloatPtr(m["confidence"]); err != nil {
			return nil, err
		} else {
			d.Confidence = f
		}
		if s, err = toStrPtr(m["evidence"]); err != nil {
			return nil, err
		}
		d.Evidence = s
		d.Messages, err = toMessages(m["messages"])
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

func toMessages(v any) ([]*discussionMessage, error) {
	if v == nil {
		return []*discussionMessage{}, nil
	}
	list, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("expected list for messages, got %T", v)
	}
	out := make([]*discussionMessage, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected map in messages, got %T", item)
		}
		msg := &discussionMessage{}
		var err error
		if msg.Ts, err = toStrPtr(m["ts"]); err != nil {
			return nil, err
		}
		if msg.PostedAt, err = toStrPtr(m["postedAt"]); err != nil {
			return nil, err
		}
		if msg.Text, err = toStrPtr(m["text"]); err != nil {
			return nil, err
		}
		if msg.AuthorName, err = toStrPtr(m["authorName"]); err != nil {
			return nil, err
		}
		if msg.AuthorEmail, err = toStrPtr(m["authorEmail"]); err != nil {
			return nil, err
		}
		if msg.Permalink, err = toStrPtr(m["permalink"]); err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	return out, nil
}

func toStrPtr(v any) (*string, error) {
	if v == nil {
		return nil, nil
	}
	s, ok := asString(v)
	if !ok {
		return nil, fmt.Errorf("expected string, got %T", v)
	}
	return &s, nil
}

func toFloatPtr(v any) (*float64, error) {
	if v == nil {
		return nil, nil
	}
	f, ok := v.(float64)
	if !ok {
		return nil, fmt.Errorf("expected float, got %T", v)
	}
	return &f, nil
}

// ----- HTTP server -----

type server struct {
	schema *graphql.Schema
}

type graphqlRequest struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
}

func (s *server) handleGraphQL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req graphqlRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Query == "" {
		http.Error(w, "missing query", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	resp := s.schema.Exec(ctx, req.Query, req.OperationName, req.Variables)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintln(w, "ok")
}

// ----- Wiring -----

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	log.SetPrefix("neo4j-api: ")

	uri := envOr("NEO4J_URI", "bolt://localhost:7687")
	user := envOr("NEO4J_USER", "neo4j")
	password := envOr("NEO4J_PASSWORD", "password")
	database := envOr("NEO4J_DATABASE", "neo4j")
	addr := envOr("NEO4J_API_ADDR", "127.0.0.1:4400")

	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(user, password, ""))
	if err != nil {
		log.Fatalf("creating Neo4j driver: %v", err)
	}
	defer driver.Close(context.Background())

	pingCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := driver.VerifyConnectivity(pingCtx); err != nil {
		log.Fatalf("cannot reach Neo4j at %s: %v", uri, err)
	}

	// UseFieldResolvers lets nested object fields resolve from struct
	// fields (root Query fields keep their methods — methods win in the
	// resolver lookup).
	schema := graphql.MustParseSchema(schemaSDL, &Resolver{driver: driver, database: database}, graphql.UseFieldResolvers())

	srv := &server{schema: schema}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /graphql", srv.handleGraphQL)
	mux.HandleFunc("GET /healthz", srv.handleHealthz)

	log.Printf("GraphQL endpoint on http://%s/graphql (Neo4j %s, database %q)", addr, uri, database)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
