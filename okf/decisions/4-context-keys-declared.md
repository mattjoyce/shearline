---
type: Decision
title: "Decision 4: context keys are declared"
description: "Load takes Config.Context mapping each key to its kind; an undeclared key or wrong kind is ErrRequest, and the declaration is the schema check on context/2."
tags: [decision, context, explicit, matt]
timestamp: 2026-10-22T00:00:00Z
---

# Context

The spec's open question 4: `context(Key, Value)` as a string-keyed `Const`, general but unchecked, or typed relations the consumer declares (`hour/1`, `origin/1`), checkable at compile time? A typo in a context key would otherwise be a silent never-true rule, or worse, a rule that fires on an unexpected value.

# Options considered

| Option | For | Against |
|---|---|---|
| Open `context/2`, any key any kind | General | A misspelt key is silently dead or wrongly live; permissive by default |
| Typed relations per key | Compile-time checkable | Every new key is a schema change in the engine's vocabulary |
| Declared keys with kinds in `Config` | Explicit; checked at `Load` and `Decide` | One more thing to write |

# Decision

Matt: "code in the future is mostly AI generated, therefore be explicit." `Load` takes `Config{Standing, Sets, Context map[string]Kind}`. A request key that is undeclared, or declared with another kind, is `ErrRequest`; a policy that declares no keys accepts no context. The declaration is also the schema check on `context/2`, so a consumer rule naming an undeclared key or comparing a declared key with a constant of the wrong kind in an atom fails at `Load`. Nothing is inferred and there is no default-permissive state.

# Known limit

The check sees atoms, not comparisons. `context("hour", H), H > "22"` compiles and is never true, since `H` is a declared number. The declared kind makes such a rule dead rather than dangerous. Catching it would need kind inference over comparisons.

# Consequences

- `Load(standing, sets...)` became `Load(Config)`.
- `policy.Schema()` alone declares no keys; `Load` builds a schema per policy.
- Numeric context values should come from `datalog.Int` and `datalog.Float`, which refuse NaN and Inf.

See [declared context](/concepts/declared-context.md), [policy.Load](/api/policy-config-load.md) and [constants](/concepts/constants.md).

# Citations

[1] [DESIGN.md, Decided with Matt, item 4](../../docs/DESIGN.md)
[2] [SPEC.md, open question 4](../../SPEC.md)
[3] [policy/facts.go](../../policy/facts.go)
