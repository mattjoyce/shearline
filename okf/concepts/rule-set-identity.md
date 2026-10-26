---
type: Concept
title: Rule-set identity
description: "A rule set is a named, versioned program with a sha256 over its text; Verify re-parses so the rules that ran are the rules that were hashed."
tags: [identity, hash, verify, receipt]
timestamp: 2026-10-22T00:00:00Z
---

> This describes the engine, which now lives in [tarski](https://github.com/mattjoyce/tarski) (required at v0.3.2); Shearline cut it out at c4f75ab and the code is no longer in this module. The contract is tarski's `docs/DESIGN.md`. Kept here because the policy layer relies on it.

# What a rule set is

`tarski.RuleSet{Name, Version, Hash, Program}`. `NewRuleSet(name, version, text)` parses the text, hashes it, and tags the program `"name/version"`. It fails rather than returning a partial set.

- The name is non-empty and free of whitespace, slash and semicolon, because `ID()` is `"name/version hash"` and several are joined with `"; "`.
- The version is not negative. It moves when the rules' meaning does, and the hash catches any change the version did not.

# ID

`RuleSet.ID()` is `"rbac/1 sha256:0bf5a121..."`. `Decision.RuleSets` carries one ID per loaded set, in load order, as the policy's own copy. A stored receipt keeps both the IDs and the proof, which names `set:line text` for each rule. See [proof](/concepts/proof.md).

# Verify

`Verify` holds a set to its text, because the text is the one authoritative representation.

1. The program must have text, else `ErrInvalid`.
2. The hash is recomputed over `Program.Text` and compared.
3. The text is parsed again, named as `NewRuleSet` would, and must deep-equal `Program`.

So an AST edited after `NewRuleSet`, or built by hand around someone else's hash, is refused. `policy.Load` calls `Verify` on every set, so the ID in a decision is the hash of the rules that ran.

# Why re-parse

The edge audit (finding 2) showed a parsed head edited before `Load` while text and hash stayed put. `Load` succeeded and authorised a different resource, while the proof's rule text still read the original. The first fix hashed the text only; the audit's lesson was that the hash must bind the executable program. Cloning alone would not repair this. See [edge audit](/reviews/edge-audit-2026-10-22.md).

# The plan owns its programs

`Compile` deep-copies every program (`Program.Clone`), so a caller's later edits reach no plan (edge audit finding 1). The proof's rule is a further deep copy, so the plan is not reachable through a receipt.

# Related

- [policy.Load](/api/policy-config-load.md) calls `Verify` and compiles.
- [tarski.Compile and Schema](/api/datalog-compile-schema.md).
- [The RBAC rule set](/rulesets/rbac.md) is `rbac/1`.

# Citations

[1] [DESIGN.md, RuleSet, ID, Verify](../../docs/DESIGN.md)
[2] [tarski/bundled.go](https://github.com/mattjoyce/tarski/blob/v0.3.2/bundled.go)
[3] [Edge audit, findings 1 and 2](../../docs/reviews/edge-audit-2026-10-22.md)
