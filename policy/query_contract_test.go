package policy

import (
	"context"
	"errors"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/mattjoyce/tarski"
)

// rawQuery is deliberately unchecked: expose the engine's set semantics so a
// rejecting Query cannot hide a divergence in these negative controls.
func rawQuery(t *testing.T, p Policy, s Space) []Allow {
	t.Helper()
	facts, sets, err := querySets(s)
	if err != nil {
		t.Fatal(err)
	}
	base, err := p.base.With(append(facts, contextFacts(s.Contexts[0])...)...)
	if err != nil {
		t.Fatal(err)
	}
	res, err := tarski.Evaluate(context.Background(), p.plan, base, tarski.Options{Proofs: true})
	if err != nil {
		t.Fatal(err)
	}
	rel, err := res.Relation("allow")
	if err != nil {
		t.Fatal(err)
	}
	var out []Allow
	for _, tuple := range rel.Tuples() {
		if !queryMember(tuple, sets) {
			continue
		}
		proof, ok := res.Proof("allow", tuple)
		if !ok {
			t.Fatal("raw allow lacks proof")
		}
		out = append(out, Allow{Principal: tuple[0].Text(), Action: tuple[1].Text(), Resource: tuple[2].Text(), Proof: proof})
	}
	return out
}

func TestQuery_KeyingViolations(t *testing.T) {
	for _, tc := range []struct {
		name, rules, foreign string
		undetected           bool
	}{
		{name: "principal_unkeyed", rules: `met("mfa") :- principal("eve").
allow(P,A,R) :- principal(P), action(A), resource(R), met("mfa").`, foreign: "principal(eve)"},
		{name: "action_unkeyed", rules: `met("ready") :- action("read").
allow(P,A,R) :- principal(P), action(A), resource(R), met("ready").`, foreign: "action(read)"},
		{name: "unjoined_keyed", rules: `owner("eve", "doc/1").
direct(P,R) :- principal(P), resource(R), owner(P,R).
allow(P,A,R) :- principal(P), action(A), resource(R), direct(_,R).`, foreign: "principal(eve)"},
		{name: "prefix_collision", rules: `owner("bo", "doc/1").
permit(P,A,R) :- principal(P), action(A), resource(R), owner(P,R).`, foreign: "resource(doc/1)"},
		{name: "negation", rules: `banned("eve").
blocked(A) :- principal(P), action(A), banned(P).
allow(P,A,R) :- principal(P), action(A), resource(R), not blocked(A).`, undetected: true},
		{name: "nested_negation", rules: `blocked(P) :- principal(P), not principal("eve").
allow(P,A,R) :- principal(P), action(A), resource(R), not blocked(P).`, undetected: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := mustLoad(t, nil, RBAC(), ruleSet(t, tc.name, tc.rules))
			s := Space{Principals: []string{"bo", "eve"}, Actions: []string{"read", "write"}, Resources: []string{"doc/1", "doc/10"}, Contexts: []map[string]tarski.Const{nil}}
			raw := rawQuery(t, p, s)
			keys := map[[3]string]bool{}
			flagged := 0
			for _, a := range raw {
				keys[[3]string{a.Principal, a.Action, a.Resource}] = true
				foreign, found := foreignRequestFact(a.Proof, a.Proof.Tuple)
				if found {
					flagged++
					if foreign != tc.foreign {
						t.Fatalf("foreign=%s want %s", foreign, tc.foreign)
					}
				}
				if tc.name == "prefix_collision" && found != (a.Resource == "doc/10") {
					t.Fatalf("prefix flag for %s=%v", a.Resource, found)
				}
			}
			divergent := 0
			for _, who := range s.Principals {
				for _, action := range s.Actions {
					for _, resource := range s.Resources {
						d, err := p.Decide(context.Background(), Request{Principal: who, Action: action, Resource: resource})
						if err != nil {
							t.Fatal(err)
						}
						if d.Allow != keys[[3]string{who, action, resource}] {
							divergent++
						}
					}
				}
			}
			if divergent == 0 {
				t.Fatal("negative control agrees with Decide")
			}
			m, err := p.Query(context.Background(), s)
			if tc.undetected {
				if err != nil || len(m.Allows) != len(raw) || flagged != 0 {
					t.Fatalf("negation limit changed: allows=%d flagged=%d err=%v", len(m.Allows), flagged, err)
				}
			} else {
				if !errors.Is(err, ErrKeying) || errors.Is(err, ErrRequest) || !strings.Contains(err.Error(), "bag 0: allow(") || !strings.Contains(err.Error(), "cites "+tc.foreign) {
					t.Fatalf("keying error=%v", err)
				}
				assertZeroMatrix(t, p, m)
				if flagged != divergent {
					t.Fatalf("flagged=%d divergent=%d", flagged, divergent)
				}
			}
			t.Logf("raw allows=%d divergent=%d flagged=%d Query err=%v", len(raw), divergent, flagged, err)
		})
	}
}

