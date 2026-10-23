package policy

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mattjoyce/shearline/datalog"
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
	if _, err := Load(Config{Standing: []datalog.Fact{Grant("r", "read", "")}, Sets: rbacOnly()}); !errors.Is(err, datalog.ErrInvalid) {
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
	seven, err := datalog.Number("7")
	if err != nil {
		t.Fatal(err)
	}
	numConst := ruleSet(t, "num", `deny(P, A, R) :- principal(P), action(A), resource(R), role(7, P).`)
	cyc := ruleSet(t, "cyc", `deny(P, A, R) :- allow(P, A, R).`)
	emptyConst := ruleSet(t, "empty", `deny(P, A, R) :- principal(P), action(A), resource(R), P = "", role(P, "").`)
	tests := []struct {
		name     string
		standing []datalog.Fact
		sets     []datalog.RuleSet
	}{
		{"unsafe variable in a consumer set", nil, []datalog.RuleSet{RBAC(), unsafe}},
		{"fact of an unknown relation", []datalog.Fact{{Relation: "nope", Tuple: datalog.Tuple{datalog.String("x")}}}, []datalog.RuleSet{RBAC()}},
		{"fact of the wrong arity", []datalog.Fact{{Relation: "role", Tuple: datalog.Tuple{datalog.String("x")}}}, []datalog.RuleSet{RBAC()}},
		{"empty role", []datalog.Fact{Role("ana", "")}, []datalog.RuleSet{RBAC()}},
		{"empty principal in a role", []datalog.Fact{Role("", "reader")}, []datalog.RuleSet{RBAC()}},
		{"empty grant prefix", []datalog.Fact{Grant("reader", "read", "")}, []datalog.RuleSet{RBAC()}},
		{"deny rule that reads allow", nil, []datalog.RuleSet{RBAC(), cyc}},
		{"empty constant in a rule set", nil, []datalog.RuleSet{RBAC(), emptyConst}},
		{"numeric standing fact", []datalog.Fact{{Relation: "role", Tuple: datalog.Tuple{seven, datalog.String("admin")}}}, []datalog.RuleSet{RBAC()}},
		{"boolean standing fact", []datalog.Fact{{Relation: "grants", Tuple: datalog.Tuple{datalog.String("r"), datalog.String("read"), datalog.Bool(true)}}}, []datalog.RuleSet{RBAC()}},
		{"numeric constant in a rule set", nil, []datalog.RuleSet{RBAC(), numConst}},
		// Declared and defined nowhere would serve a silent deny-all.
		{"allow and permit declared but not defined", nil, []datalog.RuleSet{ruleSet(t, "decl", `allow/3. permit/3.`)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(Config{Standing: tc.standing, Sets: tc.sets})
			if !errors.Is(err, datalog.ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestLoad_RefusesRequestRelationFacts(t *testing.T) {
	for _, rel := range []string{"principal", "action", "resource"} {
		_, err := Load(Config{Standing: []datalog.Fact{fact(rel, "x")}, Sets: rbacOnly()})
		if !errors.Is(err, datalog.ErrInvalid) || !strings.Contains(err.Error(), "request relation") {
			t.Errorf("%s: err = %v", rel, err)
		}
	}
	_, err := Load(Config{Standing: []datalog.Fact{{Relation: "context", Tuple: datalog.Tuple{datalog.String("k"), datalog.String("v")}}}, Sets: rbacOnly()})
	if !errors.Is(err, datalog.ErrInvalid) {
		t.Errorf("context: err = %v", err)
	}
}

func TestLoad_RequiresAllowAndPermit(t *testing.T) {
	tests := []struct {
		name string
		sets []datalog.RuleSet
		want string
	}{
		{"no rule sets", nil, "allow/3"},
		{"permit/1 without RBAC", []datalog.RuleSet{ruleSet(t, "mine", `allow(P, A, R) :- principal(P), action(A), resource(R). permit(P) :- principal(P).`)}, "permit"},
		{"allow/2", []datalog.RuleSet{RBAC(), ruleSet(t, "mine", `allow(P, A) :- principal(P), action(A).`)}, "allow"},
		{"permit missing", []datalog.RuleSet{ruleSet(t, "mine", `allow(P, A, R) :- principal(P), action(A), resource(R).`)}, "permit/3"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(Config{Sets: tc.sets})
			if !errors.Is(err, datalog.ErrInvalid) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want ErrInvalid naming %s", err, tc.want)
			}
		})
	}
}

func TestLoad_RefusesHashMismatch(t *testing.T) {
	bad := RBAC()
	bad.Hash = "sha256:0000"
	if _, err := Load(Config{Sets: []datalog.RuleSet{bad}}); !errors.Is(err, datalog.ErrInvalid) {
		t.Fatalf("a wrong hash loaded: %v", err)
	}
	edited := RBAC()
	edited.Program.Text += "\n% edited after the hash\n"
	if _, err := Load(Config{Sets: []datalog.RuleSet{edited}}); !errors.Is(err, datalog.ErrInvalid) {
		t.Fatalf("edited text loaded: %v", err)
	}
}

func TestLoad_ProofIsIndependentOfFactOrder(t *testing.T) {
	forward := []datalog.Fact{
		Role("alice", "reader"), Role("alice", "admin"),
		Grant("reader", "read", "timeline/"), Grant("admin", "read", "timeline/"),
	}
	reversed := make([]datalog.Fact, len(forward))
	for i, f := range forward {
		reversed[len(forward)-1-i] = f
	}
	req := Request{Principal: "alice", Action: "read", Resource: "timeline/bp"}
	var proofs []datalog.Proof
	for _, standing := range [][]datalog.Fact{forward, reversed} {
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
	hour := map[string]datalog.Kind{"hour": datalog.KindNumber}
	late, _ := datalog.Number("23")
	denyLate := ruleSet(t, "late", `deny(P, A, R) :- principal(P), action(A), resource(R), context("hour", H), H > 22.`)
	base := Request{Principal: "ana", Action: "read", Resource: "timeline/bp"}
	with := func(c map[string]datalog.Const) Request { r := base; r.Context = c; return r }

	t.Run("declared key of the right kind reaches a rule", func(t *testing.T) {
		p := mustLoadConfig(t, Config{Standing: fixture(), Sets: []datalog.RuleSet{RBAC(), denyLate}, Context: hour})
		d, err := p.Decide(context.Background(), with(map[string]datalog.Const{"hour": late}))
		if err != nil || d.Allow {
			t.Fatalf("Decide = %+v, %v; want the late deny to fire", d, err)
		}
	})
	t.Run("undeclared key is ErrRequest", func(t *testing.T) {
		p := mustLoadConfig(t, Config{Standing: fixture(), Sets: rbacOnly(), Context: hour})
		d, err := p.Decide(context.Background(), with(map[string]datalog.Const{"minute": late}))
		if !errors.Is(err, ErrRequest) || d.Allow || !strings.Contains(err.Error(), `"minute"`) {
			t.Fatalf("Decide = %+v, %v", d, err)
		}
	})
	t.Run("wrong kind is ErrRequest naming both kinds", func(t *testing.T) {
		p := mustLoadConfig(t, Config{Standing: fixture(), Sets: rbacOnly(), Context: hour})
		_, err := p.Decide(context.Background(), with(map[string]datalog.Const{"hour": datalog.String("23")}))
		if !errors.Is(err, ErrRequest) || !strings.Contains(err.Error(), "number") || !strings.Contains(err.Error(), "string") || !strings.Contains(err.Error(), `"hour"`) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("rule naming an undeclared key fails at Load", func(t *testing.T) {
		_, err := Load(Config{Standing: fixture(), Sets: []datalog.RuleSet{RBAC(), denyLate}})
		if !errors.Is(err, datalog.ErrInvalid) || !strings.Contains(err.Error(), "not declared in Config.Context") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("rule comparing hour to a string constant fails at Load", func(t *testing.T) {
		bad := ruleSet(t, "bad", `deny(P, A, R) :- principal(P), action(A), resource(R), context("hour", "23").`)
		_, err := Load(Config{Standing: fixture(), Sets: []datalog.RuleSet{RBAC(), bad}, Context: hour})
		if !errors.Is(err, datalog.ErrInvalid) || !strings.Contains(err.Error(), "declared number") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("nil Context refuses any key and accepts none", func(t *testing.T) {
		p := mustLoad(t, fixture(), RBAC())
		if _, err := p.Decide(context.Background(), with(map[string]datalog.Const{"hour": late})); !errors.Is(err, ErrRequest) {
			t.Fatalf("err = %v", err)
		}
		if d, err := p.Decide(context.Background(), base); err != nil || !d.Allow {
			t.Fatalf("Decide = %+v, %v", d, err)
		}
	})
	t.Run("a bad declaration is refused at Load", func(t *testing.T) {
		for name, ctx := range map[string]map[string]datalog.Kind{
			"empty key": {"": datalog.KindString},
			"bad kind":  {"hour": datalog.Kind(9)},
		} {
			if _, err := Load(Config{Sets: rbacOnly(), Context: ctx}); !errors.Is(err, datalog.ErrInvalid) {
				t.Errorf("%s: err = %v", name, err)
			}
		}
	})
	t.Run("the declaration is copied", func(t *testing.T) {
		decl := map[string]datalog.Kind{"hour": datalog.KindNumber}
		p := mustLoadConfig(t, Config{Standing: fixture(), Sets: rbacOnly(), Context: decl})
		delete(decl, "hour")
		if _, err := p.Decide(context.Background(), with(map[string]datalog.Const{"hour": late})); err != nil {
			t.Fatalf("a caller mutated the policy's declaration: %v", err)
		}
	})
}
