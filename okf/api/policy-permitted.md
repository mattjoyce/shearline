---
type: Go API
title: policy.Permitted
description: "Permitted returns the grant prefixes a principal holds for an action, from the same rules that gate requests; it does not apply deny."
resource: github.com/mattjoyce/shearline/policy.Policy.Permitted
tags: [go, api, permitted, listing]
timestamp: 2026-10-22T00:00:00Z
---

# Signature

```go
func (p Policy) Permitted(ctx context.Context, principal, action string) ([]string, error)
```

# Behaviour

- It evaluates with `principal` and `action` facts only (no resource), no proofs, and the same two bounds as `Decide`, then reads `permit/3`.
- The result is the third column for this principal and action, sorted and deduplicated. A rule set may derive `permit` tuples for anyone; the listing is this caller's only.
- It returns prefixes, not resources. `deny/3` does not apply, because deny is per concrete resource. See [prefix grants](/concepts/prefix-grants.md).
- Because it reads the same rules, the listing cannot disagree with the gate on grants.

# The permit/3 contract

The third column is a contract a consumer's rule set must keep: a non-empty string. `Load` refuses a plan where `permit` is not `permit/3`. If a tuple for this caller breaks the shape, `Permitted` returns `ErrInvalid` rather than listing a blank or `"7"`.

# Who may do A on R

There is no inverse call. Loop over your principals and call [`Decide`](/api/policy-decide.md) once each. See [decision 5](/decisions/5-inverse-listing-is-the-loop.md).

# Errors

A cancelled context, a malformed argument (empty principal or action wraps `ErrRequest`), a `*TooLarge` wrapped with the remedy, or `ErrInvalid` from the contract above.

# Related

[policy.Load](/api/policy-config-load.md), [per-request evaluation](/concepts/per-request-evaluation.md).

# Citations

[1] [policy/policy.go](../../policy/policy.go)
[2] [policy/permitted_test.go](../../policy/permitted_test.go)
[3] [DESIGN.md, Permitted](../../docs/DESIGN.md)