func TestQuery_MultiAxisAgreement(t *testing.T) {
	p := mustLoadConfig(t, Config{Standing: fixture(), Sets: rbacOnly(), Context: map[string]tarski.Kind{"mfa": tarski.KindBool}})
	// #nosec G404 -- deterministic synthetic test sampling.
	rng := rand.New(rand.NewPCG(10, 10))
	for range 12 {
		s := querySpace()
		rng.Shuffle(len(s.Principals), func(i, j int) { s.Principals[i], s.Principals[j] = s.Principals[j], s.Principals[i] })
		rng.Shuffle(len(s.Actions), func(i, j int) { s.Actions[i], s.Actions[j] = s.Actions[j], s.Actions[i] })
		rng.Shuffle(len(s.Resources), func(i, j int) { s.Resources[i], s.Resources[j] = s.Resources[j], s.Resources[i] })
		s.Principals = s.Principals[:2+rng.IntN(2)]
		s.Actions = s.Actions[:2+rng.IntN(2)]
		s.Resources = s.Resources[:2+rng.IntN(3)]
		s.Contexts = []map[string]tarski.Const{{"mfa": tarski.Bool(true)}, {"mfa": tarski.Bool(false)}}
		m := mustQuery(t, p, s)
		keys := map[[4]string]bool{}
		for _, a := range m.Allows {
			keys[[4]string{s.Contexts[a.Bag]["mfa"].Text(), a.Principal, a.Action, a.Resource}] = true
		}
		for _, bag := range s.Contexts {
			for _, who := range s.Principals {
				for _, action := range s.Actions {
					for _, resource := range s.Resources {
						d, err := p.Decide(context.Background(), Request{Principal: who, Action: action, Resource: resource, Context: bag})
						got := keys[[4]string{bag["mfa"].Text(), who, action, resource}]
						if err != nil || got != d.Allow {
							t.Fatalf("%s/%s/%s: Query=%v Decide=%v err=%v", who, action, resource, got, d.Allow, err)
						}
					}
				}
			}
		}
	}
}

func TestQuery_ValidateBeforeEvaluation(t *testing.T) {
	evaluations := 0
	guard := tarski.Builtin{Args: []tarski.BuiltinArg{{}}, Holds: func(_ []tarski.Const) bool { evaluations++; return true }}
	rules := ruleSet(t, "early", `permit(P,A,R) :- principal(P), action(A), resource(R), guard(P).`)
	p := mustLoadConfig(t, Config{Sets: []tarski.RuleSet{RBAC(), rules}, Builtins: map[string]tarski.Builtin{"guard": guard}})
	s := querySpace()
	s.Contexts = []map[string]tarski.Const{nil, {"unknown": tarski.Bool(true)}}
	m, err := p.Query(context.Background(), s)
	if !errors.Is(err, ErrRequest) || evaluations != 0 {
		t.Fatalf("err=%v evaluations=%d", err, evaluations)
	}
	if strings.Count(err.Error(), "policy:") != 1 {
		t.Fatalf("duplicate prefix: %v", err)
	}
	assertZeroMatrix(t, p, m)
}

func TestQuery_EmptyContexts(t *testing.T) {
	p := mustLoad(t, fixture(), RBAC())
	for _, bags := range [][]map[string]tarski.Const{nil, {}} {
		s := querySpace()
		s.Contexts = bags
		m, err := p.Query(context.Background(), s)
		if !errors.Is(err, ErrRequest) || !errors.Is(err, tarski.ErrInvalid) || !strings.Contains(err.Error(), "contexts set is empty") {
			t.Fatal(err)
		}
		assertZeroMatrix(t, p, m)
	}
}
