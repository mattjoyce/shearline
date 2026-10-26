---
type: Concept
title: Why-not provenance
description: "How a refusal explains itself: the farthest literal each candidate rule reached, a Truncated flag for the bounded walk, and Blocked for a negation that held."
tags: [whynot, absence, explanation]
timestamp: 2026-10-22T00:00:00Z
---

> This describes the engine, which now lives in [tarski](https://github.com/mattjoyce/tarski) (required at v0.2.0); Shearline cut it out at c4f75ab and the code is no longer in this module. The contract is tarski's `docs/DESIGN.md`. Kept here because the policy layer relies on it.

# The idea

A policy you cannot interrogate is one you will stop trusting. When `allow` is not derived, `Decision.Why` is a `tarski.Absence` built by `Result.WhyNot("allow", tuple, 0)`. It is engine-level: it knows nothing of RBAC. For `rbac.dl` it reads as "no role fact for P", "no grant for A on a held role", "no permitted prefix matches R", or "denied by <proof>". See [decision 3](/decisions/3-a-refusal-explains-itself.md).

# Shape

| Type | Field | Meaning |
|---|---|---|
| `Absence` | `Relation`, `Goal` | The goal; a position the caller left open is a variable |
| | `Rules []RuleFailure` | One per rule whose head unifies with the goal; nil for a base relation ("no such fact"), empty for a derived relation no head unifies with ("no rule's head matches") |
| | `Truncated` | The walk hit a limit; what is shown is still true |
| `RuleFailure` | `Rule` | A copy, as in [proof](/concepts/proof.md) |
| | `Bound` | Variable bindings when the walk stopped |
| | `Failed` | The farthest literal with no match |
| | `Because []Absence` | Why `Failed` found nothing, when it is a positive atom |
| | `Blocked *Proof` | When `Failed` is a negation that held: the proof of the tuple that blocked |

# Farthest literal

Walk the rule's body in evaluation order, depth-first. At each literal extend the bindings through every match. Matches go in relation order and rules in plan order, so the answer is deterministic. The literal reported is the one with the greatest index that any path reached and found no match, with that path's bindings.

A body that matched fully would have derived the goal, so it cannot occur against the result it came from. If it does show, it appears as a `RuleFailure` with a zero `Failed`. A zero `Failed` also appears when the step limit ended the walk, and `Truncated` tells which.

# Blocked: a deny that fired

When the farthest literal is `not deny(P, A, R)` and the negation held (the deny tuple exists), `Failed` is that negation and `Blocked` carries the proof of the deny tuple, down to the `denied/3` fact. The README's `timeline/secret` example prints exactly this.

# Truncated and the limits

| Limit | Value | Effect |
|---|---|---|
| `whyNotDepth` | 8 | Default nesting of `Because` when `maxDepth` is 0 |
| `whyNotSteps` | `1 << 16` | Body-literal candidates tried in all; each matched literal branches |

`Truncated` is set where the cut fell and on every `Absence` above it. A goal already explained in the same `Absence` is a cycle, not a cut: it is left with an empty `Because` and does not set `Truncated`. That is how recursion through `holds/2` ends.

`WhyNot` returns `ok=false` when the tuple is in the result (ask for the proof instead), when the relation or arity is not the plan's, or when the evaluation ran without proofs.

# Cost

One engine walk per refusal. Matt accepted that cost for the transparency; part of the value is future state.

# Related

- [Proof](/concepts/proof.md), [fail-closed decision](/concepts/fail-closed-decision.md), [value and builder](/concepts/value-and-builder.md) (why `WhyNot` can scan sealed relations safely).
- API: [Result.Proof and WhyNot](/api/datalog-result-proof-whynot.md), [policy.Decide](/api/policy-decide.md).

# Citations

[1] [README, refusal example output](../../README.md)
[2] [DESIGN.md, Absence and WhyNot contract](../../docs/DESIGN.md)
[3] [tarski/whynot.go](https://github.com/mattjoyce/tarski/blob/v0.2.0/whynot.go)
