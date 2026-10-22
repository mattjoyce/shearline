package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

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
	return mustLoadConfig(t, Config{Standing: standing, Sets: sets})
}

func mustLoadConfig(t *testing.T, cfg Config) Policy {
	t.Helper()
	p, err := Load(cfg)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return p
}

func rbacOnly() []datalog.RuleSet { return []datalog.RuleSet{RBAC()} }

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
			p := mustLoadConfig(t, Config{Standing: tc.standing, Sets: sets, Context: map[string]datalog.Kind{"hour": datalog.KindNumber}})
			d, err := p.Decide(context.Background(), tc.req)
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
			if !errors.Is(err, ErrRequest) || d.Allow {
				t.Fatalf("Decide = %+v, %v; want a deny with ErrRequest", d, err)
			}
		})
	}
}

func TestDecide_EmptyContextValueIsAllowed(t *testing.T) {
	p := mustLoadConfig(t, Config{Standing: fixture(), Sets: rbacOnly(), Context: map[string]datalog.Kind{"note": datalog.KindString}})
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

func TestPermitted_OnlyTheCallersTuples(t *testing.T) {
	// A rule set may derive permit tuples for any principal; the listing keeps the caller's.
	stray := ruleSet(t, "stray", `permit("other", "read", "secret/"). permit("bo", "write", "elsewhere/").`)
	p := mustLoad(t, fixture(), RBAC(), stray)
	got, err := p.Permitted(context.Background(), "bo", "read")
	if err != nil || !reflect.DeepEqual(got, []string{"docs/", "timeline/"}) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestDecide_ScaleManyPrincipals(t *testing.T) {
	standing := []datalog.Fact{Grant("reader", "read", "timeline/")}
	for i := 0; i < 50000; i++ {
		standing = append(standing, Role(fmt.Sprintf("principal-%05d", i), "reader"))
	}
	p := mustLoad(t, standing, RBAC())
	d, err := p.Decide(context.Background(), Request{Principal: "principal-00001", Action: "read", Resource: "timeline/bp"})
	if err != nil || !d.Allow {
		t.Fatalf("Decide = %+v, %v", d, err)
	}
	got, err := p.Permitted(context.Background(), "principal-00001", "read")
	if err != nil || !reflect.DeepEqual(got, []string{"timeline/"}) {
		t.Fatalf("Permitted = %v, %v", got, err)
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

func TestPermitted_ThirdColumnContract(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"number", `permit(P, A, 7) :- principal(P), action(A).`},
		{"empty string", `permit(P, A, "") :- principal(P), action(A).`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := mustLoad(t, fixture(), RBAC(), ruleSet(t, "bad", tc.text))
			got, err := p.Permitted(context.Background(), "bo", "read")
			if !errors.Is(err, datalog.ErrInvalid) || got != nil {
				t.Fatalf("got %v, %v; want nil and ErrInvalid", got, err)
			}
		})
	}
}

func TestErrRequest(t *testing.T) {
	p := mustLoad(t, fixture(), RBAC())
	_, err := p.Decide(context.Background(), Request{Action: "read", Resource: "x"})
	if !errors.Is(err, ErrRequest) || !errors.Is(err, datalog.ErrInvalid) {
		t.Fatalf("Decide err = %v", err)
	}
	_, err = p.Permitted(context.Background(), "", "read")
	if !errors.Is(err, ErrRequest) || !errors.Is(err, datalog.ErrInvalid) {
		t.Fatalf("Permitted err = %v", err)
	}
	// A broken policy is not the caller's fault.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Decide(ctx, Request{Principal: "ana", Action: "read", Resource: "x"}); errors.Is(err, ErrRequest) {
		t.Fatalf("a cancelled context is not a malformed request: %v", err)
	}
}

// reaches reports whether the chain of absences ends at a literal over relation. A base
// relation is not an Absence of its own: it is the Failed literal of the rule that read
// it, so both are looked at.
func reaches(a datalog.Absence, relation string) bool {
	if a.Relation == relation {
		return true
	}
	for _, r := range a.Rules {
		if r.Failed.Atom != nil && r.Failed.Atom.Name == relation {
			return true
		}
		for _, b := range r.Because {
			if reaches(b, relation) {
				return true
			}
		}
	}
	return false
}

