package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mattjoyce/shearline/policy"
)

// TestMatrix pins the permission matrix. Each row is one cell of the table a product
// owner would draw: this token, this endpoint, allowed or not. If rbac.yaml or
// tokens.yaml changes the matrix, this test says which cell moved.
func TestMatrix(t *testing.T) {
	s, err := LoadSnapshot("rbac.yaml", "tokens.yaml")
	if err != nil {
		t.Fatal(err)
	}
	p := s.Policy
	all := []string{"/query", "/submit", "/start", "/stop", "/mint", "/revoke", "/grant", "/expire"}
	want := map[string][]string{
		"tok-7f3a": {"/query"},
		"tok-9c1e": {"/query", "/submit"},
		"tok-e210": {"/query", "/start", "/stop"},
		"tok-0b44": all,
		"tok-none": nil, // a token nobody minted: no roles, no permissions
	}
	ctx := context.Background()
	for tok, allowed := range want {
		for _, ep := range all {
			d, err := p.Decide(ctx, request(tok, ep))
			if err != nil {
				t.Fatalf("%s %s: %v", tok, ep, err)
			}
			if got, exp := d.Allow, slices.Contains(allowed, ep); got != exp {
				t.Errorf("%s call %s: allow=%v, want %v", tok, ep, got, exp)
			}
		}
		got, err := p.Permitted(ctx, tok, "call")
		if err != nil {
			t.Fatal(err)
		}
		exp := slices.Clone(allowed)
		slices.Sort(exp)
		if !slices.Equal(got, exp) {
			t.Errorf("Permitted(%s) = %v, want %v", tok, got, exp)
		}
	}
}

// TestProofNamesTheInheritance checks that an allow through an inherited role shows the
// inheritance in its proof, so an operator reading a receipt can see why Admin may stop
// the service: not because Admin was granted /stop, but because Admin inherits Operator.
func TestProofNamesTheInheritance(t *testing.T) {
	s, err := LoadSnapshot("rbac.yaml", "tokens.yaml")
	if err != nil {
		t.Fatal(err)
	}
	p := s.Policy
	d, err := p.Decide(context.Background(), request("tok-0b44", "/stop"))
	if err != nil || !d.Allow {
		t.Fatalf("allow=%v err=%v", d.Allow, err)
	}
	proof := d.Proof.String()
	for _, line := range []string{`inherits("Admin", "Operator")`, `grants("Operator", "call", "/stop")`} {
		if !strings.Contains(proof, line) {
			t.Errorf("proof lacks %s:\n%s", line, proof)
		}
	}
}

// TestRefusalExplainsItself checks that a deny says how far the rule got. RW's permits
// are /query and /submit; neither is a prefix of /stop, so the rule reaches the prefix
// test and stops. Why reports the one binding that got farthest, not every binding.
func TestRefusalExplainsItself(t *testing.T) {
	s, err := LoadSnapshot("rbac.yaml", "tokens.yaml")
	if err != nil {
		t.Fatal(err)
	}
	p := s.Policy
	d, err := p.Decide(context.Background(), request("tok-9c1e", "/stop"))
	if err != nil || d.Allow {
		t.Fatalf("allow=%v err=%v", d.Allow, err)
	}
	if why := d.Why.String(); !strings.Contains(why, "stopped at prefix(Pat, R)") {
		t.Errorf("why does not name the failing literal:\n%s", why)
	}
}

// TestBadFilesFailClosed exercises the three refusal layers described on LoadPolicy.
func TestBadFilesFailClosed(t *testing.T) {
	cases := []struct {
		name, matrix, tokens, want string
	}{
		{"misspelled key", strings.Replace(read(t, "rbac.yaml"), "grants: [/submit]", "grant: [/submit]", 1),
			read(t, "tokens.yaml"), "field grant not found"},
		{"unknown role on token", read(t, "rbac.yaml"),
			strings.Replace(read(t, "tokens.yaml"), "[RO, Operator]", "[RO, Opertor]", 1), `unknown role "Opertor"`},
		{"unknown parent", strings.Replace(read(t, "rbac.yaml"), "inherits: [RO]", "inherits: [Reader]", 1),
			read(t, "tokens.yaml"), `unknown role "Reader"`},
		{"empty endpoint", strings.Replace(read(t, "rbac.yaml"), "[/query]", `[""]`, 1),
			read(t, "tokens.yaml"), "grants"},
		{"endpoint is a prefix of another", strings.Replace(read(t, "rbac.yaml"), "[/query]", "[/sub]", 1),
			read(t, "tokens.yaml"), `"/sub" is a prefix of "/submit"`},
		{"no action", strings.Replace(read(t, "rbac.yaml"), "action: call", "action: \"\"", 1),
			read(t, "tokens.yaml"), "action is empty"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			mp, tp := filepath.Join(dir, "rbac.yaml"), filepath.Join(dir, "tokens.yaml")
			write(t, mp, c.matrix)
			write(t, tp, c.tokens)
			_, err := LoadSnapshot(mp, tp)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to mention %q", err, c.want)
			}
		})
	}
}

// TestGuardKeepsOldPolicyOnBadReload checks that a failed reload leaves the policy in
// force unchanged, which is what lets the service treat a bad tokens.yaml as an alert
// rather than an outage.
func TestGuardKeepsOldPolicyOnBadReload(t *testing.T) {
	dir := t.TempDir()
	mp, tp := filepath.Join(dir, "rbac.yaml"), filepath.Join(dir, "tokens.yaml")
	write(t, mp, read(t, "rbac.yaml"))
	write(t, tp, read(t, "tokens.yaml"))
	g, err := NewGuard(mp, tp)
	if err != nil {
		t.Fatal(err)
	}
	write(t, tp, "tokens: {tok-x: [Nope]}")
	if err := g.Reload(mp, tp); err == nil {
		t.Fatal("reload of a bad file succeeded")
	}
	d, err := g.Current().Policy.Decide(context.Background(), request("tok-7f3a", "/query"))
	if err != nil || !d.Allow {
		t.Fatalf("old policy not in force: allow=%v err=%v", d.Allow, err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func write(t *testing.T, path, s string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(s), 0o600); err != nil {
		t.Fatal(err)
	}
}

func request(token, endpoint string) policy.Request {
	return policy.Request{Principal: token, Action: "call", Resource: endpoint}
}

// TestRouteTable checks that the route table is every endpoint the matrix names, sorted,
// and that a path the matrix never names is not in it. The router answers for /stopall;
// Shearline is never asked.
func TestRouteTable(t *testing.T) {
	s, err := LoadSnapshot("rbac.yaml", "tokens.yaml")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/expire", "/grant", "/mint", "/query", "/revoke", "/start", "/stop", "/submit"}
	if !slices.Equal(s.Endpoints, want) {
		t.Errorf("Endpoints = %v, want %v", s.Endpoints, want)
	}
	if _, found := slices.BinarySearch(s.Endpoints, "/stopall"); found {
		t.Error("/stopall is not an endpoint")
	}
}
