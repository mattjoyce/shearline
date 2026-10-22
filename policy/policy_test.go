package policy

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/mattjoyce/shearline/datalog"
)

// fixture: ana is an admin, admin inherits editor, editor inherits reader.
func fixture() []datalog.Fact {
	return []datalog.Fact{
		Role("ana", "admin"),
		Role("bo", "reader"),
		Inherits("admin", "editor"),
		Inherits("editor", "reader"),
		Grant("reader", "read", "timeline/"),
		Grant("reader", "read", "docs/"),
		Grant("editor", "read", "docs/"), // same prefix by a second route
		Grant("editor", "write", "timeline/"),
		Grant("admin", "delete", "timeline/"),
	}
}

func mustLoad(t *testing.T, standing []datalog.Fact, sets ...datalog.RuleSet) Policy {
	t.Helper()
	p, err := Load(standing, sets...)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return p
}

func ruleSet(t *testing.T, name, text string) datalog.RuleSet {
	t.Helper()
	s, err := datalog.NewRuleSet(name, 1, text)
	if err != nil {
		t.Fatalf("NewRuleSet: %v", err)
	}
	return s
}

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

func TestDecide(t *testing.T) {
	denyLate := ruleSet(t, "house", `deny(P, A, R) :- principal(P), action(A), resource(R), context("hour", H), H > 22.`)
	late, _ := datalog.Number("23")
	noon, _ := datalog.Number("12")
	denyFact := []datalog.Fact{Deny("bo", "read", "timeline/secret")}

	tests := []struct {
		name     string
		standing []datalog.Fact
		sets     []datalog.RuleSet
		req      Request
		want     bool
	}{
		{"direct grant allows", fixture(), nil, Request{Principal: "bo", Action: "read", Resource: "timeline/bp"}, true},
		{"inherited two levels allows", fixture(), nil, Request{Principal: "ana", Action: "read", Resource: "timeline/bp"}, true},
		{"inherited one level allows", fixture(), nil, Request{Principal: "ana", Action: "write", Resource: "timeline/bp"}, true},
		{"wrong action denies", fixture(), nil, Request{Principal: "bo", Action: "write", Resource: "timeline/bp"}, false},
		{"resource outside every prefix denies", fixture(), nil, Request{Principal: "ana", Action: "read", Resource: "secrets/x"}, false},
		{"unknown principal denies", fixture(), nil, Request{Principal: "zed", Action: "read", Resource: "timeline/bp"}, false},
		{"consumer deny overrides allow", fixture(), []datalog.RuleSet{denyLate}, Request{Principal: "ana", Action: "read", Resource: "timeline/bp", Context: map[string]datalog.Const{"hour": late}}, false},
		{"consumer deny silent when it does not match", fixture(), []datalog.RuleSet{denyLate}, Request{Principal: "ana", Action: "read", Resource: "timeline/bp", Context: map[string]datalog.Const{"hour": noon}}, true},
		{"consumer deny silent without the context key", fixture(), []datalog.RuleSet{denyLate}, Request{Principal: "ana", Action: "read", Resource: "timeline/bp"}, true},
		{"Deny fact overrides allow", append(fixture(), denyFact...), nil, Request{Principal: "bo", Action: "read", Resource: "timeline/secret"}, false},
		{"Deny fact leaves other resources", append(fixture(), denyFact...), nil, Request{Principal: "bo", Action: "read", Resource: "timeline/bp"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sets := append([]datalog.RuleSet{RBAC()}, tc.sets...)
			d, err := mustLoad(t, tc.standing, sets...).Decide(context.Background(), tc.req)
			if err != nil {
				t.Fatalf("Decide: %v", err)
			}
			if d.Allow != tc.want {
				t.Fatalf("Allow = %v, want %v", d.Allow, tc.want)
			}
			if len(d.RuleSets) != len(sets) || !strings.HasPrefix(d.RuleSets[0], "rbac/1 sha256:") {
				t.Fatalf("RuleSets = %v", d.RuleSets)
			}
			if !d.Allow && !reflect.DeepEqual(d.Proof, datalog.Proof{}) {
				t.Fatalf("a deny carries a proof: %+v", d.Proof)
			}
		})
	}
}

