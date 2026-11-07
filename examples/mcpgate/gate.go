package mcpgate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/mattjoyce/shearline/policy"
	"github.com/mattjoyce/tarski"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Gate is bound to one trusted principal and one upstream session. Operations are
// serialized; each has a deadline, including discovery and execution. Policy,
// mappings and identity are immutable for the life of the gate.
type Gate struct {
	up         Upstream
	principal  string
	server     string
	bindings   map[string]Binding
	policy     policy.Policy
	configHash string
	audit      *json.Encoder
	busy       chan struct{}
	seq        uint64
	timeout    time.Duration
}

// Entry records authorization separately from the upstream outcome. Decision
// contains the actual proof/absence, rule-set IDs and engine costs. ConfigHash
// identifies this example's config, including facts and reviewed tool hashes.
// Arguments and results are deliberately excluded from the log.
type Entry struct {
	ID         uint64           `json:"id"`
	Phase      string           `json:"phase"`
	Tool       string           `json:"tool"`
	Request    policy.Request   `json:"request"`
	Decision   *policy.Decision `json:"decision,omitempty"`
	Bounds     policy.Bounds    `json:"bounds"`
	ConfigHash string           `json:"config_hash"`
	Status     string           `json:"status"`
	Error      string           `json:"error,omitempty"`
}

// New validates all configured bindings against a live catalog before serving.
// audit must accept JSON lines; inability to write authorization prevents calls.
func New(ctx context.Context, up Upstream, principal string, c Config, audit io.Writer, timeout time.Duration) (*Gate, error) {
	if up == nil || audit == nil || strings.TrimSpace(principal) == "" || timeout <= 0 {
		return nil, fmt.Errorf("upstream, principal, audit writer and positive timeout are required")
	}
	if !segment.MatchString(c.Server) {
		return nil, fmt.Errorf("invalid server identifier")
	}
	g := &Gate{up: up, principal: principal, server: c.Server, bindings: map[string]Binding{},
		audit: json.NewEncoder(audit), busy: make(chan struct{}, 1), timeout: timeout}
	actions := map[string]bool{}
	for _, b := range c.Bindings {
		if b.Action != action(c.Server, b.Tool) {
			return nil, fmt.Errorf("binding action must be %q", action(c.Server, b.Tool))
		}
		if !toolName.MatchString(b.Tool) || b.Resource == "" || !strings.HasSuffix(b.Resource, "/") {
			return nil, fmt.Errorf("binding %q needs a valid tool and a nonempty resource ending in '/'", b.Tool)
		}
		if _, exists := g.bindings[b.Tool]; exists {
			return nil, fmt.Errorf("duplicate binding %q", b.Tool)
		}
		if b.ResourceArgument != "" && !segment.MatchString(b.ResourceArgument) {
			return nil, fmt.Errorf("invalid resource argument %q", b.ResourceArgument)
		}
		g.bindings[b.Tool] = b
		actions[action(c.Server, b.Tool)] = true
	}
	var facts []tarski.Fact
	for _, r := range c.Roles {
		facts = append(facts, policy.Role(r.Principal, r.Role))
	}
	for _, grant := range c.Grants {
		if !actions[grant.Action] {
			return nil, fmt.Errorf("grant names unknown action %q", grant.Action)
		}
		facts = append(facts, policy.Grant(grant.Role, grant.Action, grant.Prefix))
	}
	for _, d := range c.Denials {
		if !actions[d.Action] {
			return nil, fmt.Errorf("denial names unknown action %q", d.Action)
		}
		facts = append(facts, policy.Deny(d.Principal, d.Action, d.Resource))
	}
	p, err := policy.Load(policy.Config{Sets: []tarski.RuleSet{policy.RBAC()}, Standing: facts})
	if err != nil {
		return nil, err
	}
	g.policy = p
	raw, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	g.configHash = fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	tools, err := discover(ctx, up)
	if err != nil {
		return nil, fmt.Errorf("startup discovery: %w", err)
	}
	for name, b := range g.bindings {
		if !matches(tools[name], b) {
			return nil, fmt.Errorf("tool %q missing or definition changed; review and regenerate bindings", name)
		}
	}
	return g, nil
}

