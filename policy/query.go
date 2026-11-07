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
// Nil or empty Contexts is ErrRequest. For no context pass one nil or empty bag.
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
	Bag                         int // index into Space.Contexts
	Proof                       tarski.Proof
}

// Matrix is the derived allow relation over a Space under each bag.
// A Matrix is whole or absent. On error only RuleSets is filled.
// Space echoes the question rather than a digest so absence and Bag can be interpreted
// without the caller's original data. Empty bags are canonical empty maps; JSON sorts
// their keys. Costs are observation-only, like Decision: they can change with tarski
// without an answer change and must not drive decisions or appear in receipt hashes.
// MaxBytes measures derived tuples in one evaluation, not retained proof memory.
type Matrix struct {
	Space    Space    // canonical private copy; bag order preserved, sets sorted and deduplicated
	Allows   []Allow  // sorted by Bag, then Principal, Action, Resource, field by field
	RuleSets []string // the loaded sets' IDs in load order; a private copy
	MaxSteps int      // largest Steps of any one evaluation
	MaxBytes int      // largest Bytes of any one evaluation
}

// Query holds every principal, action and resource of s as facts at the same time.
// It evaluates once per bag under the decision bounds with proofs on.
// It returns only allow/3 tuples inside s. deny/3 applies through the loaded rules.
// A triple absent from the Matrix is denied only if it was in the Space.
// A resource the caller did not name is not denied: it was not asked.
// For "why not (p, a, r)", call Decide on that request and read Why.
//
// The rule sets must keep the keying contract: adding other request-axis members
// must not change allow(p, a, r) for any existing triple. Every request-dependent
// relation, including dependencies through negation, must retain and join all relevant
// request-axis keys. Cross-member request tests of either polarity are forbidden;
// request-derived patterns may match only their own resource. Under this contract
// Query agrees with the Decide loop. Violations can add or remove allows.
// Query checks retained proofs for foreign positive principal/action/resource leaves.
// Such a leaf returns ErrKeying: that derivation could not occur in Decide. Decide
// on the flagged tuple adjudicates; another valid derivation may still allow it.
// This partial check does not prove the contract. Negation can cause undetected extra
// or missing allows, since absent facts are not retained as positive proof leaves.
// Each evaluation uses the decision bounds. TooLarge is an error; Query never splits.
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
	if len(s.Contexts) == 0 {
		return zero, fmt.Errorf("policy: query: %w: %w: contexts set is empty", ErrRequest, tarski.ErrInvalid)
	}
	bags := make([]map[string]tarski.Const, len(s.Contexts))
	for i, bag := range s.Contexts {
		bags[i] = maps.Clone(bag)
		if bags[i] == nil {
			bags[i] = map[string]tarski.Const{}
		}
		if err := p.checkRequestContext(bags[i]); err != nil {
			return zero, fmt.Errorf("policy: query: bag %d: %w", i, err)
		}
	}
	out := Matrix{RuleSets: slices.Clone(p.ids), Space: Space{
		Principals: slices.Sorted(maps.Keys(sets[0])),
		Actions:    slices.Sorted(maps.Keys(sets[1])),
		Resources:  slices.Sorted(maps.Keys(sets[2])),
		Contexts:   bags,
	}}
	for i, bag := range bags {
		if err := ctx.Err(); err != nil {
			return zero, fmt.Errorf("policy: query: bag %d: %w", i, err)
		}
		allows, steps, bytes, err := p.queryBag(ctx, base, bag, sets)
		if err != nil {
			var tl *tarski.TooLarge
			if errors.As(err, &tl) {
				return zero, fmt.Errorf("policy: query: bag %d (|P|=%d |A|=%d |R|=%d): %w; a bounded evaluation is usually wrong, not large: narrow the rules first", i, len(sets[0]), len(sets[1]), len(sets[2]), err)
			}
			return zero, fmt.Errorf("policy: query: bag %d: %w", i, err)
		}
		for j := range allows {
			allows[j].Bag = i
		}
		out.Allows = append(out.Allows, allows...)
		out.MaxSteps, out.MaxBytes = max(out.MaxSteps, steps), max(out.MaxBytes, bytes)
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
	withBag, err := base.With(contextFacts(bag)...)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%w: %w", ErrRequest, err)
	}
	res, err := tarski.Evaluate(ctx, p.plan, withBag, tarski.Options{
		MaxBytes: maxDecisionBytes, MaxSteps: maxDecisionSteps, Proofs: true,
	})
	if err != nil {
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
		proof, ok := res.Proof("allow", tuple)
		if !ok {
			return nil, 0, 0, fmt.Errorf("allow(%s, %s, %s) has no proof", tuple[0].Text(), tuple[1].Text(), tuple[2].Text())
		}
		if foreign, found := foreignRequestFact(proof, tuple); found {
			return nil, 0, 0, fmt.Errorf("allow(%s, %s, %s) cites %s: %w", tuple[0].Text(), tuple[1].Text(), tuple[2].Text(), foreign, ErrKeying)
		}
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

// foreignRequestFact visits the proof DAG once. Shared proof values share the tuple's
// backing slice, so its first constant identifies the node; zero-arity nodes use the
// relation alone. Load forbids aggregates, so all dependencies are in Body.
func foreignRequestFact(proof tarski.Proof, tuple tarski.Tuple) (string, bool) {
	type nodeKey struct {
		relation string
		first    *tarski.Const
	}
	seen := map[nodeKey]bool{}
	var visit func(tarski.Proof) (string, bool)
	visit = func(node tarski.Proof) (string, bool) {
		key := nodeKey{relation: node.Relation}
		if len(node.Tuple) > 0 {
			key.first = &node.Tuple[0]
		}
		if seen[key] {
			return "", false
		}
		seen[key] = true
		if node.Rule == nil && len(node.Tuple) == 1 {
			for axis, relation := range []string{"principal", "action", "resource"} {
				if node.Relation == relation && node.Tuple[0] != tuple[axis] {
					return fmt.Sprintf("%s(%s)", relation, node.Tuple[0].Text()), true
				}
			}
		}
		for _, child := range node.Body {
			if foreign, found := visit(child); found {
				return foreign, true
			}
		}
		return "", false
	}
	return visit(proof)
}
