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
type Const; func String(string) Const; func Bool(bool) Const
func Number(lexeme string) (Const, error)      // a JSON number lexeme, finite (else ErrSyntax); -0 is 0
func Int(int64) Const; func Float(float64) (Const, error)   // the same constants Number makes; Float refuses NaN/Inf with ErrInvalid (no lexeme to blame)
// The string domain is valid UTF-8: the lexer refuses a literal that is not,
// NewBase/With refuse a fact that is not, and Compile refuses a hand-built constant
// that is not, since JSON would fold invalid bytes into U+FFFD and two distinct
// constants would share an Answer hash. A string literal's escapes are JSON's or
// Go's: what a JSON encoder writes (\/, a surrogate pair) and what a person writes
// (\x41, \U0001F600) both read; a lone surrogate reads as neither. Numbers are
// float64: an integer beyond 2^53 is not an exact identifier; use a string for one.
// A relation name is a lower-case letter then letters, digits and underscores, and a
// variable an upper-case letter or underscore then the same, wherever it comes from:
// Compile holds a hand-built Program and a Schema to it, as the lexer holds text.
type Tuple []Const                                     // unchanged
type Term, Atom, Comparison, Literal, Rule, Program    // unchanged
func Parse(text string) (Program, error)               // unchanged

// Schema names the base relations a base may hold and, per relation, an optional
// check. The check runs on every body atom that names the relation at Compile (a
// head never names a base relation), and on every fact of the relation at
// NewBase/With (as an Atom of constants), so the schema's word holds everywhere a
// tuple enters. Its error is wrapped, so errors.Is on the caller's sentinel works. A
// Schema that carries checks is not a comparable value; keep every other field plain data.
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
// carries Bound, BoundBytes or BoundSteps, and its message starts "datalog:".
// A base that holds a relation the plan derives is ErrInvalid at Evaluate, as is one
// that lacks a relation the plan reads. Compatibility between a plan and a base is by
// shape only, names and arities: a base built under a schema with different Checks
// passes, because its facts were checked by its own schema at NewBase. A caller that
// wants one set of checks everywhere compiles and builds from one Schema value, as
// policy.Load does.
func Evaluate(ctx context.Context, plan Plan, base Base, opts Options) (Result, error)

func (r Result) Relation(name string) (Answer, error)
// Answer is a value with nothing to change: Relation(), Tuples() (a copy), Len(),
// Bytes() (a copy of its one serialisation), Hash() ("sha256:" over Bytes). It
// marshals as Bytes, so what is sent, stored and hashed is one thing; a zero Answer
// marshals to an error. UnmarshalJSON reads back only an answer as written (a
// relation name; tuples sorted, no repeats, one width) and seals it again.
type Answer

// Proof is how one tuple came to be. A base fact has Rule nil and no Body. A derived
// tuple has the rule that first produced it and, in the rule's written order, the
// positive body atoms' tuples, each with its own proof. Built-ins and comparisons and
// negations contribute no body entry. The structure is finite and acyclic: a body
// tuple was always derived strictly before its head. It is a DAG shared as values:
// Result.Proof builds each (relation, tuple) proof once and reuses that value wherever
// it appears in a body, so the size is linear in the derivation records.
//
// Proofs are deterministic: for one (plan, base, options) every evaluation yields the
// same proof for every tuple; a base is its fact sequence, so policy.Load sorts
// standing facts and a proof is a function of the policy's content. Rule is a deep
// copy the holder may keep or change; the plan is not reachable through a proof. It
// carries Set ("name/version" of the rule set Compile took it from, "" for a bare
// Program), Line, and Text (the rule as written), so a receipt can name its rule.
type Proof struct {
    Relation string
    Tuple    Tuple
    Rule     *Rule
    Body     []Proof
}
// Proof is the proof of t in relation name; ok is false when t is not in the result or
// the evaluation ran without Options.Proofs.
func (r Result) Proof(name string, t Tuple) (p Proof, ok bool)

