---
type: Go API
title: policy.Decide
description: "Decide answers one request with Allow, a Proof or a Why, and the loaded rule-set IDs; it is total, and an error is always a deny."
resource: github.com/mattjoyce/shearline/policy.Policy.Decide
tags: [go, api, decide, decision]
timestamp: 2026-10-22T00:00:00Z
---

# Signature

```go
type Request struct {
    Principal, Action, Resource string
    Context map[string]datalog.Const // becomes context(Key, Value) facts, keys sorted
}
type Decision struct {
    Allow    bool
    Proof    datalog.Proof   // why allowed; zero when denied
    Why      datalog.Absence // why denied; zero when allowed or on error
    RuleSets []string        // the loaded sets' IDs in load order
}
func (p Policy) Decide(ctx context.Context, req Request) (Decision, error)
```

# Steps

1. A cancelled `ctx` is an error before any work.
2. The request context is checked against the declared keys and kinds; failure wraps `ErrRequest`. See [declared context](/concepts/declared-context.md).
3. `standing.With(principal, action, resource, context...)` builds the per-request base. An empty field is refused by the schema check and wraps `ErrRequest`. See [Base.With](/api/datalog-base-with.md).
4. `Evaluate` runs with `MaxBytes 1<<20`, `MaxSteps 1<<22` and `Proofs` on. See [Evaluate and Options](/api/datalog-evaluate-options.md).
5. Membership in `allow/3` is read from `Result.Proof` (no `Answer` is built and hashed). If present, `Allow` and `Proof` are set.
6. Otherwise `Result.WhyNot("allow", tuple, 0)` fills `Why`. See [why-not provenance](/concepts/why-not-provenance.md).

# Errors

| Error | Meaning | Caller's response |
|---|---|---|
| wraps `ErrRequest` | Malformed request | A 4xx |
| wraps `ErrTooLarge` (`*TooLarge`) | Bound crossed; message adds "narrow the grants reachable from this principal" | A 5xx; fix the policy |
| context error | `ctx` cancelled or past deadline | Retry or give up |
| other | Broken policy or engine | A 5xx |

`Decision.RuleSets` is the policy's own copy, never a shared slice, and is filled even on error.

# Receipts

`Proof` and `Why` print as indented text and marshal to compact JSON, so a consumer can log or store a receipt without learning the AST. See [proof](/concepts/proof.md).

# Related

[fail-closed decision](/concepts/fail-closed-decision.md), [per-request evaluation](/concepts/per-request-evaluation.md), [policy.Load](/api/policy-config-load.md).

# Citations

[1] [policy/policy.go](../../policy/policy.go)
[2] [DESIGN.md, Decide and Decision](../../docs/DESIGN.md)
