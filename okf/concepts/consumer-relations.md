---
type: Concept
title: Consumer relations
description: "A policy may declare base relations in Config.Relations for rule sets to read as standing facts."
tags: [policy, relations, schema]
timestamp: 2026-10-27T00:00:00Z
---

# What it is

`policy.Config.Relations` declares consumer base relations. Each entry names a relation and gives one kind per column. The relations are standing only; `Request` and `requestRelations` do not change. A nil map declares none and preserves v0.2.2 behavior.

# Invariants

- A declared name cannot be one of the eight PARC/base relations (`principal`, `action`, `resource`, `context`, `role`, `inherits`, `grants`, `denied`) or a request relation.
- A relation must have at least one column. Each kind must be `KindString`, `KindNumber`, or `KindBool`.
- For each column, constants are checked against the declared kind; empty strings are refused in string columns (variables pass).
- A rule set cannot derive a declared base relation. A built-in name cannot be declared as a base relation.
- Declarations are copied into the policy; changing the caller's map after `Load` has no effect.
- The declaration adds arities and checks to the schema used by `NewBase` and `Compile` (so standing facts and rule atoms are checked).
- A recursive consumer relation is derived in full for each decision, so derived bytes bind before steps. A 2,000-person binary tree (depth 10) uses 1% of `maxDecisionSteps` and 25% of `maxDecisionBytes`; a 2,000-person single chain exceeds `maxDecisionBytes`. Demand evaluation is the planned fix.

# Related

[policy.Config and policy.Load](/api/policy-config-load.md), [fact constructors](/api/policy-fact-constructors.md).

# Citations

[1] [policy/policy.go](../../policy/policy.go)
[2] [policy/facts.go](../../policy/facts.go)