// Absence is why a tuple was not derived: why-not provenance. For a goal not in the
// result, every rule whose head unifies with it is walked, body in evaluation order,
// against the result's relations; each rule reports the farthest literal that found no
// match and the bindings it had there. A positive literal over a derived relation that
// found nothing is explained recursively (depth-bounded, visited set, so recursion
// through holds/2 terminates). A negation that held reports the proof of the tuple that
// blocked it. A goal over a base relation with no rules is simply "no such fact".
// Options.Proofs must be on. Deterministic for one (plan, base, goal). Engine-level: it
// knows nothing of RBAC, and for rbac.dl it reads as "no role fact for P", "no grant for
// A on a held role", "no permitted prefix matches R", or "denied by <proof>".
type Absence struct {
    Relation  string
    Goal      []Term           // constants, or variables where the caller left a position open
    Rules     []RuleFailure    // one per candidate rule; empty for a base relation
    Truncated bool             // the walk hit whyNotSteps or maxDepth; what is here is still true
}
// "Farthest literal": a depth-first walk over the rule's ordered body, extending
// bindings through every match of each literal (matches in relation order, rules in
// plan order, so the answer is deterministic); the literal reported is the one with the
// greatest index that any path reached and found no match, with that path's bindings.
// A body that matches fully would have derived the goal, so it cannot occur against the
// result it came from; it shows as a RuleFailure with a zero Failed. The walk is bounded
// by whyNotSteps = 1 << 16 candidates tried (each matched literal branches) and by
// maxDepth nested Because levels; Truncated is set where the cut fell and on every
// Absence above it. A goal already explained in the same Absence is a cycle, not a cut:
// it is left with an empty Because and does not set Truncated. A failed positive atom
// over a base relation gets an Absence with no Rules, "no such fact". Derived relations
// are scanned, not indexed, since a shared Result's lazy index would race.
type RuleFailure struct {
    Rule    *Rule              // a copy, as in Proof
    Bound   map[string]Const   // bindings when the walk stopped
    Failed  Literal            // the farthest literal with no match (negation: one that held)
    Because []Absence          // why Failed found nothing, when it is a derived positive atom
    Blocked *Proof             // when Failed is a negation that held: the tuple that blocked
}
// WhyNot explains a tuple's absence. ok is false when t is in the result (ask Proof),
// when name is not a relation of the plan or t is not its arity, or when the evaluation
// ran without Options.Proofs. maxDepth bounds the recursion; 0 means the
// engine's default (whyNotDepth = 8, enough for any policy rule set, small enough that a
// pathological recursion stays cheap).
func (r Result) WhyNot(name string, t Tuple, maxDepth int) (a Absence, ok bool)

// Both Proof and Absence render: String() is an indented derivation for a log or an
// operator; MarshalJSON emits relation, tuple, rule as {set, line, text}, and body or
// rules recursively, never the parser's AST. A proof writes each (relation, tuple)
// once; a later occurrence is {relation, tuple, rule: null, shown: true}, as the text
// says "(shown above)", so a document is linear in the derivation.
func (p Proof) String() string;   func (p Proof) MarshalJSON() ([]byte, error)
func (a Absence) String() string; func (a Absence) MarshalJSON() ([]byte, error)

// RuleSet is a named, versioned program with a hash over its text.
type RuleSet struct {
    Name    string
    Version int
    Hash    string   // "sha256:<hex>" over Text
    Program Program
}
// NewRuleSet parses text and hashes it, and names the program "name/version". The name
// must be non-empty with no whitespace or control character, slash or semicolon (ID
// is "name/version hash" and decisions join several); the version must not be
// negative; the text must not be empty, since Verify would refuse it. Else ErrInvalid.
func NewRuleSet(name string, version int, text string) (RuleSet, error)
// ID is "name/version hash", the string a decision carries.
func (s RuleSet) ID() string
// Verify holds a RuleSet to its identity and its text: the name and version are
// held to NewRuleSet's rule, the hash is recomputed over Program.Text, and the text
// is parsed again and must equal Program, so the rules that run are the rules that
// were hashed. A set built or edited by hand that fails any is ErrInvalid.
// policy.Load calls it, so the ID in a decision is the hash of the rules that ran.
// Compile also deep-copies every program, so a caller's later edits reach no plan.
func (s RuleSet) Verify() error

// Arity reports a relation's arity in the plan, base or derived, and whether it exists.
func (p Plan) Arity(name string) (int, bool)
// Defines reports whether some rule has name as its head: a relation only declared
// has an arity and no rule. policy.Load requires both of allow/3 and permit/3.
func (p Plan) Defines(name string) bool
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

- **Concurrency.** A `Base`, a `Plan` and a `Result` are values: immutable once built and safe
  to share across goroutines. A relation is either a value (`relation`: read methods only, no
  `add`) or a builder (`relBuilder`: the place tuples are added, held only by the evaluator while
  `Evaluate` runs). Base relations are sealed with an index per argument position; derived
  relations are sealed without indexes when `Evaluate` returns and answer lookups by scanning.
  Nothing a caller can hold has a write method, so the safety is structural (decision 6). Anam's
  lazy-index mutation on shared relations would have raced under a concurrent `Policy`.
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
// Config is everything a Policy is built from. Context lists every key a request may
// carry, with its kind; a request with any other key, or a declared key of another
// kind, is ErrRequest, and a rule set that names an undeclared key fails at Load.
type Config struct {
    Standing []datalog.Fact
    Sets     []datalog.RuleSet
    Context  map[string]datalog.Kind
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
    Proof    datalog.Proof     // why allowed; zero when denied
    Why      datalog.Absence   // why denied: Result.WhyNot("allow", …); zero when allowed or on error
    RuleSets []string          // the loaded sets' IDs in load order, e.g. "rbac/1 sha256:…"
}
// A deny through deny/3 appears inside Why as the negation `not deny(P, A, R)` that
// held, with Blocked carrying the proof of the deny tuple. Deny by absence shows the
// farthest literal each allow rule reached. Every refusal therefore answers "why" from
// the decision itself, the spec's job story, with no RBAC knowledge in the engine.

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
