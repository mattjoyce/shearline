package policy

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/mattjoyce/tarski"
)

// TestDecide_CostOnSuccess: a successful allow and a successful deny both report the
// engine's cost. No exact number is asserted: the cost may change with any tarski
// version and no answer changes with it.
func TestDecide_CostOnSuccess(t *testing.T) {
	p := mustLoad(t, fixture(), RBAC())
	for _, tc := range []struct {
		name string
		req  Request
		want bool
	}{
		{"allow", Request{Principal: "ana", Action: "read", Resource: "timeline/bp"}, true},
		{"deny by absence", Request{Principal: "bo", Action: "write", Resource: "timeline/bp"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := p.Decide(context.Background(), tc.req)
			if err != nil {
				t.Fatalf("Decide: %v", err)
			}
			if d.Allow != tc.want {
				t.Fatalf("Allow = %v, want %v", d.Allow, tc.want)
			}
			if d.Steps <= 0 {
				t.Errorf("Steps = %d, want > 0", d.Steps)
			}
			if d.Bytes <= 0 {
				t.Errorf("Bytes = %d, want > 0", d.Bytes)
			}
		})
	}
}

// TestDecide_CostZeroOnError: every error path leaves both fields zero. A malformed
// request and a cancelled context fail before Evaluate; a *TooLarge fails inside it and
// keeps the cost on the error, which TestDecide_CostOnTooLarge covers.
func TestDecide_CostZeroOnError(t *testing.T) {
	p := mustLoad(t, fixture(), RBAC())

	t.Run("malformed request", func(t *testing.T) {
		d, err := p.Decide(context.Background(), Request{Action: "read", Resource: "timeline/bp"})
		if !errors.Is(err, ErrRequest) {
			t.Fatalf("err = %v, want ErrRequest", err)
		}
		if d.Steps != 0 || d.Bytes != 0 {
			t.Fatalf("Steps, Bytes = %d, %d; want zero", d.Steps, d.Bytes)
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		d, err := p.Decide(ctx, Request{Principal: "ana", Action: "read", Resource: "timeline/bp"})
		if err == nil {
			t.Fatal("Decide on a cancelled context returned no error")
		}
		if d.Steps != 0 || d.Bytes != 0 {
			t.Fatalf("Steps, Bytes = %d, %d; want zero", d.Steps, d.Bytes)
		}
	})
}

// chainPolicy is the v0.3.0 consumer-relations fixture: a single chain of n persons,
// reported along reports_to, with a recursive manages closure. A 2,000-person chain
// exceeds maxDecisionBytes before it reaches maxDecisionSteps.
func chainPolicy(t *testing.T, n int) (Policy, Request) {
	t.Helper()
	standing := make([]tarski.Fact, 0, n)
	for i := 1; i < n; i++ {
		standing = append(standing, tarski.Fact{Relation: "reports_to",
			Tuple: tarski.Tuple{tarski.String(fmt.Sprintf("p%d", i)), tarski.String(fmt.Sprintf("p%d", i-1))}})
	}
	standing = append(standing, tarski.Fact{Relation: "about",
		Tuple: tarski.Tuple{tarski.String("review/leaf"), tarski.String(fmt.Sprintf("p%d", n-1))}})
	p := mustLoadConfig(t, Config{
		Sets: []tarski.RuleSet{
			RBAC(),
			ruleSet(t, "cr", `
				manages(M, E) :- reports_to(E, M).
				manages(M, E) :- reports_to(E, X), manages(M, X).
				allow(P, A, R) :- principal(P), action(A), resource(R), about(R, E), manages(P, E).
			`),
		},
		Relations: map[string]Relation{
			"reports_to": {Kinds: []tarski.Kind{tarski.KindString, tarski.KindString}},
			"about":      {Kinds: []tarski.Kind{tarski.KindString, tarski.KindString}},
		},
		Standing: standing,
	})
	return p, Request{Principal: "p0", Action: "read", Resource: "review/leaf"}
}

// TestDecide_CostOnTooLarge: a bound crossed is an error, so Decision keeps both fields
// zero, and the cost stays reachable on the *TooLarge through errors.As.
func TestDecide_CostOnTooLarge(t *testing.T) {
	p, req := chainPolicy(t, 2000)
	d, err := p.Decide(context.Background(), req)
	if err == nil || d.Allow {
		t.Fatalf("Decide = %+v, %v; want a deny with *TooLarge", d, err)
	}
	if d.Steps != 0 || d.Bytes != 0 {
		t.Fatalf("Steps, Bytes = %d, %d; want zero on a *TooLarge", d.Steps, d.Bytes)
	}
	var tl *tarski.TooLarge
	if !errors.As(err, &tl) {
		t.Fatalf("errors.As lost the *TooLarge in %v", err)
	}
	if tl.Bound != tarski.BoundBytes {
		t.Fatalf("Bound = %q, want %q", tl.Bound, tarski.BoundBytes)
	}
	if tl.Bytes <= tl.MaxBytes {
		t.Fatalf("Bytes, MaxBytes = %d, %d; want Bytes > MaxBytes", tl.Bytes, tl.MaxBytes)
	}
}

// treePolicy is the same consumer manages policy over a binary tree of n persons.
func treePolicy(t *testing.T, n int) (Policy, Request) {
	t.Helper()
	standing := make([]tarski.Fact, 0, n)
	for i := 1; i < n; i++ {
		standing = append(standing, tarski.Fact{Relation: "reports_to",
			Tuple: tarski.Tuple{tarski.String(fmt.Sprintf("p%d", i)), tarski.String(fmt.Sprintf("p%d", (i-1)/2))}})
	}
	standing = append(standing, tarski.Fact{Relation: "about",
		Tuple: tarski.Tuple{tarski.String("review/leaf"), tarski.String(fmt.Sprintf("p%d", n-1))}})
	p := mustLoadConfig(t, Config{
		Sets: []tarski.RuleSet{
			RBAC(),
			ruleSet(t, "cr", `
				manages(M, E) :- reports_to(E, M).
				manages(M, E) :- reports_to(E, X), manages(M, X).
				allow(P, A, R) :- principal(P), action(A), resource(R), about(R, E), manages(P, E).
			`),
		},
		Relations: map[string]Relation{
			"reports_to": {Kinds: []tarski.Kind{tarski.KindString, tarski.KindString}},
			"about":      {Kinds: []tarski.Kind{tarski.KindString, tarski.KindString}},
		},
		Standing: standing,
	})
	return p, Request{Principal: "p0", Action: "read", Resource: "review/leaf"}
}

// TestDecide_CostGrowsWithFixture: the same request shape over a larger fixture costs
// more of both. The order is asserted, not exact numbers.
func TestDecide_CostGrowsWithFixture(t *testing.T) {
	small, req := treePolicy(t, 100)
	large, _ := treePolicy(t, 1000)
	ds, err := small.Decide(context.Background(), req)
	if err != nil || !ds.Allow {
		t.Fatalf("100-person Decide = %+v, %v", ds, err)
	}
	dl, err := large.Decide(context.Background(), req)
	if err != nil || !dl.Allow {
		t.Fatalf("1,000-person Decide = %+v, %v", dl, err)
	}
	if dl.Steps <= ds.Steps {
		t.Errorf("Steps: 1,000-person = %d, 100-person = %d; want larger", dl.Steps, ds.Steps)
	}
	if dl.Bytes <= ds.Bytes {
		t.Errorf("Bytes: 1,000-person = %d, 100-person = %d; want larger", dl.Bytes, ds.Bytes)
	}
}

// TestDecide_CostDeterministic: the same request twice gives the same cost within one
// tarski version.
func TestDecide_CostDeterministic(t *testing.T) {
	p := mustLoad(t, fixture(), RBAC())
	req := Request{Principal: "ana", Action: "read", Resource: "timeline/bp"}
	first, err := p.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("first Decide: %v", err)
	}
	second, err := p.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("second Decide: %v", err)
	}
	if first.Steps != second.Steps || first.Bytes != second.Bytes {
		t.Fatalf("cost differs between identical calls: (%d, %d) then (%d, %d)",
			first.Steps, first.Bytes, second.Steps, second.Bytes)
	}
}
