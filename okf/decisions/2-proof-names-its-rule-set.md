---
type: Decision
title: "Decision 2: a proof names its rule set"
description: "Rule gains Set and Text so a proof says which set, which line and what text; the hash stays in Decision.RuleSets."
tags: [decision, proof, rule-set, identity, matt]
timestamp: 2026-10-22T00:00:00Z
---

# Context

A proof's `Rule` was a live pointer into the shared plan with a line number and nothing else. With two loaded sets (`RBAC()` plus a consumer set) a line number is ambiguous, and a receipt stored after the rules moved could not say which text it meant. Both panels flagged the missing rule-set identity (Hickey H2; Ousterhout O-2).

# Options considered

| Option | For | Against |
|---|---|---|
| Leave the rule as a line number | No change | Ambiguous with two sets; receipt cannot name its rule |
| Put the full hash in each rule | Receipt self-contained | Hash is a property of the set, repeated per rule; bloats every proof |
| `Rule.Set` ("name/version") and `Rule.Text` | Names set, line and text; hash stays in one place | `Program` must know its set |

# Decision

`Rule` gains `Set` ("name/version", filled by `Compile` from `Program.Name`, which `NewRuleSet` sets; empty for a bare `Parse`) and `Text` (the rule as written, sliced by the parser). A proof's rule therefore says which set, which line and what text. The hash stays in `Decision.RuleSets`, and a stored receipt keeps both.

# Consequences

- Proof lines read `rbac/1:19  allow(P, A, R) :- ...`.
- `Program` carries a `Name`, and `Verify` re-derives it so a set cannot lie about its own name.
- This closed the "rule-set identity stays open" item from the module panel.

See [proof](/concepts/proof.md) and [rule-set identity](/concepts/rule-set-identity.md).

# Citations

[1] [DESIGN.md, Decided with Matt, item 2](../../docs/DESIGN.md)
[2] [Module panel collation](../../docs/reviews/luminary-module-2026-10-22/00-collation.md)
[3] [tarski/bundled.go](https://github.com/mattjoyce/tarski/blob/v0.2.2/bundled.go)
