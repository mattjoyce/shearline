package mcpgate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The normal test path uses two real SDK protocol sessions. This wrapper only
// counts forwarding and introduces otherwise hard-to-trigger upstream failures.
type observed struct {
	Upstream
	mu        sync.Mutex
	calls     int
	last      *mcp.CallToolParams
	listErr   error
	callErr   error
	blockList bool
}

func (u *observed) ListTools(ctx context.Context, p *mcp.ListToolsParams) (*mcp.ListToolsResult, error) {
	if u.blockList {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if u.listErr != nil {
		return nil, u.listErr
	}
	return u.Upstream.ListTools(ctx, p)
}

func (u *observed) CallTool(ctx context.Context, p *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	u.mu.Lock()
	u.calls++
	u.last = p
	u.mu.Unlock()
	if u.callErr != nil {
		return nil, u.callErr
	}
	return u.Upstream.CallTool(ctx, p)
}

func connect(t *testing.T, s *mcp.Server) *mcp.ClientSession {
	t.Helper()
	ctx := t.Context()
	ct, st := mcp.NewInMemoryTransports()
	ss, err := s.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, &mcp.ClientOptions{Capabilities: &mcp.ClientCapabilities{}})
	cs, err := c.Connect(ctx, ct, &mcp.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func setup(t *testing.T, change func(*Config), audit io.Writer) (*Gate, *observed, *mcp.Server) {
	t.Helper()
	s := FixtureServer()
	u := &observed{Upstream: connect(t, s)}
	c, err := DemoConfig()
	if err != nil {
		t.Fatal(err)
	}
	if change != nil {
		change(&c)
	}
	if audit == nil {
		audit = io.Discard
	}
	g, err := New(t.Context(), u, "agent-alice", c, audit, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return g, u, s
}

func TestProtocolEnforcementAndAudit(t *testing.T) {
	var log bytes.Buffer
	g, u, _ := setup(t, nil, &log)
	cs := connect(t, g.Server())
	list, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Tools) != 1 || list.Tools[0].Name != "read_record" {
		t.Fatalf("list = %+v", list.Tools)
	}
	for _, tc := range []struct {
		name, id string
		allow    bool
	}{
		{"read_record", "public", true}, {"read_record", "secret", false},
		{"save_record", "public", false}, {"unknown", "public", false},
	} {
		r, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: tc.name, Arguments: map[string]string{"id": tc.id}})
		if err != nil {
			t.Fatal(err)
		}
		if r.IsError == tc.allow {
			t.Errorf("%s(%s) isError=%v", tc.name, tc.id, r.IsError)
		}
	}
	if u.calls != 1 {
		t.Fatalf("forwarded %d calls, want 1", u.calls)
	}
	dec := json.NewDecoder(&log)
	type auditRow struct {
		ID       uint64                                       `json:"id"`
		Phase    string                                       `json:"phase"`
		Status   string                                       `json:"status"`
		Request  struct{ Principal, Action, Resource string } `json:"request"`
		Decision map[string]json.RawMessage                   `json:"decision"`
		Hash     string                                       `json:"config_hash"`
	}
	var rows []auditRow
	for dec.More() {
		var row auditRow
		if err := dec.Decode(&row); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	if len(rows) != 5 {
		t.Fatalf("audit rows = %d", len(rows))
	}
	if rows[0].Status != "allowed" || rows[1].Phase != "execution" || rows[1].ID != rows[0].ID {
		t.Fatalf("authorization/outcome mismatch: %+v", rows[:2])
	}
	if len(rows[0].Decision["Proof"]) == 0 || len(rows[2].Decision["Why"]) == 0 || len(rows[0].Decision["RuleSets"]) == 0 {
		t.Fatal("missing actual decision evidence")
	}
	var proof struct {
		Relation string            `json:"relation"`
		Tuple    []string          `json:"tuple"`
		Rule     json.RawMessage   `json:"rule"`
		Body     []json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(rows[0].Decision["Proof"], &proof); err != nil {
		t.Fatal(err)
	}
	if proof.Relation != "allow" || len(proof.Tuple) != 3 || proof.Tuple[0] != "agent-alice" || proof.Tuple[2] != "records/public/" || len(proof.Body) == 0 || string(proof.Rule) == "null" {
		t.Fatalf("missing allow derivation: %+v", proof)
	}
	var why struct {
		Relation string   `json:"relation"`
		Goal     []string `json:"goal"`
		Rules    []struct {
			Blocked *struct {
				Relation string `json:"relation"`
			} `json:"blocked"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(rows[2].Decision["Why"], &why); err != nil {
		t.Fatal(err)
	}
	if why.Relation != "allow" || len(why.Goal) != 3 || why.Goal[2] != "records/secret/" || len(why.Rules) != 1 || why.Rules[0].Blocked == nil || why.Rules[0].Blocked.Relation != "deny" {
		t.Fatalf("missing deny witness: %+v", why)
	}
	for _, row := range rows {
		if row.Request.Principal != "agent-alice" || !strings.HasPrefix(row.Hash, "sha256:") {
			t.Fatalf("bad receipt: %+v", row)
		}
	}
}

func TestResourceMappingRejectsAmbiguityAndSpoofing(t *testing.T) {
	for _, args := range []string{
		`{"id":"secret","principal":"admin","confirmed":true}`,
		`{"id":"publicity"}`, `{"id":"../public"}`, `{"id":"public/secret"}`,
		`{"id":"public%2fsecret"}`, `{"id":""}`, `{"id":7}`, `{"id":null}`,
		`{}`, `null`, `[]`, `{"id":"secret","id":"public"}`, `{"id":"public","id":"secret"}`,
		`{"id":"public","ID":"secret"}`,
	} {
		t.Run(args, func(t *testing.T) {
			g, u, _ := setup(t, func(c *Config) { c.Grants[0].Prefix = "records/public/" }, nil)
			r, err := g.Call(t.Context(), &mcp.CallToolParamsRaw{Name: "read_record", Arguments: json.RawMessage(args)})
			if err != nil || !r.IsError || u.calls != 0 {
				t.Fatalf("result=%+v err=%v forwarded=%d", r, err, u.calls)
			}
		})
	}
	g, u, _ := setup(t, nil, nil)
	args := json.RawMessage(`{ "id" : "public" }`)
	r, err := g.Call(t.Context(), &mcp.CallToolParamsRaw{Name: "read_record", Arguments: args,
		Meta: mcp.Meta{"principal": "admin", "confirmed": true}})
	if err != nil || r.IsError {
		t.Fatalf("%+v %v", r, err)
	}
	if string(u.last.Arguments.(json.RawMessage)) != string(args) || len(u.last.Meta) != 0 {
		t.Fatal("arguments changed or untrusted metadata forwarded")
	}
}

func TestCatalogChangesFailClosed(t *testing.T) {
	g, u, s := setup(t, nil, nil)
	// Neither an added tool nor a changed description is trusted automatically.
	s.AddTool(&mcp.Tool{Name: "new_tool", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) { panic("must not forward") })
	s.AddTool(&mcp.Tool{Name: "read_record", Description: "changed behavior", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) { panic("must not forward") })
	list, err := g.List(t.Context())
	if err != nil || len(list.Tools) != 0 {
		t.Fatalf("changed catalog visible: %+v %v", list, err)
	}
	for _, name := range []string{"new_tool", "read_record"} {
		r, err := g.Call(t.Context(), &mcp.CallToolParamsRaw{Name: name, Arguments: json.RawMessage(`{"id":"public"}`)})
		if err != nil || !r.IsError {
			t.Fatalf("%+v %v", r, err)
		}
	}
	if u.calls != 0 {
		t.Fatal("forwarded changed tool")
	}
	s.RemoveTools("read_record")
	r, err := g.Call(t.Context(), &mcp.CallToolParamsRaw{Name: "read_record", Arguments: json.RawMessage(`{"id":"public"}`)})
	if err != nil || !r.IsError || u.calls != 0 {
		t.Fatal("forwarded removed tool")
	}
}

func TestLiteralCapabilityAndDenialFiltering(t *testing.T) {
	g, u, _ := setup(t, func(c *Config) {
		c.Bindings[0].ResourceArgument = ""
		c.Bindings[0].Resource = "capability/read/"
		c.Grants[0].Prefix = "capability/read/"
		c.Denials[0].Resource = "capability/read/"
	}, nil)
	l, err := g.List(t.Context())
	if err != nil || len(l.Tools) != 0 {
		t.Fatalf("%+v %v", l, err)
	}
	r, err := g.Call(t.Context(), &mcp.CallToolParamsRaw{Name: "read_record", Arguments: json.RawMessage(`{}`)})
	if err != nil || !r.IsError || u.calls != 0 {
		t.Fatal("literal denial bypassed")
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("audit disk unavailable") }

func TestFailuresPreventForwarding(t *testing.T) {
	for _, mode := range []string{"audit", "catalog", "canceled", "multi-round"} {
		t.Run(mode, func(t *testing.T) {
			writer := io.Discard
			if mode == "audit" {
				writer = brokenWriter{}
			}
			g, u, _ := setup(t, nil, writer)
			ctx := t.Context()
			p := &mcp.CallToolParamsRaw{Name: "read_record", Arguments: json.RawMessage(`{"id":"public"}`)}
			switch mode {
			case "catalog":
				u.listErr = errors.New("offline")
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "multi-round":
				p.RequestState = "opaque"
			}
			r, err := g.Call(ctx, p)
			if err == nil && (r == nil || !r.IsError) {
				t.Fatal("failure allowed")
			}
			if u.calls != 0 {
				t.Fatal("failure forwarded")
			}
		})
	}
}

func TestUpstreamErrorIsNotSuccessfulExecution(t *testing.T) {
	var log bytes.Buffer
	g, u, _ := setup(t, nil, &log)
	u.callErr = errors.New("connection lost after send")
	r, err := g.Call(t.Context(), &mcp.CallToolParamsRaw{Name: "read_record", Arguments: json.RawMessage(`{"id":"public"}`)})
	if err != nil || !r.IsError || u.calls != 1 || !strings.Contains(log.String(), `"status":"unknown"`) {
		t.Fatalf("%+v %v %s", r, err, log.String())
	}
}

func TestDeadlinePreventsForwarding(t *testing.T) {
	g, u, _ := setup(t, nil, nil)
	g.timeout = 20 * time.Millisecond
	u.blockList = true
	r, err := g.Call(t.Context(), &mcp.CallToolParamsRaw{Name: "read_record", Arguments: json.RawMessage(`{"id":"public"}`)})
	if err == nil && !r.IsError {
		t.Fatal("deadline allowed")
	}
	if u.calls != 0 {
		t.Fatal("deadline forwarded")
	}
}

func TestUnrelatedPrincipalHasNoTools(t *testing.T) {
	u := connect(t, FixtureServer())
	c, _ := DemoConfig()
	g, err := New(t.Context(), u, "unassigned-agent", c, io.Discard, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	l, err := g.List(t.Context())
	if err != nil || len(l.Tools) != 0 {
		t.Fatalf("%+v %v", l, err)
	}
}

func TestConcurrentCalls(t *testing.T) {
	g, u, _ := setup(t, nil, nil)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := g.Call(t.Context(), &mcp.CallToolParamsRaw{Name: "read_record", Arguments: json.RawMessage(`{"id":"public"}`)})
			if err != nil || r.IsError {
				t.Errorf("%+v %v", r, err)
			}
		}()
	}
	wg.Wait()
	if u.calls != 8 {
		t.Fatal(u.calls)
	}
}

func TestStartupValidation(t *testing.T) {
	u := connect(t, FixtureServer())
	for _, change := range []func(*Config){
		func(c *Config) { c.Bindings[0].DefinitionHash = "sha256:wrong" },
		func(c *Config) {
			c.Bindings[0].Tool = "absent"
			c.Bindings[0].Action = "demo.absent"
			c.Grants = nil
			c.Denials = nil
		},
		func(c *Config) { c.Bindings = append(c.Bindings, c.Bindings[0]) },
		func(c *Config) { c.Grants[0].Action = "typo" },
		func(c *Config) { c.Bindings[0].Action = "pretend.read" },
		func(c *Config) { c.Bindings[0].Resource = "" },
	} {
		c, _ := DemoConfig()
		change(&c)
		if _, err := New(t.Context(), u, "alice", c, io.Discard, time.Second); err == nil {
			t.Fatalf("accepted bad config: %+v", c)
		}
	}
	if _, err := ReadConfig(strings.NewReader(`{"typo":true}`)); err == nil {
		t.Fatal("accepted unknown field")
	}
	if _, err := ReadConfig(strings.NewReader(`{} {}`)); err == nil {
		t.Fatal("accepted trailing JSON")
	}
}

type pages struct {
	results map[string]*mcp.ListToolsResult
}

func (p pages) ListTools(_ context.Context, params *mcp.ListToolsParams) (*mcp.ListToolsResult, error) {
	return p.results[params.Cursor], nil
}
func (p pages) CallTool(context.Context, *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	panic("not called")
}

func TestInventoryPaginationAndNoInferredGrants(t *testing.T) {
	a := &mcp.Tool{Name: "a", InputSchema: map[string]any{"type": "object"}, Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}
	b := &mcp.Tool{Name: "b", InputSchema: map[string]any{"type": "object"}}
	p := pages{map[string]*mcp.ListToolsResult{"": {Tools: []*mcp.Tool{b}, NextCursor: "next"}, "next": {Tools: []*mcp.Tool{a}}}}
	c, err := Generate(t.Context(), "demo", p)
	if err != nil || len(c.Bindings) != 2 || c.Bindings[0].Action != "demo.a" || len(c.Grants) != 0 || len(c.Roles) != 0 {
		t.Fatalf("%+v %v", c, err)
	}
	p.results["next"].NextCursor = "next"
	if _, err := Generate(t.Context(), "demo", p); err == nil {
		t.Fatal("accepted cursor cycle")
	}
	p.results["next"] = &mcp.ListToolsResult{Tools: []*mcp.Tool{b}}
	if _, err := Generate(t.Context(), "demo", p); err == nil {
		t.Fatal("accepted duplicate names")
	}
}

func TestFingerprintIgnoresObjectKeyOrder(t *testing.T) {
	a := &mcp.Tool{Name: "x", InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)}
	b := &mcp.Tool{Name: "x", InputSchema: json.RawMessage(`{"properties":{},"type":"object"}`)}
	ha, err := fingerprint(a)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := fingerprint(b)
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Fatalf("%s != %s", ha, hb)
	}
}
