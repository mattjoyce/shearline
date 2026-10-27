package policy

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"sort"

	"github.com/mattjoyce/tarski"
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

func mustRuleSet(name string, version int, text string) tarski.RuleSet {
	s, err := tarski.NewRuleSet(name, version, text)
	if err != nil {
		panic(fmt.Sprintf("policy: bundled rule set %s/%d: %v", name, version, err))
	}
	return s
}

// RBAC is the bundled rule set: roles, inheritance, prefix grants, and deny/3 as a
// hook for a consumer's own rule set.
func RBAC() tarski.RuleSet {
	s := rbac
	s.Program = rbac.Program.Clone()
	return s
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
	Context   map[string]tarski.Const // becomes context(Key, Value) facts, keys sorted
}

// Policy is a compiled rule set with its standing facts. It is a value: nothing
// changes after Load, so it is safe for concurrent use.
type Policy struct {
	plan tarski.Plan
	base tarski.Base
	ids  []string

	context map[string]tarski.Kind // the declared keys; a private copy
}

// Decision is the answer to one request, with what stood behind it. Proof is why it was
// allowed and Why is why it was denied, so every refusal answers "why" from the decision
// itself. A denial through deny/3 appears inside Why as the negation that held, with
// Blocked carrying the proof of the deny tuple; a denial by absence shows the farthest
// literal each allow rule reached.
type Decision struct {
	Allow    bool
	Proof    tarski.Proof   // why allowed; zero when denied
	Why      tarski.Absence // why denied: Result.WhyNot("allow", ...); zero when allowed or on error
	RuleSets []string       // the loaded sets' IDs in load order, e.g. "rbac/1 sha256:..."
}

// Config is everything a Policy is built from. Context lists every key a request may
// carry, with its kind; a request with any other key, or a declared key of another kind,
// is ErrRequest, and a rule set that names an undeclared key fails at Load. A nil Context
// declares none: such a policy accepts a request with no context and refuses any key.
//
// A declared key is one a request may carry, not one it must: the declaration is a
// schema, not attestation. A key a rule relies on (a tenant, a network, an MFA state)
// is the consumer's to require before Decide, or to require positively in the rule,
// since a rule that reads its absence through negation holds when the caller omits it.
//
// Relations declares consumer base relations that rule sets may read as standing facts.
// A nil map declares none and preserves v0.2.2 behaviour.
type Config struct {
	Standing  []tarski.Fact
	Sets      []tarski.RuleSet
	Context   map[string]tarski.Kind
	Relations map[string]Relation
}

// Relation is one consumer base relation: one kind for each column, in order.
type Relation struct{ Kinds []tarski.Kind }

