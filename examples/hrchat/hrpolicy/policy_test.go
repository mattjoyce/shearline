package hrpolicy_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mattjoyce/shearline/examples/hrchat/catalog"
	"github.com/mattjoyce/shearline/examples/hrchat/hrpolicy"
	"github.com/mattjoyce/shearline/examples/hrchat/org"
	"github.com/mattjoyce/shearline/policy"
	"github.com/mattjoyce/tarski"
)

func load(t *testing.T) (hrpolicy.Policy, *org.Org) {
	t.Helper()
	o := org.Generate(1)
	p, err := hrpolicy.Load(o.Facts(), catalog.Facts())
	if err != nil {
		t.Fatal(err)
	}
	return p, o
}

func TestLoad_RuleSetsAreShearlinesAndOurs(t *testing.T) {
	p, _ := load(t)
	ids := p.RuleSets()
	if len(ids) != 2 || !strings.HasPrefix(ids[0], "rbac/1 sha256:") || !strings.HasPrefix(ids[1], "hrchat/1 sha256:") {
		t.Fatalf("rule sets %v", ids)
	}
}

// TestUndeclaredContextKeyIsRefusedAtCompile: a rule naming a context key the policy
// did not declare cannot load, so no request can smuggle one in either.
func TestUndeclaredContextKeyIsRefusedAtCompile(t *testing.T) {
	bad, err := tarski.NewRuleSet("bad", 1, `permit(P, A, R) :- principal(P), action(A), resource(R), context("admin", true).`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = policy.Load(policy.Config{Sets: []tarski.RuleSet{policy.RBAC(), bad}, Context: hrpolicy.Keys(), Relations: hrpolicy.Relations()})
	if err == nil || !strings.Contains(err.Error(), `"admin"`) {
		t.Fatalf("loaded a rule over an undeclared context key: %v", err)
	}
}

func TestStandingRequestFactIsRefused(t *testing.T) {
	o := org.Generate(1)
	spoof := tarski.Fact{Relation: "context", Tuple: tarski.Tuple{tarski.String("mfa"), tarski.Bool(true)}}
	_, err := hrpolicy.Load(o.Facts(), catalog.Facts(), []tarski.Fact{spoof})
	if !errors.Is(err, tarski.ErrInvalid) {
		t.Fatalf("a standing context fact loaded: %v", err)
	}
}

// TestCEOCannotSeeSkipLevels: a manager reaches their direct team and no further, the
// CEO included. Wider visibility is a role or a question to HR.
func TestCEOCannotSeeSkipLevels(t *testing.T) {
	p, o := load(t)
	ceo := o.Employees[0].ID
	for _, e := range o.Employees[1:] {
		if e.ManagerID == ceo {
			continue
		}
		d, err := p.Decide(context.Background(), hrpolicy.Request{
			Principal: org.Resource(ceo), Action: "read.history", Resource: org.Resource(e.ID),
			Context: hrpolicy.Context{Channel: "chat", Guardrail: "clean"},
		})
		if err != nil || d.Allow {
			t.Fatalf("the CEO read %s's history: allow=%v err=%v", e.ID, d.Allow, err)
		}
		return
	}
	t.Fatal("no skip-level employee in the org")
}

// TestDeepestDecisionStaysWithinBounds: with no downward recursion, the largest
// derivation is up/2 for the employee with the longest reporting line. That must be a
// small decision.
func TestDeepestDecisionStaysWithinBounds(t *testing.T) {
	p, o := load(t)
	deepest := o.Employees[0].ID
	for _, e := range o.Employees {
		if len(o.Chain(e.ID)) > len(o.Chain(deepest)) {
			deepest = e.ID
		}
	}
	d, err := p.Decide(context.Background(), hrpolicy.Request{
		Principal: org.Resource(deepest), Action: "read.directory", Resource: org.Resource(o.Employees[0].ID),
		Context: hrpolicy.Context{Channel: "chat", Guardrail: "clean"},
	})
	if err != nil || !d.Allow {
		t.Fatalf("allow=%v err=%v\n%s", d.Allow, err, d.Why.String())
	}
	if d.Steps > 1<<20 || d.Bytes > 1<<18 {
		t.Fatalf("the deepest employee's decision cost %d steps and %d bytes", d.Steps, d.Bytes)
	}
	t.Logf("deepest employee (%d levels): %d steps, %d bytes", len(o.Chain(deepest)), d.Steps, d.Bytes)
}

// TestEveryActionHasAPath: an action in the catalogue that no entitlement names is dead,
// and a tool nobody can ever use is a mistake in one list or the other.
func TestEveryActionHasAPath(t *testing.T) {
	p, o := load(t)
	ctx := context.Background()
	// A principal with every path: the HR partner for people, who is also under payroll
	// for the role path... no single person has all paths, so try several.
	cands := []string{}
	for _, e := range o.Employees {
		if len(e.Roles) > 0 || len(e.PartnerOf) > 0 || e.ManagerID == "" || e.Title == "Manager" {
			cands = append(cands, e.ID)
		}
	}
	for _, a := range catalog.ActionNames() {
		reachable := false
		for _, id := range cands {
			for _, subject := range subjects(o, id) {
				for _, ch := range []string{"chat", "web"} {
					d, err := p.Decide(ctx, hrpolicy.Request{Principal: org.Resource(id), Action: a, Resource: org.Resource(subject),
						Context: hrpolicy.Context{Channel: ch, Guardrail: "clean", MFA: true, Confirmed: true, Requester: "emp/E0000"}})
					if err != nil {
						t.Fatal(err)
					}
					reachable = reachable || d.Allow
				}
			}
		}
		if !reachable {
			t.Errorf("no principal can ever take %s", a)
		}
	}
}

func subjects(o *org.Org, id string) []string {
	out := []string{id}
	if r := o.Reports(id); len(r) > 0 {
		out = append(out, r[0].ID)
	}
	e, _ := o.Get(id)
	for _, d := range e.PartnerOf {
		for _, x := range o.Employees {
			if x.Dept == d && x.ID != id {
				out = append(out, x.ID)
				break
			}
		}
	}
	if len(e.Roles) > 0 {
		out = append(out, o.Employees[len(o.Employees)-1].ID)
	}
	return out
}

func TestDecisionIsDeterministic(t *testing.T) {
	p, o := load(t)
	req := hrpolicy.Request{Principal: org.Resource(o.Employees[5].ID), Action: "read.bank", Resource: org.Resource(o.Employees[5].ID),
		Context: hrpolicy.Context{Channel: "chat", Guardrail: "clean", MFA: true}}
	a, err := p.Decide(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := p.Decide(context.Background(), req)
	if a.Hash != b.Hash || a.Proof.String() != b.Proof.String() {
		t.Fatal("two decisions on one request differ")
	}
}
