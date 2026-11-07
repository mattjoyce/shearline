# Mcpgate: put Shearline in front of an MCP tool server

A runnable tools-only stdio proxy, using the official MCP Go SDK. Every tool call
passes through Shearline before reaching the upstream. Discovery generates one
action per tool; the operator supplies roles, grants and resource semantics.

From this directory, with Go 1.25 or later:

```sh
go run ./cmd/mcpgate demo
```

Actual output:

```text
tools/list: [read_record]
read_record(public): allowed — record read by upstream: {"id":"public"}
read_record(secret): refused — mcpgate: policy denied demo.read_record on records/secret/
save_record(public): refused — mcpgate: policy denied demo.save_record on records/public/
unknown_tool(public): refused — mcpgate: tool has no reviewed binding
audit: 4 authorization records, 1 execution record
```

The demo starts a real local stdio MCP server as a child process. An SDK client
connects to the gate over an in-memory MCP transport. The acceptance test also
uses stdio for the client-to-gate hop. No model, credentials or external service
is required after downloading Go dependencies. The fixture acknowledges saves
without persisting anything; the demonstration policy never permits that tool.

## Front another server

Build the executable, discover a server, then review the proposed policy:

```sh
go build -o /tmp/mcpgate ./cmd/mcpgate
/tmp/mcpgate inventory -server demo -- /tmp/mcpgate fixture > /tmp/proposed-policy.json
```

Replace `/tmp/mcpgate fixture` with any stdio MCP server command and its arguments.
The command is an argv array, with no shell expansion. The gate handles the MCP
handshake and traverses paginated `tools/list` responses.

Inventory emits bindings like this, with a real hash in place of the abbreviation:

```json
{
  "tool": "read_record",
  "action": "demo.read_record",
  "definition_hash": "sha256:...",
  "resource": "mcp/demo/read_record/"
}
```

The action is always `<server>.<tool>`; the generated resource identifies a whole
tool capability. A trailing `/` prevents a grant for `read_record/` from also
matching `read_record_extra/`. The stable server namespace is operator supplied.
Annotations, names and descriptions never create authority: the generated
`roles` and `grants` arrays are empty, so serving that inventory allows no calls.

Review the upstream's behavior and definition, then edit the policy. For example,
the generated capability can be granted with:

```json
"roles": [{"principal": "agent-alice", "role": "reader"}],
"grants": [{
  "role": "reader",
  "action": "demo.read_record",
  "prefix": "mcp/demo/read_record/"
}]
```

Serve the checked-in demo policy:

```sh
/tmp/mcpgate serve -principal agent-alice -config demo-policy.json \
  -audit /tmp/mcpgate-audit.jsonl -- /tmp/mcpgate fixture
```

This command waits for an MCP client on stdin/stdout. Point your MCP client at the
gate executable rather than the upstream. For a client using an `mcpServers`
configuration, the equivalent entry is:

```json
{
  "mcpServers": {
    "guarded-demo": {
      "command": "/tmp/mcpgate",
      "args": [
        "serve", "-principal", "agent-alice",
        "-config", "/absolute/path/to/shearline/examples/mcpgate/demo-policy.json",
        "-audit", "/tmp/mcpgate-audit.jsonl",
        "--", "/tmp/mcpgate", "fixture"
      ]
    }
  }
}
```

Use absolute paths in client configuration. Without `-audit`, audit JSON lines go
to stderr. Stdout is reserved for MCP. `-timeout` defaults to 10 seconds for the
handshake and each subsequent gate operation. A tool error or transport failure
is never retried automatically.

## Map arguments to resources explicitly

The checked-in [demo-policy.json](demo-policy.json) changes the read binding to:

```json
"resource": "records/",
"resource_argument": "id"
```

The call `read_record({"id":"public"})` becomes:

```text
principal = agent-alice
action    = demo.read_record
resource  = records/public/
```

A grant for `records/` permits reads in that namespace. An exact denial for
`records/secret/` blocks that record despite the grant. A narrower grant for
`records/public/` does not include `records/publicity/`.

The configured argument must be a nonempty string containing only ASCII letters,
digits, `_` or `-`. Missing values, numbers, traversal, encoded separators,
duplicate top-level keys and alternate casing of the scope argument are refused.
These are literal identifiers, never paths to normalize. Arbitrary paths,
nested fields, composite resources and tenant ownership need a different,
domain-specific adapter; this example does not guess their meaning from schemas.

Static bindings are listed only after a full decision on their resource.
Argument-scoped bindings are listed when the principal has an overlapping grant
prefix. This is coarse visibility: an exact denial on one record does not hide
the whole tool. Once arguments exist, every call receives a full `Decide`, even
if the tool was hidden or the caller guessed its name. The listing uses the
`Permitted` API available in the pinned Shearline v0.5.0; the core checkout marks
that API deprecated. A future migration should preserve these visibility semantics.

