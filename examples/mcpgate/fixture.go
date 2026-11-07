package mcpgate

import (
	"context"
	_ "embed"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed demo-policy.json
var demoPolicy string

// DemoConfig is a reviewed fixture, not a policy inferred at connection time.
func DemoConfig() (Config, error) { return ReadConfig(strings.NewReader(demoPolicy)) }

// FixtureServer is a deterministic, credential-free upstream used in the demo.
// It models two tools; save_record acknowledges a save without persisting data.
func FixtureServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "mcpgate-fixture", Version: "1"}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{},
	})
	schema := map[string]any{"type": "object", "properties": map[string]any{
		"id": map[string]any{"type": "string"},
	}, "required": []string{"id"}, "additionalProperties": false}
	s.AddTool(&mcp.Tool{Name: "read_record", Description: "Read a demo record by id.", InputSchema: schema,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}},
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "record read by upstream: " + string(req.Params.Arguments)}}}, nil
		})
	s.AddTool(&mcp.Tool{Name: "save_record", Description: "Save a demo record by id.", InputSchema: schema},
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "record save acknowledged by upstream"}}}, nil
		})
	return s
}
