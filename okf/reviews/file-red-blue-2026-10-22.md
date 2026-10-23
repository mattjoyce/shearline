---
title: Red/blue file audit, 2026-10-22
description: "Two independent reviewers per implementation file at 5fe3a78: 19 files, 40 reports, 14 intersection findings; none an authorization bypass; all resolved on master the next day."
type: review
date: 2026-10-22
related:
  - "[[edge-audit-2026-10-22]]"
  - "[[../concepts/proof]]"
  - "[[../concepts/why-not-provenance]]"
  - "[[../api/datalog-result-proof-whynot]]"
sources:
  - "[docs/reviews/file-red-blue-2026-10-22/00-findings-matrix.md](../../docs/reviews/file-red-blue-2026-10-22/00-findings-matrix.md)"
  - "[docs/reviews/file-red-blue-2026-10-22/00-resolution.md](../../docs/reviews/file-red-blue-2026-10-22/00-resolution.md)"
---

# What was done

Every implementation file in `datalog` and `policy` was read by a red reviewer (seeking failures) and, independently, by a blue reviewer (checking defensive contracts), each with one file and no history. The matrix lists the findings both passes made.

# What was found

Nothing reachable through `policy.Request`: `Decide` was sound. The findings clustered on the explanation and the receipts: a why-not walk that lost a goal's repeated variables, proof JSON exponential in a shared derivation, rule text with its constants' whitespace collapsed, an `Answer` whose fields could disagree with its hash, a decoder that admitted what no evaluation answers, and hand-built values (a NUL in a name, an invalid UTF-8 constant, a zero-arity head) that the parser could never make but the exported types could.

# What changed

Three commits on master, each finding named in `00-resolution.md`. The API changes: `Answer` is immutable with accessors; `Plan.Defines`; a string literal reads JSON or Go escapes; `Number` is JSON's grammar; `NewRuleSet` refuses empty text; proof JSON writes a shared sub-proof once with `shown: true`.
