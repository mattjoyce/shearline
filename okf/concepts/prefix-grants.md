---
type: Concept
title: Prefix grants
description: "Grants are plain string prefixes with no path semantics; deny is exact; the consumer ends prefixes with its separator and canonicalises names first."
tags: [grants, prefix, resources, deny]
timestamp: 2026-10-22T00:00:00Z
---

# The rule

`Grant("reader", "read", "timeline/")` covers every resource whose name starts with `timeline/`. Matching is `strings.HasPrefix` through the `prefix(Pattern, Subject)` [built-in](/concepts/stratified-negation.md). A resource name is an opaque string.

# What there is not

- **No path semantics.** `Grant(..., "timeline")` also matches `timeline/health` and `timeline-secrets`. End the prefix with your separator: `"timeline/"`. Even then `"timeline/health"` as a prefix matches `"timeline/healthy"`.
- **No canonicalisation.** Case, dots, `..` and separators are the consumer's job, before `Decide`, and the consumer must execute the same target it asked about. The edge audit lists this as an integration edge: a trusted integration canonicalises resources before evaluation.
- **No glob.** The original spec's `"timeline/*"` was dropped because `*` is not a prefix. A glob built-in is a later option if a consumer needs it.
- **No exact-match grant.** A grant on a leaf also matches longer strings.

# Deny is exact

`Deny("ana", "read", "timeline/secret")` is a `denied/3` fact for that exact resource string. It is not a subtree. A subtree denial is a consumer rule over `prefix/2`, for example:

```
deny(P, A, R) :- principal(P), action(A), resource(R), prefix("timeline/health", R).
```

A deny rule that reads `allow/3` is a negation cycle and `Load` refuses it. See [stratified negation](/concepts/stratified-negation.md) and the [RBAC rule set](/rulesets/rbac.md).

# `Permitted` lists prefixes

`Permitted(ctx, principal, action)` returns the grant patterns, not resources, and does not apply `deny`, which is per concrete resource. See [policy.Permitted](/api/policy-permitted.md).

# Empty prefix

An empty prefix matches everything, so the schema refuses an empty role, action or prefix at `Load` as a missing value, never a wildcard. An empty context value is the one way to reintroduce it: see [declared context](/concepts/declared-context.md).

# Future

If a consumer shows segmented names, add `under(Sep, Pat, R)`: true when R equals Pat or starts with Pat then Sep, the separator the consumer's. See [decision 1](/decisions/1-resource-matching.md).

# Citations

[1] [README, "Using it"](../../README.md)
[2] [DESIGN.md, Decision 1](../../docs/DESIGN.md)
[3] [Edge audit, integration edges](../../docs/reviews/edge-audit-2026-10-22.md)
[4] [policy/facts.go](../../policy/facts.go)
[5] [datalog/builtins.go](../../datalog/builtins.go)
