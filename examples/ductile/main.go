package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mattjoyce/shearline/policy"
)

// main prints the matrix, or decides one request.
//
//	go run .                                  # every token against every route
//	go run . readonly POST /pipeline/{pipeline}   # one decision, with its working
//
// A route is named as chi matches it. The third form of a plugin command, the
// write-class one, is asked as "write" on plugin/invoke; the router knows the class
// from the manifest before it asks, so the policy never reads the command.
func main() {
	p, err := Load(Tokens)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx := context.Background()
	switch len(os.Args) {
	case 1:
		if err := printMatrix(ctx, p); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case 4:
		if err := decide(ctx, p, os.Args[1], os.Args[2], os.Args[3]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: ductile [token method path]")
		os.Exit(2)
	}
}

// printMatrix decides every token against every route and prints a table: one Decide
// per cell, which is what a service does per request. Permitted would list the
// capabilities a token reaches, but a table of decisions is what an operator reads.
func printMatrix(ctx context.Context, p policy.Policy) error {
	names := make([]string, 0, len(Tokens))
	for tok := range Tokens {
		names = append(names, tok)
	}
	sort.Strings(names)
	fmt.Printf("%-42s", "")
	for _, tok := range names {
		fmt.Printf(" %-8s", tok)
	}
	fmt.Println()
	for _, r := range Routes {
		fmt.Printf("%-6s%-36s", r.Action, r.Method+" "+r.Path)
		for _, tok := range names {
			d, err := p.Decide(ctx, policy.Request{Principal: tok, Action: r.Action, Resource: r.Capability})
			if err != nil {
				return err
			}
			cell := "."
			if d.Allow {
				cell = "allow"
			}
			fmt.Printf(" %-8s", cell)
		}
		fmt.Println()
	}
	return nil
}

// decide asks one question and prints the answer with its working. The route is
// resolved first, as the router would: a path that matches no route is unknown before
// it is a question of permission.
func decide(ctx context.Context, p policy.Policy, token, method, path string) error {
	var rows []Route
	for _, r := range Routes {
		if r.Method == method && r.Path == path {
			rows = append(rows, r)
		}
	}
	if len(rows) == 0 {
		return fmt.Errorf("no route %s %s (the router answers this, not the policy)", method, path)
	}
	for _, r := range rows {
		d, err := p.Decide(ctx, policy.Request{Principal: token, Action: r.Action, Resource: r.Capability})
		if err != nil {
			return err
		}
		if d.Allow {
			fmt.Printf("allow  %s %s %s  by %s\n%s\n", token, r.Action, r.Capability, strings.Join(d.RuleSets, "; "), d.Proof)
			continue
		}
		fmt.Printf("deny   %s %s %s\n%s\n", token, r.Action, r.Capability, d.Why)
	}
	return nil
}
