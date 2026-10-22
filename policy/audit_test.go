package policy

import (
	"context"
	"testing"

	"github.com/mattjoyce/shearline/datalog"
)

// These four came from an adversarial audit of the module (EDGE_AUDIT.md findings 1 and
// 2, plus two that held). The first two reach through the exported AST, so they guard the
// ownership boundary: what Load runs is what the hash names, and nothing the caller keeps
// can change it afterward.

func aliceOnly(t *testing.T) datalog.RuleSet {
	t.Helper()
	return ruleSet(t, "custom", `allow("alice", "read", "alice/private").`)
}

func TestLoad_ASTMustMatchHash(t *testing.T) {
	set := aliceOnly(t)
	set.Program.Rules[0].Head.Args[2].Const = datalog.String("bob/private") // text and hash still say alice
	p, err := Load(Config{Sets: []datalog.RuleSet{RBAC(), set}})
	if err != nil {
		return // refusing the mismatch at Load is the right answer
	}
	d, err := p.Decide(context.Background(), Request{Principal: "alice", Action: "read", Resource: "bob/private"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Allow {
		t.Fatalf("an edited AST authorised bob's resource under the original hash: %v", d.RuleSets)
	}
}

func TestLoad_DetachedFromCallersProgram(t *testing.T) {
	set := aliceOnly(t)
	p := mustLoad(t, nil, RBAC(), set)
	set.Program.Rules[0].Head.Args[2].Const = datalog.String("bob/private")
	d, err := p.Decide(context.Background(), Request{Principal: "alice", Action: "read", Resource: "bob/private"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Allow {
		t.Fatal("mutating the caller's Program after Load changed an authorisation")
	}
}

func TestZeroPolicy_FailsClosed(t *testing.T) {
	var p Policy
	d, err := p.Decide(context.Background(), Request{Principal: "alice", Action: "read", Resource: "bob"})
	if err == nil || d.Allow {
		t.Fatalf("Decide = %+v, %v; want a deny with an error", d, err)
	}
	if got, err := p.Permitted(context.Background(), "alice", "read"); err == nil || got != nil {
		t.Fatalf("Permitted = %v, %v; want nil and an error", got, err)
	}
}

// A delegated-ownership policy: alice reads her own prefix, only through the declared
// channel, and a near-miss prefix or a missing or wrong channel is a deny.
func TestDecide_DelegateOwnership(t *testing.T) {
	set := ruleSet(t, "delegate", `deny(P, A, R) :- principal(P), action(A), resource(R), not context("delegate", "corporate-chat").`)
	p := mustLoadConfig(t, Config{
		Standing: []datalog.Fact{Role("alice", "self"), Grant("self", "read", "employees/alice/")},
		Sets:     []datalog.RuleSet{RBAC(), set},
		Context:  map[string]datalog.Kind{"delegate": datalog.KindString},
	})
	tests := []struct {
		name, resource, delegate string
		allow                    bool
	}{
		{"self", "employees/alice/private", "corporate-chat", true},
		{"other", "employees/bob/private", "corporate-chat", false},
		{"near prefix", "employees/alice2/private", "corporate-chat", false},
		{"wrong delegate", "employees/alice/private", "rogue", false},
		{"missing delegate", "employees/alice/private", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := map[string]datalog.Const{}
			if tc.delegate != "" {
				c["delegate"] = datalog.String(tc.delegate)
			}
			d, err := p.Decide(context.Background(), Request{Principal: "alice", Action: "read", Resource: tc.resource, Context: c})
			if err != nil || d.Allow != tc.allow {
				t.Fatalf("Allow = %v, err = %v; want %v", d.Allow, err, tc.allow)
			}
		})
	}
}
