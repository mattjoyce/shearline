---
type: Decision
title: "Decision 5: the inverse listing is the loop"
description: "Who may do A on R is answered by calling Decide once per known principal; no Principals call and no second rule set is planned."
tags: [decision, listing, scale, matt]
timestamp: 2026-10-22T00:00:00Z
---

# Context

`Permitted` answers "what may this principal do". Consumers will also ask the inverse: "who may do A on R". The obvious implementation is the bundled rules run without the `principal(P)` seed, which would reintroduce the whole-world evaluation that the [38k cliff](/concepts/per-request-evaluation.md) removed.

# Options considered

| Option | For | Against |
|---|---|---|
| Unseeded rules for listing | One call | Whole-world evaluation; trips the byte bound at scale |
| A second bundled rule set for listing, with a larger bound | Fast at scale | Two evaluation modes to keep consistent, with a test that they agree; built for a scale that does not exist |
| `Decide` in a loop over the consumer's principals | The listing is the gate, so it cannot disagree; carries context if the caller supplies it | O(principals) evaluations |

The first draft of this decision (option 2, "wait until a consumer asks") was replaced the same day by the third.

# Decision

"Who may do A on R" is answered by calling `Decide` once per principal the consumer knows. Shearline plans no `Principals` call and no second rule set.

# Rationale

At the scale its consumers have, tens of tokens, the loop costs microseconds. A second evaluation mode would be a thing to keep consistent for a scale that does not exist. Revisit only when a consumer has many thousands of principals and asks often.

# Consequences

- `Permitted` stays the forward listing, of prefixes, without `deny` applied.
- The spec's job story about listing is met by one gate, not two.

See [policy.Permitted](/api/policy-permitted.md), [policy.Decide](/api/policy-decide.md) and [the RBAC rule set](/rulesets/rbac.md).

# Citations

[1] [DESIGN.md, Decided with Matt, item 5](../../docs/DESIGN.md)
[2] [SPEC.md, job stories](../../SPEC.md)
