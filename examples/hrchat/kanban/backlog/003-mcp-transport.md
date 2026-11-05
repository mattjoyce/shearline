# 003 MCP transport

## Job story
When a real model client connects, I want the gate served over MCP stdio or HTTP, so the catalogue's manifest is what `tools/list` returns and `gate.Call` is what `tools/call` runs.

## Notes
- The session (principal, channel, MFA) must come from the transport's authentication, never from tool arguments.
- The guardrail verdict and the confirmation are the host app's to attest per turn.
