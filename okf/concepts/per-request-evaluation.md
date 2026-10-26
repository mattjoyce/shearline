---
type: Concept
title: Per-request evaluation
description: "The bounds on an evaluation are per request, so the bundled rules are seeded by the request; the 38k-pair cliff is the story of why."
tags: [evaluation, bounds, performance, rbac]
timestamp: 2026-10-22T00:00:00Z
---

> This describes the engine, which now lives in [tarski](https://github.com/mattjoyce/tarski) (required at v0.3.2); Shearline cut it out at c4f75ab and the code is no longer in this module. The contract is tarski's `docs/DESIGN.md`. Kept here because the policy layer relies on it.

# The rule

`Decide` evaluates the whole plan over `standing facts + this request's facts` with a fixed budget. Because the budget is per request, every rule that matters must start from something the request supplies: `principal(P)`, `action(A)` or `resource(R)`. Then a decision derives only this principal's world, however many other principals are loaded.

# The budget

| Bound | Value | Why this value |
|---|---|---|
| `maxDecisionBytes` | `1 << 20` | A decision over one request should never derive a megabyte; if it does, the grants are wrong, not large |
| `maxDecisionSteps` | `1 << 22` | A decision is a few joins over one principal's roles; millions of visited tuples is a wrong policy |

Both are named constants, invariants and not knobs. `Decide` also turns on `Proofs`. `Permitted` uses the same two bounds without proofs. Details of the options are in [tarski.Evaluate and Options](/api/datalog-evaluate-options.md).

`MaxBytes` charges unescaped lengths, an estimate of the JSON text. `MaxSteps` is exact: every candidate visited by a join or a negation probe counts, and since tarski v0.2.2 so does every rule application and every built-in call (`prefix` is one). The bundled rules make a handful of each per request, far under the bound. Neither can be crossed with a nil error. A crossing is a `*TooLarge` that names which bound (`Bound` is `MaxBytes` or `MaxSteps`), and `Decide` wraps it with the remedy "narrow the grants reachable from this principal".

# The 38k cliff

The whole-module panel found that the first version of `rbac.dl` had `holds(P, R) :- role(P, R).` with no request seed. Bottom-up evaluation derived every principal's roles on every decision.

- A `holds` tuple of two 10-byte strings is charged about 27 bytes.
- `1 << 20` bytes divided by 27 is about 38,800 tuples.
- A deployment with more `(principal, role)` pairs than that could not decide anything: every request returned `*TooLarge`, and the remedy text pointed the operator at the wrong thing.
- Below the cliff, every decision still cost O(all roles), and the step budget was spent on strangers.

It was a fail-closed bug, so never a wrong allow, but it was an availability cliff in the one bundled asset. Confirmed at 50,000 principals. The fix (commit 085d076) seeds `holds`, `permit` and `deny` with `principal(P)`, `action(A)` and `resource(R)`, and a 50,000-principal test holds it.

The review's note on the trade: the unseeded `holds/2` is the truer relation, but the runtime wins. The bounds are per request, so the rules are too. See [the module panel](/reviews/module-panel-2026-10-22.md).

# Consequences for consumer rule sets

A rule set you add after RBAC should seed its own rules the same way, or it reintroduces the cliff. A rule that derives a wide relation unseeded will be charged for every principal in the standing base on every request.

# Related

- [Fail-closed decision](/concepts/fail-closed-decision.md): a crossed bound is an error, never a partial answer.
- [The RBAC rule set](/rulesets/rbac.md): each rule and its seed.
- [Stratified negation](/concepts/stratified-negation.md): why the engine can always reach a fixpoint within bounds.

# Citations

[1] [Module panel collation](../../docs/reviews/luminary-module-2026-10-22/00-collation.md)
[2] [Module panel, Hickey x Armstrong, finding A1](../../docs/reviews/luminary-module-2026-10-22/01-hickey-armstrong.md)
[3] [DESIGN.md, `rbac.dl` and evaluation invariants](../../docs/DESIGN.md)
[4] [policy/policy.go](../../policy/policy.go)
