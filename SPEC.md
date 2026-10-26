# Shearline — project spec (draft)

> History: this is the original draft. The `datalog` package it describes is now the separate module [tarski](https://github.com/mattjoyce/tarski), required at v0.3.2 (see `docs/ENGINE.md`). Where this draft says "stdlib only", "two packages" or `shearline/datalog`, read the notes marked History.

Drafted 2026-10-22 by an agent on Matt's behalf, from a read of Anam's `internal/query` at
master `bf0b0f9` and the policy survey `reviews/policy-touch-points-2026-10-22.md` (card 223).
Marked as drafted, for Matt to correct.

## What it is

Shearline is a Go module that answers one question: **may this bearer take this action on this
asset in this context?** The question is written as facts, the policy as Datalog rules, and the
answer is derived, deterministic, and hashable. It is a general-purpose policy engine for RBAC and
every problem of that shape: scoped tokens, tenant boundaries, config-driven grants, "what may this
caller see".

It is a library, not a service. It owns no state, reads no file and no network, and knows no
vocabulary of its own. The caller brings the facts; Shearline derives the decision.

The engine is Anam's Datalog evaluator (card 173), cleaved out of `internal/query` and made
general. Anam imports it back as its first consumer; Ductile's token and config wrangling is the
second.

## Job stories

- When I add a new rule about who may do what, I want to write it as one line of policy beside the
  others, so I can read the whole access model in one file and see what it allows.
- When a request is refused or allowed, I want the derivation that decided it, so I can answer "why"
  from the decision itself and not from a log.
- When a caller asks what they may see, I want the list derived from the same rules that gate each
  read, so the listing and the gate cannot disagree.
- When I ship a policy, I want its hash in every decision and receipt, so I can say which rule set
  stood behind an answer after the rules have moved.
- When a rule set is wrong (unsafe variable, negation cycle, unknown relation), I want it refused at
  load with a message that says what to change, so a bad policy never reaches a request.

## Scope

In:

- A small Datalog: rules, facts, stratified negation, comparisons, relation declarations.
- Compile-time validation: arities, safety, stratification, caller-supplied per-relation checks.
- Semi-naive bottom-up evaluation to a fixpoint, with a deadline and a size bound.
- Standing facts and per-request facts as separate inputs, so a policy is compiled once and
  evaluated many times.
- Decisions with proofs: which rules and which facts derived the answer.
- Listing: every tuple of a relation, for "what may P do" and "what may P see".
- Named, versioned, hashed rule sets.
- A `policy` layer with the PARC vocabulary (principal, action, resource, context) and a bundled
  RBAC rule set written in Shearline's own language.

Out:

- Storage of any kind. No file formats beyond the rule text. Loading tokens or config from disk is
  the consumer's job (Ductile's, Anam's).
- A server, CLI, or network surface.
- Function symbols, arithmetic in heads, aggregates, or anything that makes evaluation unbounded.
- Attribute-based access over arbitrary JSON payloads. Anam's `attr/3` stays in Anam.
- Policy authoring UI or a policy language other than the Datalog.

## The shape of a decision

A request is four facts:

```
principal("tok-7a2f").
action("read").
resource("timeline/health.blood-pressure").
context("hour", 14).  % any number of context(Key, Value) facts
```

A policy is rules over those, plus standing facts the consumer loads:

```
role("tok-7a2f", "importer").
grants("importer", "submit", "envelope").
grants("reader",   "read",   "timeline/*").
inherits("operator", "reader").

holds(P, R) :- role(P, R).
holds(P, R) :- holds(P, Q), inherits(Q, R).

allow(P, A, R) :- principal(P), action(A), resource(R),
                  holds(P, Role), grants(Role, A, Pattern), prefix(Pattern, R),
                  not deny(P, A, R).
```

The engine evaluates and the policy layer checks `allow(P, A, R)` against the request. Absent
`allow` is deny. An error of any kind is deny.

## Architecture

Two packages, one module, stdlib only.

> History: now one package, `policy`, and one dependency, the tarski engine (v0.3.2). The `datalog` package below was cut out into tarski with its history and deleted from this module; `docs/DESIGN.md` has the current contract.

### `datalog` — the engine

What comes across from Anam's `internal/query` largely as is:

| From | Holds | Change |
|---|---|---|
| `term.go`, `number.go` | `Const` (string, exact decimal, bool), `Tuple`, ordering, keys | none |
| `parse.go` | lexer, parser, `Program`, `Rule`, `Atom`, `Literal` | none |
| `plan.go`, `stratify.go` | `Compile`, arity and safety checks, Tarjan stratification | the two Anam checks (`checkInstant`, `checkPointer`) leave; `Schema` gains an optional per-relation `Check func(Atom) error` |
| `eval.go` | `Evaluate`, semi-naive, lazy join indexes, deadline every 1024 steps, byte cap | add opt-in proof recording |
| `answer.go` | sorted relation, canonical JSON, sha256 | none |
| `bundled.go` | `RuleSet{Name, Version, Hash, Program}` | the type stays; Anam's rules go home |

What stays in Anam: `base.go` (fold-backed base relations), `describe.go` (introspection over
Definitions), `check.go` (equality with `fold.Snapshot`), `run.go`, `rules/anam.dl`.

New in the engine:

- `NewBase(schema, facts)` from plain tuples. Anam's `Base` is built only from a fold value.
- `Base.With(facts)` returning a new base: standing facts plus a request's, without copying the
  standing relations. Compile once, evaluate per request.
- Built-ins beyond comparison, each deterministic and finite: `prefix/2`, `suffix/2`,
  `contains/2` on strings. Nothing that produces new values.
- Proofs. For each derived tuple, the first rule and body tuples that produced it, kept only when
  asked for (it touches the hot loop).

Public face:

```go
func Parse(text string) (Program, error)
func Compile(schema Schema, programs ...Program) (Plan, error)
func NewBase(schema Schema, facts ...Fact) (Base, error)
func (b Base) With(facts ...Fact) Base
func Evaluate(ctx context.Context, plan Plan, base Base, opts Options) (Result, error)
func (r Result) Relation(name string) (Answer, error)
func (r Result) Proof(name string, t Tuple) (Proof, bool)
```

`Options` carries `MaxBytes` and `Proofs bool`. Errors stay the three Anam has: `ErrSyntax`,
`ErrInvalid`, `ErrTooLarge`, renamed from the `query:` prefix.

### `policy` — the PARC layer

Owns the vocabulary and nothing else:

```go
type Request struct {
    Principal string
    Action    string
    Resource  string
    Context   map[string]Const
}

type Policy struct { /* plan, standing base, rule-set hash */ }

func Load(rules RuleSet, standing ...Fact) (Policy, error)
func (p Policy) Decide(ctx context.Context, req Request) (Decision, error)
func (p Policy) Permitted(ctx context.Context, principal, action string) ([]string, error)

type Decision struct {
    Allow    bool
    Proof    Proof     // empty when denied by absence
    RuleSet  string    // "name/version sha256:…"
}
```

`Decide` is total over its inputs: a nil error and `Allow=false` is a deny by absence; a non-nil
error is also a deny, and the caller never has to remember that. `Permitted` is the listing;
the same rules answer both, so the gate and the list cannot disagree.

Bundled: `rbac.dl`, a named, versioned rule set with `role/2`, `inherits/2`, `grants/3`,
`holds/2`, `allow/3`, `deny/3` declared for the consumer to extend. Shearline bundles no other
vocabulary. "Importer", "source", "projection", "tenant" never appear in the module.

## Design lens

**Hickey.** The engine and the vocabulary are two things and stay apart: `datalog` knows no
principal, `policy` knows no evaluation strategy. A policy is a value with a hash, never a place
that mutates. Standing facts and request facts are two inputs, not one mutable base. The
consumer's storage (importers file, Ductile config) is the consumer's; Shearline only ever sees
tuples. Nothing here reads a payload: the request is a dozen fields and the decision is a function
of them and the rules.

**Armstrong.** The error kernel is small: `Compile` and `Evaluate`. Both are pure, and both fail
closed. `Compile` refuses at load what would fail at request time, so a bad policy never serves.
`Evaluate` takes a `context` deadline and a byte cap and returns `ErrTooLarge` or the context's
error rather than hanging or growing; evaluation always terminates because there are no function
symbols. In `policy`, every error path is deny. A consumer that crashes mid-request loses nothing,
since the module holds no state to corrupt.

## Cleaving from Anam

1. `git filter-repo --path internal/query/` into the new repo, so the card 173 history and its
   reasoning come along. Module path `github.com/mattjoyce/shearline`.
2. First commit is the cut: remove `base.go`, `describe.go`, `check.go`, `run.go`,
   `rules/anam.dl`, and the ledger-backed tests (`ledger_test`, `edges_test`, `lazy_attr_test`,
   `container_pointer_test`, `docs_test`); replace `Definitions` in `Compile` with the schema
   hook; rename the package `datalog`. Everything after is a normal diff.
3. The engine tests that stay (`eval`, `parse`, `plan`, `stratify`, `equality`, `number`,
   `base`, `container`) are the regression suite from day one.
4. Add `NewBase` from facts, `With`, built-ins, proofs, `Options`.
5. Add `policy` with `rbac.dl` and table-driven tests covering the PARC shape: hierarchy,
   deny-overrides, listing, proof, hash in decision.
6. Bring back: Anam's `internal/query` becomes a thin layer over `shearline/datalog` (history: the engine has since moved to github.com/mattjoyce/tarski, which Anam will import directly) — its base,
   its bundled rules, `Describe`, `Check`, `Run`. The removed ledger tests are the proof the
   extraction preserved behaviour, and `anam query check` against `fold.Snapshot` is the standing
   one. Adding the dependency to Anam's `go.mod` is Matt's call under AGENTS.md; the case is that
   Anam loses ~1,000 lines and gains a second user for its engine. Shearline's own `go.mod` stays
   stdlib-only. (History: it now requires tarski, the engine itself, and nothing else.)
7. Ductile: load tokens and config as standing facts, requests as `Request`, and see whether any
   Anam-shaped assumption leaks. If it does, that is a design fault in Shearline, not in Ductile.

Card 223 is unchanged by this: Anam's seventeen rules stay static Go until the survey's triggers
arrive (a dozen rules, a third principal class, or a rule that reads beyond the request). When
one does, Shearline is where they go.

## Standards

Matt's Go standards apply as in Anam's `AGENTS.md`: stdlib first, no new dependency without a
case, `gofmt` + `goimports`, `golangci-lint`, `gosec`, table-driven tests with real components,
`-race -shuffle=on`, >80% coverage, exported names documented, functions under ~60 lines, files
under ~400. Defaults (step count between deadline checks, default byte cap) are named constants
with a reason; invariants (the answer's serialisation, the rule-set hash's shape) are constants
with a reason they are not configurable.

Naming: relation names lower-case, `snake_case` for multi-word; rule-set files `*.dl`; the module
and packages lower-case single words.

## Open questions, for Matt

1. **Two packages or one.** `datalog` and `policy` as described, or `policy` alone with the engine
   unexported? Two lets Anam import the engine without the PARC layer it does not need.
2. **Proof cost.** Opt-in per `Evaluate`, or always on and measured? Anam's queries over a whole
   value would pay for it; a policy request would not notice.
3. **Deny.** Bundle `deny/3` with deny-overrides in `rbac.dl`, or leave negation to the consumer
   and keep the bundled set allow-only?
4. **Context typing.** `context(Key, Value)` as string-keyed `Const`, or typed relations the
   consumer declares (`hour/1`, `origin/1`)? The first is general; the second is checkable at
   compile.
5. **Ductile's actual shape.** This spec guesses from "token and config wrangling". Its real
   requests should be written down before the `policy` API is fixed.
6. **Name.** Shearline, as given.

