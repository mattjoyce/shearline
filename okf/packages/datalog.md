---
type: Go Package
title: Package datalog
description: "The engine: parse, compile, evaluate, proofs, why-not and hashed rule sets, with no knowledge of what any relation means."
resource: github.com/mattjoyce/shearline/datalog
tags: [go, engine, datalog]
timestamp: 2026-10-22T00:00:00Z
---

# What it owns

Evaluation. The caller owns the vocabulary. Nothing here reads a file or the network, keeps state between calls, or knows what a relation means. It was cleaved from Anam's `internal/query` with the package renamed; error messages are prefixed `datalog:`.

| Area | Owns |
|---|---|
| Language | `Parse`, `Program`, `Rule`, `Atom`, `Literal`, `Comparison`, `Term` |
| Validation | `Compile` against a `Schema`: arities, safety, stratification, schema checks |
| Data | `Const`, `Tuple`, `Fact`, `Base` (`NewBase`, `With`, `Len`) |
| Evaluation | `Evaluate`, `Options`, `Result`, `Answer` |
| Provenance | `Proof`, `Result.Proof`, `Absence`, `RuleFailure`, `Result.WhyNot` |
| Identity | `RuleSet`, `NewRuleSet`, `ID`, `Verify` |
| Errors | `ErrSyntax`, `ErrInvalid`, `ErrTooLarge`, `*TooLarge` |

# Public surface, grouped

- **Constants**: `String`, `Bool`, `Number`, `Int`, `Float`, `Kind`. See [constants](/concepts/constants.md).
- **Plan**: `Compile(schema, programs...) (Plan, error)`, `Plan.Arity(name)`. See [Compile and Schema](/api/datalog-compile-schema.md).
- **Base**: `NewBase`, `Base.With`. See [Base.With](/api/datalog-base-with.md).
- **Evaluate**: `Evaluate(ctx, plan, base, Options)`. See [Evaluate and Options](/api/datalog-evaluate-options.md).
- **Results**: `Result.Relation`, `Result.Proof`, `Result.WhyNot`, `Answer` (sorted tuples, canonical JSON, sha256). See [Result.Proof and WhyNot](/api/datalog-result-proof-whynot.md).
- **Rendering**: `Proof` and `Absence` have `String()` and `MarshalJSON()`.

# Invariants

- Always terminates: no function symbols, no built-in produces a value. See [stratified negation](/concepts/stratified-negation.md).
- Deterministic: same `(plan, base, options)` gives same `Answer` hashes and the same proof for every tuple. See [proof](/concepts/proof.md).
- A `Base` and a `Plan` are immutable once built; a `Result` is read-only. See [value and builder](/concepts/value-and-builder.md).
- The plan owns deep copies of its programs; a proof's rule is a copy. See [rule-set identity](/concepts/rule-set-identity.md).
- The string domain is valid UTF-8 and numbers are finite float64. See [constants](/concepts/constants.md).
- Bounds are approximate on bytes, exact on steps. See [per-request evaluation](/concepts/per-request-evaluation.md).

# Files

`parse.go` and `lex.go` (language), `plan.go`, `shape.go` and `stratify.go` (compile), `base.go`, `relation.go` and `builder.go` (data), `eval.go`, `bounds.go` and `builtins.go` (evaluation), `proof.go`, `whynot.go` and `render.go` (provenance), `answer.go`, `bundled.go` (rule sets), `term.go` (constants).

# Used by

[Package policy](/packages/policy.md) is the first consumer in the module. Anam and Ductile are the planned external ones.

# Citations

[1] [datalog/doc.go](../../datalog/doc.go)
[2] [DESIGN.md, Package datalog](../../docs/DESIGN.md)
[3] [README, Reference](../../README.md)
