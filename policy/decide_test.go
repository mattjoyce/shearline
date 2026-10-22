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
