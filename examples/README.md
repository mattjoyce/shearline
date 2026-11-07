# Examples

Four runnable programs that embed Shearline, each pinned by tests that CI runs.
Each answers a different question. Read them in this order.

| Example | The question it answers | Size | Module |
|---|---|---|---|
| [`logsvc`](logsvc/README.md) | How does a service embed Shearline? | 4 roles, 8 endpoints | own (needs YAML) |
| [`ductile`](ductile/README.md) | Does Shearline express another system's rules exactly? | 154 tokens, 19 route rows | root |
| [`hrchat`](hrchat/README.md) | Does it hold up on relationships, context and audit at scale? | 1001 people, 29 tools | root |
| [`mcpgate`](mcpgate/README.md) | Can it enforce policy in front of an existing MCP server? | one stdio upstream, generated tool actions | own (MCP SDK) |

**logsvc** is the tutorial. Roles and grants come from files the service owns, a typo stops the
service at start, a reload swaps the policy atomically, and every answer prints with its proof.
Start here if you have not used Shearline before.

**ductile** is the parity check. It models Ductile's token scopes as roles and grants and
compares every answer against a transcription of Ductile's own Go. It found no Anam-shaped
assumption in the bundled `rbac/1`, and it pinned that as a test.

**hrchat** is the stress case. Access follows the organisation (a manager reaches their direct
team, an HR partner their department) through consumer relations, and every request carries
what the application attests. Refusals reduce to one line a chatbot may relay. Every decision
goes on a hashed ledger with its cost, and a second tarski program audits the ledger for
probing, fan-out and costly decisions.

**mcpgate** is a runnable tools-only MCP proxy. It discovers upstream tools, generates
one action per tool without granting access, and checks every call against reviewed
bindings and Shearline policy. Its local demo shows an allowed read, a denied record,
a hidden write tool called directly, and an unknown tool; only the allowed read reaches
the upstream. Definition changes fail closed and authorization is audited separately
from execution.

The examples in the root module (`ductile`, `hrchat`) build against the `policy` package in
this tree, so a change to Shearline that moves one of their answers fails CI. `logsvc` pins a
released Shearline so it builds the same way outside this repository; its README says how to
test it against an unreleased change.
`mcpgate` also pins a released Shearline in its own module; CI checks it against both
that release and this checkout. [Deploygate](deploygate/README.md) and
[classifiergate](classifiergate/README.md) contain separate implementation briefs;
they are not runnable examples.
