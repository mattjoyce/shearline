// mcpgate fronts one stdio MCP process with a reviewed Shearline policy.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	gate "github.com/mattjoyce/shearline/examples/mcpgate"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "mcpgate:", err)
		os.Exit(1)
	}
}

func upstream(ctx context.Context, argv []string) (*mcp.ClientSession, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("upstream command required after '--'")
	}
	// The operator supplies this command as argv; no shell interprets it.
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) // #nosec G204 G702 -- operator-owned argv is the intended upstream command; no shell
	cmd.Stderr = os.Stderr
	c := mcp.NewClient(&mcp.Implementation{Name: "mcpgate-upstream", Version: "1"}, &mcp.ClientOptions{
		Capabilities: &mcp.ClientCapabilities{},
	})
	// Keep this tools-only example on the widely deployed initialize lifecycle.
	return c.Connect(ctx, &mcp.CommandTransport{Command: cmd}, &mcp.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
}

func run(ctx context.Context, argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("usage: mcpgate {demo|fixture|inventory|serve} [options] -- upstream [args]")
	}
	switch argv[0] {
	case "fixture":
		return gate.FixtureServer().Run(ctx, &mcp.StdioTransport{})
	case "demo":
		return demo(ctx)
	case "inventory", "serve":
	default:
		return fmt.Errorf("unknown command %q", argv[0])
	}
	flags := flag.NewFlagSet(argv[0], flag.ContinueOnError)
	server := flags.String("server", "upstream", "stable upstream namespace (inventory)")
	principal := flags.String("principal", "", "identity bound by the trusted launcher (serve)")
	configPath := flags.String("config", "", "reviewed policy JSON (serve)")
	auditPath := flags.String("audit", "-", "audit JSONL file, or '-' for stderr (serve)")
	timeout := flags.Duration("timeout", 10*time.Second, "deadline per operation")
	if err := flags.Parse(argv[1:]); err != nil {
		return err
	}
	if *timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	var config gate.Config
	var audit io.Writer = os.Stderr
	if argv[0] == "serve" {
		if *principal == "" || *configPath == "" {
			return fmt.Errorf("serve requires -principal and -config")
		}
		f, err := os.Open(*configPath) // #nosec G703 -- the local operator explicitly selects the policy file
		if err != nil {
			return err
		}
		config, err = gate.ReadConfig(f)
		_ = f.Close()
		if err != nil {
			return err
		}
		if *auditPath != "-" {
			f, err := os.OpenFile(*auditPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600) // #nosec G703 -- the local operator explicitly selects the audit destination
			if err != nil {
				return err
			}
			defer func() { _ = f.Close() }()
			audit = f
		}
	}
	// CommandContext owns the subprocess lifetime. Stop the handshake timer on
	// success so it cannot terminate a healthy, long-lived upstream session.
	connectCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	timer := time.AfterFunc(*timeout, cancel)
	up, err := upstream(connectCtx, flags.Args())
	timer.Stop()
	if err != nil {
		return err
	}
	defer func() { _ = up.Close() }()
	if argv[0] == "inventory" {
		bounded, stop := context.WithTimeout(ctx, *timeout)
		defer stop()
		c, err := gate.Generate(bounded, *server, up)
		if err != nil {
			return err
		}
		e := json.NewEncoder(os.Stdout)
		e.SetIndent("", "  ")
		return e.Encode(c)
	}
	g, err := gate.New(ctx, up, *principal, config, audit, *timeout)
	if err != nil {
		return err
	}
	return g.Server().Run(ctx, &mcp.StdioTransport{})
}

func demo(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	up, err := upstream(ctx, []string{exe, "fixture"})
	if err != nil {
		return err
	}
	defer func() { _ = up.Close() }()
	c, err := gate.DemoConfig()
	if err != nil {
		return err
	}
	var audit bytes.Buffer
	g, err := gate.New(ctx, up, "agent-alice", c, &audit, 5*time.Second)
	if err != nil {
		return err
	}
	ct, st := mcp.NewInMemoryTransports()
	ss, err := g.Server().Connect(ctx, st, nil)
	if err != nil {
		return err
	}
	defer func() { _ = ss.Close() }()
	client := mcp.NewClient(&mcp.Implementation{Name: "demo-agent", Version: "1"}, nil)
	cs, err := client.Connect(ctx, ct, &mcp.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
	if err != nil {
		return err
	}
	defer func() { _ = cs.Close() }()
	list, err := cs.ListTools(ctx, nil)
	if err != nil {
		return err
	}
	names := []string{}
	for _, t := range list.Tools {
		names = append(names, t.Name)
	}
	fmt.Printf("tools/list: %v\n", names)
	for _, call := range []struct{ name, id string }{
		{"read_record", "public"}, {"read_record", "secret"}, {"save_record", "public"}, {"unknown_tool", "public"},
	} {
		r, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: call.name, Arguments: map[string]string{"id": call.id}})
		if err != nil {
			return err
		}
		status := "allowed"
		if r.IsError {
			status = "refused"
		}
		fmt.Printf("%s(%s): %s — %s\n", call.name, call.id, status, r.Content[0].(*mcp.TextContent).Text)
	}
	dec := json.NewDecoder(&audit)
	authorizations, executions := 0, 0
	for dec.More() {
		var e struct {
			Phase string `json:"phase"`
		}
		if err := dec.Decode(&e); err != nil {
			return err
		}
		if e.Phase == "authorization" {
			authorizations++
		} else {
			executions++
		}
	}
	fmt.Printf("audit: %d authorization records, %d execution record\n", authorizations, executions)
	return nil
}
