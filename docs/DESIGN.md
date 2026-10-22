# Shearline — design contract

Fixed API for the first cut. `SPEC.md` says why; this says exactly what. Where the two differ,
this file wins and the difference is listed under *Deviations from the spec*.

Module `github.com/mattjoyce/shearline`, `go 1.25`, stdlib only. Two packages: `datalog`, `policy`.

## Package `datalog`

Carried from Anam's `internal/query` with the package renamed. Error messages are prefixed
`datalog:` where they were `query:`.

```go
var ErrSyntax, ErrInvalid, ErrTooLarge error          // the three; *TooLarge still Is(ErrTooLarge)

type Kind; const KindString, KindNumber, KindBool      // unchanged
type Const; func String(string) Const; func Bool(bool) Const; func Number(lexeme string) (Const, error)
type Tuple []Const                                     // unchanged
type Term, Atom, Comparison, Literal, Rule, Program    // unchanged
func Parse(text string) (Program, error)               // unchanged

// Schema names the base relations a base may hold and, per relation, an optional
// check. The check runs on every atom (head or body) that names the relation at
// Compile, and on every fact of the relation at NewBase/With (as an Atom of
// constants), so the schema's word holds everywhere a tuple enters. A Schema that
// carries checks is not a comparable value; keep every other field plain data.
type Schema struct {
    Arities map[string]int
    Checks  map[string]func(Atom) error               // nil map or nil entry: no check
}

// Fact is one tuple of a base relation.
type Fact struct {
    Relation string
    Tuple    Tuple
}

// NewBase holds facts under schema. A fact of a relation the schema lacks, or of the
// wrong arity, is ErrInvalid. Every relation in schema.Arities exists, empty or not.
func NewBase(schema Schema, facts ...Fact) (Base, error)

// With is the standing base plus more facts, as a new value. Postcondition, and the
// property a test holds over random fact sets: b.With(f...) is observationally equal
// to NewBase(schema, standing..., f...) in Len, in every Evaluate, and in every
// Answer hash. Implementation: relations the new facts do not touch are shared, not
// copied; a relation they do touch is rebuilt. Same validation as NewBase.
func (b Base) With(facts ...Fact) (Base, error)

func (b Base) Len(relation string) int                 // unchanged

func Compile(schema Schema, programs ...Program) (Plan, error)   // Definitions parameter gone

type Options struct {
    MaxBytes int   // over every derived tuple's JSON text; 0 is no bound; over it, *TooLarge
    MaxSteps int   // join steps (tuples visited) over the whole evaluation; 0 is no bound; over it, *TooLarge
    Proofs   bool  // record, for each derived tuple, the first derivation that produced it
}
// MaxBytes bounds what is derived; MaxSteps bounds the work to derive it. A rule that
// joins a wide product and derives only duplicates is charged no bytes, so without
// MaxSteps its termination would rest on the caller's deadline alone. *TooLarge
// carries which bound was crossed.
func Evaluate(ctx context.Context, plan Plan, base Base, opts Options) (Result, error)

func (r Result) Relation(name string) (Answer, error)  // unchanged
type Answer                                            // unchanged (sorted tuples, canonical JSON, sha256)

// Proof is how one tuple came to be. A base fact has Rule nil and no Body. A derived
// tuple has the rule that first produced it and, in the rule's written order, the
// positive body atoms' tuples, each with its own proof. Built-ins and comparisons and
// negations contribute no body entry. The structure is finite and acyclic: a body
// tuple was always derived strictly before its head. It is a DAG shared as values:
// Result.Proof builds each (relation, tuple) proof once and reuses that value wherever
// it appears in a body, so the size is linear in the derivation records.
//
// Proofs are deterministic: for one (plan, base, options) every evaluation yields the
// same proof for every tuple. Rule points into the plan's rules, shared not copied,
// and is never written through.
type Proof struct {
    Relation string
    Tuple    Tuple
    Rule     *Rule
    Body     []Proof
}
// Proof is the proof of t in relation name; ok is false when t is not in the result or
// the evaluation ran without Options.Proofs.
func (r Result) Proof(name string, t Tuple) (p Proof, ok bool)

// RuleSet is a named, versioned program with a hash over its text.
type RuleSet struct {
    Name    string
    Version int
    Hash    string   // "sha256:<hex>" over Text
    Program Program
}
// NewRuleSet parses text and hashes it.
func NewRuleSet(name string, version int, text string) (RuleSet, error)
// ID is "name/version hash", the string a decision carries.
func (s RuleSet) ID() string
// Verify recomputes the hash over Program.Text; a RuleSet built or edited by hand
// whose Hash does not match is ErrInvalid. policy.Load calls it, so the ID in a
// decision is the hash of the rules that ran.
func (s RuleSet) Verify() error

// Arity reports a relation's arity in the plan, base or derived, and whether it exists.
func (p Plan) Arity(name string) (int, bool)
```