// failures lists every RuleFailure under a, depth first.
func failures(a datalog.Absence) []datalog.RuleFailure {
	var out []datalog.RuleFailure
	for _, r := range a.Rules {
		out = append(out, r)
		for _, b := range r.Because {
			out = append(out, failures(b)...)
		}
	}
	return out
}

func proofLeaves(p datalog.Proof, into map[string]bool) {
	if p.Rule == nil {
		into[p.Relation] = true
		return
	}
	for _, b := range p.Body {
		proofLeaves(b, into)
	}
}

func TestDecide_Why(t *testing.T) {
	p := mustLoad(t, append(fixture(), Deny("bo", "read", "timeline/secret")), RBAC())
	decide := func(req Request) Decision {
		t.Helper()
		d, err := p.Decide(context.Background(), req)
		if err != nil {
			t.Fatalf("Decide: %v", err)
		}
		return d
	}

	t.Run("unknown principal reaches a role absence", func(t *testing.T) {
		d := decide(Request{Principal: "zed", Action: "read", Resource: "timeline/bp"})
		if d.Allow || len(d.Why.Rules) == 0 || !reaches(d.Why, "role") {
			t.Fatalf("Why = %v", d.Why)
		}
	})
	t.Run("wrong action reaches grants", func(t *testing.T) {
		d := decide(Request{Principal: "bo", Action: "write", Resource: "timeline/bp"})
		if d.Allow || !reaches(d.Why, "grants") {
			t.Fatalf("Why = %v", d.Why)
		}
	})
	t.Run("resource outside every prefix stops at prefix", func(t *testing.T) {
		d := decide(Request{Principal: "bo", Action: "read", Resource: "secrets/x"})
		found := false
		for _, f := range failures(d.Why) {
			if f.Failed.Atom != nil && f.Failed.Atom.Name == "prefix" {
				if _, ok := f.Bound["Pat"]; ok {
					found = true
				}
			}
		}
		if d.Allow || !found {
			t.Fatalf("no failure at prefix with Pat bound in %v", d.Why)
		}
	})
	t.Run("explicit Deny is a negation that held, with the deny proof", func(t *testing.T) {
		d := decide(Request{Principal: "bo", Action: "read", Resource: "timeline/secret"})
		found := false
		for _, f := range failures(d.Why) {
			if f.Blocked != nil {
				leaves := map[string]bool{}
				proofLeaves(*f.Blocked, leaves)
				found = found || leaves["denied"]
			}
		}
		if d.Allow || !found {
			t.Fatalf("no Blocked proof leaving at denied in %v", d.Why)
		}
	})
	t.Run("allowed request has no Why", func(t *testing.T) {
		d := decide(Request{Principal: "bo", Action: "read", Resource: "timeline/bp"})
		if !d.Allow || !reflect.DeepEqual(d.Why, datalog.Absence{}) {
			t.Fatalf("Why = %v", d.Why)
		}
	})
	t.Run("renders", func(t *testing.T) {
		d := decide(Request{Principal: "zed", Action: "read", Resource: "timeline/bp"})
		if s := d.Why.String(); s == "" || !strings.Contains(s, "zed") {
			t.Fatalf("String() = %q", s)
		}
		b, err := json.Marshal(d.Why)
		if err != nil || !strings.Contains(string(b), "allow") {
			t.Fatalf("MarshalJSON = %s, %v", b, err)
		}
	})
	t.Run("error carries no Why", func(t *testing.T) {
		d, err := p.Decide(context.Background(), Request{Action: "read", Resource: "x"})
		if err == nil || d.Allow || !reflect.DeepEqual(d.Why, datalog.Absence{}) {
			t.Fatalf("Decide = %+v, %v", d, err)
		}
	})
}

func TestDecide_WhyAtScale(t *testing.T) {
	standing := []datalog.Fact{Grant("reader", "read", "timeline/")}
	for i := 0; i < 50000; i++ {
		standing = append(standing, Role(fmt.Sprintf("principal-%05d", i), "reader"))
	}
	p := mustLoad(t, standing, RBAC())
	start := time.Now()
	d, err := p.Decide(context.Background(), Request{Principal: "principal-00001", Action: "write", Resource: "timeline/bp"})
	if err != nil || d.Allow || len(d.Why.Rules) == 0 {
		t.Fatalf("Decide = %+v, %v", d, err)
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("a deny over 50,000 principals took %v", took)
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
