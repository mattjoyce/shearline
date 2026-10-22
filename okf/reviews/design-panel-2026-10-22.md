---
type: Review
title: Design panel, 2026-10-22
description: "Luminary panel on DESIGN.md before any code: Hickey x Armstrong and Ousterhout x Liskov, twelve findings, all folded into the contract."
tags: [review, luminary, design, panel]
timestamp: 2026-10-22T00:00:00Z
---

# Method

Two pairs of luminary lenses read `docs/DESIGN.md` before any teammate coded against it. The pairs held two axes in tension on one artifact.

| Pair | Axes |
|---|---|
| Hickey x Armstrong | Structure x runtime |
| Ousterhout x Liskov | Surface x substitution |

Resolutions were folded into the contract. The collation lists each finding and where it landed.

# Counts

| Severity | Count |
|---|---|
| Red | 1 |
| Orange | 7 |
| Yellow | 4 |
| Total | 12 (3 converged, 9 from one pair) |

# Top findings and where they landed

| # | Finding | Severity | Landed |
|---|---|---|---|
| 1 | Proofs must be deterministic; map-ordered merge makes them not | Red | Sorted relation order on the derivation path; determinism stated; twice-evaluate test |
| 2 | Proof over a DAG: share, do not re-expand | Orange | Memoised per `(relation, tuple)`; shared `Proof` values. See [proof](/concepts/proof.md) |
| 3 | `Decision.RuleSet` string hides a list | Orange | `RuleSets []string` |
| 4 | Bounded bytes, unbounded work | Orange | `Options.MaxSteps`; `maxDecisionSteps`. See [per-request evaluation](/concepts/per-request-evaluation.md) |
| 6 | `Schema.Checks` must run on facts too | Orange | Checks run in `NewBase` and `With` |
| 7 | `With` needs a stated, tested postcondition | Orange | `With` equals `NewBase(all)` property test. See [Base.With](/api/datalog-base-with.md) |
| 8 | `policy` fact constructors | Orange | `Role`, `Inherits`, `Grant`, `Deny`. See [fact constructors](/api/policy-fact-constructors.md) |

The remaining five (frozen must be structural, `RBAC()` parsed at init, rule sets are policy not user input, `Permitted` doc order, `TooLarge` names the remedy) were accepted as written.

# Correction after implementation

Finding 1 was overstated. The cutter showed each derived relation appends only from its own delta, so map order in `merge` never interleaved tuples within one relation and the derivation order was already a function of the inputs. The sorted walk stays as a guard for the proof recorder; the determinism test and the twice-evaluate proof comparison remain the standing evidence.

# Held tension

`Schema.Checks` makes `Schema` a non-value. Hickey objected; the spec and Ousterhout wanted the small surface. The checks stayed in `Schema` with their promise widened to facts, and a schema is not hashable. Revisit only if a consumer needs to hash one. See [Compile and Schema](/api/datalog-compile-schema.md).

# Next

[Module panel](/reviews/module-panel-2026-10-22.md) reviewed the code; [edge audit](/reviews/edge-audit-2026-10-22.md) attacked it.

# Citations

[1] [Design panel collation](../../docs/reviews/luminary-design-2026-10-22/00-collation.md)
[2] [Hickey x Armstrong](../../docs/reviews/luminary-design-2026-10-22/01-hickey-armstrong.md)
[3] [Ousterhout x Liskov](../../docs/reviews/luminary-design-2026-10-22/02-ousterhout-liskov.md)