// Load verifies each rule set against its hash, compiles Sets in order over the schema built
// from Config.Context and Config.Relations and holds Standing. The first set is normally
// RBAC(); the plan must define allow/3 and permit/3. Rule sets are policy, never user input:
// a later set may add rules to any relation, allow/3 included, and the only promise that
// survives an arbitrary set is deny by absence. Any error is the consumer's to fix before serving.
//
// The context keys and their kinds come only from Config.Context; see it for what a rule
// or a request may name. Config.Relations declares consumer base relations (standing only);
// a nil map declares none.
//
// The standing facts are sorted (by relation, then by each tuple position's kind and text)
// before they are held, so a proof is a function of the policy's content and not of the
// order the caller listed it in.
//
// An empty context value is allowed, and becomes a wildcard if a consumer rule feeds it
// to prefix/2, since the empty prefix matches every resource.
func Load(cfg Config) (Policy, error) {
	standing, sets := cfg.Standing, cfg.Sets
	declared, err := checkContext(cfg.Context)
	if err != nil {
		return Policy{}, fmt.Errorf("policy: load: %w", err)
	}
	declaredRelations, err := checkRelations(cfg.Relations)
	if err != nil {
		return Policy{}, fmt.Errorf("policy: load: %w", err)
	}
	for _, f := range standing {
		if requestRelations[f.Relation] {
			return Policy{}, fmt.Errorf("policy: load: %w: a standing fact of %s is a request relation; a standing fact there would speak for every request",
				tarski.ErrInvalid, f.Relation)
		}
	}
	standing = sortedFacts(standing)
	programs := make([]tarski.Program, 0, len(sets))
	ids := make([]string, len(sets))
	for i, s := range sets {
		if err := s.Verify(); err != nil {
			return Policy{}, fmt.Errorf("policy: load: %w", err)
		}
		programs = append(programs, s.Program)
		ids[i] = s.ID()
	}
	schema := schemaWithRelations(declared, declaredRelations)
	plan, err := tarski.Compile(schema, programs...)
	if err != nil {
		return Policy{}, fmt.Errorf("policy: load: %w", err)
	}
	if plan.HasAggregates() {
		// A decision is one request against standing facts; an aggregate (count, sum, min,
		// max) summarises a whole relation and belongs in a query. Refusing it here also
		// keeps every proof a plain derivation.
		return Policy{}, fmt.Errorf("policy: load: %w: policy rule sets cannot use aggregates; a decision is per request, aggregates are for queries", tarski.ErrInvalid)
	}
	if err := requireRelations(plan); err != nil {
		return Policy{}, fmt.Errorf("policy: load: %w", err)
	}
	b, err := tarski.NewBase(schema, standing...)
	if err != nil {
		return Policy{}, fmt.Errorf("policy: load: %w", err)
	}
	return Policy{plan: plan, base: b, ids: ids, context: declared}, nil
}

