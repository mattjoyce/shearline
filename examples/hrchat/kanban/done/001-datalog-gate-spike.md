# 001 Datalog gate spike

## Job story
When an HR chatbot can call payroll through MCP, I want every tool call decided by a Datalog policy over the organisation, so I can see what the engine buys against a jailbroken or confused model before designing a real one.

## Done 2026-10-26
- 1001-person synthetic org (`org/`), 29-tool catalogue (`catalog/`), `hrchat/1` rule set over Shearline's `rbac/1` (`hrpolicy/`), the gate with a ledger (`gate/`), aggregate audit queries (`audit/`), CLI.
- 39 scenarios pinned; `go test -race -shuffle=on` green; go-quality gate green (lint, vet, gofmt, govulncheck via a current build).
- Findings in README § Findings for the engines.
