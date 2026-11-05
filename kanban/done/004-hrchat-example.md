# 004 Bring hrchat in as an example

Asked for by: Matt (2026-10-27). Built by: datalog-module, on Matt's instruction.
Release: none needed (examples and docs only).

## Job story

When I show someone what Shearline is for, I want a worked example at the scale of a real
organisation, with relationships, attested context and an audit, so I can point at one directory
instead of explaining it, and so a change to Shearline that breaks that use fails CI.

## Change

- `../hrchat` copied to `examples/hrchat` (packages `org`, `catalog`, `hrpolicy`, `gate`, `audit`,
  `cmd/hrchat`). Imports rewritten to `github.com/mattjoyce/shearline/examples/hrchat/...`.
- Root module, not its own: hrchat needs only Shearline and tarski, so it builds against `policy`
  in this tree and the existing CI steps (vet, test, lint, gosec) cover it with no workflow change.
- Left behind: hrchat's `kanban/` (the spike's history) and its `go.mod`.
- Covering documents: `examples/README.md` (index), and a "why it is here", run and shape section
  in each of `logsvc`, `ductile` and `hrchat`. Root README "See it embedded" names all three.

## Results

- `go test -race -shuffle=on ./...`: all packages pass; `hrchat scenarios`: 39 of 39, largest
  decision 136 steps, 1% of a bound.
- golangci-lint v2.15.0 (CI config): 0 issues. gosec: clean. govulncheck: no vulnerabilities.
- go-quality gate: the two lint findings it reports are on master already (`policy/policy.go:44`
  forbidigo panic, `:57` revive comment form), not from this change. govulncheck had to be
  rebuilt with Go 1.27, since hrchat's `math/rand/v2` tripped a Go 1.26 build of it.
