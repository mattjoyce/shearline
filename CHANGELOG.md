# Changelog

Shearline is pre-1.0 and the API may change. Each entry says what a consumer of the module can see.
Rule-set hashes and answer hashes are part of that: a change to either is listed here.

## Unreleased

- **Requires tarski v0.3.1.** Both modules moved up from v0.2.2. No answer, hash or proof byte
  changes, and the README example prints the same output. v0.3.0 added demand-driven evaluation
  (`Plan.Demand`, `Options.Goal`), additive. v0.3.1 is a documentation release: Proof and Absence
  JSON are now pinned by a golden test, the determinism promise says proofs also depend on fact
  order, and builtin refusals name the argument and show the signature. Shearline does not adopt
  demand evaluation here. `Load` already sorts the standing facts.
- **`examples/logsvc` requires the release.** Its `go.mod` requires Shearline v0.2.0 from the
  module proxy; the `replace ../..` directive is gone. ([PR #4](https://github.com/mattjoyce/shearline/pull/4))

## [v0.2.0](https://github.com/mattjoyce/shearline/releases/tag/v0.2.0) - 2026-10-26

[Compare v0.1.0...v0.2.0](https://github.com/mattjoyce/shearline/compare/v0.1.0...v0.2.0)

The rule engine moved out to its own module, [tarski](https://github.com/mattjoyce/tarski).
Shearline now requires tarski v0.2.2 and is the PARC layer over it.
([PR #1](https://github.com/mattjoyce/shearline/pull/1), [PR #3](https://github.com/mattjoyce/shearline/pull/3))

What a v0.1 user will see:

- **The `datalog/` package is deleted.** Import `github.com/mattjoyce/tarski` for `Fact`,
  `RuleSet` and the rest. Shearline's `policy`, `examples/ductile` and `examples/logsvc` do.
- **Engine errors start `tarski:`** instead of `datalog:`.
- **Large and small numbers print in full.** A number at or above 1e6 or below 1e-4 is now
  written out in full. This changes the hash of any answer that carries one. Nothing bundled
  here does.
- **`policy.Load` refuses rule sets that use aggregates** (`count`, `sum`, `min`, `max`). The
  engine has them now. A decision is per request, and aggregates are for queries.
- **A step costs more.** Since tarski v0.2.1 and v0.2.2, each rule application and each built-in
  call is charged a step, as well as each tuple visited. `Decide` runs under a fixed bound of
  1<<22 steps, so a decision that came close to it on v0.1.0 may now cross it and return an
  error, never a partial answer.
- **Bundled rule-set hashes are unchanged.** RBAC is still `rbac/1 sha256:0bf5a121…`, and the
  README example prints the same output.

Also in this release:

- **Two worked examples.** `examples/logsvc` embeds Shearline with roles and tokens in YAML.
  `examples/ductile` models Ductile's token scopes, with a parity test against Ductile's own
  semantics.
- **Engine fixes made before the cut,** all carried into tarski. An `Answer` is an immutable
  value with accessors, `Plan.Defines` is new, proof JSON writes a shared sub-proof once, and
  why-not holds a goal's repeated variables. A file audit filed with its resolution lists the rest;
  see `okf/log.md` for 2026-10-23.
- **Docs.** The README, `docs/DESIGN.md`, `docs/ENGINE.md` and the OKF bundle point to tarski.
  The OKF bundle is trued up to v0.2.2, `SPEC.md` keeps its text and gains history notes, and
  `docs/reviews/` is verbatim.
- **Repo.** The built `examples/logsvc` binary (5 MB) is no longer tracked, and a `.gitignore`
  covers it. CI caches modules, since the repo now has a `go.sum`.

## [v0.1.0](https://github.com/mattjoyce/shearline/releases/tag/v0.1.0) - 2026-10-22

The first release. The Datalog evaluator cut from Anam, and the PARC policy layer over it.

- **`datalog` package.** A rule engine in the standard library only: parse, compile, evaluate,
  proofs, why-not, hashed rule sets, stratified negation, and the string built-ins `prefix`,
  `suffix` and `contains`. `Options.MaxSteps` bounds the work and `MaxBytes` the size.
- **`policy` package.** `Load` compiles a policy once and refuses a bad one. `Decide` answers
  whether a principal may take an action on a resource in a context, and every error is a deny.
  `Permitted` lists the grant prefixes a principal holds.
- **Bundled RBAC rule set,** `rbac/1`, with roles, inheritance, prefix grants and exact denials.
- **Receipts.** An allowed decision carries its proof and a refused one carries why. Both print as
  indented text and marshal to compact JSON.
- **Declared context keys.** A request carrying a key not listed in `Config.Context` is refused.
- **Docs and checks.** The README with a compiled twin of its example, `docs/DESIGN.md`, the
  original `SPEC.md`, three filed reviews, an OKF bundle, an Apache-2.0 licence, and CI with
  `gofmt`, `go vet`, race tests, `golangci-lint` and `gosec`.
