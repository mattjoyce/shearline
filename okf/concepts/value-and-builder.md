---
type: Concept
title: Value and builder
description: "A relation is a builder while evaluation runs and a sealed value afterwards, never both; Base and Result hold values only, so reads are safe by construction."
tags: [concurrency, immutability, relation, design]
timestamp: 2026-10-22T00:00:00Z
---

> This describes the engine, which now lives in [tarski](https://github.com/mattjoyce/tarski) (required at v0.2.2); Shearline cut it out at c4f75ab and the code is no longer in this module. The contract is tarski's `docs/DESIGN.md`. Kept here because the policy layer relies on it.

# The cut

Hickey's cut is value versus builder, not base versus derived.

| Phase | Type | Can add tuples | Indexes |
|---|---|---|---|
| During evaluation | `relBuilder` | yes (`add`) | built lazily, dropped when the relation grows |
| After `Evaluate` returns, or `NewBase`/`With` | `relation` | no write method | base relations: one per argument position, built at seal; derived: none, scanned |

`relBuilder.seal` gives up its tuples to the value and zeroes itself. `relation` has `has`, `len` and `candidates` and nothing that writes.

# Consequences

- **Concurrency.** A `Base` and a `Plan` are immutable once built and safe for concurrent `Evaluate` calls. A `Result` is read-only and safe to share. `Proof` and `WhyNot` are safe by construction because they only read values.
- The `frozen` flag, its panic on `add`, and the scan workaround `WhyNot` once needed for the lazy index were all removed.
- **Sharing in `With`.** `Base.With` shares every relation the new facts do not touch and rebuilds only the ones they do. See [Base.With](/api/datalog-base-with.md).
- **Answers.** `Answer.Tuples` are cloned and `Bytes` returns a copy, so no caller aliases the sealed base (module panel H1/L-3).

# Why

Anam's lazy index mutated shared base relations, which would race under a concurrent `Policy`. The first fix was a `frozen` bit. The design panel's follow-up (Hickey, H5) asked for two types instead of a bit; it was deferred, then taken as [decision 6](/decisions/6-relation-is-value-or-builder.md) and cut along value and builder.

# Gate

A pure refactor gated by the benchmark: kept only within noise. The module panel's benchmark is `BenchmarkEvaluate/proofs=false`.

# Related

- [Proof](/concepts/proof.md) and [why-not provenance](/concepts/why-not-provenance.md) read sealed values.
- [Per-request evaluation](/concepts/per-request-evaluation.md): the per-request base is `standing.With(request facts)`.

# Citations

[1] [DESIGN.md, Concurrency invariant and Decision 6](../../docs/DESIGN.md)
[2] [tarski/relation.go](https://github.com/mattjoyce/tarski/blob/v0.2.2/relation.go)
[3] [tarski/builder.go](https://github.com/mattjoyce/tarski/blob/v0.2.2/builder.go)
[4] [tarski/base.go](https://github.com/mattjoyce/tarski/blob/v0.2.2/base.go)