func TestDecide_EmptyRequestFieldsFailClosed(t *testing.T) {
	tests := []struct {
		name string
		req  Request
	}{
		{"empty principal", Request{Action: "read", Resource: "timeline/bp"}},
		{"empty action", Request{Principal: "ana", Resource: "timeline/bp"}},
		{"empty resource", Request{Principal: "ana", Action: "read"}},
		{"empty context key", Request{Principal: "ana", Action: "read", Resource: "timeline/bp", Context: map[string]datalog.Const{"": datalog.String("x")}}},
	}
	p := mustLoad(t, fixture(), RBAC())
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d, err := p.Decide(context.Background(), tc.req)
			if !errors.Is(err, datalog.ErrInvalid) || d.Allow {
				t.Fatalf("Decide = %+v, %v; want a deny with ErrInvalid", d, err)
			}
		})
	}
}

func TestDecide_EmptyContextValueIsAllowed(t *testing.T) {
	p := mustLoad(t, fixture(), RBAC())
	d, err := p.Decide(context.Background(), Request{Principal: "ana", Action: "read", Resource: "timeline/bp",
		Context: map[string]datalog.Const{"note": datalog.String("")}})
	if err != nil || !d.Allow {
		t.Fatalf("Decide = %+v, %v", d, err)
	}
}

func TestDecide_RuleSetsIsACopy(t *testing.T) {
	p := mustLoad(t, fixture(), RBAC())
	d, _ := p.Decide(context.Background(), Request{Principal: "ana", Action: "read", Resource: "timeline/bp"})
	d.RuleSets[0] = "tampered"
	d2, _ := p.Decide(context.Background(), Request{Principal: "ana", Action: "read", Resource: "timeline/bp"})
	if d2.RuleSets[0] == "tampered" {
		t.Fatal("a caller mutated the policy through Decision.RuleSets")
	}
}

func TestDecide_ProofOnAllow(t *testing.T) {
	d, err := mustLoad(t, fixture(), RBAC()).Decide(context.Background(),
		Request{Principal: "ana", Action: "read", Resource: "timeline/bp"})
	if err != nil || !d.Allow {
		t.Fatalf("Decide = %+v, %v", d, err)
	}
	if d.Proof.Relation != "allow" || d.Proof.Rule == nil || len(d.Proof.Body) == 0 {
		t.Fatalf("proof = %+v", d.Proof)
	}
	leaves := map[string]bool{}
	var walk func(datalog.Proof)
	walk = func(p datalog.Proof) {
		if p.Rule == nil {
			leaves[p.Relation] = true
			return
		}
		if len(p.Body) == 0 {
			t.Errorf("derived tuple %s has no body", p.Relation)
		}
		for _, b := range p.Body {
			walk(b)
		}
	}
	walk(d.Proof)
	for _, want := range []string{"principal", "grants", "role"} {
		if !leaves[want] {
			t.Errorf("proof leaves %v lack a %s fact", leaves, want)
		}
	}
}

