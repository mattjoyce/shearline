---
type: Go Package
title: Package policy
description: "The PARC layer over the engine: Load, Decide, Permitted, fact constructors and the bundled RBAC rule set."
resource: github.com/mattjoyce/shearline/policy
tags: [go, policy, parc, rbac]
timestamp: 2026-10-22T00:00:00Z
---

# What it owns

The PARC vocabulary (principal, action, resource, context) and the bundled `rbac.dl`. It owns nothing about how rules are evaluated; that is [the engine, tarski](/packages/datalog.md). A `Policy` is a value built once and safe to share between goroutines. It holds no state a request can change. The current release is v0.2.2, which requires tarski v0.3.2; [CHANGELOG.md](../../CHANGELOG.md) lists what changed from v0.1.0.

# Public surface

| Name | Kind | Summary | Concept or API |
|---|---|---|---|
| `Config` | struct | `Standing`, `Sets`, `Context`, `Relations` | [Load](/api/policy-config-load.md) |
| `Load` | func | Verify, compile, hold standing facts | [Load](/api/policy-config-load.md) |
| `Policy.Decide` | method | One request to one `Decision` | [Decide](/api/policy-decide.md) |
| `Policy.Permitted` | method | Grant prefixes a principal holds for an action | [Permitted](/api/policy-permitted.md) |
| `Request`, `Decision` | structs | PARC in, allow plus proof or why-not out | [Decide](/api/policy-decide.md) |
| `Role`, `Inherits`, `Grant`, `Deny` | funcs | Build standing facts | [Fact constructors](/api/policy-fact-constructors.md) |
| `Schema` | func | The PARC base relations, a fresh value per call | [Fact constructors](/api/policy-fact-constructors.md) |
| `RBAC` | func | The bundled rule set, `rbac/1` | [RBAC](/rulesets/rbac.md) |
| `ErrRequest` | var | Marks a malformed request | [Fail-closed decision](/concepts/fail-closed-decision.md) |

# Invariants

- Every failure is a deny; `Allow` is false whenever `err != nil`. See [fail-closed decision](/concepts/fail-closed-decision.md).
- `Load` refuses what `Decide` or `Permitted` would fail on: bad hash, missing `allow/3` or `permit/3`, a standing fact in a request relation.
- Context keys are declared or refused. See [declared context](/concepts/declared-context.md).
- The decision bounds are constants, not knobs. See [per-request evaluation](/concepts/per-request-evaluation.md).
- Resource names are matched as given. See [prefix grants](/concepts/prefix-grants.md).
- `RBAC()` and `Schema()` return fresh values so no caller can mutate what later Loads compile against. `RBAC` is parsed at package init, so a broken asset fails at process start (`TestRBAC_Parses` holds it).
- Consumer relations may be declared via `Config.Relations` (standing only). See [consumer relations](/concepts/consumer-relations.md).
- Vocabulary rule: "importer", "source", "projection" and "tenant" never appear in the module; the consumer owns what names mean.

# Files

`policy.go` (Load, Decide, Permitted, errors), `facts.go` (schema, constructors, request facts), `rbac.dl` (embedded), `doc.go`, and tests split by topic: `load_test.go`, `decide_test.go`, `permitted_test.go`, `audit_test.go`, `example_test.go`.

# Citations

[1] [policy/doc.go](../../policy/doc.go)
[2] [policy/policy.go](../../policy/policy.go)
[3] [policy/facts.go](../../policy/facts.go)
[4] [DESIGN.md, Package policy](../../docs/DESIGN.md)
[5] [CHANGELOG.md](../../CHANGELOG.md)
