# Classifiergate: AI judgments as inputs to explicit policy

Status: implementation brief. Build this as a standalone worked example, independent
of deploygate. No runnable implementation exists yet.

## Job story

When AI models classify a situation, I want explicit rules to determine which actions
those judgments permit, so I can change policy, inspect decisions and replay outcomes
without asking a model to invent the decision procedure.

The question is **“The classifier gave us these judgments. What are we allowed to do?”**

```text
Input -> classifier(s) -> recorded typed judgments
Judgments + authoritative records -> Shearline rules -> allow/deny + proof/why-not
```

Jev is one possible producer, alongside a conventional classifier or a structured
LLM response. The example should make that interchangeability visible. Model inference
happens outside Shearline; the rule engine evaluates an immutable set of admitted facts.

## Scenario: may a support bot send this drafted reply?

A support bot has a draft response to a customer ticket. Classifiers assess:

1. Whether the draft is supported by the supplied support documentation and ticket.
2. Whether the draft contains sensitive information that should not be sent.

Authoritative application records separately establish the bot's identity, ticket
assignment, permitted recipient, exact draft revision and any human approval or hold.
The model does not supply those records.

The bot also operates under a named human sponsor and an explicit delegation for this
task. A classifier judges content; it does not appoint the sponsor, create a delegation
or expand the bot's authority.

Shearline combines both kinds of input. The bot may send only when it has the proper
scope, the destination is approved, the required assessments are accepted and meet
the policy thresholds, and no hold applies. For the initial example, uncertain or
missing assessments leave automatic sending unauthorized. A human-review workflow
can handle that denial outside Shearline; denial alone must never be treated as an
automatic authorization for some alternative action.

Use synthetic tickets and drafts. The example evaluates authorization; it sends no
messages and requires no external account.

## Facts and rules

Keep judgments visibly distinct from authoritative facts. Suggested consumer relations:

- `bot_scope(Bot, Ticket)` and `recipient_ok(Ticket, Recipient)` from the application.
- `draft(Ticket, DraftDigest, Recipient)` identifying the exact proposed outgoing text.
- `assessment(DraftDigest, Question, Probability, JudgmentID)` from a model response.
- `accepted(JudgmentID)` from the admission adapter.
- `hold(Ticket)` from an authoritative operator or application record.

Retain provenance separately, keyed by judgment ID: input bundle digest, resolved model
version, question/criteria version, raw response, adapter version and timestamps.
The input bundle includes the ticket and supporting documents, not only the draft.
Admission must match all of these to the snapshot being evaluated.

Illustrative Datalog fragment, not a complete policy:

```text
assessed_reply(T, D) :- draft(T, D, R),
    assessment(D, "supported", G, J1), accepted(J1),
    assessment(D, "sensitive", S, J2), accepted(J2),
    G >= 0.98, S <= 0.01.
```

These numbers are demonstration thresholds, not recommended production values. The
team must complete the request-scoped `permit` and `allow` rules required by Shearline,
including bot scope, recipient and hold checks. There must be no parallel RBAC allow
path that bypasses the classifier requirements. Explain each rule in the walkthrough.

Changing a threshold changes policy without changing the classifier. Changing the
classifier changes evidence without changing policy. Demonstrate those separately.

## Walkthrough and fixtures

| Case | Expected result |
|---|---|
| In-scope bot, approved recipient, supported draft, low sensitivity, no hold | Allow |
| Same judgments but bot lacks scope | Deny |
| Same judgments but recipient is not approved | Deny |
| Supported draft but sensitivity exceeds the threshold | Deny |
| Uncertain support assessment | Deny automatic sending |
| Missing required assessment | Deny automatic sending |
| Strong assessments but an active hold | Deny |
| Assessment belongs to an earlier draft or evidence revision | Reject admission or deny; never reuse |
| Malformed, conflicting or unavailable classifier output | Explicit admission failure; no allowance |

For each case show the recorded classifier input, normalized facts, rules and actual
decision receipt. An allowed case should expose the assessment IDs and threshold
comparisons that contributed to the decision. Do not imply a proof explains the model's
internal reasoning or establishes the truth of its judgment.

Provide a second policy with a different threshold and replay the same judgments to
show exactly which outcomes change. Also compare two sets of recorded model judgments
against one fixed policy, identifying changed outcomes and their evidence.

## Implementation boundaries

- Import Shearline directly. Use Config.Relations for typed standing facts and normal
  Decide requests; do not copy its evaluation path or add model calls to builtins.
- Use recorded judgments by default. The example and CI must work offline without an
  API key. A live Jev or other model adapter may be an explicitly selected extra.
- Validate expected question IDs, labels, numeric ranges and response structure before
  creating facts. Missing or invalid numbers are not zero. Preserve exact decimal
  values or document a deliberate fixed-point conversion.
- Select one explicit accepted assessment set for the evaluated snapshot. Do not union
  conflicting responses and let a favourable one satisfy the rules accidentally.