func TestLoad_RBACAloneSucceeds(t *testing.T) {
	// deny/3 is declared and undefined: the empty relation, not a compile error.
	if _, err := Load(nil, RBAC()); err != nil {
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
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(tc.standing, tc.sets...)
			if !errors.Is(err, datalog.ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestPermitted(t *testing.T) {
	tests := []struct {
		name              string
		principal, action string
		want              []string
	}{
		{"deduped across inherited roles, sorted", "ana", "read", []string{"docs/", "timeline/"}},
		{"direct role", "bo", "read", []string{"docs/", "timeline/"}},
		{"inherited action", "ana", "write", []string{"timeline/"}},
		{"action not granted", "bo", "delete", []string{}},
		{"unknown principal", "zed", "read", []string{}},
	}
	p := mustLoad(t, fixture(), RBAC())
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := p.Permitted(context.Background(), tc.principal, tc.action)
			if err != nil {
				t.Fatalf("Permitted: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPermitted_IgnoresDeny(t *testing.T) {
	p := mustLoad(t, append(fixture(), Deny("bo", "read", "timeline/secret")), RBAC())
	got, err := p.Permitted(context.Background(), "bo", "read")
	if err != nil || !reflect.DeepEqual(got, []string{"docs/", "timeline/"}) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestDecide_AndPermittedAgree(t *testing.T) {
	p := mustLoad(t, fixture(), RBAC())
	prefixes, err := p.Permitted(context.Background(), "ana", "read")
	if err != nil {
		t.Fatal(err)
	}
	for _, pre := range prefixes {
		d, err := p.Decide(context.Background(), Request{Principal: "ana", Action: "read", Resource: pre + "x"})
		if err != nil || !d.Allow {
			t.Fatalf("listed prefix %q but Decide = %+v, %v", pre, d, err)
		}
	}
}

func TestDecide_CancelledContextDenies(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := mustLoad(t, fixture(), RBAC())
	d, err := p.Decide(ctx, Request{Principal: "ana", Action: "read", Resource: "timeline/bp"})
	if err == nil || d.Allow {
		t.Fatalf("Decide on a cancelled context = %+v, %v; want a deny with an error", d, err)
	}
}

func TestEvalError_WrapsTooLarge(t *testing.T) {
	err := evalError("decide", &datalog.TooLarge{Relation: "holds", MaxBytes: 1})
	if !errors.Is(err, datalog.ErrTooLarge) || !strings.Contains(err.Error(), "narrow the grants reachable from this principal") {
		t.Fatalf("err = %v", err)
	}
	var tl *datalog.TooLarge
	if !errors.As(err, &tl) {
		t.Fatal("the *TooLarge is lost in the wrap")
	}
	plain := errors.New("boom")
	if got := evalError("decide", plain); !errors.Is(got, plain) || strings.Contains(got.Error(), "narrow") {
		t.Fatalf("plain error wrapped wrongly: %v", got)
	}
}

func TestDecide_ConcurrentSafe(t *testing.T) {
	p := mustLoad(t, fixture(), RBAC())
	var wg sync.WaitGroup
	errs := make(chan string, 8)
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				d, err := p.Decide(context.Background(), Request{Principal: "ana", Action: "read", Resource: "timeline/bp"})
				if err != nil || !d.Allow {
					errs <- "allow lost under concurrency"
					return
				}
				if _, err := p.Permitted(context.Background(), "ana", "read"); err != nil {
					errs <- err.Error()
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

func TestRequestFacts_ContextSorted(t *testing.T) {
	one, _ := datalog.Number("1")
	r := Request{Principal: "p", Action: "a", Resource: "r", Context: map[string]datalog.Const{"b": one, "a": one, "c": one}}
	fs := r.facts()
	var keys []string
	for _, f := range fs[3:] {
		keys = append(keys, f.Tuple[0].Text())
	}
	if !reflect.DeepEqual(keys, []string{"a", "b", "c"}) {
		t.Fatalf("context keys = %v", keys)
	}
}

func TestLoad_RefusesRequestRelationFacts(t *testing.T) {
	for _, rel := range []string{"principal", "action", "resource"} {
		_, err := Load([]datalog.Fact{fact(rel, "x")}, RBAC())
		if !errors.Is(err, datalog.ErrInvalid) || !strings.Contains(err.Error(), "request relation") {
			t.Errorf("%s: err = %v", rel, err)
		}
	}
	_, err := Load([]datalog.Fact{{Relation: "context", Tuple: datalog.Tuple{datalog.String("k"), datalog.String("v")}}}, RBAC())
	if !errors.Is(err, datalog.ErrInvalid) {
		t.Errorf("context: err = %v", err)
	}
}

func TestPermitted_OnlyTheCallersTuples(t *testing.T) {
	// A rule set may derive permit tuples for any principal; the listing keeps the caller's.
	stray := ruleSet(t, "stray", `permit("other", "read", "secret/"). permit("bo", "write", "elsewhere/").`)
	p := mustLoad(t, fixture(), RBAC(), stray)
	got, err := p.Permitted(context.Background(), "bo", "read")
	if err != nil || !reflect.DeepEqual(got, []string{"docs/", "timeline/"}) {
		t.Fatalf("got %v, %v", got, err)
	}
}