func matches(t *mcp.Tool, b Binding) bool {
	if t == nil {
		return false
	}
	h, err := fingerprint(t)
	return err == nil && h == b.DefinitionHash
}

func (g *Gate) enter(ctx context.Context) (context.Context, func(), error) {
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	select {
	case g.busy <- struct{}{}:
		return ctx, func() { <-g.busy; cancel() }, nil
	case <-ctx.Done():
		cancel()
		return nil, nil, ctx.Err()
	}
}

// List returns reviewed tools visible to this principal. Static resources get a
// full decision. Argument-scoped tools use coarse grant-prefix overlap; specific
// resources (and exact denials) are checked only when arguments exist at Call.
func (g *Gate) List(ctx context.Context) (*mcp.ListToolsResult, error) {
	ctx, leave, err := g.enter(ctx)
	if err != nil {
		return nil, err
	}
	defer leave()
	tools, err := discover(ctx, g.up)
	if err != nil {
		return nil, err
	}
	out := &mcp.ListToolsResult{Tools: []*mcp.Tool{}}
	for name, b := range g.bindings {
		if !matches(tools[name], b) {
			continue
		}
		visible := false
		if b.ResourceArgument == "" {
			d, err := g.policy.Decide(ctx, policy.Request{Principal: g.principal, Action: action(g.server, name), Resource: b.Resource})
			if err != nil {
				return nil, err
			}
			visible = d.Allow
		} else {
			// Permitted is available in the pinned v0.5.0. It is intentionally
			// coarse: it does not claim to apply concrete-resource denials.
			prefixes, err := g.policy.Permitted(ctx, g.principal, action(g.server, name))
			if err != nil {
				return nil, err
			}
			for _, prefix := range prefixes {
				visible = visible || strings.HasPrefix(b.Resource, prefix) || strings.HasPrefix(prefix, b.Resource)
			}
		}
		if visible {
			out.Tools = append(out.Tools, tools[name])
		}
	}
	sort.Slice(out.Tools, func(i, j int) bool { return out.Tools[i].Name < out.Tools[j].Name })
	return out, nil
}

func resource(b Binding, args json.RawMessage) (string, error) {
	// Require an object even for a literal capability. We do not infer scope
	// from arbitrary strings or accept identity/context via request metadata.
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(args, &obj); err != nil || obj == nil {
		return "", fmt.Errorf("arguments must be a JSON object")
	}
	// Different parsers may choose different values for duplicate keys. Refuse
	// ambiguous top-level arguments rather than authorize one interpretation.
	dec := json.NewDecoder(bytes.NewReader(args))
	_, _ = dec.Token() // the object was checked above
	seen := map[string]bool{}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return "", err
		}
		name := key.(string)
		if b.ResourceArgument != "" && name != b.ResourceArgument && strings.EqualFold(name, b.ResourceArgument) {
			return "", fmt.Errorf("resource argument must use exact spelling %q", b.ResourceArgument)
		}
		if seen[name] {
			return "", fmt.Errorf("duplicate argument %q", name)
		}
		seen[name] = true
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return "", err
		}
	}
	if b.ResourceArgument == "" {
		return b.Resource, nil
	}
	var value string
	if err := json.Unmarshal(obj[b.ResourceArgument], &value); err != nil || !segment.MatchString(value) {
		return "", fmt.Errorf("resource argument %q must be a nonempty single-segment identifier", b.ResourceArgument)
	}
	return b.Resource + value + "/", nil
}

func failure(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "mcpgate: " + s}}}
}

