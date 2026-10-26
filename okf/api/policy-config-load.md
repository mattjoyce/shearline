---
type: Go API
title: policy.Config and policy.Load
description: "Load verifies each rule set, compiles them over the PARC schema, holds the sorted standing facts, and refuses anything Decide or Permitted would fail on."
resource: github.com/mattjoyce/shearline/policy.Load
tags: [go, api, load, config]
timestamp: 2026-10-22T00:00:00Z
---

# Signature

```go
type Config struct {
    Standing []tarski.Fact
    Sets     []tarski.RuleSet
    Context  map[string]tarski.Kind
}
func Load(cfg Config) (Policy, error)
```

# What Load does, in order

1. Copies and checks `Config.Context` (non-empty keys, a real kind). See [declared context](/concepts/declared-context.md).
2. Refuses a standing fact in `principal`, `action`, `resource` or `context`: it would speak for every request.
3. Sorts a copy of the standing facts by relation, then by each position's kind and text, so a [proof](/concepts/proof.md) is a function of policy content.
4. Calls `Verify` on every set. See [rule-set identity](/concepts/rule-set-identity.md).
5. Compiles the sets in order over the PARC schema built from the declared context. See [Compile and Schema](/api/datalog-compile-schema.md).
6. Requires the plan to define `allow/3` and `permit/3`, else `ErrInvalid` naming the gap ("load RBAC() first or define it", or the wrong arity).
7. Builds the standing base with [NewBase](/api/datalog-base-with.md), which applies the schema checks to every fact.

# Contract

- The first set is normally `RBAC()`. A consumer extends with a later set to define denials, add roles or read context.
- Rule sets are policy, never user input. A later set may add rules to any relation, `allow/3` included. The only promise that survives an arbitrary set is deny by absence.
- Any error is the consumer's to fix before serving: a bad policy never serves.
- Every error is prefixed `policy: load:`.
- An empty context value is allowed and becomes a wildcard if a consumer rule feeds it to `prefix/2`.
- A `Policy` zero value fails closed without panicking for a well-formed request (edge audit evidence).

# Example

```go
p, err := policy.Load(policy.Config{
    Standing: []tarski.Fact{
        policy.Role("ana", "operator"),
        policy.Inherits("operator", "reader"),
        policy.Grant("reader", "read", "timeline/"),
    },
    Sets: []tarski.RuleSet{policy.RBAC()},
})
```

# Related

[policy.Decide](/api/policy-decide.md), [policy.Permitted](/api/policy-permitted.md), [fact constructors](/api/policy-fact-constructors.md), [package policy](/packages/policy.md).

# Citations

[1] [policy/policy.go](../../policy/policy.go)
[2] [DESIGN.md, Load and Config](../../docs/DESIGN.md)
[3] [README, Try it in a minute](../../README.md)
