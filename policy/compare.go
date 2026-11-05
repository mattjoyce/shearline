package policy

import (
	"context"
	"fmt"
	"iter"
	"maps"
)

// Diff is one request where two policies decide differently, with both decisions. Each
// decision carries its Proof or its Why, so a diff says why each side decided as it did.
type Diff struct {
	Request Request
	A, B    Decision
	ErrA    error // nil when Decide on a succeeded
	ErrB    error // nil when Decide on b succeeded
}

// Compare decides every request on a and on b, and returns each request where the two
// decisions differ, in request order. Two decisions differ when Allow differs, or when
// one Decide returns an error and the other does not; two errors are equal here. Proof,
// Why, RuleSets and cost are not compared, so two policies that allow a request for
// different reasons agree on it.
//
// a and b may differ in rule sets, standing facts, context declarations, or all three. A
// request that one policy refuses as malformed (ErrRequest) and the other decides is a
// diff, since the two policies accept different requests.
//
// Compare checks only the requests you give, over the standing facts each policy holds.
// No diff is not a proof that the policies are equivalent for all requests or for other
// facts: it is evidence for the requests you gave. With it you can check that a change only
// removes access (no diff where b allows and a denies), or that a deny property holds
// over a request space (compare with the policy plus the property as a deny rule).
//
// When ctx is done, Compare stops and returns the diffs found before it, with ctx.Err().
func Compare(ctx context.Context, a, b Policy, reqs iter.Seq[Request]) ([]Diff, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("policy: compare: %w", err)
	}
	var diffs []Diff
	for req := range reqs {
		if err := ctx.Err(); err != nil {
			return diffs, fmt.Errorf("policy: compare: %w", err)
		}
		da, errA := a.Decide(ctx, req)
		db, errB := b.Decide(ctx, req)
		// A Decide that failed because ctx ended is not a decision; do not record it.
		if err := ctx.Err(); err != nil {
			return diffs, fmt.Errorf("policy: compare: %w", err)
		}
		if (errA == nil) == (errB == nil) && (errA != nil || da.Allow == db.Allow) {
			continue
		}
		// The iterator may reuse its Context map; the diff keeps its own copy, so the
		// request it records is the one that was decided.
		req.Context = maps.Clone(req.Context)
		diffs = append(diffs, Diff{Request: req, A: da, B: db, ErrA: errA, ErrB: errB})
	}
	if err := ctx.Err(); err != nil {
		return diffs, fmt.Errorf("policy: compare: %w", err)
	}
	return diffs, nil
}
