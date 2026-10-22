---
type: Review
title: Module panel, 2026-10-22
description: "Whole-module luminary panel at commit 009a13a: 24 findings, one availability cliff, every one resolved on master with a commit."
tags: [review, luminary, module, panel]
timestamp: 2026-10-22T00:00:00Z
---

# Method

Artifact: `github.com/mattjoyce/shearline` at 009a13a. Two pairs, each run blind to the other and to the design review, by read-only Max agents.

| Pair | Axes | Notes |
|---|---|---|
| Hickey x Armstrong | Structure x runtime | Verdict: concerns. No wrong allow on the bundled rules |
| Ousterhout x Liskov | Surface x substitution | Fourteen hostile-consumer probes |

The lead collated. Every finding was resolved on master before the collation was written.

# Counts

| Group | Red | Orange | Yellow | Total |
|---|---|---|---|---|
| Converged by both pairs | 1 (mixed red/orange) | 6 | 3 | 10 |
| One pair, accepted | 1 | 2 | 11 | 14 |
| Total | 2 | 8 | 14 | 24 |

Five further items were not taken or deferred, and one of them (value versus builder) was taken later as [decision 6](/decisions/6-relation-is-value-or-builder.md).

# Top findings and where they landed

| Finding | Landed | Commit |
|---|---|---|
| Bundled `holds/2` unseeded: byte bound trips near 38k pairs, confirmed at 50,000 | Rules seeded by the request. See [per-request evaluation](/concepts/per-request-evaluation.md) | 085d076 |
| `Load` accepts a set that `Decide` fails on or `Permitted` panics on | `Plan.Arity`; `Load` requires `allow/3` and `permit/3` | 085d076 |
| `RuleSet.Hash` advisory on a hand-built set | `RuleSet.Verify`, called by `Load`. See [rule-set identity](/concepts/rule-set-identity.md) | 07ba88d, 085d076 |
| `Proof.Rule` a live pointer into the shared plan | Deep copy per `Result.Proof` call. See [proof](/concepts/proof.md) | e59eaec |
| `MaxBytes` crossed with `err == nil` by the last tuple | Guard after the strata | 07ba88d |
| `Number` accepts NaN and Inf; NaN equals everything | Refused; `-0` folds to `0`. See [constants](/concepts/constants.md) | 07ba88d |
| `Tuple.key` not injective over NUL | Length-prefixed self-delimiting keys | 07ba88d, 2114160 |
| `Answer.Tuples` alias the frozen base | Tuples cloned, `Bytes` copied | 07ba88d |
| Standing-fact order changes the proof | `Load` sorts a copy | b926d98 |
| One sentinel for request-malformed and policy-broken | `policy.ErrRequest` | b926d98 |

# Tensions held

- **Let it crash versus total function.** Refuse at `Load`, never guard in `Permitted`. See [fail-closed decision](/concepts/fail-closed-decision.md).
- **Open struct versus enforceable invariant.** Keep `RuleSet` and `Proof.Rule` open for Anam's sake; enforce at the boundary that matters (`Verify`, a copy).
- **"Rule sets are policy" versus the `permit/3` contract.** Extension is free, but the shape `Permitted` reads is refused loudly if broken.
- **Unseeded `holds` is the truer relation.** The runtime wins: bounds are per request, so the rules are.

# Evidence

Final gates after every resolution: `go test -race -shuffle=on -count=2 -cover ./...` with datalog at 96.8 percent and policy at 92.8 percent; `golangci-lint` 0 issues; `gosec` clean. Benchmark `BenchmarkEvaluate/proofs=false` at 94,017 allocs/op, from 116,532 before the panel. One finding was the lead's own: a key change that doubled allocations, fixed to land below the original.

# Citations

[1] [Module panel collation](../../docs/reviews/luminary-module-2026-10-22/00-collation.md)
[2] [Hickey x Armstrong](../../docs/reviews/luminary-module-2026-10-22/01-hickey-armstrong.md)
[3] [Ousterhout x Liskov](../../docs/reviews/luminary-module-2026-10-22/02-ousterhout-liskov.md)