- Keep probability, confidence and ordinal scores distinct. Their meanings are part
  of the adapter contract, not interchangeable numbers called confidence.
- Bound inference separately from evaluation. Timeouts and engine errors must remain
  distinguishable from ordinary denials. Report evaluation cost and effective bounds
  where supported by the dependency version.
- Keep any eventual send operation tied to the checked draft and recipient. The example
  must state that checking one revision does not authorize sending another.

## Acceptance tests

Use real Shearline evaluations and table-driven fixtures. Cover the walkthrough,
threshold equality and values on both sides, malformed numbers, wrong question IDs,
stale evidence, conflicting assessments, caller-forged provenance, and bound/cancellation
errors. Test proof/absence structure as well as allow/deny.

Remove each required assessment or authority fact from an allowed fixture and confirm
allowance disappears. Raising a requirement must not introduce a new allow. Replaying
the same admitted snapshot and policy must reproduce the decision without inference.
Keep observed model quality separate from deterministic policy tests.

Include at least one intentionally wrong, high-confidence classification: show that
Shearline follows its admitted inputs and cannot correct that mistake. The useful
guarantee is faithful policy evaluation, not infallible AI. Before real automation,
evaluate false automatic sends and review workload on labelled domain examples;
classification accuracy alone is insufficient.

## Jev mapping and sources

TypeSafe documents Choice and Score outputs with probability distributions and
confidence, and Noul as a yes-probability without a separate confidence field. Noul
fits the two binary judgments in this example. See [primitives](https://docs.typesafe.ai/primitives).

Pin a resolved model version and retain recorded outputs. The provider documents that
moving aliases can change answers; see [models](https://docs.typesafe.ai/models).
These details describe an optional adapter, not a dependency on Jev for the example.

The completed README should lead with an executed transcript: **same judgments,
different policy; same policy, different judgments; every outcome inspectable.**

## Accountability: who entrusted the bot with this action?

Use the [Agent Accountability Framework](https://agentaccountabilityframework.org/)
as a design lens. Its overview identifies six principles: Sponsor, Role, Authority,
Oversight, Recourse and Identity. This is our proposed application to this example,
not a claim of framework certification or full implementation.

| Principle | What the worked example should make visible |
|---|---|
| Sponsor | A named person owns the bot's assignment and can suspend it. |
| Role | Its purpose is replying to assigned tickets; permitted tasks and excluded tasks are explicit. |
| Authority | A delegation specifies actions, scope and validity; it can be revoked. |
| Oversight | Judgment, authorization, attempted action and outcome records can be joined. |
| Recourse | A suspended bot cannot obtain a new allowance; reinstatement requires a recorded decision. |
| Identity | The actor is an authenticated, registered bot, distinct from its model and human sponsor. |

Keep these records outside model control. Admit their validated snapshot as consumer
facts and require a current assignment/delegation in the final allow rule alongside
the classifier conditions. A model's favourable assessment cannot compensate for
missing authority. A framework role is a documented purpose, not merely an RBAC label.

Demonstrate one delegation hop from sponsor to bot. Record who issued it, the authority
under which it was issued, its scope and revision, and who may revoke it. Reject a
bot's attempt to grant itself wider scope. Any later multi-agent extension must check
each delegation hop and prevent a child from gaining authority its parent lacks.

Add three offline scenarios with unchanged high-confidence judgments:

1. The bot's delegation expires: automatic sending is denied.
2. Its sponsor suspends it: a new evaluation is denied.
3. An authorized person records reinstatement and a valid delegation: reevaluate;
   never treat the earlier allow as a reusable authorization token.

Trusted evaluation time is explicit input. Fixtures must also cover absent or inactive
sponsors and a forged delegation. Record the authority snapshot/revision used by each
decision. An immutable Policy does not discover later revocations automatically: the
host must refresh or invalidate snapshots and enforce current authority at the action
boundary. The example must not advertise real-time revocation based on replay alone.

For the demonstration, a local action simulator records an attempted send and its
outcome without contacting a recipient. Link records using request, judgment, delegation,
policy-snapshot and decision IDs, plus the exact draft/recipient. Keep the execution
record separate from the decision receipt. An allow is permission, not evidence that
a send succeeded; a later suspension cannot recall an already-sent message.

The operator's inspection view should answer: who requested this, on whose authority,
using which assessments and policy, what was permitted, and what actually happened?
Document that authentication, evidence storage, action enforcement and operational
containment belong to the host application. Shearline supplies the policy evaluation
and explanation component within that larger accountability system.

## An agent record in Anam

The proposed longitudinal home for these records is Anam: an agent's assignments,
assessments, Shearline allows/denials/errors, attempted actions, observed results and
later corrections. This is a history of the agent's operation across sessions, not
merely a prompt-memory store or a personal record the agent reads.

See [Anam as an agent's record](../../docs/AGENT-RECORD.md) for the event contract,
two-clock investigations, receipt distinctions and integration gaps. Keep it optional
to this standalone example. Anam's current person-bound model needs an explicit
adaptation decision before claiming support for independent agent records.
