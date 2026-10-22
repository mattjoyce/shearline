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
// compile-time check run on every atom (head or body) that names it.
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

// With is the standing base plus more facts, as a new value. Relations the new facts
// do not touch are shared, not copied; a relation they do touch is rebuilt from the
// standing tuples plus the new ones. Same validation as NewBase.
func (b Base) With(facts ...Fact) (Base, error)

func (b Base) Len(relation string) int                 // unchanged

func Compile(schema Schema, programs ...Program) (Plan, error)   // Definitions parameter gone

type Options struct {
    MaxBytes int   // over every derived tuple's JSON text; 0 is no bound; over it, *TooLarge
    Proofs   bool  // record, for each derived tuple, the first derivation that produced it
}
func Evaluate(ctx context.Context, plan Plan, base Base, opts Options) (Result, error)

func (r Result) Relation(name string) (Answer, error)  // unchanged
type Answer                                            // unchanged (sorted tuples, canonical JSON, sha256)

// Proof is how one tuple came to be. A base fact has Rule nil and no Body. A derived
// tuple has the rule that first produced it and, in the rule's written order, the
// positive body atoms' tuples, each with its own proof. Built-ins and comparisons and
// negations contribute no body entry. The tree is finite and acyclic: a body tuple was
// always derived strictly before its head.
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

- **Concurrency.** A `Base` is immutable once built and safe for concurrent `Evaluate` calls. Base
  relations are *frozen* at `NewBase`/`With`: every per-position index is built then, and
  `relation.candidates` never mutates a frozen relation. Derived relations keep the lazy index.
  Anam's lazy-index mutation on shared base relations would race under a concurrent `Policy`.
- **Termination.** No function symbols, no built-in produces a value; evaluation reaches a
  fixpoint. The deadline is checked every `stepsPerDeadlineCheck = 1024` join steps (unchanged).
- **Declared and undefined is empty.** Anam's `checkDefined` is removed. A relation a program
  declares (`deny/3.`) and nothing defines evaluates as the empty relation. This is how a bundled
  rule set exposes a hook the consumer may fill. Anam re-adds its `instant/1` check in its own
  layer if it wants it; `Schema.Checks` is not the place (it sees one atom, not reachability).

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

// Load compiles the rule sets in order over Schema and holds the standing facts. The
// first set is normally RBAC(). Any error is the consumer's to fix before serving.
func Load(standing []datalog.Fact, sets ...datalog.RuleSet) (Policy, error)

// Decide is total: nil error with Allow=false is deny by absence; a non-nil error is
// also a deny, and Decision.Allow is false whenever err != nil.
func (p Policy) Decide(ctx context.Context, req Request) (Decision, error)

// Permitted lists the grant patterns principal holds for action: the third column of
// permit/3. It is the prefixes, not concrete resources, and deny/3 does not apply to
// it (deny is per concrete resource). Sorted, deduplicated.
func (p Policy) Permitted(ctx context.Context, principal, action string) ([]string, error)

type Decision struct {
    Allow   bool
    Proof   datalog.Proof   // zero value when denied
    RuleSet string          // the loaded sets' IDs joined by "; ", e.g. "rbac/1 sha256:…"
}

// Schema is the PARC base relations. Consumers' standing facts use role, inherits,
// grants; a request supplies principal, action, resource, context.
var Schema = datalog.Schema{Arities: map[string]int{
    "principal": 1, "action": 1, "resource": 1, "context": 2,
    "role": 2, "inherits": 2, "grants": 3,
}}

// RBAC is the bundled rule set, rbac.dl, embedded.
func RBAC() datalog.RuleSet
```

`rbac.dl` (name `rbac`, version 1):

```
% deny(P, A, R) is the consumer's hook: declared here, defined by a later rule set or
% not at all. Absent, nothing is denied by it.
deny/3.

holds(P, R) :- role(P, R).
holds(P, R) :- holds(P, Q), inherits(Q, R).

% permit(P, A, Pat): P holds a role that grants A over resources with prefix Pat.
permit(P, A, Pat) :- principal(P), action(A), holds(P, Role), grants(Role, A, Pat).

% allow(P, A, R): the request's resource is under a permitted prefix and not denied.
allow(P, A, R) :- permit(P, A, Pat), resource(R), prefix(Pat, R), not deny(P, A, R).
```

`Decide` evaluates with `Options{MaxBytes: maxDecisionBytes, Proofs: true}` and reads
`allow(principal, action, resource)`. `maxDecisionBytes = 1 << 20` is a named constant: a
decision over one request should never derive a megabyte, and a policy that does is wrong, not
large. `Permitted` evaluates with `principal` and `action` facts only and reads `permit/3`.

Vocabulary rule: "importer", "source", "projection", "tenant" never appear in the module.

## Deviations from the spec

| Spec | Here | Why |
|---|---|---|
| `Schema` gains `Check func(Atom) error` | `Checks map[string]func(Atom) error` | per relation, as the sentence meant |
| `With(facts) Base` | `With(facts) (Base, error)` | a wrong-arity fact must fail closed, not panic |
| `Load(rules RuleSet, standing ...Fact)` | `Load(standing []Fact, sets ...RuleSet)` | the consumer extends RBAC with its own set (deny rules, extra roles) |
| `Decision.RuleSet` one ID | IDs joined by `; ` | one decision may stand on two sets |
| `grants(..., "timeline/*")` | `grants(..., "timeline/")`, matched by `prefix/2` | `*` is not a prefix; glob matching is a later built-in if a consumer needs it |
| `Compile` keeps Anam's reachability check | removed | declared-and-undefined is the hook mechanism |

## Open, for Matt

1. A `glob/2` built-in over `path.Match` semantics, if Ductile's resource names want `*`.
2. Whether `Permitted` should also exist per resource: "who may read R" (a `principals/3`
   relation). Cheap to add once Ductile's real requests are written down.
