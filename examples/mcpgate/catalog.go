// Package mcpgate is a tools-only MCP policy enforcement example. The launcher,
// not a tool argument, supplies the identity of its single downstream session.
package mcpgate

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Upstream is implemented by an MCP ClientSession.
type Upstream interface {
	ListTools(context.Context, *mcp.ListToolsParams) (*mcp.ListToolsResult, error)
	CallTool(context.Context, *mcp.CallToolParams) (*mcp.CallToolResult, error)
}

var segment = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Binding pins the full SDK-decoded tool definition. Without ResourceArgument,
// Resource is a literal capability. Otherwise it is a prefix ending in '/', to
// which the named, single-segment string argument and a final '/' are appended.
type Binding struct {
	Tool             string `json:"tool"`
	Action           string `json:"action"`
	DefinitionHash   string `json:"definition_hash"`
	Resource         string `json:"resource"`
	ResourceArgument string `json:"resource_argument,omitempty"`
}

type Grant struct {
	Role   string `json:"role"`
	Action string `json:"action"`
	Prefix string `json:"prefix"`
}

type Role struct {
	Principal string `json:"principal"`
	Role      string `json:"role"`
}

type Deny struct {
	Principal string `json:"principal"`
	Action    string `json:"action"`
	Resource  string `json:"resource"`
}

// Config is operator-owned input. Discovery creates bindings, never authority.
type Config struct {
	Server   string    `json:"server"`
	Bindings []Binding `json:"bindings"`
	Roles    []Role    `json:"roles"`
	Grants   []Grant   `json:"grants"`
	Denials  []Deny    `json:"denials,omitempty"`
}

// ReadConfig refuses misspelled fields and trailing JSON.
func ReadConfig(r io.Reader) (Config, error) {
	var c Config
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return c, err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return c, fmt.Errorf("config must contain one JSON object")
	}
	return c, nil
}

func action(server, tool string) string { return server + "." + tool }

// fingerprint is over the SDK's decoded definition, not server behavior or a
// signed executable. Re-encoding through generic JSON sorts object keys.
func fingerprint(t *mcp.Tool) (string, error) {
	b, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	var value any
	if err := json.Unmarshal(b, &value); err != nil {
		return "", err
	}
	b, err = json.Marshal(value)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256(b)), nil
}

// discover traverses pagination, rejects ambiguity, and bounds a broken server.
func discover(ctx context.Context, up Upstream) (map[string]*mcp.Tool, error) {
	out := map[string]*mcp.Tool{}
	seen := map[string]bool{}
	cursor := ""
	for page := 0; page < 100; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if seen[cursor] {
			return nil, fmt.Errorf("upstream repeated tools cursor")
		}
		seen[cursor] = true
		r, err := up.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			return nil, err
		}
		if r == nil {
			return nil, fmt.Errorf("upstream returned no catalog")
		}
		for _, t := range r.Tools {
			if t == nil || t.Name == "" || t.InputSchema == nil {
				return nil, fmt.Errorf("upstream returned an incomplete tool")
			}
			if _, exists := out[t.Name]; exists {
				return nil, fmt.Errorf("duplicate upstream tool %q", t.Name)
			}
			out[t.Name] = t
			if len(out) > 10000 {
				return nil, fmt.Errorf("upstream catalog exceeds 10000 tools")
			}
		}
		cursor = r.NextCursor
		if cursor == "" {
			return out, nil
		}
	}
	return nil, fmt.Errorf("upstream catalog exceeds 100 pages")
}

// Generate makes one action per tool (server.tool), with a capability resource.
// Annotations are preserved in the definition hash but never imply a grant.
func Generate(ctx context.Context, server string, up Upstream) (Config, error) {
	c := Config{Server: server, Bindings: []Binding{}, Roles: []Role{}, Grants: []Grant{}}
	if !segment.MatchString(server) {
		return c, fmt.Errorf("server must be a nonempty identifier using letters, digits, '_' or '-'")
	}
	tools, err := discover(ctx, up)
	if err != nil {
		return c, err
	}
	for name, t := range tools {
		h, err := fingerprint(t)
		if err != nil {
			return c, err
		}
		// MCP names may contain '.', but slashes are not allowed in our IDs.
		if !toolName.MatchString(name) {
			return c, fmt.Errorf("unsupported tool name %q", name)
		}
		c.Bindings = append(c.Bindings, Binding{Tool: name, Action: action(server, name), DefinitionHash: h,
			Resource: "mcp/" + server + "/" + name + "/"})
	}
	sort.Slice(c.Bindings, func(i, j int) bool { return c.Bindings[i].Tool < c.Bindings[j].Tool })
	return c, nil
}

var toolName = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
