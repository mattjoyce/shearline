---
type: Go API
title: tarski.Result, Proof and WhyNot
description: "Result.Relation returns a hashed Answer; Result.Proof returns the first derivation; Result.WhyNot explains an absence; both render as text and JSON."
resource: github.com/mattjoyce/tarski.Result
tags: [go, api, result, proof, whynot, answer]
timestamp: 2026-10-22T00:00:00Z
---

> This describes the engine, which now lives in [tarski](https://github.com/mattjoyce/tarski) (required at v0.3.1); Shearline cut it out at c4f75ab and the code is no longer in this module. The contract is tarski's `docs/DESIGN.md`. Kept here because the policy layer relies on it.

# Signatures

```go
func (r Result) Relation(name string) (Answer, error)
func (r Result) Proof(name string, t Tuple) (Proof, bool)
func (r Result) WhyNot(name string, t Tuple, maxDepth int) (Absence, bool)
func (p Proof) String() string;   func (p Proof) MarshalJSON() ([]byte, error)
func (a Absence) String() string; func (a Absence) MarshalJSON() ([]byte, error)
```

# Result.Relation and Answer

`Relation` returns a named relation as an `Answer`, a value with nothing to change: `Relation()`, `Tuples()` as a copy, sorted, `Len()`, `Bytes()` as a copy of its one serialisation, a sha256 `Hash()` over it. A relation the program never named is an error, not an empty answer. `Answer` marshals as `Bytes()` and round-trips through JSON (`UnmarshalJSON` admits only an answer as written, sorted with no repeats and one width, and seals it again), so a client or a store holds the same bytes and hash.

# Result.Proof

`ok` is false when the tuple is not in the result or the run had no `Options.Proofs`. Each call builds each `(relation, tuple)` proof once. See [proof](/concepts/proof.md).

# Result.WhyNot

`ok` is false when the tuple is in the result, when the relation or tuple arity is not the plan's, or when proofs were off. `maxDepth` 0 means the default of 8. Deterministic for one `(plan, base, goal)`. See [why-not provenance](/concepts/why-not-provenance.md).

# Rendering

| Type | `String()` | `MarshalJSON()` |
|---|---|---|
| `Proof` | Indented tree, `set:line text` or `fact`; a shared tuple expanded once | `{relation, tuple, rule: {set, line, text} or null, body}` |
| `Absence` | Goal, then per rule the literal it stopped at and bindings, then why or the blocking proof | `{relation, goal, truncated, rules: [{rule, bound, failed, because, blocked}]}`; an open goal position is null |

Neither renders the parser's AST. The JSON of `Proof` and `Absence` is frozen in tarski, and since v0.3.1 `TestGolden_ProofAndAbsenceJSON` pins it byte for byte, so the JSON shape of a Shearline receipt is a frozen promise of the engine, not only of this module.

Answers and hashes depend on the set of facts; proofs, witness order and why-not bindings also depend on the order the facts were given. `policy` fixes that order: `Load` sorts the standing facts, and a request's own facts are built as principal, action, resource, then context keys sorted by key, so a proof is a function of the policy's content and the request. A consumer that pins proof bytes and builds its own base from `tarski` directly must sort its facts.

# Related

[Evaluate and Options](/api/datalog-evaluate-options.md), [policy.Decide](/api/policy-decide.md).

# Citations

[1] [tarski/proof.go](https://github.com/mattjoyce/tarski/blob/v0.3.1/proof.go)
[2] [tarski/whynot.go](https://github.com/mattjoyce/tarski/blob/v0.3.1/whynot.go)
[3] [tarski/render.go](https://github.com/mattjoyce/tarski/blob/v0.3.1/render.go)
[4] [tarski/answer.go](https://github.com/mattjoyce/tarski/blob/v0.3.1/answer.go)
[5] [DESIGN.md, Proof, Absence, WhyNot](../../docs/DESIGN.md)
[6] [tarski README, What it promises consumers](https://github.com/mattjoyce/tarski/blob/v0.3.1/README.md)
