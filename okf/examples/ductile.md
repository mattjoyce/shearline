---
type: Example
title: ductile, Ductile's token scopes as a policy
description: "SPEC step 7 done: Ductile's scope lattice as Inherits facts, its route table as grants on capabilities, its tokens as Role facts, and a parity test holding the model to Ductile's own normalizeScopes and HasAnyScope on every scope, every pair and every route."
resource: github.com/mattjoyce/shearline/examples/ductile
tags: [example, ductile, rbac, parity, consumer]
timestamp: 2026-10-23T00:00:00Z
---

# What it shows

Ductile's flat scope set and hand-written implication table, restated as the three fact kinds of [package policy](/packages/policy.md): the implications as `Inherits`, each route as a `Grant` of `read` or `write` on a capability to the narrowest scope that reaches it, each token as `Role` facts. The [RBAC rule set](/rulesets/rbac.md) does the rest; no consumer rule set is needed.

# What it found

- No Anam-shaped assumption leaks: `TestParityWithDuctile` compares Shearline's decision with a transcription of Ductile's own semantics for every single scope, every pair, and every route, and they agree.
- Ductile's request shape, open question 5 of SPEC.md, is principal, action, resource and no context.
- Resources are capabilities (`jobs/status`, `plugin/invoke`), not paths, because paths are prefixes of one another ([prefix grants](/concepts/prefix-grants.md)).
- A denial carries its reason ([why-not provenance](/concepts/why-not-provenance.md)), where Ductile's 403 says "insufficient scope".

# Reading order

`scopes.go` (package comment, `Implies`, `Routes`, `Facts`), `main.go`, `scopes_test.go`.
