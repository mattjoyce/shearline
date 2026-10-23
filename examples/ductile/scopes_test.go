package main

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/mattjoyce/shearline/policy"
)

// ductileNormalize is Ductile's normalizeScopes (internal/auth/auth.go, 2026-10-23),
// transcribed as the oracle: the implications it hard-codes, in the order it applies
// them. Each block is one of Ductile's.
func ductileNormalize(scopes []string) map[string]bool {
	out := map[string]bool{}
	for _, s := range scopes {
		if s = strings.TrimSpace(s); s != "" {
			out[s] = true
		}
	}
	if out["plugin:rw"] {
		out["plugin:ro"], out["plugin:catalog:ro"], out["plugin:invoke:ro"] = true, true, true
	}
	if out["plugin:ro"] {
		out["plugin:catalog:ro"] = true
	}
	if out["jobs:rw"] {
		out["jobs:ro"] = true
	}
	if out["jobs:ro"] {
		out["jobs:status:ro"], out["jobs:result:ro"], out["jobs:logs:ro"], out["jobs:tree:ro"] = true, true, true, true
	}
	if out["events:rw"] {
		out["events:ro"] = true
	}
	if out["events:ro"] {
		out["events:meta:ro"], out["events:payload:ro"] = true, true
	}
	return out
}

// ductileHasAny is Ductile's HasAnyScope: the wildcard passes everything; else any
// one of the required scopes.
func ductileHasAny(held map[string]bool, required []string) bool {
	if held["*"] {
		return true
	}
	for _, s := range required {
		if held[s] {
			return true
		}
	}
	return false
}

// allScopes is every scope the lattice or the route table names.
func allScopes() []string {
	set := map[string]bool{}
	for _, im := range Implies {
		set[im[0]], set[im[1]] = true, true
	}
	for _, r := range Routes {
		for _, s := range r.Scopes {
			set[s] = true
		}
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// TestParityWithDuctile holds the Shearline model to Ductile's own semantics: for
// every single scope and every pair of scopes, for every route, the two must agree.
// A disagreement names the token, the route, and which side allowed.
func TestParityWithDuctile(t *testing.T) {
	scopes := allScopes()
	var sets [][]string
	for i, a := range scopes {
		sets = append(sets, []string{a})
		for _, b := range scopes[i+1:] {
			sets = append(sets, []string{a, b})
		}
	}
	sets = append(sets, nil) // a token with no scopes reaches nothing
	ctx := context.Background()
	for _, held := range sets {
		tok := "tok:" + strings.Join(held, "+")
		if held == nil {
			tok = "tok:none"
		}
		p, err := Load(map[string][]string{tok: held})
		if err != nil {
			t.Fatalf("Load(%v): %v", held, err)
		}
		want := ductileNormalize(held)
		for _, r := range Routes {
			d, err := p.Decide(ctx, policy.Request{Principal: tok, Action: r.Action, Resource: r.Capability})
			if err != nil {
				t.Fatalf("Decide(%s, %s %s): %v", tok, r.Action, r.Capability, err)
			}
			if got, exp := d.Allow, ductileHasAny(want, r.Scopes); got != exp {
				t.Errorf("%s on %s %s (%s %s): shearline %v, ductile %v", tok, r.Method, r.Path, r.Action, r.Capability, got, exp)
			}
		}
	}
}

// TestMatrix pins the documented tokens against a few cells an operator would check
// by hand, so a change to the lattice or the table says which cell moved.
func TestMatrix(t *testing.T) {
	p, err := Load(Tokens)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	cases := []struct {
		token, action, capability string
		allow                     bool
	}{
		{"admin", "write", "system/reload", true},
		{"operator", "write", "pipeline/trigger", true},
		{"operator", "write", "system/reload", false},
		{"operator", "read", "system/config", false},
		{"readonly", "read", "plugin/catalog", true},
		{"readonly", "read", "plugin/invoke", false}, // P2-07: plugin:ro is catalog only
		{"readonly", "read", "jobs/result", true},
		{"status", "read", "jobs/status", true},
		{"status", "read", "jobs/result", false}, // D1: the field class is a separate decision
		{"invoker", "read", "plugin/invoke", true},
		{"invoker", "write", "plugin/invoke", false},
		{"invoker", "read", "plugin/catalog", false},
		{"nobody", "read", "jobs/status", false},
	}
	for _, c := range cases {
		d, err := p.Decide(ctx, policy.Request{Principal: c.token, Action: c.action, Resource: c.capability})
		if err != nil {
			t.Fatalf("Decide(%s %s %s): %v", c.token, c.action, c.capability, err)
		}
		if d.Allow != c.allow {
			t.Errorf("%s %s %s: allow %v, want %v\n%s%s", c.token, c.action, c.capability, d.Allow, c.allow, d.Proof, d.Why)
		}
	}
}

// TestFacts_RefusesUnknownScope: a token holding a scope nothing grants would, in
// Ductile, be a scope that silently does nothing; here it is refused at load.
func TestFacts_RefusesUnknownScope(t *testing.T) {
	if _, err := Load(map[string][]string{"x": {"jobs:rww"}}); err == nil {
		t.Fatal("a scope no route names must be refused")
	}
}