### Built-ins

Three reserved relations over strings, each a guard like a comparison: both arguments must be
bound by a positive body atom (else `ErrInvalid` at compile), they bind nothing, they derive
nothing, and they never appear in a head, a declaration, or a schema (else `ErrInvalid`).
Argument order is pattern first, subject second. A non-string argument makes the guard false.

| Atom | True when |
|---|---|
| `prefix(P, S)` | `strings.HasPrefix(S, P)` |
| `suffix(P, S)` | `strings.HasSuffix(S, P)` |
| `contains(P, S)` | `strings.Contains(S, P)` |

Implementation: `builtins.go` holds `var builtins = map[string]builtin` keyed by name, with
`type builtin struct{ arity int; eval func(args []Const) bool }`. `plan.go` consults it in
`relations`/`checkAtom`/`checkRule`; `stratify.go`'s `order`/`ready` treats a built-in literal as a
guard; `eval.go`'s `join` evaluates it where it evaluates a `Comparison`.

### Evaluation invariants

- **Concurrency.** A `Base` and a `Plan` are immutable once built and safe for concurrent
  `Evaluate` calls; a `Result` is read-only and safe to share. Base relations are *frozen* at
  `NewBase`/`With`: every per-position index is built then, `relation.candidates` never mutates a
  frozen relation, and a frozen relation's `add` panics (a programming error, not a condition).
  Derived relations are created per evaluation and keep the lazy index. Anam's lazy-index
  mutation on shared base relations would race under a concurrent `Policy`.