// Decide is total: nil error with Allow=false is deny by absence; a non-nil error is
// also a deny, and Decision.Allow is false whenever err != nil. A *TooLarge from the
// engine is wrapped with the remedy: narrow the grants reachable from this principal.
// When the request is well formed and allow is absent, Decision.Why says why.
// A context key the policy did not declare in Config.Context, or one of the wrong kind, is
// ErrRequest.
func (p Policy) Decide(ctx context.Context, req Request) (Decision, error) {
	d := Decision{RuleSets: append([]string(nil), p.ids...)}
	if err := ctx.Err(); err != nil {
		return d, fmt.Errorf("policy: decide: %w", err) // the engine checks at entry, then every 1024 steps
	}
	if err := p.checkRequestContext(req.Context); err != nil {
		return d, fmt.Errorf("policy: decide: %w", err)
	}
	base, err := p.base.With(req.facts()...)
	if err != nil {
		return d, fmt.Errorf("policy: decide: %w: %w", ErrRequest, err)
	}
	res, err := tarski.Evaluate(ctx, p.plan, base, tarski.Options{
		MaxBytes: maxDecisionBytes, MaxSteps: maxDecisionSteps, Proofs: true,
	})
	if err != nil {
		return d, evalError("decide", err)
	}
	// Proofs are on, so a recorded proof is membership in allow/3.
	want := tarski.Tuple{tarski.String(req.Principal), tarski.String(req.Action), tarski.String(req.Resource)}
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
	res, err := tarski.Evaluate(ctx, p.plan, base, tarski.Options{MaxBytes: maxDecisionBytes, MaxSteps: maxDecisionSteps})
	if err != nil {
		return nil, evalError("permitted", err)
	}
	ans, err := res.Relation("permit")
	if err != nil {
		return nil, fmt.Errorf("policy: permitted: %w", err)
	}
	out := make([]string, 0, ans.Len())
	for _, t := range ans.Tuples() {
		// A rule set may derive permit tuples for anyone; the listing is this caller's.
		if !sameTuple(t[:2], tarski.Tuple{tarski.String(principal), tarski.String(action)}) {
			continue
		}
		if t[2].Kind() != tarski.KindString || t[2].Text() == "" {
			return nil, fmt.Errorf("policy: permitted: %w: permit/3 has a third column that is not a non-empty string (%q)", tarski.ErrInvalid, t[2].Text())
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
func sortedFacts(facts []tarski.Fact) []tarski.Fact {
	out := append([]tarski.Fact(nil), facts...)
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

// checkContext copies the declared context and refuses a key or kind that cannot be one.
func checkContext(in map[string]tarski.Kind) (map[string]tarski.Kind, error) {
	out := make(map[string]tarski.Kind, len(in))
	for k, kind := range in {
		if k == "" {
			return nil, fmt.Errorf("%w: Config.Context has an empty key", tarski.ErrInvalid)
		}
		if kind != tarski.KindString && kind != tarski.KindNumber && kind != tarski.KindBool {
			return nil, fmt.Errorf("%w: Config.Context key %q has no such kind (%d)", tarski.ErrInvalid, k, kind)
		}
		out[k] = kind
	}
	return out, nil
}

var baseRelations = map[string]bool{
	"principal": true,
	"action":    true,
	"resource":  true,
	"context":   true,
	"role":      true,
	"inherits":  true,
	"grants":    true,
	"denied":    true,
}

func checkRelations(in map[string]Relation) (map[string]Relation, error) {
	if in == nil {
		return map[string]Relation{}, nil
	}
	out := make(map[string]Relation, len(in))
	for name, rel := range in {
		if name == "" {
			return nil, fmt.Errorf("%w: consumer relation has an empty name", tarski.ErrInvalid)
		}
		if baseRelations[name] {
			return nil, fmt.Errorf("%w: %s is a base relation; it cannot be declared again", tarski.ErrInvalid, name)
		}
		if len(rel.Kinds) == 0 {
			return nil, fmt.Errorf("%w: consumer relation %s has no columns", tarski.ErrInvalid, name)
		}
		kinds := make([]tarski.Kind, len(rel.Kinds))
		for i, k := range rel.Kinds {
			if k != tarski.KindString && k != tarski.KindNumber && k != tarski.KindBool {
				return nil, fmt.Errorf("%w: consumer relation %s has no such kind (%d)", tarski.ErrInvalid, name, k)
			}
			kinds[i] = k
		}
		out[name] = Relation{Kinds: kinds}
	}
	return out, nil
}

// checkRequestContext holds a request's context to the declared keys and kinds. It runs
// before the facts are built so the error names the key and both kinds.
func (p Policy) checkRequestContext(c map[string]tarski.Const) error {
	if len(c) > len(p.context) {
		// More keys than are declared: at least one is undeclared, and this says so
		// before the keys are collected and sorted.
		return fmt.Errorf("%w: %w: the request carries %d context keys, but Config.Context declares %d", ErrRequest, tarski.ErrInvalid, len(c), len(p.context))
	}
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		want, ok := p.context[k]
		if !ok {
			return fmt.Errorf("%w: %w: context key %q is not declared in Config.Context", ErrRequest, tarski.ErrInvalid, k)
		}
		if got := c[k].Kind(); got != want {
			return fmt.Errorf("%w: %w: context key %q is declared %s, but the request gave a %s", ErrRequest, tarski.ErrInvalid, k, kindName(want), kindName(got))
		}
	}
	return nil
}

func requireRelations(plan tarski.Plan) error {
	for _, r := range []struct {
		name string
		want int
	}{{"allow", 3}, {"permit", 3}} {
		name, want := r.name, r.want
		got, ok := plan.Arity(name)
		switch {
		case !ok:
			return fmt.Errorf("%w: no rule set defines %s/%d; load RBAC() first or define it", tarski.ErrInvalid, name, want)
		case got != want:
			return fmt.Errorf("%w: %s is %s/%d, but a decision reads %s/%d", tarski.ErrInvalid, name, name, got, name, want)
		case !plan.Defines(name):
			// Declared and defined nowhere would be an empty relation, and a policy that
			// denies everything without a rule saying so.
			return fmt.Errorf("%w: %s/%d is declared but no rule defines it; load RBAC() first or define it", tarski.ErrInvalid, name, want)
		}
	}
	return nil
}

func evalError(op string, err error) error {
	var tl *tarski.TooLarge
	if errors.As(err, &tl) {
		return fmt.Errorf("policy: %s: %w; narrow the grants reachable from this principal", op, err)
	}
	return fmt.Errorf("policy: %s: %w", op, err)
}

func sameTuple(a, b tarski.Tuple) bool {
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
