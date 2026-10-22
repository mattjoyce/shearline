---
type: Review
title: Adversarial edge audit, 2026-10-22
description: "Adversarial module audit with fuzzing and oracles at b7a6717: six findings, none reachable through policy.Request, all accepted and resolved."
tags: [review, audit, fuzz, adversarial]
timestamp: 2026-10-22T00:00:00Z
---

# Method

An adversarial audit on its own branch against base commit b7a6717, in a separate worktree. Production code was left unchanged; tests were written red to demonstrate each finding. Scope: public API integrity, authorisation isolation, malformed program values, serialisation identity, schema enforcement, recursive evaluation, negation and concurrency or limit regressions.

Evidence beyond the findings:

| Technique | Result |
|---|---|
| Parser, compiler and bounded evaluator fuzz, 30 s | 773,297 executions, no failure |
| Transitive closure versus a Floyd-Warshall oracle, 30 s | 188,602 executions, no failure |
| Repeated-variable joins and stratified negation versus a Boolean-matrix oracle, 30 s | 344,021 executions, no failure |
| Corporate-chat delegate and ownership cases | Self allowed; other employee, near-prefix ID, missing and wrong delegate denied |
| Zero-valued `Policy` | Fails closed without panicking |

Fuzz runs are finite exploration, not proof. The audit did not establish a total memory or wall-clock ceiling for parsing, compilation, proof construction or rendering.

# Counts

| Severity | Count |
|---|---|
| High | 2 |
| Medium | 3 |
| Contract gap | 1 |

# Findings and where they landed

| # | Severity | Finding | Landed |
|---|---|---|---|
| 1 | High | A loaded policy aliases caller-owned program data; editing the original `Program` changed the loaded policy's authorisation | `Compile` deep-copies every program. See [rule-set identity](/concepts/rule-set-identity.md) |
| 2 | High | `Verify` hashes only the text, while evaluation uses the independently mutable AST | `Verify` re-parses the text and requires the result to equal `Program` |
| 3 | Medium | A malformed public AST (an empty `Literal`) panics at `Compile` | Structural validation; `ErrInvalid` |
| 4 | Medium | An unknown comparison operator silently means `>=` | Operators validated at `Compile` |
| 5 | Medium | Distinct strings (invalid UTF-8 versus U+FFFD) share answer bytes and hash | Valid UTF-8 domain enforced at the lexer and base. See [constants](/concepts/constants.md) |
| 6 | Contract gap | A plan's schema checks do not constrain a foreign base | Contract corrected: compatibility is by shape only. See [Compile and Schema](/api/datalog-compile-schema.md) |

Findings 1 to 5 were fixed on master and the audit's tests adopted as regressions (each red on b7a6717, green after), plus the three fuzz suites and oracles in `datalog/fuzz_test.go` and `datalog/oracle_test.go`. None of the six was reachable through `policy.Request`; all need the Go program object.

# Integration edges the audit recorded

- Principal and delegate identity must come from trusted caller context.
- Prefix grants have no path normalisation: canonicalise before evaluation and execute the same target. See [prefix grants](/concepts/prefix-grants.md).
- `Permitted` returns prefixes and does not apply concrete denials.
- Numbers are float64; integers beyond 2^53 are not exact identifiers.
- Consumer rules can add `allow` directly and omit the bundled deny guard. See [the RBAC rule set](/rulesets/rbac.md).

# Citations

[1] [Edge audit](../../docs/reviews/edge-audit-2026-10-22.md)
[2] [datalog/fuzz_test.go](../../datalog/fuzz_test.go)
[3] [datalog/oracle_test.go](../../datalog/oracle_test.go)
