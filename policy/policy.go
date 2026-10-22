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
func RBAC() datalog.RuleSet {
	s := rbac
	s.Program = copyProgram(rbac.Program)
	return s
}

// copyProgram is a deep copy: the AST holds pointers, and the package-level rule set must
// not be reachable through a value a caller holds.
func copyProgram(p datalog.Program) datalog.Program {
	out := datalog.Program{Text: p.Text, Declared: make(map[string]int, len(p.Declared))}
	for k, v := range p.Declared {
		out.Declared[k] = v
	}
	out.Rules = make([]datalog.Rule, len(p.Rules))
	for i, r := range p.Rules {
		out.Rules[i] = datalog.Rule{Head: copyAtom(r.Head), Line: r.Line, Body: make([]datalog.Literal, len(r.Body))}
		for j, l := range r.Body {
			cl := datalog.Literal{Negated: l.Negated}
			if l.Atom != nil {
				a := copyAtom(*l.Atom)
				cl.Atom = &a
			}
			if l.Compare != nil {
				c := *l.Compare
				cl.Compare = &c
			}
			out.Rules[i].Body[j] = cl
		}
	}
	return out
}

func copyAtom(a datalog.Atom) datalog.Atom {
	a.Args = append([]datalog.Term(nil), a.Args...)
	return a
}

// requestRelations are the relations only a request may supply.
// ErrRequest marks a request the policy refused as malformed (an empty principal, action
// or resource, say). Errors that wrap it are the caller's to fix, a 4xx; any other error
// is a broken policy or engine, a 5xx. The underlying ErrInvalid stays reachable.
var ErrRequest = errors.New("policy: malformed request")

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

// Decision is the answer to one request, with what stood behind it. Proof is why it was
// allowed and Why is why it was denied, so every refusal answers "why" from the decision
// itself. A denial through deny/3 appears inside Why as the negation that held, with
// Blocked carrying the proof of the deny tuple; a denial by absence shows the farthest
// literal each allow rule reached.
type Decision struct {
	Allow    bool
	Proof    datalog.Proof   // why allowed; zero when denied
	Why      datalog.Absence // why denied: Result.WhyNot("allow", ...); zero when allowed or on error
	RuleSets []string        // the loaded sets' IDs in load order, e.g. "rbac/1 sha256:..."
}

// Load verifies each rule set against its hash, compiles the sets in order over
// Schema() and holds the standing facts. The first set is normally RBAC(); the plan
// must define allow/3 and permit/3. Rule sets are policy, never user input: a later set
// may add rules to any relation, allow/3 included, and the only promise that survives
// an arbitrary set is deny by absence. Any error is the consumer's to fix before
// serving.
//
// A consumer rule that compares a context value must guard its kind: constants of
// different kinds compare false (and != true), so context("hour", "23") with H > 22
// does not fire.
//
// The standing facts are sorted (by relation, then by each tuple position's kind and text)
// before they are held, so a proof is a function of the policy's content and not of the
// order the caller listed it in.
//
// An empty context value is allowed, and becomes a wildcard if a consumer rule feeds it
// to prefix/2, since the empty prefix matches every resource.
func Load(standing []datalog.Fact, sets ...datalog.RuleSet) (Policy, error) {
	for _, f := range standing {
		if requestRelations[f.Relation] {
			return Policy{}, fmt.Errorf("policy: load: %w: a standing fact of %s is a request relation; a standing fact there would speak for every request",
				datalog.ErrInvalid, f.Relation)
		}
	}
	standing = sortedFacts(standing)
	programs := make([]datalog.Program, 0, len(sets))
	ids := make([]string, len(sets))
	for i, s := range sets {
		if err := s.Verify(); err != nil {
			return Policy{}, fmt.Errorf("policy: load: %w", err)
		}
		programs = append(programs, s.Program)
		ids[i] = s.ID()
	}
	plan, err := datalog.Compile(Schema(), programs...)
	if err != nil {
		return Policy{}, fmt.Errorf("policy: load: %w", err)
	}
	if err := requireRelations(plan); err != nil {
		return Policy{}, fmt.Errorf("policy: load: %w", err)
	}
	b, err := datalog.NewBase(Schema(), standing...)
	if err != nil {
		return Policy{}, fmt.Errorf("policy: load: %w", err)
	}
	return Policy{plan: plan, base: b, ids: ids}, nil
}