// Call checks every invocation, including guessed names absent from List. It
// forwards only name and the same argument bytes that were authorized. MCP
// metadata is not a trusted identity or attestation and is not forwarded.
func (g *Gate) Call(ctx context.Context, params *mcp.CallToolParamsRaw) (*mcp.CallToolResult, error) {
	ctx, leave, err := g.enter(ctx)
	if err != nil {
		return nil, err
	}
	defer leave()
	g.seq++
	e := Entry{ID: g.seq, Phase: "authorization", ConfigHash: g.configHash, Bounds: g.policy.Bounds(),
		Request: policy.Request{Principal: g.principal}, Status: "refused"}
	refuse := func(reason string) (*mcp.CallToolResult, error) {
		e.Error = reason
		if err := g.audit.Encode(e); err != nil {
			return nil, fmt.Errorf("authorization audit failed; not forwarded: %w", err)
		}
		return failure(reason), nil
	}
	if params == nil {
		return refuse("missing call parameters")
	}
	e.Tool = params.Name
	b, ok := g.bindings[params.Name]
	if !ok {
		return refuse("tool has no reviewed binding")
	}
	if params.RequestState != "" || len(params.InputResponses) != 0 {
		return refuse("multi-round tool calls are unsupported")
	}
	// Copy once so the scope evaluated and the forwarded payload are identical.
	args := append(json.RawMessage(nil), params.Arguments...)
	r, err := resource(b, args)
	if err != nil {
		return refuse(err.Error())
	}
	e.Request.Action, e.Request.Resource = action(g.server, b.Tool), r
	d, err := g.policy.Decide(ctx, e.Request)
	e.Decision = &d
	if err != nil {
		return refuse("policy evaluation failed: " + err.Error())
	}
	if !d.Allow {
		e.Status = "denied"
		return refuse("policy denied " + e.Request.Action + " on " + r)
	}
	tools, err := discover(ctx, g.up)
	if err != nil {
		return refuse("catalog recheck failed: " + err.Error())
	}
	if !matches(tools[b.Tool], b) {
		return refuse("tool definition changed or disappeared; operator review required")
	}
	if err := ctx.Err(); err != nil {
		return refuse(err.Error())
	}
	e.Status = "allowed"
	if err := g.audit.Encode(e); err != nil {
		return nil, fmt.Errorf("authorization audit failed; not forwarded: %w", err)
	}
	result, callErr := g.up.CallTool(ctx, &mcp.CallToolParams{Name: b.Tool, Arguments: args})
	e.Phase, e.Status, e.Error = "execution", "completed", ""
	e.Decision = nil
	if callErr != nil || result == nil {
		e.Status, e.Error = "unknown", "upstream failed; execution may have occurred"
	} else if result.IsError {
		e.Status = "tool_error"
	}
	if err := g.audit.Encode(e); err != nil {
		return nil, fmt.Errorf("outcome audit failed after forwarding; execution may have occurred: %w", err)
	}
	if callErr != nil || result == nil {
		return failure(e.Error), nil
	}
	return result, nil
}

// Server exposes tools only. No upstream prompts, resources, sampling,
// elicitation, tasks or arbitrary methods can bypass the tools/call check.
// Discovery and initialization are answered locally by the SDK.
func (g *Gate) Server() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "shearline-mcpgate", Version: "0.1.0"}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}},
	})
	s.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			switch method {
			case "tools/list":
				if p := req.(*mcp.ListToolsRequest).Params; p != nil && p.Cursor != "" {
					return nil, fmt.Errorf("mcpgate returns its entire filtered list; cursor must be empty")
				}
				return g.List(ctx)
			case "tools/call":
				return g.Call(ctx, req.(*mcp.CallToolRequest).Params)
			case "initialize", "notifications/initialized", "notifications/cancelled", "ping", "server/discover":
				return next(ctx, method, req)
			default:
				return nil, &jsonrpc.Error{Code: -32601, Message: "method not exposed by mcpgate"}
			}
		}
	})
	return s
}