## Definition changes and receipts

Startup refuses missing or changed configured tools. Each list refreshes upstream
discovery, and each otherwise permitted call rechecks its definition immediately
before forwarding. Newly discovered tools have no binding. A changed or missing
tool is hidden and refused; it can work again if its original pinned definition
returns. To accept a new definition, regenerate inventory, review the changes,
preserve the intended grants, and restart with the revised configuration. There
is no automatic grant expansion or live policy reload. The gate advertises no
list-change notifications; clients must list again to refresh visibility.

The hash covers the SDK-decoded tool definition, including description, schemas,
annotations and metadata, with JSON object keys sorted. It does **not** attest to
server code or include unknown wire fields that the SDK discards. Unchanged
definitions cannot prove unchanged behavior. Rechecking also cannot atomically
bind an upstream implementation to execution; a server can change after the
check. The gate is not an upstream sandbox.

Audit records contain the trusted principal, action, concrete resource, actual
Shearline decision (proof or absence, rule-set IDs and cost), effective bounds,
and an example-specific configuration digest. The configuration digest includes
bindings and policy facts; it is not a Shearline answer hash. Array order affects
that digest. Arguments and results are omitted to avoid copying tool payloads.

An authorization entry must be written before forwarding; audit failure prevents
the call. A separate execution entry shares its request ID and records completion,
tool error, or unknown outcome. Transport failure after sending can mean an action
already happened. Failure to write the outcome also returns an error and cannot
undo execution. A successful tool response is the upstream's report, not independent
evidence of a completed side effect. Request IDs are local to one gate process.
This JSONL audit is neither a tamper-evident chain nor a durable transactional log.

The client gets a short, accurate refusal; the full absence explanation is in the
audit. Errors while entering the gate (such as cancellation while queued) return
without forwarding and without an authorization row. List operations are not
audited. Calls are serialized; upstream discovery is bounded to 100 pages and
10,000 tools, but this is not a bound on message size or server process resources.

## Trust boundary and scope

- The launcher binds a single principal to the gate process. This models a trusted
  application/session identity; it does not authenticate a user. If an agent can
  rewrite its own launcher arguments, policy or executable, it controls the gate.
- Tool arguments and `_meta` cannot set identity, approval or policy context.
  Only the authorized name and unchanged argument bytes are forwarded; client
  metadata is dropped. The example supports no approval/confirmation workflow.
- An argument mapping is sound only if the upstream actually uses that argument
  as the resource identity. Schemas and hints cannot establish that contract.
  The upstream remains responsible for validating its complete input schema.
- Only tools are proxied. Prompts, MCP resources, sampling, elicitation, task
  execution, progress relaying and multi-round tool input are unsupported.
  Downstream initialization/discovery is answered locally, without upstream
  instructions or capabilities. The upstream client advertises no optional
  client capabilities. The SDK handles transport, lifecycle and cancellation.
- The deployment must prevent direct access to the upstream for enforcement to
  mean anything. A caller that can launch the upstream directly can bypass this
  proxy. The child inherits the launcher's environment and OS permissions.
- Replies are passed through. There is no field filtering, redaction, prompt
  injection detection or guarantee that a permitted tool is harmless.

This is a working enforcement example for a single stdio server, not a complete
multi-tenant MCP gateway. HTTP authentication, credential isolation, stronger
audit storage and domain-aware mappings are further work.

## Checks and module isolation

```sh
go vet ./...
go test -race -shuffle=on -count=1 ./...
```

Tests use real MCP sessions, plus observed forwarding counts. They exercise hidden
and unknown calls, record denials, similarly named resources, spoofed identities,
ambiguous arguments, paginated catalogs, changed/removed/new tools, malformed
configuration, cancellation/deadlines, audit failure, upstream failure, and
concurrent calls. The stdio acceptance test runs both proxy hops as subprocesses.

This directory is its own module to keep the MCP SDK out of Shearline's core.
It pins Shearline v0.5.0 and the official MCP Go SDK v1.8.0. CI also checks this
example against the root checkout using an alternate modfile, without editing
either module's `go.mod`:

```sh
mcpgate_tmp=$(mktemp -d)
cp go.mod "$mcpgate_tmp/local.mod"
cp go.sum "$mcpgate_tmp/local.sum"
go mod edit -modfile="$mcpgate_tmp/local.mod" -replace=github.com/mattjoyce/shearline=../..
go test -modfile="$mcpgate_tmp/local.mod" -race -count=1 ./...
```
