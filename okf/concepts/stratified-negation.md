---
type: Concept
title: Stratified negation
description: "What the engine's Datalog is and is not: no function symbols, no aggregates in policy, built-ins as guards, stratified negation, and a hook mechanism for declared-but-undefined relations."
tags: [datalog, language, negation, builtins, termination]
timestamp: 2026-10-22T00:00:00Z
---

> This describes the engine, which now lives in [tarski](https://github.com/mattjoyce/tarski) (required at v0.2.2); Shearline cut it out at c4f75ab and the code is no longer in this module. The contract is tarski's `docs/DESIGN.md`. Kept here because the policy layer relies on it.

# The language

Rules, facts, comparisons (`=`, `!=`, `<`, `<=`, `>`, `>=`), negation and relation declarations. A relation name is lower-case, a variable starts upper-case, `_` is anonymous. Constants are double-quoted strings, numbers, `true` and `false`. See [constants](/concepts/constants.md).

```
% a comment runs to the end of the line
name/arity.                                    declares a relation another program defines
head(X, "c", 1).                               a fact
head(X, Y) :- rel(X, Z), not other(Z), Y > 3.  a rule
```

# What it is not

- **No function symbols and no arithmetic in heads.** Together with built-ins that produce no value, the derived relations are finite and evaluation always reaches a fixpoint. This is the termination argument.
- **No aggregates in policy.** The engine has them since tarski v0.2 (`count`, `sum`, `min`, `max`), stratified like negation. `policy.Load` refuses a rule set that uses one, because a decision is per request and an aggregate summarises a relation.
- **No attribute-style payloads.** Facts are tuples of scalars.

# Safety and stratification

`Compile` refuses, before anything is read:

| Problem | Where |
|---|---|
| Unsafe variable (in the head, a negated atom, a comparison or a built-in, not bound by a positive body atom) | `checkRule` |
| Unknown relation, wrong arity | `checkAtom` |
| `_` in a head | `checkRule` |
| A rule over a base relation (a head naming one) | `ErrInvalid` |
| A built-in as a head, declaration or schema entry | `ErrInvalid` |
| Negation that cannot be stratified (a negation cycle) | `stratify.go` |
| An atom a schema check refuses | the schema's `Checks` |

Evaluation runs stratum by stratum, semi-naive within each: after the first round only the tuples new in the last round are joined again. A deny rule reading `allow/3` is a negation cycle (`allow` depends on `not deny`) and is refused at `Load`. See [the RBAC rule set](/rulesets/rbac.md).

# Built-ins as guards

| Atom | True when |
|---|---|
| `prefix(P, S)` | `strings.HasPrefix(S, P)` |
| `suffix(P, S)` | `strings.HasSuffix(S, P)` |
| `contains(P, S)` | `strings.Contains(S, P)` |

Pattern first, subject second. Each is a guard like a comparison: both arguments must be bound by a positive body atom, it binds nothing and derives nothing, and it never appears in a head, declaration or schema. A non-string argument makes the guard false, which makes `not prefix(...)` true for a number. The table `builtins` in `builtins.go` is written once at init and only read after, so evaluations may share it.

# Declared and undefined is empty

A relation a program declares and nothing defines evaluates as the empty relation. That is how a rule set leaves a hook open: the bundled `rbac.dl` defines `deny/3` itself from `denied/3`, so it does not use the hook, but a consumer set may. Programs given to `Compile` are read as one, so a later program defines what an earlier one declared.

# Related

- [Prefix grants](/concepts/prefix-grants.md) use `prefix/2`.
- [Per-request evaluation](/concepts/per-request-evaluation.md) states the bounds that complement termination.
- API: [tarski.Compile and Schema](/api/datalog-compile-schema.md).

# Citations

[1] [tarski docs/DESIGN.md, Built-ins and Evaluation invariants](https://github.com/mattjoyce/tarski/blob/v0.2.2/docs/DESIGN.md)
[2] [tarski/doc.go](https://github.com/mattjoyce/tarski/blob/v0.2.2/doc.go)
[3] [tarski/plan.go](https://github.com/mattjoyce/tarski/blob/v0.2.2/plan.go)
[4] [tarski/stratify.go](https://github.com/mattjoyce/tarski/blob/v0.2.2/stratify.go)
[5] [tarski/builtins.go](https://github.com/mattjoyce/tarski/blob/v0.2.2/builtins.go)
