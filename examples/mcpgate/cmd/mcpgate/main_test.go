package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gate "github.com/mattjoyce/shearline/examples/mcpgate"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestCLIProcess makes the Go test executable stand in for the built CLI, so
// the acceptance test runs real stdio subprocesses on both sides of the gate.
func TestCLIProcess(t *testing.T) {
	if os.Getenv("SHEARLINE_MCPGATE_TEST_PROCESS") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			if err := run(context.Background(), os.Args[i+1:]); err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		}
	}
	os.Exit(2)
}

func TestStdioGate(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	configPath := filepath.Join(dir, "policy.json")
	auditPath := filepath.Join(dir, "audit.jsonl")
	c, err := gate.DemoConfig()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestCLIProcess$", "--", "serve",
		"-principal", "agent-alice", "-config", configPath, "-audit", auditPath, "--",
		exe, "-test.run=^TestCLIProcess$", "--", "fixture")
	cmd.Env = append(os.Environ(), "SHEARLINE_MCPGATE_TEST_PROCESS=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "stdio-test", Version: "1"}, nil)
	cs, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, &mcp.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()
	list, err := cs.ListTools(ctx, nil)
	if err != nil || len(list.Tools) != 1 || list.Tools[0].Name != "read_record" {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	for _, tc := range []struct {
		name, id string
		allow    bool
	}{
		{"read_record", "public", true}, {"read_record", "secret", false}, {"save_record", "public", false},
	} {
		r, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tc.name, Arguments: map[string]string{"id": tc.id}})
		if err != nil || r.IsError == tc.allow {
			t.Fatalf("%s: %+v %v", tc.name, r, err)
		}
	}
	// Resources and prompts are local unsupported methods, never forwarded.
	if _, err := cs.ListResources(ctx, nil); err == nil {
		t.Fatal("unexpected resources support")
	}
	if err := cs.Close(); err != nil {
		t.Fatal(err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	audit, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(audit), `"phase":"authorization"`) != 3 || strings.Count(string(audit), `"phase":"execution"`) != 1 {
		t.Fatalf("audit: %s", audit)
	}
}
