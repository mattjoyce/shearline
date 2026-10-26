---
type: Go API
title: tarski.NewBase and Base.With
description: "NewBase holds facts under a schema; With adds facts as a new value, sharing untouched relations, and equals NewBase over all the facts."
resource: github.com/mattjoyce/tarski.Base.With
tags: [go, api, base, facts]
timestamp: 2026-10-22T00:00:00Z
---

> This describes the engine, which now lives in [tarski](https://github.com/mattjoyce/tarski) (required at v0.2.0); Shearline cut it out at c4f75ab and the code is no longer in this module. The contract is tarski's `docs/DESIGN.md`. Kept here because the policy layer relies on it.

# Signatures

```go
func NewBase(schema Schema, facts ...Fact) (Base, error)
func (b Base) With(facts ...Fact) (Base, error)
func (b Base) Len(relation string) int
type Fact struct { Relation string; Tuple Tuple }
```

# NewBase

- A fact of a relation the schema lacks, of the wrong arity, with a string that is not valid UTF-8, or that the relation's check refuses is `ErrInvalid`.
- Every relation in `schema.Arities` exists, empty or not. Duplicates count once. Tuples are copied, so the caller may reuse its slices.

# With

`b.With(f...)` returns the standing base plus more facts as a new value; `b` is unchanged. Relations the new facts do not touch are shared, not copied. A relation they touch is rebuilt from the standing tuples plus the new ones and sealed again. Validation is the same as `NewBase`.

Postcondition, held by a seeded property test (200 iterations): `b.With(f...)` is observationally equal to `NewBase(schema, standing..., f...)` in `Len`, in every `Evaluate`, and in every `Answer` hash. The module panel found the contract claiming this property before any test held it; the seeded test was added in response.

# Why it matters

A policy compiles once and holds its standing base once. Each `Decide` calls `With` with the request's four-plus facts, so per-request cost is the touched relations only. See [per-request evaluation](/concepts/per-request-evaluation.md) and [value and builder](/concepts/value-and-builder.md).

# Gotchas

- `Base.Len` returns zero for an unknown relation while `Result.Relation` errors. Both are documented and left.
- Base relations are sealed with an index per argument position at build time; this is the cost of the touched relations.

# Related

[Compile and Schema](/api/datalog-compile-schema.md), [constants](/concepts/constants.md).

# Citations

[1] [tarski/base.go](https://github.com/mattjoyce/tarski/blob/v0.2.0/base.go)
[2] [DESIGN.md, NewBase and With](../../docs/DESIGN.md)
[3] [Module panel collation](../../docs/reviews/luminary-module-2026-10-22/00-collation.md)
