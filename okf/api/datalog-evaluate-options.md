---
type: Go API
title: datalog.Evaluate and Options
description: "Evaluate runs a plan over a base to its fixpoint, semi-naively, bounded by context, MaxBytes and MaxSteps, optionally recording proofs."
resource: github.com/mattjoyce/shearline/datalog.Evaluate
tags: [go, api, evaluate, options, bounds]
timestamp: 2026-10-22T00:00:00Z
---

# Signature

```go
type Options struct {
    MaxBytes int  // approx JSON text of every derived tuple; 0 is no bound
    MaxSteps int  // join steps over the whole evaluation; 0 is no bound
    Proofs   bool // record the first derivation of each derived tuple
}
func Evaluate(ctx context.Context, plan Plan, base Base, opts Options) (Result, error)
```

# Behaviour

- Runs stratum by stratum, semi-naive within each. Reads values and writes nothing, so calls may share one `Base` and `Plan`.
- A context already done fails at once, before any join. Otherwise the deadline is checked every `stepsPerDeadlineCheck` = 1024 join steps.
- `MaxBytes` bounds what is derived and `MaxSteps` bounds the work to derive it. A rule that joins a wide product and derives only duplicates is charged no bytes, so without `MaxSteps` its termination would rest on the caller's deadline alone. The negation probe is charged steps like a join.
- A guard runs after the last stratum, so the final tuple cannot cross `MaxBytes` with a nil error.
- Over either bound is `*TooLarge`, which `Is(ErrTooLarge)`, carries `Bound` (`MaxBytes` or `MaxSteps`), the `Relation` being derived, and its message starts `datalog:`. Asking again unchanged will fail again.
- A base that holds a relation the plan derives is `ErrInvalid`, as is one that lacks a relation the plan reads. See [Compile and Schema](/api/datalog-compile-schema.md) for the shape-only compatibility rule.
- Determinism: same inputs, same `Answer` hashes, same [proof](/concepts/proof.md).

# What policy passes

`policy.Decide` uses `MaxBytes 1<<20`, `MaxSteps 1<<22`, `Proofs true`. `Permitted` is the same without proofs. See [per-request evaluation](/concepts/per-request-evaluation.md).

# Related

[Result.Proof and WhyNot](/api/datalog-result-proof-whynot.md), [value and builder](/concepts/value-and-builder.md).

# Citations

[1] [datalog/eval.go](../../datalog/eval.go)
[2] [datalog/bounds.go](../../datalog/bounds.go)
[3] [DESIGN.md, Options and Evaluation invariants](../../docs/DESIGN.md)
