---
type: Concept
title: Declared context
description: "Every context key a request may carry is declared with its kind in Config.Context, or the request is refused; the declaration is also the schema check on context/2."
tags: [context, schema, explicit]
timestamp: 2026-10-22T00:00:00Z
---

# The rule

`Config.Context` is `map[string]datalog.Kind`. A request whose `Context` carries a key not in it, or a declared key of another kind, is `ErrRequest`. A policy that declares no keys accepts a request with no context and refuses any key. Nothing is inferred and there is no default-permissive state.

Matt's reason, in his words: "code in the future is mostly AI generated, therefore be explicit." See [decision 4](/decisions/4-context-keys-declared.md).

# Where it is enforced

| When | By | Effect |
|---|---|---|
| `Load` | `checkContext` | An empty key or an unknown kind in `Config.Context` is `ErrInvalid` |
| `Load` | `contextCheck` as the schema check on `context/2` | A rule naming an undeclared key, or comparing a declared key with a constant of the wrong kind in an atom, fails to compile |
| `Decide` | `checkRequestContext` | Names the key and both kinds in the error, before facts are built; keys checked in sorted order |

The rules see context as `context(Key, Value)` facts, keys sorted so the base and any proof do not depend on map order. The declared map is copied at `Load`, so later edits by the caller reach nothing.

# Known limits

- **Comparisons are not inferred.** The check sees atoms, not comparisons, so `context("hour", H), H > "22"` compiles and is never true, because `H` is a declared number. The declared kind makes such a rule dead rather than dangerous.
- **An empty context value is allowed.** If a consumer rule feeds it to `prefix/2` it becomes a wildcard, since the empty prefix matches every resource. Documented on `Load`.
- **The key must be a non-empty string**, as for every other string position in the schema; only the value of `context/2` may be a number or boolean.
- Building numeric values safely: use [`datalog.Int` and `datalog.Float`](/concepts/constants.md), which cannot produce NaN.

# Related

- [policy.Load](/api/policy-config-load.md), [policy.Decide](/api/policy-decide.md).
- [Fail-closed decision](/concepts/fail-closed-decision.md): an undeclared key is a refusal, not a skipped fact.

# Citations

[1] [DESIGN.md, Decision 4](../../docs/DESIGN.md)
[2] [policy/facts.go](../../policy/facts.go)
[3] [policy/policy.go](../../policy/policy.go)