// Decide is total: nil error with Allow=false is deny by absence; a non-nil error is
// also a deny, and Decision.Allow is false whenever err != nil. A *TooLarge from the
// engine is wrapped with the remedy: narrow the grants reachable from this principal.
// When the request is well formed and allow is absent, Decision.Why says why.
func (p Policy) Decide(ctx context.Context, req Request) (Decision, error) {
	d := Decision{RuleSets: append([]string(nil), p.ids...)}
	if err := ctx.Err(); err != nil {
		return d, fmt.Errorf("policy: decide: %w", err) // the engine checks at entry, then every 1024 steps
	}
	base, err := p.base.With(req.facts()...)
	if err != nil {
		return d, fmt.Errorf("policy: decide: %w: %w", ErrRequest, err)
	}
	res, err := datalog.Evaluate(ctx, p.plan, base, datalog.Options{
		MaxBytes: maxDecisionBytes, MaxSteps: maxDecisionSteps, Proofs: true,
	})
	if err != nil {
		return d, evalError("decide", err)
	}
	// Proofs are on, so a recorded proof is membership in allow/3.
	want := datalog.Tuple{datalog.String(req.Principal), datalog.String(req.Action), datalog.String(req.Resource)}
	if proof, ok := res.Proof("allow", want); ok {
		d.Allow, d.Proof = true, proof
		return d, nil
	}
	why, ok := res.WhyNot("allow", want, 0)
	if !ok {
		return d, fmt.Errorf("policy: decide: allow%v is neither proved nor absent", want)
	}
	d.Why = why
	return d, nil
}

// Permitted is the prefixes, not the resources: the grant patterns principal holds for
// action, the third column of permit/3, sorted and deduplicated. deny/3 does not apply
// (deny is per concrete resource).
//
// The third column of permit/3 is a contract the consumer's rule sets must keep: a
// non-empty string. A permit tuple for this principal and action that breaks it makes
// Permitted return ErrInvalid, not a listing with a blank or a number in it.
func (p Policy) Permitted(ctx context.Context, principal, action string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("policy: permitted: %w", err)
	}
	base, err := p.base.With(fact("principal", principal), fact("action", action))
	if err != nil {
		return nil, fmt.Errorf("policy: permitted: %w: %w", ErrRequest, err)
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
		if t[2].Kind() != datalog.KindString || t[2].Text() == "" {
			return nil, fmt.Errorf("policy: permitted: %w: permit/3 has a third column that is not a non-empty string (%q)", datalog.ErrInvalid, t[2].Text())
		}
		out = append(out, t[2].Text())
	}
	sort.Strings(out)
	return dedupe(out), nil
}

// requireRelations refuses a plan that lacks the relations Decide and Permitted read, at
// the arity they read them: a set without RBAC would otherwise deny everything silently,
// or panic in Permitted on a tuple of the wrong width.
// sortedFacts is a sorted copy of facts; the caller's slice is left alone.
func sortedFacts(facts []datalog.Fact) []datalog.Fact {
	out := append([]datalog.Fact(nil), facts...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Relation != out[j].Relation {
			return out[i].Relation < out[j].Relation
		}
		a, b := out[i].Tuple, out[j].Tuple
		for k := 0; k < len(a) && k < len(b); k++ {
			if a[k].Kind() != b[k].Kind() {
				return a[k].Kind() < b[k].Kind()
			}
			if a[k].Text() != b[k].Text() {
				return a[k].Text() < b[k].Text()
			}
		}
		return len(a) < len(b)
	})
	return out
}

func requireRelations(plan datalog.Plan) error {
	for _, r := range []struct {
		name string
		want int
	}{{"allow", 3}, {"permit", 3}} {
		name, want := r.name, r.want
		got, ok := plan.Arity(name)
		switch {
		case !ok:
			return fmt.Errorf("%w: no rule set defines %s/%d; load RBAC() first or define it", datalog.ErrInvalid, name, want)
		case got != want:
			return fmt.Errorf("%w: %s is %s/%d, but a decision reads %s/%d", datalog.ErrInvalid, name, name, got, name, want)
		}
	}
	return nil
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
