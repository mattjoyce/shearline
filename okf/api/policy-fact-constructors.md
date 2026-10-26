---
type: Go API
title: policy fact constructors and Schema
description: "Role, Inherits, Grant and Deny build standing facts without learning the engine's vocabulary; Schema is the PARC relations with non-empty-string checks."
resource: github.com/mattjoyce/shearline/policy.Role
tags: [go, api, facts, schema]
timestamp: 2026-10-22T00:00:00Z
---

# Constructors

Each returns a `tarski.Fact` for `Config.Standing`.

| Call | Relation | Meaning |
|---|---|---|
| `Role(principal, role)` | `role/2` | principal holds role |
| `Inherits(role, parent)` | `inherits/2` | role also holds everything parent grants |
| `Grant(role, action, prefix)` | `grants/3` | role may take action on every resource starting with prefix |
| `Deny(principal, action, resource)` | `denied/3` | exact resource denied; the [RBAC rules](/rulesets/rbac.md) derive `deny/3` from it |

`Grant`'s prefix has no path semantics, and `Deny` is exact: see [prefix grants](/concepts/prefix-grants.md).

# Schema

`policy.Schema()` returns the base relations as a fresh `tarski.Schema` each call.

| Relation | Arity | Supplied by |
|---|---|---|
| `principal`, `action`, `resource` | 1 | the request |
| `context` | 2 | the request; key declared, value of the declared kind |
| `role`, `inherits`, `denied` | 2, 2, 3 | standing facts |
| `grants` | 3 | standing facts |

Every argument of every relation, except the value of `context/2`, must be a non-empty string constant (`Schema.Checks`). An empty principal, role, action or prefix is a missing value, not a wildcard, and is refused at `Load` or at the request, never silently matched. The check also runs on rule atoms, where variables pass.

`Schema()` alone declares no context keys and so refuses any; `Load` builds its own from `Config.Context`. See [declared context](/concepts/declared-context.md).

# Request relations

`principal`, `action`, `resource` and `context` are request relations: a standing fact in one is refused at `Load`.

# Related

[policy.Load](/api/policy-config-load.md), [Compile and Schema](/api/datalog-compile-schema.md), [package policy](/packages/policy.md).

# Citations

[1] [policy/facts.go](../../policy/facts.go)
[2] [DESIGN.md, Fact constructors and Schema](../../docs/DESIGN.md)
