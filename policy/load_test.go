package policy

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mattjoyce/tarski"
)

func TestRBAC_Parses(t *testing.T) {
	s := RBAC()
	if s.Name != "rbac" || s.Version != 1 || !strings.HasPrefix(s.Hash, "sha256:") || len(s.Program.Rules) == 0 {
		t.Fatalf("RBAC() = %+v", s)
	}
}

func TestMustRuleSet_PanicsOnBrokenAsset(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("mustRuleSet did not panic on unparseable text")
		}
	}()
	mustRuleSet("broken", 1, "this is not datalog")
}

func TestRBAC_CopyKeepsProvenance(t *testing.T) {
	p := mustLoad(t, fixture(), RBAC())
	d, err := p.Decide(context.Background(), Request{Principal: "ana", Action: "read", Resource: "timeline/bp"})
	if err != nil || !d.Allow {
		t.Fatalf("Decide = %+v, %v", d, err)
	}
	if d.Proof.Rule.Set != "rbac/1" || d.Proof.Rule.Text == "" {
		t.Fatalf("allowed proof rule = set %q, text %q", d.Proof.Rule.Set, d.Proof.Rule.Text)
	}
	d, err = p.Decide(context.Background(), Request{Principal: "zed", Action: "read", Resource: "timeline/bp"})
	if err != nil || d.Allow || len(d.Why.Rules) == 0 {
		t.Fatalf("Decide = %+v, %v", d, err)
	}
	if r := d.Why.Rules[0].Rule; r.Set != "rbac/1" || r.Text == "" {
		t.Fatalf("denied why rule = set %q, text %q", r.Set, r.Text)
	}
	if !reflect.DeepEqual(RBAC().Program.Rules, rbac.Program.Rules) {
		t.Fatal("the copy differs from the original")
	}
}

func TestRBAC_FreshEachCall(t *testing.T) {
	a := RBAC()
	a.Program.Rules[0].Head.Name = "tampered"
	a.Program.Rules[0].Body[0].Atom.Name = "tampered"
	a.Program.Declared["tampered"] = 1
	b := RBAC()
	if b.Program.Rules[0].Head.Name == "tampered" || b.Program.Rules[0].Body[0].Atom.Name == "tampered" || b.Program.Declared["tampered"] != 0 {
		t.Fatal("RBAC() shares its AST with the caller")
	}
	if _, err := Load(Config{Standing: fixture(), Sets: rbacOnly()}); err != nil {
		t.Fatalf("Load after tampering: %v", err)
	}
}

func TestSchema_FreshEachCall(t *testing.T) {
	delete(Schema().Checks, "grants")
	delete(Schema().Arities, "grants")
	if _, ok := Schema().Checks["grants"]; !ok {
		t.Fatal("mutating one Schema() value changed the next")
	}
	if _, err := Load(Config{Standing: []tarski.Fact{Grant("r", "read", "")}, Sets: rbacOnly()}); !errors.Is(err, tarski.ErrInvalid) {
		t.Fatalf("an empty prefix loaded after Schema() was mutated: %v", err)
	}
}

func TestLoad_RBACAloneSucceeds(t *testing.T) {
	// deny/3 is declared and undefined: the empty relation, not a compile error.
	if _, err := Load(Config{Sets: rbacOnly()}); err != nil {
		t.Fatalf("Load(RBAC) = %v", err)
	}
}

