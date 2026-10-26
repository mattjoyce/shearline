---
type: Go Package
title: Engine (tarski)
description: "The engine is no longer a package of this module: it is github.com/mattjoyce/tarski, required at v0.2.2. What it owns and what the policy layer relies on."
resource: github.com/mattjoyce/tarski
tags: [go, engine, datalog, tarski]
timestamp: 2026-10-26T00:00:00Z
---

# Where it went

Until 2026-10-26 this was `github.com/mattjoyce/shearline/datalog`. It was cut out, with its history, at c4f75ab into [tarski](https://github.com/mattjoyce/tarski) (package `tarski`), and Shearline now requires `github.com/mattjoyce/tarski` v0.2.2. The `datalog/` directory is gone. The contract is tarski's `docs/DESIGN.md` and `CHANGELOG.md`; this entry only says what Shearline leans on. Shearline v0.2.0 is the first release on tarski; see [CHANGELOG.md](../../CHANGELOG.md).

# What it owns

Evaluation. The caller owns the vocabulary. Nothing in the engine reads a file or the network, keeps state between calls, or knows what a relation means. Error messages are prefixed `tarski:` (they were `datalog:`).

| Area | Owns |
|---|---|
| Language | `Parse`, `Program`, `Rule`, `Atom`, `Literal`, `Comparison`, `Term`; v0.2 adds `Aggregate` |
| Validation | `Compile` against a `Schema`: arities, safety, stratification, schema checks |
| Data | `Const`, `Tuple`, `Fact`, `Base` (`NewBase`, `With`, `Len`) |
| Evaluation | `Evaluate`, `Options`, `Result`, `Answer` |
| Provenance | `Proof`, `Result.Proof`, `Absence`, `RuleFailure`, `Result.WhyNot` |
| Identity | `RuleSet`, `NewRuleSet`, `ID`, `Verify` |
| Errors | `ErrSyntax`, `ErrInvalid`, `ErrTooLarge`, `*TooLarge` |

# What changed for Shearline in the move

- Import path and package name: `tarski.X` where it was `datalog.X`.
- Error prefix `tarski:`.
- Numbers are exact decimals, and one at or above 1e6 or below 1e-4 prints in full, which moves the hash of an answer that carries one. Nothing in the bundled rules or tests does.
- Aggregates exist. `policy.Load` refuses a rule set that uses one (`plan.HasAggregates()`, wrapping `tarski.ErrInvalid`): a decision is per request and aggregates are for queries.

# Invariants the policy layer relies on

- Always terminates; deterministic answers, proofs and hashes. See [proof](/concepts/proof.md).
- A `Base` and a `Plan` are immutable once built; a `Result` is read-only. See [value and builder](/concepts/value-and-builder.md).
- Bounds are approximate on bytes, exact on steps. Since v0.2.2 a step is a tuple visited, a rule applied or a built-in called. See [per-request evaluation](/concepts/per-request-evaluation.md).

# Used by

[Package policy](/packages/policy.md) is the consumer in this module. Anam and Ductile will import tarski too; when they do, Go resolves them and Shearline to one version.

# Citations

[1] [tarski README](https://github.com/mattjoyce/tarski/blob/v0.2.2/README.md)
[2] [tarski docs/DESIGN.md](https://github.com/mattjoyce/tarski/blob/v0.2.2/docs/DESIGN.md)
[3] [ENGINE.md](../../docs/ENGINE.md)
