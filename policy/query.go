package policy

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"

	"github.com/mattjoyce/tarski"
)

// Space is the sets one Query ranges over. Principals, Actions and Resources are sets.
// Order does not matter. A duplicate counts once. An empty set is ErrRequest.
// Contexts is a list. Query evaluates once per bag. Allow.Bag names its index.
// Nil or empty Contexts means one evaluation with no context.
// Query keeps none of the caller's slices or maps.
type Space struct {
	Principals []string
	Actions    []string
	Resources  []string
	Contexts   []map[string]tarski.Const
}

// Allow is one tuple of allow/3 with its proof and context bag index.
type Allow struct {
	Principal, Action, Resource string
	Bag                         int // index into Space.Contexts; 0 when Contexts is empty
	Proof                       tarski.Proof
}

// Matrix is the derived allow relation over a Space under each bag.
// A Matrix is whole or absent. On error only RuleSets is filled.
type Matrix struct {
	Allows   []Allow  // sorted by Bag, then Principal, Action, Resource, bytewise
	RuleSets []string // the loaded sets' IDs in load order; a private copy
	Steps    int      // the largest Steps of any one evaluation
	Bytes    int      // the largest Bytes of any one evaluation
}

// Query holds every principal, action and resource of s as facts at the same time.
// It evaluates once per bag under the decision bounds with proofs on.
// It returns only allow/3 tuples inside s. deny/3 applies through the loaded rules.
// A triple absent from the Matrix is denied only if it was in the Space.
// A resource the caller did not name is not denied: it was not asked.
// For "why not (p, a, r)", call Decide on that request and read Why.
//
// The rule sets must keep the keying contract. A derived relation that depends on a
// request fact carries that fact's variable in its head. A pattern joined to resource(R)
// by prefix/2 ends with the separator, or the resource names are prefix-free.
// Query does not check this contract. Under this contract Query agrees with Decide.
// Each evaluation uses the decision bounds. On TooLarge, split the Space.
func (p Policy) Query(ctx context.Context, s Space) (Matrix, error) {
	zero := Matrix{RuleSets: slices.Clone(p.ids)}
	if err := ctx.Err(); err != nil {
		return zero, fmt.Errorf("policy: query: %w", err)
	}
	facts, sets, err := querySets(s)
	if err != nil {
		return zero, err
	}
	base, err := p.base.With(sortedFacts(facts)...)
	if err != nil {
		return zero, fmt.Errorf("policy: query: %w: %w", ErrRequest, err)
	}
	bags := make([]map[string]tarski.Const, max(1, len(s.Contexts)))
	for i, bag := range s.Contexts {
		bags[i] = maps.Clone(bag)
	}
	out := Matrix{RuleSets: slices.Clone(p.ids)}
	for i, bag := range bags {
		if err := ctx.Err(); err != nil {
			return zero, fmt.Errorf("policy: query: bag %d: %w", i, err)
		}
		if err := p.checkRequestContext(bag); err != nil {
			return zero, fmt.Errorf("policy: query: bag %d: %w", i, err)
		}
		allows, steps, bytes, err := p.queryBag(ctx, base, bag, sets)
		if err != nil {
			return zero, fmt.Errorf("policy: query: bag %d: %w", i, err)
		}
		for j := range allows {
			allows[j].Bag = i
		}
		out.Allows = append(out.Allows, allows...)
		out.Steps, out.Bytes = max(out.Steps, steps), max(out.Bytes, bytes)
	}
	sort.Slice(out.Allows, func(i, j int) bool {
		a, b := out.Allows[i], out.Allows[j]
		if a.Bag != b.Bag {
			return a.Bag < b.Bag
		}
		return slices.Compare([]string{a.Principal, a.Action, a.Resource}, []string{b.Principal, b.Action, b.Resource}) < 0
	})
	return out, nil
}

func querySets(s Space) ([]tarski.Fact, [3]map[string]bool, error) {
	var facts []tarski.Fact
	var sets [3]map[string]bool
	for i, values := range [][]string{s.Principals, s.Actions, s.Resources} {
		relation := []string{"principal", "action", "resource"}[i]
		if len(values) == 0 {
			return nil, sets, fmt.Errorf("policy: query: %w: %w: %s set is empty", ErrRequest, tarski.ErrInvalid, relation)
		}
		sets[i] = make(map[string]bool, len(values))
		for _, value := range values {
			if !sets[i][value] {
				facts = append(facts, fact(relation, value))
				sets[i][value] = true
			}
		}
	}
	return facts, sets, nil
}

func (p Policy) queryBag(ctx context.Context, base tarski.Base, bag map[string]tarski.Const, sets [3]map[string]bool) ([]Allow, int, int, error) {
	// Request.facts sorts context keys. The first three facts are the request axes.
	withBag, err := base.With((Request{Context: bag}).facts()[3:]...)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%w: %w", ErrRequest, err)
	}
	res, err := tarski.Evaluate(ctx, p.plan, withBag, tarski.Options{
		MaxBytes: maxDecisionBytes, MaxSteps: maxDecisionSteps, Proofs: true,
	})
	if err != nil {
		var tl *tarski.TooLarge
		if errors.As(err, &tl) {
			return nil, 0, 0, fmt.Errorf("%w; split the Space", err)
		}
		return nil, 0, 0, err
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, 0, err
	}
	ans, err := res.Relation("allow")
	if err != nil {
		return nil, 0, 0, err
	}
	var allows []Allow
	for _, tuple := range ans.Tuples() {
		if !queryMember(tuple, sets) {
			continue
		}
		proof, _ := res.Proof("allow", tuple)
		allows = append(allows, Allow{Principal: tuple[0].Text(), Action: tuple[1].Text(), Resource: tuple[2].Text(), Proof: proof})
	}
	steps, bytes := res.Cost()
	return allows, steps, bytes, nil
}

func queryMember(tuple tarski.Tuple, sets [3]map[string]bool) bool {
	for i, value := range tuple {
		if value.Kind() != tarski.KindString || !sets[i][value.Text()] {
			return false
		}
	}
	return true
}
