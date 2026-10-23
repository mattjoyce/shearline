package main

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/mattjoyce/shearline/policy"
)

// main prints the matrix, or decides one request.
//
// With no arguments it lists, for every token, the endpoints it may call. That list comes
// from policy.Permitted, which runs the same rules as a decision and returns the grant
// prefixes the token reaches through its roles.
//
// With a token and an endpoint it calls policy.Decide and prints the decision. An allow
// comes with the proof: every rule that fired and every fact it used. A fact prints as
// `fact`. Beside a rule, `rbac/1:19` means line 19 of the bundled rule set, rbac version
// 1, whose source is policy/rbac.dl in the Shearline repository. A deny comes with the
// reason: which rule got how far, and what it did not find.
func main() {
	g, err := NewGuard("rbac.yaml", "tokens.yaml")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx := context.Background()
	switch len(os.Args) {
	case 1:
		if err := printMatrix(ctx, g.Current()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case 3:
		if err := decide(ctx, g.Current(), os.Args[1], os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: logsvc [token endpoint]")
		os.Exit(2)
	}
}

// printMatrix lists every token and what it may call.
func printMatrix(ctx context.Context, s Snapshot) error {
	for _, tok := range s.Tokens {
		eps, err := s.Policy.Permitted(ctx, tok, "call")
		if err != nil {
			return err
		}
		fmt.Printf("%-9s %s\n", tok, strings.Join(eps, " "))
	}
	return nil
}

// decide asks one question and prints the answer with its working.
//
// The path is resolved against the route table first. In a real service the router does
// this: a request for /stopall matches no route and is a 404 before any question of
// permission arises. The resolved route name is what Shearline is asked about. Passing the
// raw path instead would let a grant of /stop cover /stopall, because grants match by
// prefix. Authorisation answers "may this token call this route"; it does not decide what
// the route is.
//
// Decide never returns Allow=true alongside an error. A malformed request (an empty
// field, an undeclared context key) wraps policy.ErrRequest and is a deny. Any other
// error means the policy or the engine is broken, and is also a deny. Every error is a
// deny; the service only has to decide how loudly to say so.
func decide(ctx context.Context, s Snapshot, token, path string) error {
	if _, found := slices.BinarySearch(s.Endpoints, path); !found {
		return fmt.Errorf("unknown endpoint %q (the router answers this, not the policy)", path)
	}
	d, err := s.Policy.Decide(ctx, policy.Request{Principal: token, Action: "call", Resource: path})
	if err != nil {
		return err
	}
	if d.Allow {
		fmt.Printf("allow  %s call %s  by %v\n%s\n", token, path, d.RuleSets, d.Proof)
		return nil
	}
	fmt.Printf("deny   %s call %s\n%s\n", token, path, d.Why)
	return nil
}
