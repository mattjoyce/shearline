---
type: Rule Set
title: rbac/1
description: "The bundled rule set: roles, inheritance, prefix grants and a deny hook, every rule seeded by the request."
resource: github.com/mattjoyce/shearline/policy/rbac.dl
tags: [rbac, rules, bundled, datalog]
timestamp: 2026-10-22T00:00:00Z
---

# Identity

Name `rbac`, version 1, embedded from `policy/rbac.dl` and parsed once at package init (`mustRuleSet`), so a broken asset fails at process start. `policy.RBAC()` returns a fresh copy with a cloned program. Its ID is `rbac/1 sha256:<hex>`; the hash changes whenever the file does. See [rule-set identity](/concepts/rule-set-identity.md).

# The file, verbatim

```
% The bounds on an evaluation (bytes derived, join steps) are per request, so the rules
% must be too: every rule is seeded by the request's principal, action or resource, and a
% decision derives only this principal's world, however many other principals are loaded.

% deny(P, A, R): the consumer's denials. The base relation denied/3 carries them as
% facts; a consumer rule set may add rules to deny/3 for derived denials. A deny rule
% that reads allow/3 is a negation cycle and is refused at Load.
deny(P, A, R) :- principal(P), action(A), resource(R), denied(P, A, R).

% holds(P, R): P holds role R, directly or through inherits. Seeded by principal(P) so
% the closure is computed for the requester alone.
holds(P, R) :- principal(P), role(P, R).
holds(P, R) :- holds(P, Q), inherits(Q, R).

% permit(P, A, Pat): P holds a role that grants A over resources with prefix Pat.
permit(P, A, Pat) :- principal(P), action(A), holds(P, Role), grants(Role, A, Pat).

% allow(P, A, R): the request's resource is under a permitted prefix and not denied.
allow(P, A, R) :- permit(P, A, Pat), resource(R), prefix(Pat, R), not deny(P, A, R).
```

# Rules

| Line | Rule | What it says |
|---|---|---|
| 8 | `deny(P, A, R) :- principal(P), action(A), resource(R), denied(P, A, R).` | The deny hook. The base relation `denied/3` carries the consumer's denials (`policy.Deny`); `deny/3` is derived from it for this request. A consumer set may add more `deny/3` rules for derived denials |
| 12 | `holds(P, R) :- principal(P), role(P, R).` | The requester's direct roles |
| 13 | `holds(P, R) :- holds(P, Q), inherits(Q, R).` | Inheritance closure, recursive, for the requester alone |
| 16 | `permit(P, A, Pat) :- principal(P), action(A), holds(P, Role), grants(Role, A, Pat).` | The requester holds a role that grants this action over prefix `Pat`. `Permitted` reads this relation |
| 19 | `allow(P, A, R) :- permit(P, A, Pat), resource(R), prefix(Pat, R), not deny(P, A, R).` | The resource is under a permitted prefix and is not denied. `Decide` reads this relation |

# The deny hook

`denied/3` is a base relation and `deny/3` is derived from it. A relation cannot be both base and declared, which is why the facts are `denied` and the hook is `deny` (found when the policy teammate built it). `deny` is negated in `allow`, so the engine stratifies `deny` below `allow`. A consumer rule that makes `deny` depend on `allow` is a negation cycle and `Load` refuses it. Deny is exact per resource; a subtree denial is a consumer rule over `prefix/2`. See [prefix grants](/concepts/prefix-grants.md) and [stratified negation](/concepts/stratified-negation.md).

A consumer's own `allow` rule does not inherit the guard. A consumer set can add rules to `allow/3` and omit `not deny`, and the engine will accept it. Rule authors must preserve every constraint in every allow path.

# Request seeding

Each rule that starts a derivation begins with `principal(P)`, `action(A)` or `resource(R)`, the facts a request supplies. `holds` is seeded by `principal(P)`, `permit` by `principal` and `action`, `deny` by all three, and `allow` by `resource(R)` plus `permit`. So one decision derives one principal's roles and nothing about strangers. Without seeding, the bounds trip at about 38,000 `(principal, role)` pairs. See [per-request evaluation](/concepts/per-request-evaluation.md).

# Reading a refusal

For this rule set a [why-not](/concepts/why-not-provenance.md) reads: no `role` fact for P (stops in `holds`), no `grants` for A on a held role (stops in `permit`), no permitted prefix matches R (stops at `prefix`), or denied by a proof of `deny(P, A, R)` (stops at `not deny`).

# Not in this set

No context rules. The bundled rules do not read `context/2`; a consumer set adds them, after [declaring the keys](/concepts/declared-context.md).

# Citations

[1] [policy/rbac.dl](../../policy/rbac.dl)
[2] [DESIGN.md, rbac.dl](../../docs/DESIGN.md)
[3] [README, example output](../../README.md)
