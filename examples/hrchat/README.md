# hrchat: a Datalog gate for an HR chatbot

An HR chatbot answers staff and managers about their records. It acts for them through an
MCP server that reaches payroll. The model that drives the conversation is the untrusted
component: it can be jailbroken, confused, or simply wrong about who it is talking to. This
example puts a Shearline policy between every tool call the model emits and the payroll
store, and shows what that buys.

The organisation is synthetic: one seed, 1001 people, seven divisions, a tree four to five
deep. Nothing here is real.

## Why this example is here

`examples/logsvc` shows the smallest useful embedding: roles, grants, tokens. `examples/ductile`
holds Shearline to another system's semantics. This one is the large case. It is the only
example that uses what v0.3.0 and v0.4.0 added, and it puts them under load:

- **Consumer relations.** The rules read the organisation itself (`reports_to`, `dept`,
  `hr_partner`) and the entitlement lists, declared in `policy.Config.Relations`. Access
  follows relationships, not only roles.
- **Context.** Every request carries what the application attests: channel, guardrail
  verdict, MFA, confirmation, read velocity, requester. A missing attestation fails closed.
- **Proofs and why-nots as product.** A refusal is reduced to one line the bot may relay,
  and the full derivation goes on a hashed ledger.
- **Cost.** Each ledger row carries `Decision.Steps` and `Decision.Bytes` against
  `Policy.Bounds()`, and an audit query flags a decision that used more than a quarter of a
  bound.
- **Policy and audit split.** The policy decides one request and cannot count. A second
  tarski program counts over the ledger and cannot decide.

It lives in Shearline's root module and imports `policy` from this tree, not a release. CI
runs its 39 scenarios on every change. A change to Shearline that moves an hrchat answer
fails here first.

## Run it

Go 1.25 or later, from the repository root:

```
go test -race -shuffle=on ./examples/hrchat/...
go run ./examples/hrchat/cmd/hrchat org                    # the org and the cast
go run ./examples/hrchat/cmd/hrchat who E0004
go run ./examples/hrchat/cmd/hrchat tools                  # the MCP manifest
go run ./examples/hrchat/cmd/hrchat rules                  # rule set IDs and hashes
go run ./examples/hrchat/cmd/hrchat decide --as E0005 --tool get_bank_details --subject E0005        # refused: no MFA
go run ./examples/hrchat/cmd/hrchat decide --as E0005 --tool get_bank_details --subject E0005 --mfa  # allowed, with proof
go run ./examples/hrchat/cmd/hrchat matrix --as E0004      # every tool against self, report, peer, manager
go run ./examples/hrchat/cmd/hrchat scenarios              # the table, then the audit
```

## The shape

One tool call goes through five packages, in this order. Read them in the same order.

```
session + tool call
   |
   v
catalog/   tool -> action ("<verb>.<field>") and level. Unknown tool: refused here.
   |
   v
gate/      the enforcement point. Principal from the session, never from the model.
   |       Employee ID -> resource "emp/<id>". Unknown employee: refused here.
   |       Attaches the context the app attests, then asks the policy.
   v
hrpolicy/  rbac/1 + hrchat/1 over Shearline. Declares the consumer relations and the
   |       context keys, holds org/ and catalog/ facts as standing data, decides,
   |       hashes the decision, and reduces a why-not to one line.
   v
gate/      allowed: the call runs on the fake store. Either way: a ledger row with the
   |       hash, the proof or why-not, and the cost.
   v
audit/     audit.dl: aggregate queries over the ledger (probing, fan-out, costly).
```

`org/` generates the organisation and its facts. It is deterministic, so one seed gives
the same people, the same decisions and the same hashes. `cmd/hrchat/` is the CLI over all
of it.

## How Datalog prevents inappropriate actions