- **Determinism.** `Evaluate` is a function of (plan, base, options): same inputs, same `Answer`
  hashes and the same `Proof` for every tuple. Every walk over a map on the derivation path
  (`merge` over the round's delta above all) iterates in sorted relation order, since insertion
  order decides which derivation is first. A test evaluates twice and compares proofs deeply.
- **Termination.** No function symbols, no built-in produces a value; evaluation reaches a
  fixpoint. The deadline is checked every `stepsPerDeadlineCheck = 1024` join steps (unchanged).
- **Declared and undefined is empty.** Anam's `checkDefined` is removed. A relation a program
  declares and nothing defines evaluates as the empty relation, so a rule set can leave a hook
  open; the bundled `rbac.dl` does not use it (its `deny/3` is defined from `denied/3`), a
  consumer set may. Anam re-adds its `instant/1` check in its own layer if it wants it;
  `Schema.Checks` is not the place (it sees one atom, not reachability).
- **Bounds are approximate on bytes, exact on steps.** `MaxBytes` charges unescaped lengths, an
  estimate of the JSON text; `MaxSteps` counts every candidate visited by a join or a negation
  probe. Neither can be crossed with a nil error.

### What leaves (stays in Anam)

`base.go` (fold-backed), `describe.go`, `check.go`, `run.go`, `rules/anam.dl`, `Bundled()`,
`RelInstant`/`RelEntry`/`RelAttr`/`RelCloses`, `ReadsAttr`, `Versions`, `checkInstant`,
`checkPointer`, `leaves`/`escape`, `ErrBase`, and the tests `ledger_test`, `edges_test`,
`lazy_attr_test`, `container_pointer_test`, `container_test`, `docs_test`, `describe_test`,
`check_test`, `equality_test`. `eval_test`, `plan_test`, `parse_test`, `base_test` stay and are
rewritten onto `NewBase(schema, facts...)` with a small test schema of their own.

## Package `policy`

```go
// Request is the PARC shape: who, does what, to which, under what.
type Request struct {
    Principal string
    Action    string
    Resource  string
    Context   map[string]datalog.Const   // becomes context(Key, Value) facts, keys sorted
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
func Load(standing []datalog.Fact, sets ...datalog.RuleSet) (Policy, error)

// Decide is total: nil error with Allow=false is deny by absence; a non-nil error is
// also a deny, and Decision.Allow is false whenever err != nil. A request with an
// empty principal, action or resource is refused by the schema's checks and so denied
// with an error, not special-cased. A cancelled ctx is an error before any work. A
// *TooLarge from the engine is wrapped with the remedy: narrow the grants reachable
// from this principal. Decision.RuleSets is the policy's own copy, never a shared slice.
func (p Policy) Decide(ctx context.Context, req Request) (Decision, error)

// Permitted is the prefixes, not the resources: the grant patterns principal holds for
// action, the third column of permit/3, sorted and deduplicated. deny/3 does not apply
// (deny is per concrete resource).
func (p Policy) Permitted(ctx context.Context, principal, action string) ([]string, error)

type Decision struct {
    Allow    bool
    Proof    datalog.Proof   // zero value when denied
    RuleSets []string        // the loaded sets' IDs in load order, e.g. "rbac/1 sha256:…"
}

// The fact constructors are how a consumer builds standing facts without learning
// the engine's vocabulary. Each returns a datalog.Fact for Load or Decide.
func Role(principal, role string) datalog.Fact
func Inherits(role, parent string) datalog.Fact
func Grant(role, action, prefix string) datalog.Fact
func Deny(principal, action, resource string) datalog.Fact   // a denied/3 fact; rbac.dl derives deny/3 from it

// Schema is the PARC base relations, a fresh value per call so no caller can mutate
// what a later Load compiles against. Consumers' standing facts use role, inherits,
// grants, denied; a request supplies principal, action, resource, context. Every
// argument of every relation but context's value is checked to be a non-empty string
// (Schema.Checks): an empty principal, role, action or prefix is a missing value, not a
// wildcard, and is refused at Load or at the request, never silently matched.
func Schema() datalog.Schema   // returns datalog.Schema{
    Arities: map[string]int{
        "principal": 1, "action": 1, "resource": 1, "context": 2,
        "role": 2, "inherits": 2, "grants": 3, "denied": 3,
    },
    Checks: /* nonEmptyString on every position above except context's value */,
}

// RBAC is the bundled rule set, rbac.dl, embedded and parsed once at package init
// (var rbac = mustRuleSet(...)): a broken asset fails at process start, never at the
// first request, and TestRBAC_Parses holds it.
func RBAC() datalog.RuleSet
```

`rbac.dl` (name `rbac`, version 1). Every rule is seeded by the request: the engine evaluates
bottom-up with a per-request budget, so a rule that derived every principal's roles would spend
the budget on strangers and trip the byte bound on a large standing base (found by the module
panel, confirmed at 50,000 principals).

```
% deny(P, A, R): the consumer's denials. The base relation denied/3 carries them as
% facts (policy.Deny); a later rule set may add rules to deny/3 for denials that are
% derived rather than listed.
deny(P, A, R) :- principal(P), action(A), resource(R), denied(P, A, R).

holds(P, R) :- principal(P), role(P, R).
holds(P, R) :- holds(P, Q), inherits(Q, R).

% permit(P, A, Pat): P holds a role that grants A over resources with prefix Pat.
permit(P, A, Pat) :- principal(P), action(A), holds(P, Role), grants(Role, A, Pat).

% allow(P, A, R): the request's resource is under a permitted prefix and not denied.
allow(P, A, R) :- permit(P, A, Pat), resource(R), prefix(Pat, R), not deny(P, A, R).
```

`Decide` evaluates with `Options{MaxBytes: maxDecisionBytes, MaxSteps: maxDecisionSteps, Proofs:
true}` and reads `allow(principal, action, resource)`. Both are named constants, invariants not
knobs: `maxDecisionBytes = 1 << 20` because a decision over one request should never derive a
megabyte, and `maxDecisionSteps = 1 << 22` because a decision is a few joins over one principal's
roles and a policy that visits millions of tuples for one request is wrong, not large. `Permitted`
evaluates with `principal` and `action` facts only and reads `permit/3`.

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
above.

## Open, for Matt

1. A `glob/2` built-in over `path.Match` semantics, if Ductile's resource names want `*`; and
   whether grant prefixes should be segment-aware (`timeline/health` matching `timeline/healthy`
   today) or resources canonicalised at the boundary. Shearline has no path semantics by design;
   the consumer owns the vocabulary and the canonical form. Raised by the security review.
2. `Proof.Rule` as a pointer into the plan carries no rule-set identity, so "line 4" is ambiguous
   once two sets are loaded, and a receipt that is serialised keeps nothing of the pointer. A
   value `{RuleSet, Line, Text}` would survive serialisation; the pointer buys only size.
3. A denied request carries no proof. The spec wants the derivation behind a refusal; a deny
   through `deny/3` could carry the proof of the `deny` tuple that blocked it, while deny by
   absence has none. Intent versus contract.
4. Typed context keys (spec open question 4): a deny rule comparing a context value of the wrong
   kind silently does not fire, since mismatched kinds compare false. Declaring kinds per key
   would catch it at the request; today the rule must guard the kind itself.
5. Whether `Permitted` should also exist per resource: "who may read R" (a `principals/3`
   relation). Cheap to add once Ductile's real requests are written down.
