# Collation — Luminary panel on the Shearline module, 2026-10-22

Artifact: `github.com/mattjoyce/shearline` at 009a13a (every landing through the first policy
fix). Pairs, each run blind to the other and to the design-stage review, by read-only Max agents:
Hickey × Armstrong (`01-hickey-armstrong.md`, structure × runtime) and Ousterhout × Liskov
(`02-ousterhout-liskov.md`, surface × substitution, with fourteen hostile-consumer probes).
Collated by the lead. Every finding below was resolved on master before this file was written;
the commit column says where.

## Converged (both pairs, independently): highest confidence

| Finding | 01 | 02 | Severity | Resolution | Commit |
|---|---|---|---|---|---|
| `Load` accepts a set that `Decide` fails on or `Permitted` panics on | A2 | L-1 | 🔴/🟠 | `Plan.Arity`; `Load` requires `allow/3`, `permit/3` | 085d076 |
| `Proof.Rule` is a live pointer into the shared plan | H2 | O-2, L-7 | 🟠 | deep copy per `Result.Proof` call; rule-set identity stays open | e59eaec |
| `RuleSet.Hash` is advisory on a hand-built set | H4 | O-1, L-6 | 🟠 | `RuleSet.Verify`, called by `Load` | 07ba88d, 085d076 |
| `MaxBytes` crossed with `err == nil` by the last tuple | A3 | L-2 | 🟠 | guard after the strata | 07ba88d |
| `Answer.Tuples` alias the frozen base | H1 | L-3 | 🟠 | tuples cloned, `Bytes` copied | 07ba88d |
| `Tuple.key` not injective over NUL | A6 | L-4 | 🟠 | length-prefixed, self-delimiting keys | 07ba88d, 2114160 |
| `Number` accepts NaN/Inf; NaN equals everything | A5 | O-7 | 🟠 | refused as not finite; `-0` folds to `0` | 07ba88d |
| `MaxBytes` is an estimate, not "the JSON text" | A8 | L-9 | 🟡 | "approximately" in contract and comments | 07ba88d |
| DESIGN claims a `With` property test that did not exist | H8 | O-10 | 🟡 | seeded property test, 200 iterations | 07ba88d |
| `policy.Schema` is a mutable package variable | H3 | L-11 | 🟡 | `Schema()` and `RBAC()` return fresh values | 085d076 |

## One pair, accepted

| Finding | Pair | Severity | Resolution | Commit |
|---|---|---|---|---|
| Bundled `holds/2` unseeded by `principal`: every decision derives every principal's roles; byte bound trips at ~38k pairs (confirmed at 50,000) | 01 A1 | 🔴 | rules seeded by the request; 50,000-principal test | 085d076 |
| `exists` (negation probe) charged no steps | 01 A4 | 🟡 | charged like a join | 07ba88d |
| Lexer reads bytes as runes | 01 A9 | 🟡 | `utf8.DecodeRuneInString` | 07ba88d |
| Empty context value becomes a wildcard through `prefix/2` | 01 A7 | 🟡 | documented on `Load` | 085d076 |
| Superset base silently drops a relation the plan derives | 02 L-5 | 🟠 | `accepts` refuses it | 3633a02 |
| Standing-fact order changes the proof | 02 L-8 | 🟡 | `Load` sorts a copy | b926d98 |
| `permit/3` third column unguarded; `Permitted` lists `"7"` or `""` | 02 L-10 | 🟡 | contract; `Permitted` refuses | b926d98 |
| One sentinel for request-malformed and policy-broken | 02 O-4, L-12 | 🟡 | `policy.ErrRequest` | b926d98 |
| Schema check errors flattened with `%v` | 02 O-5 | 🟡 | `%w` | 3633a02 |
| `Decide` builds and hashes an `Answer` it discards | 02 O-6 | 🟡 | membership from `Result.Proof` | b926d98 |
| `known(nil)` dangles; `TooLarge` unprefixed and bound unnamed | 02 O-8, O-9 | 🟡 | phrase, prefix, `Bound` | 3633a02 |
| `Request.Context` needs a parser-shaped constructor | 02 O-3 | 🟡 | `datalog.Int`, `datalog.Float` | 2114160 |
| `NewRuleSet` doc omits the slash | 02 O-10 | 🟡 | doc | 3633a02 |
| Key change doubled allocations (found by the lead's benchmark) | lead | 🟠 | one writer, `Const.same` in `bind`; below the original | 2114160 |

## Not taken, or deferred to Matt (in `docs/DESIGN.md` § Open)

- ~~`relation` as two types instead of a `frozen` bit (01 H5)~~: taken after all, as decision 6
  with Matt, and cut along value/builder rather than base/derived; it also removed the scan
  workaround `WhyNot` had needed for the lazy index.
- `Proof.Rule` carrying rule-set identity (01 H2, 02 O-2): needs `Program` to know its set.
- Proofs for denials (01 tension): intent versus contract.
- Typed context keys (01 A5 consequence, 02 O-3): spec open question 4.
- `Schema.Checks` taking a constants-only shape for facts (02 O-5): API churn for a small gain.
- `Base.Len` returning zero for an unknown relation while `Result.Relation` errors (02 O-8): both documented; left.

## Tensions held, not averaged

- **Let it crash vs total function.** Both pairs: the contract promised totality, so refuse at
  `Load`, never guard in `Permitted`. Done that way.
- **Open struct vs enforceable invariant** (`RuleSet`, `Proof.Rule`). Kept the open structs for
  Anam's sake and made the invariant enforceable at the boundary that matters: `Verify` at
  `Load`, a copy at `Proof`.
- **"Rule sets are policy" vs the `permit/3` contract.** A consumer set can widen `allow/3` at
  will; that is extension. But what `Permitted` reads has a shape, and a set that breaks the
  shape is refused loudly rather than listed quietly.
- **Unseeded `holds` is the truer relation.** The runtime wins: the bounds are per request, so
  the bundled rules are per request.

## Evidence

Final gates on master after every resolution: `go test -race -shuffle=on -count=2 -cover ./...`
datalog 96.8%, policy 92.8%; `golangci-lint` 0 issues; `gosec` clean; benchmark
`BenchmarkEvaluate/proofs=false` 94,017 allocs/op (from 116,532 before the panel).
