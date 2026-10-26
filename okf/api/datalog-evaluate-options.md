---
type: Go API
title: tarski.Evaluate and Options
description: "Evaluate runs a plan over a base to its fixpoint, semi-naively, bounded by context, MaxBytes and MaxSteps, optionally recording proofs."
resource: github.com/mattjoyce/tarski.Evaluate
tags: [go, api, evaluate, options, bounds]
timestamp: 2026-10-22T00:00:00Z
---

> This describes the engine, which now lives in [tarski](https://github.com/mattjoyce/tarski) (required at v0.3.2); Shearline cut it out at c4f75ab and the code is no longer in this module. The contract is tarski's `docs/DESIGN.md`. Kept here because the policy layer relies on it.

# Signature

```go
type Options struct {
    MaxBytes int  // approx JSON text of every derived tuple; 0 is no bound
    MaxSteps int  // steps over the whole evaluation; 0 is no bound
    Proofs   bool // record the first derivation of each derived tuple
}
func Evaluate(ctx context.Context, plan Plan, base Base, opts Options) (Result, error)
```

# Behaviour

- Runs stratum by stratum, semi-naive within each. Reads values and writes nothing, so calls may share one `Base` and `Plan`.
- A context already done fails at once, before any join. Otherwise the deadline is checked every `stepsPerDeadlineCheck` = 1024 steps.
- `MaxBytes` bounds what is derived and `MaxSteps` bounds the work to derive it. A rule that joins a wide product and derives only duplicates is charged no bytes, so without `MaxSteps` its termination would rest on the caller's deadline alone. What a step is (tarski v0.2.2): each tuple a join visits, each negation probe, each rule application (so a rule of guards alone is bounded), and each built-in call, so `MaxSteps` bounds `Holds` calls directly. Comparisons are free. The same evaluation costs more steps than under v0.2.0 (v0.2.1 added the rule-application charge, v0.2.2 the built-in one); answers and hashes do not change.
- A negative `MaxBytes` or `MaxSteps` is `ErrInvalid` since tarski v0.3.2; zero is still no bound. `policy` passes fixed positive bounds.
- A guard runs after the last stratum, so the final tuple cannot cross `MaxBytes` with a nil error.
- Over either bound is `*TooLarge`, which `Is(ErrTooLarge)`, carries `Bound` (`MaxBytes` or `MaxSteps`; `Number` for a sum too wide to hold exactly, which policy cannot reach because `Load` refuses aggregates), the `Relation` being derived, and its message starts `tarski:`. Asking again unchanged will fail again.
- A base that holds a relation the plan derives is `ErrInvalid`, as is one that lacks a relation the plan reads. See [Compile and Schema](/api/datalog-compile-schema.md) for the shape-only compatibility rule.
- Determinism: same inputs, same `Answer` hashes, same [proof](/concepts/proof.md).

# What policy passes

`policy.Decide` uses `MaxBytes 1<<20`, `MaxSteps 1<<22`, `Proofs true`. `Permitted` is the same without proofs. See [per-request evaluation](/concepts/per-request-evaluation.md).

# Related

[Result.Proof and WhyNot](/api/datalog-result-proof-whynot.md), [value and builder](/concepts/value-and-builder.md).

# Citations

[1] [tarski/eval.go](https://github.com/mattjoyce/tarski/blob/v0.3.2/eval.go)
[2] [tarski/bounds.go](https://github.com/mattjoyce/tarski/blob/v0.3.2/bounds.go)
[3] [tarski docs/DESIGN.md, Evaluation](https://github.com/mattjoyce/tarski/blob/v0.3.2/docs/DESIGN.md)
