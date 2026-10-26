# Shearline — design contract

Fixed API for the first cut. `SPEC.md` says why; this says exactly what. Where the two differ,
this file wins and the difference is listed under *Deviations from the spec*.

Module `github.com/mattjoyce/shearline`, `go 1.25`, one dependency: the engine, `github.com/mattjoyce/tarski` v0.3.1. One package, `policy`; the engine was the second until it moved to its own module. The current release is v0.2.1; [`CHANGELOG.md`](../CHANGELOG.md) lists what changed between releases.

## The engine: `tarski`

The engine is no longer a package of this module. It lives in
[`github.com/mattjoyce/tarski`](https://github.com/mattjoyce/tarski), cut out of
`shearline/datalog` at c4f75ab with its history, and Shearline requires it at v0.3.1. Its
public API, built-ins, evaluation invariants, bounds and the error contract are in tarski's own
`docs/DESIGN.md` and `CHANGELOG.md`; that is the contract now, and this file does not repeat it.
Error messages are prefixed `tarski:` (they were `datalog:`), and numbers at or above 1e6 or below
1e-4 print in full, which is the one change a Shearline answer hash can see.

What the policy layer relies on, stated here because the rest of this file leans on it:
`Compile` refuses an unsafe or unstratifiable program; `Evaluate` is a pure function of (plan,
base, options) with the same answer bytes, proof and hash on every run; a bound crossed is a
`*TooLarge`, never a partial answer; `Proof` and `Absence` render as text and JSON.

Aggregates arrived with tarski v0.2. `policy.Load` refuses any rule set that uses one
(`plan.HasAggregates()`): a decision is per request, and aggregates summarise a relation, which is
a query. The refusal wraps `tarski.ErrInvalid`.

The engine's earlier text, as it stood inside this module, is in git history at c4f75ab
(`docs/DESIGN.md` § Package `datalog`).

## Package `policy`

```go
// Request is the PARC shape: who, does what, to which, under what.
type Request struct {
    Principal string
    Action    string
    Resource  string
    Context   map[string]tarski.Const   // becomes context(Key, Value) facts, keys sorted
}

type Policy struct { /* plan, frozen standing base, rule-set IDs */ }

// Load compiles the rule sets in order over Schema and holds the standing facts. A
// standing fact in a request relation (principal, action, resource, context) is
// refused: it would speak for every request. Each set's Verify must pass, and the
// compiled plan must define allow/3 and permit/3, else ErrInvalid naming the gap:
// what Decide or Permitted would fail on is refused at Load. The
// first set is normally RBAC(). Rule sets are policy, never user input: a later set
// may add rules to any relation, allow/3 included, and the only promise that survives
// an arbitrary set is deny by absence. Any error is the consumer's to fix before
// serving.
// Config is everything a Policy is built from. Context lists every key a request may
// carry, with its kind; a request with any other key, or a declared key of another
// kind, is ErrRequest, and a rule set that names an undeclared key fails at Load.
type Config struct {
    Standing []tarski.Fact
    Sets     []tarski.RuleSet
    Context  map[string]tarski.Kind
}
func Load(cfg Config) (Policy, error)

// Decide is total: nil error with Allow=false is deny by absence; a non-nil error is
// also a deny, and Decision.Allow is false whenever err != nil. A request with an
// empty principal, action or resource is refused by the schema's checks and so denied
// with an error wrapping ErrRequest (the caller's fault; any other error is a broken
// policy or engine). A cancelled ctx is an error before any work. A *TooLarge from the
// engine is wrapped with the remedy: narrow the grants reachable from this principal.
// Membership in allow/3 is read from the proof, which is on. Decision.RuleSets is the
// policy's own copy, never a shared slice.
func (p Policy) Decide(ctx context.Context, req Request) (Decision, error)

// ErrRequest marks a malformed request, apart from a broken policy.
var ErrRequest error

// Permitted is the prefixes, not the resources: the grant patterns principal holds for
// action, the third column of permit/3, sorted and deduplicated. deny/3 does not apply
// (deny is per concrete resource). The third column is a contract a consumer's set must
// keep, a non-empty string; a tuple that breaks it is ErrInvalid, never listed.
func (p Policy) Permitted(ctx context.Context, principal, action string) ([]string, error)

type Decision struct {
    Allow    bool
    Proof    tarski.Proof     // why allowed; zero when denied
    Why      tarski.Absence   // why denied: Result.WhyNot("allow", …); zero when allowed or on error
    RuleSets []string          // the loaded sets' IDs in load order, e.g. "rbac/1 sha256:…"
}
// A deny through deny/3 appears inside Why as the negation `not deny(P, A, R)` that
// held, with Blocked carrying the proof of the deny tuple. Deny by absence shows the
// farthest literal each allow rule reached. Every refusal therefore answers "why" from
// the decision itself, the spec's job story, with no RBAC knowledge in the engine.

// The fact constructors are how a consumer builds standing facts without learning
// the engine's vocabulary. Each returns a tarski.Fact for Load or Decide.
func Role(principal, role string) tarski.Fact
func Inherits(role, parent string) tarski.Fact
func Grant(role, action, prefix string) tarski.Fact
func Deny(principal, action, resource string) tarski.Fact   // a denied/3 fact; rbac.dl derives deny/3 from it

// Schema is the PARC base relations, a fresh value per call so no caller can mutate
// what a later Load compiles against. Consumers' standing facts use role, inherits,
// grants, denied; a request supplies principal, action, resource, context. Every
// argument of every relation but context's value is checked to be a non-empty string
// (Schema.Checks): an empty principal, role, action or prefix is a missing value, not a
// wildcard, and is refused at Load or at the request, never silently matched.
func Schema() tarski.Schema   // returns tarski.Schema{
    Arities: map[string]int{
        "principal": 1, "action": 1, "resource": 1, "context": 2,
        "role": 2, "inherits": 2, "grants": 3, "denied": 3,
    },
    Checks: /* nonEmptyString on every position above except context's value */,
}

// RBAC is the bundled rule set, rbac.dl, embedded and parsed once at package init
// (var rbac = mustRuleSet(...)): a broken asset fails at process start, never at the
// first request, and TestRBAC_Parses holds it.
func RBAC() tarski.RuleSet
```

`rbac.dl` (name `rbac`, version 1). Every rule is seeded by the request: the engine evaluates
bottom-up with a per-request budget, so a rule that derived every principal's roles would spend
the budget on strangers and trip the byte bound on a large standing base (found by the module
panel, confirmed at 50,000 principals).

The rules, as in `policy/rbac.dl` (that file is authoritative; its comments say why each rule is
seeded by the request):

```
deny(P, A, R) :- principal(P), action(A), resource(R), denied(P, A, R).

holds(P, R) :- principal(P), role(P, R).
holds(P, R) :- holds(P, Q), inherits(Q, R).

permit(P, A, Pat) :- principal(P), action(A), holds(P, Role), grants(Role, A, Pat).

allow(P, A, R) :- permit(P, A, Pat), resource(R), prefix(Pat, R), not deny(P, A, R).
```

A consumer set may add rules to `deny/3`; one that reads `allow/3` is a negation cycle and is
refused at Load.

`Decide` evaluates with `Options{MaxBytes: maxDecisionBytes, MaxSteps: maxDecisionSteps, Proofs:
true}` and reads `allow(principal, action, resource)`. Both are named constants, invariants not
knobs: `maxDecisionBytes = 1 << 20` because a decision over one request should never derive a
megabyte, and `maxDecisionSteps = 1 << 22` because a decision is a few joins over one principal's
roles and a policy that visits millions of tuples for one request is wrong, not large. `Permitted`
evaluates with `principal` and `action` facts only and reads `permit/3`.
Since tarski v0.2.2 a step is also charged per rule application and per built-in call, not only
per tuple visited; the bound is unchanged and the 50,000-principal test still passes under it.

Vocabulary rule: "importer", "source", "projection", "tenant" never appear in the module.

## Deviations from the spec

| Spec | Here | Why |
|---|---|---|
| `Schema` gains `Check func(Atom) error` | `Checks map[string]func(Atom) error` | per relation, as the sentence meant |
| `With(facts) Base` | `With(facts) (Base, error)` | a wrong-arity fact must fail closed, not panic |
| `Load(rules RuleSet, standing ...Fact)` | `Load(standing []Fact, sets ...RuleSet)` | the consumer extends RBAC with its own set (deny rules, extra roles) |
| `Decision.RuleSet` one ID | `RuleSets []string` | one decision may stand on two sets; a list, not a string to split |
| `Options{MaxBytes, Proofs}` | plus `MaxSteps` | bytes bound what is derived, not the work to derive it |
| `grants(..., "timeline/*")` | `grants(..., "timeline/")`, matched by `prefix/2` | `*` is not a prefix; glob matching is a later built-in if a consumer needs it |
| `Compile` keeps Anam's reachability check | removed | declared-and-undefined is the hook mechanism |
| `deny/3` declared only | `denied/3` base facts, `deny/3` derived from them | a Deny fact is a fact; a relation cannot be both base and declared (found by the policy teammate) |

## Review

Luminary panel (Hickey × Armstrong, Ousterhout × Liskov) before any code:
`docs/reviews/luminary-design-2026-10-22/`. The collation lists each finding and where it landed
above. Red/blue file audit at `5fe3a78`: `docs/reviews/file-red-blue-2026-10-22/`, with its
resolution; the contract above is as amended by it.

## Decided with Matt

1. **Resource matching (2026-10-22).** Grants stay plain string prefixes and a resource name is an
   opaque string. The consumer ends a prefix with its separator and makes names canonical before
   Decide. No `glob/2`, no canonicalisation in `policy`. If a consumer shows segmented names, add
   `under(Sep, Pat, R)`: true when R equals Pat or starts with Pat then Sep, the separator the
   consumer's, so Shearline still names no vocabulary.

2. **A proof names its rule set (2026-10-22).** `Rule` gains `Set` ("name/version", filled by
   `Compile` from `Program.Name`, which `NewRuleSet` sets; empty for a bare `Parse`) and `Text` (the
   rule as written, sliced by the parser). A proof's rule therefore says which set, which line and
   what text; the hash stays in `Decision.RuleSets`, and a stored receipt keeps both.

3. **A refusal explains itself (2026-10-22).** Matt chose transparency over minimalism: `Decide`
   fills `Decision.Why` with why-not provenance from the engine (`Result.WhyNot`), deterministic
   and RBAC-blind, with the blocking deny's proof inside it when `deny/3` fired. Proof and Absence
   both render as text and compact JSON, so a consumer can log or store a receipt without
   learning the AST. The value is partly future state; the cost is one engine walk per refusal.
   Proof and Absence JSON are frozen in tarski, and since v0.3.1 `TestGolden_ProofAndAbsenceJSON`
   pins them byte for byte, so a receipt's JSON shape is a frozen promise of the engine. Answers
   and hashes depend on the set of facts, but proofs, witness order and why-not bindings also
   depend on fact order. `policy` fixes that order: `Load` sorts the standing facts and a request's
   facts are built as principal, action, resource, then context keys sorted by key. A consumer
   that pins proof bytes from its own base sorts its facts.

4. **Context keys are declared or refused (2026-10-22).** Matt: "code in the future is mostly AI
   generated, therefore be explicit." `Load` takes `Config{Standing, Sets, Context map[string]Kind}`.
   A request key that is undeclared, or declared with another kind, is `ErrRequest`; a policy that
   declares no keys accepts no context. The declaration is also the schema check on `context/2`,
   so a consumer rule naming an undeclared key or comparing a declared key with a constant of the
   wrong kind fails at `Load`. Nothing is inferred and there is no default-permissive state.
   Known limit: the check sees atoms, not comparisons, so `context("hour", H), H > "22"` compiles
   and is simply never true, since H is a declared number; the declared kind makes such a rule
   dead rather than dangerous. Catching it would need kind inference over comparisons.

5. **The inverse listing is the loop (2026-10-22).** "Who may do A on R" is answered by calling
   `Decide` once per principal the consumer knows; the listing is the gate, so it cannot disagree
   with it, and it carries context if the caller supplies it. Shearline plans no `Principals` call
   and no second rule set: at the scale its consumers have, tens of tokens, the loop costs
   microseconds, and a second evaluation mode would be a thing to keep consistent for a scale that
   does not exist. Revisit only when a consumer has many thousands of principals and asks often.

6. **A relation is a value or a builder, never both (2026-10-22).** Hickey's cut, not base-versus-
   derived: during evaluation every relation is a builder that accepts tuples; when `Evaluate`
   returns every relation is sealed into a value with no write method, base relations with their
   indexes, derived ones without. `Base` and `Result` hold values only, so concurrent reads,
   `Proof` and `WhyNot` are safe by construction and the `frozen` flag, its panic and the scan
   workaround go. A pure refactor gated by the benchmark: kept only within noise.

## Open, for Matt

None. The six questions above were decided with Matt on 2026-10-22.
