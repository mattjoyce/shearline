package policy

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mattjoyce/shearline/policy/builtins"
	"github.com/mattjoyce/tarski"
)

// officeRules is RBAC's rules with one more literal on allow: the request comes from the
// office network, read through a consumer builtin. It is RBAC's text, not RBAC(), since
// RBAC's own allow rule would allow from anywhere.
const officeRules = `
holds(P, R) :- principal(P), role(P, R).
holds(P, R) :- holds(P, Q), inherits(Q, R).
permit(P, A, Pat) :- principal(P), action(A), holds(P, Role), grants(Role, A, Pat).
deny(P, A, R) :- principal(P), action(A), resource(R), denied(P, A, R).
allow(P, A, R) :- permit(P, A, Pat), resource(R), prefix(Pat, R), not deny(P, A, R),
	context("ip", IP), in_net(IP, "10.0.0.0/8").
`

func netConfig(t *testing.T) Config {
	t.Helper()
	return Config{
		Standing: []tarski.Fact{Role("ana", "staff"), Grant("staff", "read", "docs/")},
		Sets:     []tarski.RuleSet{ruleSet(t, "office", officeRules)},
		Context:  map[string]tarski.Kind{"ip": tarski.KindString},
		Builtins: map[string]tarski.Builtin{"in_net": builtins.InCIDR},
	}
}

func ipRequest(ip string) Request {
	return Request{Principal: "ana", Action: "read", Resource: "docs/a", Context: map[string]tarski.Const{"ip": tarski.String(ip)}}
}

func TestBuiltins_GuardAllowsAndExplains(t *testing.T) {
	p := mustLoadConfig(t, netConfig(t))
	ctx := context.Background()

	d, err := p.Decide(ctx, ipRequest("10.1.2.3"))
	if err != nil || !d.Allow {
		t.Fatalf("from 10.1.2.3: Allow=%v err=%v, want allowed", d.Allow, err)
	}
	if !strings.Contains(d.Proof.String(), `in_net(IP, "10.0.0.0/8")`) {
		t.Errorf("proof does not name the guard:\n%v", d.Proof)
	}

	d, err = p.Decide(ctx, ipRequest("192.168.0.1"))
	if err != nil || d.Allow {
		t.Fatalf("from 192.168.0.1: Allow=%v err=%v, want denied", d.Allow, err)
	}
	var stopped *tarski.RuleFailure
	for _, f := range failures(d.Why) {
		if f.Failed.Atom != nil && f.Failed.Atom.Name == "in_net" {
			stopped = &f
		}
	}
	if stopped == nil {
		t.Fatalf("why-not does not stop at in_net:\n%v", d.Why)
	}
	if got := stopped.Bound["IP"]; got.Text() != "192.168.0.1" {
		t.Errorf("why-not bound IP = %q, want 192.168.0.1", got.Text())
	}
	if !strings.Contains(d.Why.String(), "stopped at in_net(IP") {
		t.Errorf("rendered why-not does not name the guard:\n%v", d.Why)
	}

	// A malformed address is false, not an error: it came from a request.
	if d, err := p.Decide(ctx, ipRequest("not-an-ip")); err != nil || d.Allow {
		t.Errorf("from not-an-ip: Allow=%v err=%v, want a plain deny", d.Allow, err)
	}
}

func TestBuiltins_LoadRefuses(t *testing.T) {
	cases := []struct {
		name   string
		edit   func(*Config)
		reason string
	}{
		{"engine name", func(c *Config) { c.Builtins = map[string]tarski.Builtin{"prefix": builtins.InCIDR} }, "engine builtin"},
		{"PARC name", func(c *Config) { c.Builtins = map[string]tarski.Builtin{"context": builtins.InCIDR} }, "both a base relation and a builtin"},
		{"declared relation", func(c *Config) {
			c.Relations = map[string]Relation{"office": {Kinds: []tarski.Kind{tarski.KindString}}}
			c.Builtins = map[string]tarski.Builtin{"office": builtins.InCIDR, "in_net": builtins.InCIDR}
		}, "office is both a base relation and a builtin"},
		{"a rule head", func(c *Config) {
			c.Builtins = map[string]tarski.Builtin{"in_net": builtins.InCIDR, "deny": builtins.Before}
		}, "deny is a built-in and cannot be the head of a rule"},
		{"no Holds", func(c *Config) {
			b := builtins.InCIDR
			b.Holds = nil
			c.Builtins = map[string]tarski.Builtin{"in_net": b}
		}, "builtin in_net has no Holds"},
		{"no Args", func(c *Config) {
			b := builtins.InCIDR
			b.Args = nil
			c.Builtins = map[string]tarski.Builtin{"in_net": b}
		}, "builtin in_net has no arguments"},
		{"undeclared", func(c *Config) { c.Builtins = nil }, "in_net"},
		{"bad constant", func(c *Config) {
			c.Sets = append(c.Sets, ruleSet(t, "typo", `allow(P, A, R) :- principal(P), action(A), resource(R), context("ip", IP), in_net(IP, "not-a-cidr").`))
		}, `line 1: in_net(Addr, Network): argument Network: "not-a-cidr"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := netConfig(t)
			c.edit(&cfg)
			_, err := Load(cfg)
			if !errors.Is(err, tarski.ErrInvalid) || !strings.Contains(err.Error(), c.reason) {
				t.Fatalf("Load = %v, want ErrInvalid containing %q", err, c.reason)
			}
		})
	}
}

// Same policy, same request: the same decision, proof, why-not and rule-set IDs, every time.
func TestBuiltins_Deterministic(t *testing.T) {
	p := mustLoadConfig(t, netConfig(t))
	q := mustLoadConfig(t, netConfig(t))
	for _, ip := range []string{"10.1.2.3", "192.168.0.1"} {
		first := marshalDecision(t, p, ip)
		for i := 0; i < 20; i++ {
			for _, pol := range []Policy{p, q} {
				if got := marshalDecision(t, pol, ip); got != first {
					t.Fatalf("%s: decision %d differs:\n%s\nwant\n%s", ip, i, got, first)
				}
			}
		}
	}
}

func marshalDecision(t *testing.T, p Policy, ip string) string {
	t.Helper()
	d, err := p.Decide(context.Background(), ipRequest(ip))
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	b, err := json.Marshal(struct {
		Allow    bool
		Proof    tarski.Proof
		Why      tarski.Absence
		RuleSets []string
	}{d.Allow, d.Proof, d.Why, d.RuleSets})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return string(b)
}

// Changing the caller's map after Load changes nothing the policy does.
func TestBuiltins_LoadCopies(t *testing.T) {
	cfg := netConfig(t)
	p := mustLoadConfig(t, cfg)
	cfg.Builtins["in_net"] = tarski.Builtin{Args: builtins.InCIDR.Args, Holds: func([]tarski.Const) bool { return true }}
	delete(cfg.Builtins, "in_net")
	d, err := p.Decide(context.Background(), ipRequest("192.168.0.1"))
	if err != nil || d.Allow {
		t.Fatalf("after editing the map: Allow=%v err=%v, want denied", d.Allow, err)
	}
	if !reflect.DeepEqual(d.RuleSets, mustLoadConfig(t, netConfig(t)).ids) {
		t.Errorf("rule-set IDs changed")
	}
}