The chatbot is never asked to behave. Every tool call becomes a request with four parts:
**who** is asking (the principal, bound from the authenticated session and never from the
model's arguments), **what** they are doing (the action the tool maps to, `read.bank`),
**to which record** (the resource, `emp/E0042`), and **under what** (the context the
application attests: the channel, the guardrail's verdict on this turn, whether MFA is
fresh, whether the human confirmed a change, how many high reads this hour, who raised the
change an approval is for). The policy derives `allow(P, A, R)` from standing facts and
the request, or it does not. Nothing reaches payroll without that tuple, and every
decision carries its proof or its why-not, hashed, onto a ledger.

What Datalog buys over an if-chain in the MCP server:

1. **Relationships are the policy, and the reach is deliberate.** A people manager sees
   their direct team and nobody further down: not a director, not the CEO. A skip-level
   question goes to HR, whose partner reaches their department, and the few who see
   everyone hold a named role, which is a fact anyone can audit. The one recursive
   relation is the requester's own reporting line, read only to deny: nobody touches
   high data of anyone above them. A thousand-person tree is 1,000 facts, the deepest
   employee's decision costs 135 steps, and adding a hire is a fact, not a deploy.
2. **Attestations are required positively, so a missing one fails closed.** `cleared(A)`
   holds only when the guardrail said clean, the channel is a known one that carries this
   action, and every attestation the action's level needs is present: MFA for high, a
   confirmation for a high write, a velocity count under the limit for a high read, a
   requester other than the approver for a pay change. A request that omits the guardrail
   verdict is refused, not waved through.
3. **Separation of duties is a deny rule nothing overrides.** Your own record is reachable
   by the self path only, so a payroll officer's role does not reach their own bank
   details and an HR partner cannot retitle themselves. Nobody touches high data of
   anyone above them in their reporting line, so payroll cannot read the CFO's account.
4. **Every refusal explains itself from the decision.** The why-not walks the rules and
   stops at the literal no path could satisfy. The gate reduces it to one line the bot may
   relay: `blocked by unmet("read.bank") [needs("read.bank", "mfa")]`, or
   `blocked by channel_forbids("chat", "write.status")`, or `no path: ...`. The full
   derivation is on the decision for the audit.
5. **The same policy serves chat and the HR web app.** The channel is a context fact; chat
   carries less. Setting a salary, terminating someone and changing a tax file number are
   things no conversation does, whoever is talking.
6. **Decisions do not count; audits do.** Aggregates are refused in a policy (a decision
   is one request) and used in queries over the ledger: `probing(P, N)` for a principal
   refused three or more times, `fanout(P, A, N)` for one allowed the same high action on
   three or more records, `costly(Seq, P, M)` for a decision that used more than a
   quarter of a bound. Same engine, same proofs, run after the fact.

The jailbreak guardrail the brief assumes is a sensor, not an actuator: it attests
`guardrail = clean | flagged` and the policy decides what a flagged turn may still do
(nothing). The two layers compose: the guardrail lowers the number of attempts, the policy
bounds the damage of the ones that get through. A model that is fully jailbroken can do at
most what the authenticated user could do themselves, on this channel, with their MFA
state, with a human's confirmation for anything high.

## What the model's mistakes look like at the gate

| The model does | The gate does |
|---|---|
| claims to be someone else in an argument | ignores it: the principal is the session's |
| invents a tool | refuses before the policy: unknown tool |
| invents an employee ID | refuses before the policy: unknown employee |
| reads a peer's record for a curious user | refuses: no path |
| reads a report's bank details for a manager | refuses: no path (managers hold salary, not bank) |
| reads a skip-level's history for a director | refuses: no path (a manager's reach is their direct team) |
| approves the user's own leave | refuses: no path on self |
| changes bank details on a confident user | refuses until the app attests a confirmation the human gave the app |
| reads a 30th salary in an hour | refuses: velocity unmet; the audit sees the fan-out |
| approves a pay change the user raised | refuses: four eyes unmet |
| terminates someone over chat | refuses: the chat channel never carries it |
| acts on a turn the guardrail flagged | refuses: not attested |

`go run ./cmd/hrchat scenarios` runs 39 of these and audits the ledger they leave; the
same table is pinned as `gate/scenarios_test.go`.

## The policy

`hrpolicy/hrchat.dl` is 21 rules over Shearline's `rbac/1`. The paths:

```
permit(P, A, P) :- principal(P), action(A), resource(P), self_may(A), cleared(A).
permit(P, A, S) :- principal(P), action(A), resource(S), direct(P, S), manager_may(A), cleared(A).
permit(P, A, S) :- principal(P), action(A), resource(S), hr_partner(P, D), dept(S, D), hr_may(A), cleared(A).
permit(P, A, S) :- principal(P), action(A), resource(S), holds(P, Role), role_may(Role, A), dept(S, _), cleared(A).
```

The attestation gate every path ends in:

```
needs(A, "mfa")       :- action(A), level(A, "high").
needs(A, "velocity")  :- action(A), level(A, "high"), verb(A, "read").
needs(A, "confirmed") :- action(A), level(A, "high"), verb(A, V), writes(V).
needs(A, Req)         :- action(A), also_needs(A, Req).
met("mfa")       :- context("mfa", true).
met("confirmed") :- context("confirmed", true).
met("velocity")  :- context("high_reads", N), N < 25.
met("four_eyes") :- principal(P), context("requester", Q), Q != P.
unmet(A)    :- action(A), needs(A, Req), not met(Req).
attested(A) :- action(A), context("guardrail", "clean"), context("channel", C), channel(C), not channel_forbids(C, A).
cleared(A)  :- attested(A), not unmet(A).
```

And the denials:

```
deny(P, A, P) :- principal(P), action(A), resource(P), not self_may(A).
deny(P, A, M) :- principal(P), action(A), resource(M), up(P, M), level(A, "high").
```

Which actions each path carries is data in `hrpolicy/entitlements.go`, reviewed like code.
The tool surface is `catalog/catalog.go`: 29 tools over 27 actions, each `<verb>.<field>`
with a level. `go run ./cmd/hrchat tools` prints the manifest in MCP `tools/list` shape
with the action and level beside each tool.

| | low | high |
|---|---|---|
| read | directory, profile, history, leave, next of kin, emergency contact, address | bank, tax file number, salary, payslip, performance, medical |
| write | next of kin, emergency contact, address, leave (submit, cancel, approve), job title | bank, tax file number, performance rating, pay change (request, approve), salary, medical, employment status |

## What Datalog does not do here

- It cannot build strings, so the principal and the subject share one namespace
  (`emp/<id>`) and the field lives in the action, not the resource.
- It cannot count inside a decision, so the velocity limit arrives as a number the app
  counted from the ledger. The policy compares; the app measures.
- It cannot tell time, so "MFA is fresh" and "the guardrail said clean" are booleans and
  strings the app attests. Freshness is the app's judgment.
- It cannot validate a value. Whether a new bank account is well formed, or is the user's
  own, is the store's check after the decision.
- Shearline's `Permitted` lists grant prefixes for a principal and action; relationship
  permits are per resource, so it lists nothing useful here. The matrix is computed by
  deciding, which is what the README of Shearline says to do anyway.

## What it taught Shearline

- **Shearline closed the base schema over PARC; v0.3.0 opened it.** A consumer's own
  standing relations (`reports_to`, `dept`, `hr_partner`, the entitlement lists) were
  refused as unknown, so this example once copied `Load` and `Decide` over a wider schema.
  Shearline v0.3.0 added `Config.Relations`, and `hrpolicy` now declares its relations
  there and calls `policy.Load` and `Decide` directly.
- **A decision's hash is the consumer's choice of evidence.** Shearline's `Decision` has
  no hash. The ledger keeps `sha256` over the rule-set IDs, the answer and its proof or
  why-not (`hrpolicy.Decision.Hash`), and leaves the cost out, since the cost changes
  with the engine and the answer does not.
- **The why-not needs a headline.** `Absence` is complete and correct, and too much for a
  chat reply. `hrpolicy.Explain` reduces it by the policy's shape: a blocked deny first,
  else a path that reached `cleared(A)` descended to the attestation it lacked, else the
  literal each path stopped at. The reduction knows this policy's vocabulary; the engine
  rightly does not.
- **Cost is small, and watched.** A typical decision is 120 to 200 steps and under a
  kilobyte derived; the deepest employee, four levels down, is 135 steps and 256 bytes
  against Shearline's bounds of 4M steps and 1 MB. Before the chain path was removed, the
  CEO derived the whole tree and cost 4,138 steps and 25 KB. Since Shearline v0.4.0 each
  ledger row carries the decision's `Steps` and `Bytes` and its margin, the larger of
  the two as a percent of `Policy.Bounds()`. The audit's `costly` finding fires past
  25%; that is the signal for Shearline's card 003, configurable bounds
  (`kanban/backlog/003-configurable-bounds.md`), or for
  narrowing the rule that grew. The largest scenario today is 1%. The 39-row audit is
  625 steps.

Not built: an MCP transport. The gate is the server's handler; wiring it to stdio or HTTP
is a card, not a design question.
