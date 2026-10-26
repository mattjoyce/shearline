---
type: Standard
title: Engineering standards
description: "The Go standards Shearline holds to, and the gates CI runs on every push."
tags: [standard, go, ci, testing, lint]
timestamp: 2026-10-22T00:00:00Z
---

# Standards

| Area | Standard |
|---|---|
| Language | Go 1.25 or later, one dependency, the tarski engine (our own); no other without a case. The `logsvc` example is its own module and adds YAML there only |
| Format | `gofmt` and `goimports` |
| Lint | `golangci-lint`, `gosec` |
| Tests | Table-driven, real components, no mocks of the engine |
| Test flags | `go test -race -shuffle=on` |
| Coverage | Above 80 percent |
| Size | Functions under about 60 lines, files under about 400 |
| Docs | Every exported name documented; defaults are named constants with a reason, invariants are constants with a reason they are not configurable |
| Naming | Relation names lower-case, `snake_case` for multi-word; rule-set files `*.dl`; modules and packages lower-case single words |
| Vocabulary | The module never uses "importer", "source", "projection" or "tenant" |
| Licence | Apache 2.0 |
| Changes | Each tagged release has an entry in `CHANGELOG.md`. The current release is v0.2.2 |

# CI gates

`.github/workflows/ci.yml` runs on push to master and on every pull request, with read-only `contents` permission. The Go module cache is keyed on both `go.sum` files (root and `examples/logsvc`).

1. `gofmt`: fails if `gofmt -l .` prints anything.
2. `go vet ./...`.
3. `go test -race -shuffle=on -count=1 -cover ./...`.
4. `golangci-lint` v2.15.0 via its action.
5. `gosec -quiet ./...`.
6. The `logsvc` example, as its own module: `go vet`, `go test -race` and `gosec` from `examples/logsvc`.

# Tests that hold the docs

- `policy/example_test.go` is a compiled twin of the README program and checks its output, so the README cannot drift from the API.
- `TestRBAC_Parses` holds the embedded rule set.
- A seeded property test holds `With` equal to `NewBase` over all the facts. See [Base.With](/api/datalog-base-with.md).
- Fuzz suites and oracles (`fuzz_test.go`, `oracle_test.go`, now in tarski) came from the [edge audit](/reviews/edge-audit-2026-10-22.md).
- A 50,000-principal test holds the [seeded rules](/concepts/per-request-evaluation.md).

# Performance gate

Refactors that touch the hot loop are gated by `BenchmarkEvaluate`: kept only within noise. See [decision 6](/decisions/6-relation-is-value-or-builder.md).

# Review gates

Design, whole module and adversarial review each ran before release. See [design panel](/reviews/design-panel-2026-10-22.md) and [module panel](/reviews/module-panel-2026-10-22.md).

# Citations

[1] [README, engineering standards](../../README.md)
[2] [CI workflow](../../.github/workflows/ci.yml)
[3] [SPEC.md, Standards](../../SPEC.md)
[4] [CHANGELOG.md](../../CHANGELOG.md)
