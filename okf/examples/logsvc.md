---
type: Example
title: logsvc, embedding Shearline in a service
description: "A worked example: a log service whose roles and tokens live in two YAML files, one function that turns them into facts, an atomic policy swap for run-time change, and the matrix pinned as a test. The code comments are the tutorial."
resource: github.com/mattjoyce/shearline/examples/logsvc
tags: [example, tutorial, rbac, yaml, embedding]
timestamp: 2026-10-23T00:00:00Z
---

# What it shows

A log store with eight endpoints (`/query`, `/submit`, `/start`, `/stop`, `/mint`, `/revoke`, `/grant`, `/expire`), four roles (RO, RW, Operator, Admin) and a few tokens. The matrix is two YAML files: `rbac.yaml` holds roles, inheritance and grants and changes with the product; `tokens.yaml` holds token-to-role assignments and changes at run time.

One function, `Facts`, translates both into the three fact kinds of [package policy](/packages/policy.md): `Inherits`, `Grant`, `Role`. It is the only place the service's vocabulary meets the engine's, so it also catches a role name one file uses and the other does not define.

# Why it is shaped this way

- **Its own Go module.** The example needs a YAML library, and the root module's only dependency is the tarski engine ([engineering standards](/standards/engineering.md)). A nested `go.mod` keeps the YAML dependency out of the root. Its `replace ../..` is deliberate: the one Shearline tag, v0.1.0, predates tarski, so the example builds against the checkout until a tarski-based release is tagged.
- **Roles as a lattice.** Admin inherits both RW and Operator, which do not touch. The [RBAC rule set](/rulesets/rbac.md) computes the transitive closure; the proof shows the path (Admin → Operator → `/stop`).
- **Fail closed three times.** The YAML decoder refuses unknown keys, `Facts` refuses dangling names, `Load` refuses malformed facts ([fail-closed decision](/concepts/fail-closed-decision.md)). A bad reload leaves the old policy in force.
- **Policy as a value.** `Guard` swaps a new `Policy` behind an `atomic.Pointer`; readers never lock ([value and builder](/concepts/value-and-builder.md)).
- **Endpoints as prefixes.** Grants match by prefix ([prefix grants](/concepts/prefix-grants.md)), so no endpoint may be a prefix of another; the comments say so.

# Reading order

`matrix.go` (package comment, then the YAML shapes and `Facts`), `load.go` (`LoadPolicy`, `Guard`), `main.go` (print the matrix or decide one request, with [proof](/concepts/proof.md) or [why-not](/concepts/why-not-provenance.md)), `logsvc_test.go` (the matrix as a table-driven test).

# Review

Authored 2026-10-23 and given a Naur × Procida authoring pass before commit.
