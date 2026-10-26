---
type: Decision
title: "Decision 6: a relation is a value or a builder"
description: "During evaluation every relation is a builder; when Evaluate returns each is sealed into a value with no write method, so reads are safe by construction."
tags: [decision, relation, immutability, concurrency, matt]
timestamp: 2026-10-22T00:00:00Z
---

# Context

Anam's relation kept a lazy index and mutated it on first read. Shared as base relations under a concurrent `Policy`, that would race. The first fix was a `frozen` bit on the relation, with a panic in `add` and a scan workaround in `WhyNot` for derived relations. The design panel's follow-up (Hickey, H5) proposed two types in place of the bit; it was deferred as an open question.

# Options considered

| Option | For | Against |
|---|---|---|
| A `frozen` bit | Smallest change | One type that is sometimes a value and sometimes not; a panic as the enforcement |
| Two types, cut base versus derived | Matches where indexes differ | Hickey's objection was that the seam is mutability, not where a relation came from |
| Two types, cut value versus builder | Matches mutability, the real difference | A refactor across `eval.go` |

# Decision

Hickey's cut, not base-versus-derived. During evaluation every relation is a builder that accepts tuples. When `Evaluate` returns, every relation is sealed into a value with no write method: base relations with their indexes, derived ones without. `Base` and `Result` hold values only, so concurrent reads, `Proof` and `WhyNot` are safe by construction, and the `frozen` flag, its panic and the scan workaround go. A pure refactor gated by the benchmark: kept only within noise.

# Consequences

- `relBuilder` (`builder.go`) is the only place tuples are added; `relation` (`relation.go`) has read methods only.
- Derived relations are scanned, not indexed. Base relations carry one index per argument position.
- The decision closed the last open question; DESIGN.md's "Open, for Matt" is empty.

See also [Base.With](/api/datalog-base-with.md) and [why-not provenance](/concepts/why-not-provenance.md).

# Citations

[1] [DESIGN.md, Decided with Matt, item 6](../../docs/DESIGN.md); [tarski docs/DESIGN.md, Evaluation invariants](https://github.com/mattjoyce/tarski/blob/v0.2.2/docs/DESIGN.md)
[2] [Module panel collation, not-taken list](../../docs/reviews/luminary-module-2026-10-22/00-collation.md)
[3] [tarski/builder.go](https://github.com/mattjoyce/tarski/blob/v0.2.2/builder.go)
[4] [tarski/relation.go](https://github.com/mattjoyce/tarski/blob/v0.2.2/relation.go)
