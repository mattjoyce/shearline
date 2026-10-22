---
type: Concept
title: Fail-closed decision
description: "Every error is a deny: Decide never returns Allow=true with an error, and an absent allow/3 tuple is a deny with a nil error."
tags: [security, decide, invariant]
timestamp: 2026-10-22T00:00:00Z
---

# What it means

Shearline answers one question and has two kinds of "no". Both are a deny.

| Outcome | `Allow` | `error` | Meaning |
|---|---|---|---|
| Allowed | true | nil | `allow(P, A, R)` is in the result; `Decision.Proof` says why |
| Deny by absence | false | nil | No rule derived `allow`; `Decision.Why` says why |
| Refused request | false | wraps `policy.ErrRequest` | The caller's fault: empty field, undeclared context key, wrong kind |
| Broken policy or engine | false | any other | A 5xx: bound crossed, bad rule set, cancelled context |

`Decision.Allow` is false whenever `err != nil`. There is no path that returns an allow and an error together, so a caller that ignores the error still cannot be tricked into permitting.

# Where the closure happens

Failure is refused as early as it can be seen.

- **At `Load`**: a rule set whose hash does not match its text, a plan with no `allow/3` or `permit/3`, a standing fact in a request relation, a context key the rules name but the config did not declare. See [policy.Load](/api/policy-config-load.md) and [rule-set identity](/concepts/rule-set-identity.md).
- **At `Decide`**: a malformed request is refused by the schema checks; a [declared context](/concepts/declared-context.md) violation is `ErrRequest`. See [policy.Decide](/api/policy-decide.md).
- **During evaluation**: a bound crossed is an error, never a partial answer. See [per-request evaluation](/concepts/per-request-evaluation.md).

# The one promise that survives any rule set

Rule sets are policy, not user input. A later set may add rules to any relation, `allow/3` included, and nothing in the engine stops it from widening access. The only promise that holds for an arbitrary set is deny by absence: if nothing derives `allow`, the answer is no. The bundled [RBAC rule set](/rulesets/rbac.md) adds its own `not deny(P, A, R)` guard, but a consumer's own `allow` rule must keep that guard itself.

# Why

A policy engine that fails open on a bug is worse than no engine. The refusal also has to be useful, so a deny explains itself: see [why-not provenance](/concepts/why-not-provenance.md).

# Citations

[1] [README, "Using it" and "Why it is built this way"](../../README.md)
[2] [DESIGN.md, `Decide` contract and `Load` contract](../../docs/DESIGN.md)
[3] [policy/policy.go](../../policy/policy.go)
