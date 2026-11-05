package policy

import (
	"context"
	"errors"
	"iter"
	"reflect"
	"slices"
	"testing"

	"github.com/mattjoyce/tarski"
)

// space is every principal, action and resource the fixture names, plus ones it does not.
func space() []Request {
	var out []Request
	for _, p := range []string{"ana", "bo", "zed"} {
		for _, a := range []string{"read", "write", "delete"} {
			for _, r := range []string{"timeline/bp", "timeline/secret", "docs/x", "secrets/x"} {
				out = append(out, Request{Principal: p, Action: a, Resource: r})
			}
		}
	}
	return out
}

func TestCompare_SamePolicyHasNoDiffs(t *testing.T) {
	p := mustLoad(t, fixture(), RBAC())
	q := mustLoad(t, fixture(), RBAC()) // a second Load of the same content
	for _, other := range []Policy{p, q} {
		diffs, err := Compare(context.Background(), p, other, slices.Values(space()))
		if err != nil || len(diffs) != 0 {
			t.Fatalf("Compare = %d diffs, %v; want none", len(diffs), err)
		}
	}
}

func TestCompare_DenyFactIsExactlyItsRequests(t *testing.T) {
	a := mustLoad(t, fixture(), RBAC())
	b := mustLoad(t, append(fixture(), Deny("bo", "read", "timeline/secret"), Deny("ana", "write", "timeline/bp")), RBAC())
	diffs, err := Compare(context.Background(), a, b, slices.Values(space()))
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	var got []Request
	for _, d := range diffs {
		got = append(got, d.Request)
		if !d.A.Allow || d.B.Allow || d.ErrA != nil || d.ErrB != nil {
			t.Fatalf("diff %+v: want allowed on a, denied on b, no errors", d.Request)
		}
		if d.A.Proof.Relation != "allow" {
			t.Fatalf("diff %+v: a carries no proof of allow: %+v", d.Request, d.A.Proof)
		}
		if !reaches(d.B.Why, "deny") {
			t.Fatalf("diff %+v: b's why-not does not reach deny", d.Request)
		}
	}
	// In request order: ana's requests come before bo's in space().
	want := []Request{
		{Principal: "ana", Action: "write", Resource: "timeline/bp"},
		{Principal: "bo", Action: "read", Resource: "timeline/secret"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffs = %+v, want %+v", got, want)
	}
	// Direction: b only removes access, so nothing b allows does a deny.
	again, _ := Compare(context.Background(), b, a, slices.Values(space()))
	for _, d := range again {
		if d.A.Allow {
			t.Fatalf("reversed compare: %+v allowed by the narrower policy", d.Request)
		}
	}
}

func TestCompare_Deterministic(t *testing.T) {
	a := mustLoad(t, fixture(), RBAC())
	b := mustLoad(t, append(fixture(), Deny("bo", "read", "docs/x")), RBAC())
	first, err1 := Compare(context.Background(), a, b, slices.Values(space()))
	second, err2 := Compare(context.Background(), a, b, slices.Values(space()))
	if err1 != nil || err2 != nil || len(first) != 1 || !reflect.DeepEqual(first, second) {
		t.Fatalf("two runs differ or wrong: %d/%v, %d/%v", len(first), err1, len(second), err2)
	}
}

func TestCompare_CancelReturnsDiffsSoFar(t *testing.T) {
	a := mustLoad(t, fixture(), RBAC())
	narrow := mustLoad(t, append(fixture(), Deny("ana", "read", "timeline/bp"), Deny("bo", "read", "docs/x")), RBAC())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reqs := space()
	// Cancel after the iterator has yielded ana's requests (the first 12), so the diff for
	// ana is found and the one for bo is not.
	seq := iter.Seq[Request](func(yield func(Request) bool) {
		for i, r := range reqs {
			if i == 12 {
				cancel()
			}
			if !yield(r) {
				return
			}
		}
	})
	diffs, err := Compare(ctx, a, narrow, seq)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if len(diffs) != 1 || diffs[0].Request.Principal != "ana" {
		t.Fatalf("diffs = %+v, want only ana's", diffs)
	}
}

func TestCompare_MalformedOnOneSideIsADiff(t *testing.T) {
	withHour := mustLoadConfig(t, Config{Standing: fixture(), Sets: rbacOnly(), Context: map[string]tarski.Kind{"hour": tarski.KindNumber}})
	without := mustLoad(t, fixture(), RBAC())
	noon, _ := tarski.Number("12")
	req := Request{Principal: "bo", Action: "read", Resource: "timeline/bp"}

	hour := req
	hour.Context = map[string]tarski.Const{"hour": noon}
	diffs, err := Compare(context.Background(), withHour, without, slices.Values([]Request{hour}))
	if err != nil || len(diffs) != 1 {
		t.Fatalf("one side malformed: %d diffs, %v; want 1", len(diffs), err)
	}
	if d := diffs[0]; d.ErrA != nil || !errors.Is(d.ErrB, ErrRequest) || !d.A.Allow {
		t.Fatalf("diff = %+v; want a allowed, b ErrRequest", d)
	}
	// The other way round: the error is on a, and b denies, so Allow agrees (false).
	denyAll := mustLoadConfig(t, Config{Standing: append(fixture(), Deny("bo", "read", "timeline/bp")), Sets: rbacOnly(), Context: map[string]tarski.Kind{"hour": tarski.KindNumber}})
	diffs, err = Compare(context.Background(), without, denyAll, slices.Values([]Request{hour}))
	if err != nil || len(diffs) != 1 {
		t.Fatalf("malformed on a only: %d diffs, %v; want 1", len(diffs), err)
	}
	if d := diffs[0]; !errors.Is(d.ErrA, ErrRequest) || d.ErrB != nil || d.B.Allow {
		t.Fatalf("diff = %+v; want a ErrRequest, b denied", d)
	}

	both := req
	both.Context = map[string]tarski.Const{"undeclared": noon}
	diffs, err = Compare(context.Background(), withHour, without, slices.Values([]Request{both}))
	if err != nil || len(diffs) != 0 {
		t.Fatalf("both sides malformed: %d diffs, %v; want none", len(diffs), err)
	}
}

// An iterator may reuse one Context map between requests; a diff keeps the request it
// decided, not what the map holds later.
func TestCompare_DiffKeepsItsOwnContext(t *testing.T) {
	withHour := mustLoadConfig(t, Config{Standing: fixture(), Sets: rbacOnly(), Context: map[string]tarski.Kind{"hour": tarski.KindNumber}})
	without := mustLoad(t, fixture(), RBAC())
	noon, _ := tarski.Number("12")
	shared := map[string]tarski.Const{"hour": noon}
	seq := iter.Seq[Request](func(yield func(Request) bool) {
		if !yield(Request{Principal: "bo", Action: "read", Resource: "timeline/bp", Context: shared}) {
			return
		}
		delete(shared, "hour") // the same map, now empty: both policies decide it alike
		yield(Request{Principal: "bo", Action: "read", Resource: "timeline/bp", Context: shared})
	})
	diffs, err := Compare(context.Background(), withHour, without, seq)
	if err != nil || len(diffs) != 1 {
		t.Fatalf("Compare = %d diffs, %v; want 1", len(diffs), err)
	}
	if got, ok := diffs[0].Request.Context["hour"]; !ok || got.Text() != "12" {
		t.Fatalf("diff context = %v, want hour 12 as decided", diffs[0].Request.Context)
	}
}

// Cancellation is reported even when no request follows it.
func TestCompare_CancelWithNoFurtherRequest(t *testing.T) {
	a := mustLoad(t, fixture(), RBAC())
	b := mustLoad(t, append(fixture(), Deny("ana", "read", "timeline/bp")), RBAC())

	done, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Compare(done, a, b, slices.Values([]Request(nil))); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled before, empty iterator: err = %v, want context.Canceled", err)
	}

	ctx, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	last := iter.Seq[Request](func(yield func(Request) bool) {
		if yield(Request{Principal: "ana", Action: "read", Resource: "timeline/bp"}) {
			cancel2() // after the last request was decided
		}
	})
	diffs, err := Compare(ctx, a, b, last)
	if !errors.Is(err, context.Canceled) || len(diffs) != 1 {
		t.Fatalf("cancelled after the last yield: %d diffs, %v; want 1 and context.Canceled", len(diffs), err)
	}
}
