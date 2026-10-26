---
type: Decision
title: "Decision 3: a refusal explains itself"
description: "Decide fills Decision.Why with engine-level why-not provenance, deterministic and RBAC-blind, with the blocking deny's proof when deny/3 fired."
tags: [decision, whynot, transparency, matt]
timestamp: 2026-10-22T00:00:00Z
---

# Context

The spec's job story: when a request is refused, show the derivation that decided it, so "why" is answered from the decision and not from a log. A proof only explains an allow. A deny by absence has no derivation to show. The module panel also noted "proofs for denials" as a tension between intent and contract.

# Options considered

| Option | For | Against |
|---|---|---|
| Minimalism: `Decision` carries only `Allow` and `Proof` | No extra walk per refusal | A refusal is opaque; operators fall back to guessing or logs |
| Why-not in `policy`, RBAC-aware | Friendlier text | `policy` would need to know the rules' shape; breaks for a consumer set |
| Engine-level `WhyNot`, RBAC-blind | Works for any rule set; deterministic | One walk per refusal; value is partly future state |

# Decision

Matt chose transparency over minimalism (2026-10-22). `Decide` fills `Decision.Why` with why-not provenance from the engine (`Result.WhyNot`), deterministic and RBAC-blind, with the blocking deny's proof inside it when `deny/3` fired. `Proof` and `Absence` both render as text and compact JSON, so a consumer can log or store a receipt without learning the AST.

# Consequences

- Cost: one engine walk per refusal, bounded by 65,536 candidates and a depth of 8.
- A deny through `deny/3` shows as the negation that held, with `Blocked` carrying the proof.
- `Truncated` and the cycle rule keep the walk finite and honest.
- The engine gained `Absence`, `RuleFailure`, `WhyNot` and the renderers; the same machinery serves any consumer.

See [why-not provenance](/concepts/why-not-provenance.md), [policy.Decide](/api/policy-decide.md) and [Result.WhyNot](/api/datalog-result-proof-whynot.md).

# Citations

[1] [DESIGN.md, Decided with Matt, item 3](../../docs/DESIGN.md)
[2] [SPEC.md, job stories](../../SPEC.md)
[3] [tarski/whynot.go](https://github.com/mattjoyce/tarski/blob/v0.3.2/whynot.go)