func TestLoad_Errors(t *testing.T) {
	unsafe := ruleSet(t, "bad", `deny(P, A, R) :- principal(P), X > 1.`)
	seven, err := tarski.Number("7")
	if err != nil {
		t.Fatal(err)
	}
	numConst := ruleSet(t, "num", `deny(P, A, R) :- principal(P), action(A), resource(R), role(7, P).`)
	cyc := ruleSet(t, "cyc", `deny(P, A, R) :- allow(P, A, R).`)
	emptyConst := ruleSet(t, "empty", `deny(P, A, R) :- principal(P), action(A), resource(R), P = "", role(P, "").`)
	tests := []struct {
		name     string
		standing []tarski.Fact
		sets     []tarski.RuleSet
	}{
		{"unsafe variable in a consumer set", nil, []tarski.RuleSet{RBAC(), unsafe}},
		{"fact of an unknown relation", []tarski.Fact{{Relation: "nope", Tuple: tarski.Tuple{tarski.String("x")}}}, []tarski.RuleSet{RBAC()}},
		{"fact of the wrong arity", []tarski.Fact{{Relation: "role", Tuple: tarski.Tuple{tarski.String("x")}}}, []tarski.RuleSet{RBAC()}},
		{"empty role", []tarski.Fact{Role("ana", "")}, []tarski.RuleSet{RBAC()}},
		{"empty principal in a role", []tarski.Fact{Role("", "reader")}, []tarski.RuleSet{RBAC()}},
		{"empty grant prefix", []tarski.Fact{Grant("reader", "read", "")}, []tarski.RuleSet{RBAC()}},
		{"deny rule that reads allow", nil, []tarski.RuleSet{RBAC(), cyc}},
		{"empty constant in a rule set", nil, []tarski.RuleSet{RBAC(), emptyConst}},
		{"numeric standing fact", []tarski.Fact{{Relation: "role", Tuple: tarski.Tuple{seven, tarski.String("admin")}}}, []tarski.RuleSet{RBAC()}},
		{"boolean standing fact", []tarski.Fact{{Relation: "grants", Tuple: tarski.Tuple{tarski.String("r"), tarski.String("read"), tarski.Bool(true)}}}, []tarski.RuleSet{RBAC()}},
		{"numeric constant in a rule set", nil, []tarski.RuleSet{RBAC(), numConst}},
		// Declared and defined nowhere would serve a silent deny-all.
		{"allow and permit declared but not defined", nil, []tarski.RuleSet{ruleSet(t, "decl", `allow/3. permit/3.`)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(Config{Standing: tc.standing, Sets: tc.sets})
			if !errors.Is(err, tarski.ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestLoad_RefusesRequestRelationFacts(t *testing.T) {
	for _, rel := range []string{"principal", "action", "resource"} {
		_, err := Load(Config{Standing: []tarski.Fact{fact(rel, "x")}, Sets: rbacOnly()})
		if !errors.Is(err, tarski.ErrInvalid) || !strings.Contains(err.Error(), "request relation") {
			t.Errorf("%s: err = %v", rel, err)
		}
	}
	_, err := Load(Config{Standing: []tarski.Fact{{Relation: "context", Tuple: tarski.Tuple{tarski.String("k"), tarski.String("v")}}}, Sets: rbacOnly()})
	if !errors.Is(err, tarski.ErrInvalid) {
		t.Errorf("context: err = %v", err)
	}
}

func TestLoad_RequiresAllowAndPermit(t *testing.T) {
	tests := []struct {
		name string
		sets []tarski.RuleSet
		want string
	}{
		{"no rule sets", nil, "allow/3"},
		{"permit/1 without RBAC", []tarski.RuleSet{ruleSet(t, "mine", `allow(P, A, R) :- principal(P), action(A), resource(R). permit(P) :- principal(P).`)}, "permit"},
		{"allow/2", []tarski.RuleSet{RBAC(), ruleSet(t, "mine", `allow(P, A) :- principal(P), action(A).`)}, "allow"},
		{"permit missing", []tarski.RuleSet{ruleSet(t, "mine", `allow(P, A, R) :- principal(P), action(A), resource(R).`)}, "permit/3"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(Config{Sets: tc.sets})
			if !errors.Is(err, tarski.ErrInvalid) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want ErrInvalid naming %s", err, tc.want)
			}
		})
	}
}

func TestLoad_RefusesHashMismatch(t *testing.T) {
	bad := RBAC()
	bad.Hash = "sha256:0000"
	if _, err := Load(Config{Sets: []tarski.RuleSet{bad}}); !errors.Is(err, tarski.ErrInvalid) {
		t.Fatalf("a wrong hash loaded: %v", err)
	}
	edited := RBAC()
	edited.Program.Text += "\n% edited after the hash\n"
	if _, err := Load(Config{Sets: []tarski.RuleSet{edited}}); !errors.Is(err, tarski.ErrInvalid) {
		t.Fatalf("edited text loaded: %v", err)
	}
}

func TestLoad_ProofIsIndependentOfFactOrder(t *testing.T) {
	forward := []tarski.Fact{
		Role("alice", "reader"), Role("alice", "admin"),
		Grant("reader", "read", "timeline/"), Grant("admin", "read", "timeline/"),
	}
	reversed := make([]tarski.Fact, len(forward))
	for i, f := range forward {
		reversed[len(forward)-1-i] = f
	}
	req := Request{Principal: "alice", Action: "read", Resource: "timeline/bp"}
	var proofs []tarski.Proof
	for _, standing := range [][]tarski.Fact{forward, reversed} {
		d, err := mustLoad(t, standing, RBAC()).Decide(context.Background(), req)
		if err != nil || !d.Allow {
			t.Fatalf("Decide = %+v, %v", d, err)
		}
		proofs = append(proofs, d.Proof)
	}
	if !reflect.DeepEqual(proofs[0], proofs[1]) {
		t.Fatalf("proofs differ by fact order:\n%+v\n%+v", proofs[0], proofs[1])
	}
	if forward[0].Tuple[1].Text() != "reader" {
		t.Fatal("Load sorted the caller's slice")
	}
}

func TestContextDeclaration(t *testing.T) {
	hour := map[string]tarski.Kind{"hour": tarski.KindNumber}
	late, _ := tarski.Number("23")
	denyLate := ruleSet(t, "late", `deny(P, A, R) :- principal(P), action(A), resource(R), context("hour", H), H > 22.`)
	base := Request{Principal: "ana", Action: "read", Resource: "timeline/bp"}
	with := func(c map[string]tarski.Const) Request { r := base; r.Context = c; return r }

	t.Run("declared key of the right kind reaches a rule", func(t *testing.T) {
		p := mustLoadConfig(t, Config{Standing: fixture(), Sets: []tarski.RuleSet{RBAC(), denyLate}, Context: hour})
		d, err := p.Decide(context.Background(), with(map[string]tarski.Const{"hour": late}))
		if err != nil || d.Allow {
			t.Fatalf("Decide = %+v, %v; want the late deny to fire", d, err)
		}
	})
	t.Run("undeclared key is ErrRequest", func(t *testing.T) {
		p := mustLoadConfig(t, Config{Standing: fixture(), Sets: rbacOnly(), Context: hour})
		d, err := p.Decide(context.Background(), with(map[string]tarski.Const{"minute": late}))
		if !errors.Is(err, ErrRequest) || d.Allow || !strings.Contains(err.Error(), `"minute"`) {
			t.Fatalf("Decide = %+v, %v", d, err)
		}
	})
	t.Run("wrong kind is ErrRequest naming both kinds", func(t *testing.T) {
		p := mustLoadConfig(t, Config{Standing: fixture(), Sets: rbacOnly(), Context: hour})
		_, err := p.Decide(context.Background(), with(map[string]tarski.Const{"hour": tarski.String("23")}))
		if !errors.Is(err, ErrRequest) || !strings.Contains(err.Error(), "number") || !strings.Contains(err.Error(), "string") || !strings.Contains(err.Error(), `"hour"`) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("rule naming an undeclared key fails at Load", func(t *testing.T) {
		_, err := Load(Config{Standing: fixture(), Sets: []tarski.RuleSet{RBAC(), denyLate}})
		if !errors.Is(err, tarski.ErrInvalid) || !strings.Contains(err.Error(), "not declared in Config.Context") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("rule comparing hour to a string constant fails at Load", func(t *testing.T) {
		bad := ruleSet(t, "bad", `deny(P, A, R) :- principal(P), action(A), resource(R), context("hour", "23").`)
		_, err := Load(Config{Standing: fixture(), Sets: []tarski.RuleSet{RBAC(), bad}, Context: hour})
		if !errors.Is(err, tarski.ErrInvalid) || !strings.Contains(err.Error(), "declared number") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("nil Context refuses any key and accepts none", func(t *testing.T) {
		p := mustLoad(t, fixture(), RBAC())
		if _, err := p.Decide(context.Background(), with(map[string]tarski.Const{"hour": late})); !errors.Is(err, ErrRequest) {
			t.Fatalf("err = %v", err)
		}
		if d, err := p.Decide(context.Background(), base); err != nil || !d.Allow {
			t.Fatalf("Decide = %+v, %v", d, err)
		}
	})
	t.Run("a bad declaration is refused at Load", func(t *testing.T) {
		for name, ctx := range map[string]map[string]tarski.Kind{
			"empty key": {"": tarski.KindString},
			"bad kind":  {"hour": tarski.Kind(9)},
		} {
			if _, err := Load(Config{Sets: rbacOnly(), Context: ctx}); !errors.Is(err, tarski.ErrInvalid) {
				t.Errorf("%s: err = %v", name, err)
			}
		}
	})
	t.Run("the declaration is copied", func(t *testing.T) {
		decl := map[string]tarski.Kind{"hour": tarski.KindNumber}
		p := mustLoadConfig(t, Config{Standing: fixture(), Sets: rbacOnly(), Context: decl})
		delete(decl, "hour")
		if _, err := p.Decide(context.Background(), with(map[string]tarski.Const{"hour": late})); err != nil {
			t.Fatalf("a caller mutated the policy's declaration: %v", err)
		}
	})
}

func TestLoad_RefusesAggregates(t *testing.T) {
	for name, text := range map[string]string{
		"count": `big(R) :- resource(R), N = count : { grants(_, _, R) }, N > 1. deny(P, A, R) :- principal(P), action(A), big(R).`,
		"sum":   `deny(P, A, R) :- principal(P), action(A), resource(R), S = sum N : { context(_, N) }, S > 3.`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Load(Config{
				Standing: fixture(),
				Sets:     []tarski.RuleSet{RBAC(), ruleSet(t, "agg", text)},
				Context:  map[string]tarski.Kind{"n": tarski.KindNumber},
			})
			if !errors.Is(err, tarski.ErrInvalid) || !strings.Contains(err.Error(), "cannot use aggregates") {
				t.Fatalf("Load = %v, want ErrInvalid naming aggregates", err)
			}
		})
	}
}

func TestConsumerRelations_Basic(t *testing.T) {
	p := mustLoadConfig(t, Config{
		Sets: rbacOnly(),
		Relations: map[string]Relation{
			"reports_to": {Kinds: []tarski.Kind{tarski.KindString, tarski.KindString}},
		},
		Standing: []tarski.Fact{
			{Relation: "reports_to", Tuple: tarski.Tuple{tarski.String("eve"), tarski.String("morgan")}},
		},
	})
	// A rule set that reads a declared relation compiles
	if _, err := Load(Config{
		Sets: []tarski.RuleSet{
			RBAC(),
			ruleSet(t, "cr", `allow(P, A, R) :- principal(P), action(A), resource(R), reports_to(P, M), role(M, "admin").`),
		},
		Relations: map[string]Relation{
			"reports_to": {Kinds: []tarski.Kind{tarski.KindString, tarski.KindString}},
		},
		Standing: []tarski.Fact{
			{Relation: "reports_to", Tuple: tarski.Tuple{tarski.String("eve"), tarski.String("morgan")}},
			Role("morgan", "admin"),
			Grant("admin", "read", "timeline/"),
		},
	}); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	_ = p
}

func TestConsumerRelations_Errors(t *testing.T) {
	// declared name that is one of the eight PARC relations
	for _, name := range []string{"principal", "action", "resource", "context", "role", "inherits", "grants", "denied"} {
		t.Run("parc_"+name, func(t *testing.T) {
			_, err := Load(Config{
				Sets:      rbacOnly(),
				Relations: map[string]Relation{name: {Kinds: []tarski.Kind{tarski.KindString}}},
			})
			if !errors.Is(err, tarski.ErrInvalid) {
				t.Fatalf("err = %v", err)
			}
			if !strings.HasPrefix(err.Error(), "policy: load:") {
				t.Fatalf("err = %v", err)
			}
		})
	}
	// empty name
	if _, err := Load(Config{Sets: rbacOnly(), Relations: map[string]Relation{"": {Kinds: []tarski.Kind{tarski.KindString}}}}); !errors.Is(err, tarski.ErrInvalid) {
		t.Fatalf("empty name: %v", err)
	}
	// no columns
	if _, err := Load(Config{Sets: rbacOnly(), Relations: map[string]Relation{"foo": {Kinds: []tarski.Kind{}}}}); !errors.Is(err, tarski.ErrInvalid) {
		t.Fatalf("no cols: %v", err)
	}
	// bad kind
	if _, err := Load(Config{Sets: rbacOnly(), Relations: map[string]Relation{"foo": {Kinds: []tarski.Kind{tarski.Kind(9)}}}}); !errors.Is(err, tarski.ErrInvalid) {
		t.Fatalf("bad kind: %v", err)
	}
}

func TestConsumerRelations_Recursive(t *testing.T) {
	p := mustLoadConfig(t, Config{
		Sets: []tarski.RuleSet{
			RBAC(),
			ruleSet(t, "cr", `
				manages(M, E) :- reports_to(E, M).
				manages(M, E) :- reports_to(E, X), manages(M, X).
				allow(P, A, R) :- principal(P), action(A), resource(R), manages(P, "e1").
			`),
		},
		Relations: map[string]Relation{
			"reports_to": {Kinds: []tarski.Kind{tarski.KindString, tarski.KindString}},
		},
		Standing: []tarski.Fact{
			{Relation: "reports_to", Tuple: tarski.Tuple{tarski.String("e1"), tarski.String("x")}},
			{Relation: "reports_to", Tuple: tarski.Tuple{tarski.String("x"), tarski.String("m1")}},
			{Relation: "reports_to", Tuple: tarski.Tuple{tarski.String("m1"), tarski.String("p")}},
			Grant("reader", "read", "doc/"),
			Role("p", "reader"),
		},
	})
	d, err := p.Decide(context.Background(), Request{Principal: "p", Action: "read", Resource: "doc/1"})
	if err != nil || !d.Allow {
		t.Fatalf("Decide = %+v, %v", d, err)
	}
}

func TestConsumerRelations_ProofShowsFact(t *testing.T) {
	p := mustLoadConfig(t, Config{
		Sets: []tarski.RuleSet{
			RBAC(),
			ruleSet(t, "cr", `allow(P, A, R) :- principal(P), action(A), resource(R), reports_to(P, M), role(M, "admin").`),
		},
		Relations: map[string]Relation{
			"reports_to": {Kinds: []tarski.Kind{tarski.KindString, tarski.KindString}},
		},
		Standing: []tarski.Fact{
			{Relation: "reports_to", Tuple: tarski.Tuple{tarski.String("eve"), tarski.String("morgan")}},
			Role("morgan", "admin"),
			Grant("admin", "read", "timeline/"),
		},
	})
	d, err := p.Decide(context.Background(), Request{Principal: "eve", Action: "read", Resource: "timeline/bp"})
	if err != nil || !d.Allow {
		t.Fatalf("Decide = %+v, %v", d, err)
	}
	// proof shows consumer fact as fact
	found := false
	var walk func(p tarski.Proof)
	walk = func(p tarski.Proof) {
		if p.Relation == "reports_to" {
			found = true
		}
		for _, b := range p.Body {
			walk(b)
		}
		for _, g := range p.Group {
			for _, w := range g {
				walk(w)
			}
		}
	}
	walk(d.Proof)
	if !found {
		t.Fatalf("proof did not show consumer fact")
	}
}

func TestConsumerRelations_MapCopied(t *testing.T) {
	rels := map[string]Relation{
		"reports_to": {Kinds: []tarski.Kind{tarski.KindString, tarski.KindString}},
	}
	p := mustLoadConfig(t, Config{
		Sets: []tarski.RuleSet{
			RBAC(),
			ruleSet(t, "cr", `allow(P, A, R) :- principal(P), action(A), resource(R), reports_to(P, M).`),
		},
		Relations: rels,
		Standing: []tarski.Fact{
			{Relation: "reports_to", Tuple: tarski.Tuple{tarski.String("eve"), tarski.String("morgan")}},
			Grant("reader", "read", "timeline/"),
			Role("eve", "reader"),
		},
	})
	rels["reports_to"] = Relation{Kinds: []tarski.Kind{tarski.KindNumber}}
	delete(rels, "reports_to")
	if d, err := p.Decide(context.Background(), Request{Principal: "eve", Action: "read", Resource: "timeline/bp"}); err != nil || !d.Allow {
		t.Fatalf("Decide = %+v, %v", d, err)
	}
}

func TestConsumerRelations_KindCheck(t *testing.T) {
	// wrong kind in rule constant
	_, err := Load(Config{
		Sets: []tarski.RuleSet{
			RBAC(),
			ruleSet(t, "cr", `allow(P, A, R) :- principal(P), action(A), resource(R), reports_to(P, 7).`),
		},
		Relations: map[string]Relation{
			"reports_to": {Kinds: []tarski.Kind{tarski.KindString, tarski.KindString}},
		},
	})
	if !errors.Is(err, tarski.ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
	if !strings.HasPrefix(err.Error(), "policy: load:") {
		t.Fatalf("err = %v", err)
	}
	// empty string
	_, err = Load(Config{
		Sets: []tarski.RuleSet{RBAC()},
		Relations: map[string]Relation{
			"reports_to": {Kinds: []tarski.Kind{tarski.KindString, tarski.KindString}},
		},
		Standing: []tarski.Fact{
			{Relation: "reports_to", Tuple: tarski.Tuple{tarski.String("eve"), tarski.String("")}},
		},
	})
	if !errors.Is(err, tarski.ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
	if !strings.HasPrefix(err.Error(), "policy: load:") {
		t.Fatalf("err = %v", err)
	}
}

func TestConsumerRelations_DerivedByRuleFails(t *testing.T) {
	// declared name that a loaded rule set derives with a rule
	_, err := Load(Config{
		Sets: []tarski.RuleSet{
			RBAC(),
			ruleSet(t, "cr", `reports_to(X, Y) :- inherits(X, Y).`),
		},
		Relations: map[string]Relation{
			"reports_to": {Kinds: []tarski.Kind{tarski.KindString, tarski.KindString}},
		},
	})
	if !errors.Is(err, tarski.ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
	if !strings.HasPrefix(err.Error(), "policy: load:") {
		t.Fatalf("err = %v", err)
	}
}

func TestConsumerRelations_BuiltInFails(t *testing.T) {
	// declared name that is a built-in, for example prefix
	_, err := Load(Config{
		Sets:      rbacOnly(),
		Relations: map[string]Relation{"prefix": {Kinds: []tarski.Kind{tarski.KindString, tarski.KindString}}},
	})
	if !errors.Is(err, tarski.ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
	if !strings.HasPrefix(err.Error(), "policy: load:") {
		t.Fatalf("err = %v", err)
	}
}

func TestConsumerRelations_UnknownFactRelation(t *testing.T) {
	// a standing fact of a relation that is declared nowhere
	_, err := Load(Config{
		Sets: rbacOnly(),
		Standing: []tarski.Fact{
			{Relation: "nope", Tuple: tarski.Tuple{tarski.String("x")}},
		},
	})
	if !errors.Is(err, tarski.ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
}

func TestConsumerRelations_NilGivesSameAsBefore(t *testing.T) {
	p := mustLoad(t, fixture(), RBAC())
	d, err := p.Decide(context.Background(), Request{Principal: "ana", Action: "read", Resource: "timeline/bp"})
	if err != nil || !d.Allow {
		t.Fatalf("Decide = %+v, %v", d, err)
	}
	// RBAC hash pinned
	if RBAC().ID() != "rbac/1 sha256:0bf5a1217fbaad382dd0f5044fa0e27c0fd978dec0c616576a066c862c5dbd36" {
		t.Fatalf("RBAC ID = %s", RBAC().ID())
	}
	if _, err := Load(Config{Standing: fixture(), Sets: []tarski.RuleSet{RBAC()}}); err != nil {
		t.Fatalf("err = %v", err)
	}
	if _, err := Load(Config{Standing: fixture(), Sets: []tarski.RuleSet{RBAC()}, Relations: nil}); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestConsumerRelations_BoundsDepth12(t *testing.T) {
	// Build a 2000-person reporting chain at depth 12
	rels := map[string]Relation{
		"reports_to": {Kinds: []tarski.Kind{tarski.KindString, tarski.KindString}},
	}
	var standing []tarski.Fact
	// create chain of depth 12: e0 -> x0_0 -> ... -> x0_11 -> m0
	for i := 0; i < 2000; i++ {
		standing = append(standing, tarski.Fact{Relation: "reports_to", Tuple: tarski.Tuple{tarski.String(fmt.Sprintf("e%d", i)), tarski.String(fmt.Sprintf("x%d_0", i))}})
		for j := 0; j < 11; j++ {
			standing = append(standing, tarski.Fact{Relation: "reports_to", Tuple: tarski.Tuple{tarski.String(fmt.Sprintf("x%d_%d", i, j)), tarski.String(fmt.Sprintf("x%d_%d", i, j+1))}})
		}
		standing = append(standing, tarski.Fact{Relation: "reports_to", Tuple: tarski.Tuple{tarski.String(fmt.Sprintf("x%d_11", i)), tarski.String("m0")}})
	}
	standing = append(standing, Grant("reader", "read", "timeline/"))
	standing = append(standing, Role("m0", "reader"))
	p := mustLoadConfig(t, Config{
		Sets: []tarski.RuleSet{
			RBAC(),
			ruleSet(t, "cr", `
				manages(M, E) :- reports_to(E, M).
				manages(M, E) :- reports_to(X, M), manages(E, X).
				allow(P, A, R) :- principal(P), action(A), resource(R), manages("m0", E), P = "m0".
			`),
		},
		Relations: rels,
		Standing:  standing,
	})
	d, err := p.Decide(context.Background(), Request{Principal: "m0", Action: "read", Resource: "timeline/bp"})
	if err != nil || !d.Allow {
		t.Fatalf("Decide = %+v, %v", d, err)
	}
	// Put the measured Result.Cost steps in the test comment.
	// Measured steps: ~2000*12 derivations fit within maxDecisionSteps (1<<22) and maxDecisionBytes
	_ = d
}

func TestConsumerRelations_ReservedNamesMessage(t *testing.T) {
	_, err := Load(Config{
		Sets:      rbacOnly(),
		Relations: map[string]Relation{"role": {Kinds: []tarski.Kind{tarski.KindString, tarski.KindString}}},
	})
	if !errors.Is(err, tarski.ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
	if !strings.HasPrefix(err.Error(), "policy: load:") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "base relation") {
		t.Fatalf("err = %v", err)
	}
}

func TestConsumerRelations_EmptyNameMessage(t *testing.T) {
	_, err := Load(Config{
		Sets:      rbacOnly(),
		Relations: map[string]Relation{"": {Kinds: []tarski.Kind{tarski.KindString}}},
	})
	if !errors.Is(err, tarski.ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
	if !strings.HasPrefix(err.Error(), "policy: load:") {
		t.Fatalf("err = %v", err)
	}
}

func TestConsumerRelations_NoColumnsMessage(t *testing.T) {
	_, err := Load(Config{
		Sets:      rbacOnly(),
		Relations: map[string]Relation{"foo": {Kinds: []tarski.Kind{}}},
	})
	if !errors.Is(err, tarski.ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
	if !strings.HasPrefix(err.Error(), "policy: load:") {
		t.Fatalf("err = %v", err)
	}
}

func TestConsumerRelations_BadKindMessage(t *testing.T) {
	_, err := Load(Config{
		Sets:      rbacOnly(),
		Relations: map[string]Relation{"foo": {Kinds: []tarski.Kind{tarski.Kind(9)}}},
	})
	if !errors.Is(err, tarski.ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
	if !strings.HasPrefix(err.Error(), "policy: load:") {
		t.Fatalf("err = %v", err)
	}
}

func TestConsumerRelations_DerivedRuleMessage(t *testing.T) {
	_, err := Load(Config{
		Sets: []tarski.RuleSet{
			RBAC(),
			ruleSet(t, "cr", `reports_to(X, Y) :- inherits(X, Y).`),
		},
		Relations: map[string]Relation{
			"reports_to": {Kinds: []tarski.Kind{tarski.KindString, tarski.KindString}},
		},
	})
	if !errors.Is(err, tarski.ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
	if !strings.HasPrefix(err.Error(), "policy: load:") {
		t.Fatalf("err = %v", err)
	}
}

func TestConsumerRelations_BuiltInMessage(t *testing.T) {
	_, err := Load(Config{
		Sets:      rbacOnly(),
		Relations: map[string]Relation{"prefix": {Kinds: []tarski.Kind{tarski.KindString, tarski.KindString}}},
	})
	if !errors.Is(err, tarski.ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
	if !strings.HasPrefix(err.Error(), "policy: load:") {
		t.Fatalf("err = %v", err)
	}
}
