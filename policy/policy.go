package policy

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"sort"

	"github.com/mattjoyce/shearline/datalog"
)

// maxDecisionBytes bounds what one decision may derive. A decision is a few joins over
// one principal's roles, so a megabyte of derived tuples means the grants reachable
// from the principal are wrong, not large. It is not a knob: a consumer that needs
// more has a policy that should be narrowed.
const maxDecisionBytes = 1 << 20

// maxDecisionSteps bounds the join work of one decision. A policy that visits millions
// of tuples for one request is wrong, not large, and the deadline alone would let a
// wide product of duplicates run until it fires. Not a knob, for the same reason.
const maxDecisionSteps = 1 << 22

//go:embed rbac.dl
var rbacText string

// rbac is parsed at init so a broken asset fails at process start, never at the first
// request. TestRBAC_Parses holds it.
var rbac = mustRuleSet("rbac", 1, rbacText)

func mustRuleSet(name string, version int, text string) datalog.RuleSet {
	s, err := datalog.NewRuleSet(name, version, text)
	if err != nil {
		panic(fmt.Sprintf("policy: bundled rule set %s/%d: %v", name, version, err))
	}
	return s
}

// RBAC is the bundled rule set: roles, inheritance, prefix grants, and deny/3 as a
// hook for a consumer's own rule set.
func RBAC() datalog.RuleSet { return rbac }

// requestRelations are the relations only a request may supply.
var requestRelations = map[string]bool{"principal": true, "action": true, "resource": true, "context": true}

// Request is the PARC shape: who, does what, to which, under what.
type Request struct {
	Principal string
	Action    string
	Resource  string
	Context   map[string]datalog.Const // becomes context(Key, Value) facts, keys sorted
}

// Policy is a compiled rule set with its standing facts. It is a value: nothing
// changes after Load, so it is safe for concurrent use.
type Policy struct {
	plan datalog.Plan
	base datalog.Base
	ids  []string
}

// Decision is the answer to one request, with what stood behind it.
type Decision struct {
	Allow    bool
	Proof    datalog.Proof // zero value when denied
	RuleSets []string      // the loaded sets' IDs in load order, e.g. "rbac/1 sha256:..."
}

// Load compiles the rule sets in order over Schema and holds the standing facts. The
// first set is normally RBAC(). Rule sets are policy, never user input: a later set
// may add rules to any relation, allow/3 included, and the only promise that survives
// an arbitrary set is deny by absence. Any error is the consumer's to fix before
// serving.
//
// A consumer rule that compares a context value must guard its kind: constants of
// different kinds compare false (and != true), so context("hour", "23") with H > 22
// does not fire.
func Load(standing []datalog.Fact, sets ...datalog.RuleSet) (Policy, error) {
	for _, f := range standing {
		if requestRelations[f.Relation] {
			return Policy{}, fmt.Errorf("policy: load: %w: a standing fact of %s is a request relation; a standing fact there would speak for every request",
				datalog.ErrInvalid, f.Relation)
		}
	}
	programs := make([]datalog.Program, 0, len(sets))
	ids := make([]string, len(sets))
	for i, s := range sets {
		programs = append(programs, s.Program)
		ids[i] = s.ID()
	}
	plan, err := datalog.Compile(Schema, programs...)
	if err != nil {
		return Policy{}, fmt.Errorf("policy: load: %w", err)
	}
	b, err := datalog.NewBase(Schema, standing...)
	if err != nil {
		return Policy{}, fmt.Errorf("policy: load: %w", err)
	}
	return Policy{plan: plan, base: b, ids: ids}, nil
}

// Decide is total: nil error with Allow=false is deny by absence; a non-nil error is
// also a deny, and Decision.Allow is false whenever err != nil. A *TooLarge from the
// engine is wrapped with the remedy: narrow the grants reachable from this principal.
func (p Policy) Decide(ctx context.Context, req Request) (Decision, error) {
	d := Decision{RuleSets: append([]string(nil), p.ids...)}
	if err := ctx.Err(); err != nil {
		return d, fmt.Errorf("policy: decide: %w", err) // the engine checks at entry, then every 1024 steps
	}
	base, err := p.base.With(req.facts()...)
	if err != nil {
		return d, fmt.Errorf("policy: decide: %w", err)
	}
	res, err := datalog.Evaluate(ctx, p.plan, base, datalog.Options{
		MaxBytes: maxDecisionBytes, MaxSteps: maxDecisionSteps, Proofs: true,
	})
	if err != nil {
		return d, evalError("decide", err)
	}
	ans, err := res.Relation("allow")
	if err != nil {
		return d, fmt.Errorf("policy: decide: %w", err)
	}
	want := datalog.Tuple{datalog.String(req.Principal), datalog.String(req.Action), datalog.String(req.Resource)}
	for _, t := range ans.Tuples {
		if sameTuple(t, want) {
			d.Allow = true
			d.Proof, _ = res.Proof("allow", want)
			return d, nil
		}
	}
	return d, nil
}

// Permitted is the prefixes, not the resources: the grant patterns principal holds for
// action, the third column of permit/3, sorted and deduplicated. deny/3 does not apply
// (deny is per concrete resource).
func (p Policy) Permitted(ctx context.Context, principal, action string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("policy: permitted: %w", err)
	}
	base, err := p.base.With(fact("principal", principal), fact("action", action))
	if err != nil {
		return nil, fmt.Errorf("policy: permitted: %w", err)
	}
	res, err := datalog.Evaluate(ctx, p.plan, base, datalog.Options{MaxBytes: maxDecisionBytes, MaxSteps: maxDecisionSteps})
	if err != nil {
		return nil, evalError("permitted", err)
	}
	ans, err := res.Relation("permit")
	if err != nil {
		return nil, fmt.Errorf("policy: permitted: %w", err)
	}
	out := make([]string, 0, len(ans.Tuples))
	for _, t := range ans.Tuples {
		// A rule set may derive permit tuples for anyone; the listing is this caller's.
		if !sameTuple(t[:2], datalog.Tuple{datalog.String(principal), datalog.String(action)}) {
			continue
		}
		out = append(out, t[2].Text())
	}
	sort.Strings(out)
	return dedupe(out), nil
}

func evalError(op string, err error) error {
	var tl *datalog.TooLarge
	if errors.As(err, &tl) {
		return fmt.Errorf("policy: %s: %w; narrow the grants reachable from this principal", op, err)
	}
	return fmt.Errorf("policy: %s: %w", op, err)
}

func sameTuple(a, b datalog.Tuple) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Kind() != b[i].Kind() || a[i].Text() != b[i].Text() {
			return false
		}
	}
	return true
}

func dedupe(s []string) []string {
	out := s[:0]
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}
