# Anam as an agent's longitudinal record

Status: design proposal connecting the worked examples to Anam. No Anam integration,
agent-subject support or new Definition is implemented by this document.

## The idea

An agent has a history worth preserving across sessions, model upgrades and process
restarts: what it was tasked to do, what evidence it received, what it judged, what
Shearline permitted or refused, what it attempted and what resulted.

Use Anam's accreting-facts model for that history. Current state is a view derived
from the record, not a mutable agent profile that erases earlier decisions.

```text
Authoritative identity, assignment and delegation records --+
Recorded classifier judgments -----------------------------+--> admitted policy inputs
                                                          |            |
Anam: source records, versions and read receipts <-----------+       Shearline
                                                                       |
                                                        allow / deny / error
                                                                       |
Trusted host records decision --> action adapter --> attempted action / outcome
                |                                        |
                +----------- importer --> Anam <----------+
                                              |
                                 history, replay and served views
```

Anam records and serves evidence. Shearline evaluates permission. The host controls
execution and captures what happened. A model's judgment, policy permission and an
observed outcome remain different kinds of facts.

## What belongs in the record

| Record kind | Essential content | Authoritative source |
|---|---|---|
| Assignment/delegation | Agent, named sponsor, purpose, issuer, scope, validity, revision | Identity/control service |
| Evidence supplied | Task, input bundle/version, exact documents or their retained digests, read receipt | Host or evidence adapter |
| Model judgment | Judgment ID, input digest, model and question versions, typed output | Classifier adapter |
| Policy decision | Request, actor, action/resource/context, input snapshot, rule sets, outcome | Shearline host |
| Action attempt/result | Decision ID, exact target/content, attempt ID, status, external receipt if any | Tool/execution adapter |
| Review/correction | What was challenged, reviewer, evidence, correction or replacement | Authorized reviewer/source |
| Lifecycle event | Suspension, revocation, reinstatement or retirement, issuer and reason | Control service |

These are proposed record kinds, not existing Anam schemas. Define versioned
Definitions and source permissions before implementation, following Anam's existing
domain vocabulary. The agent may contribute its own observations, but cannot write
an authoritative approval, execution success or sponsor decision under another source.

Give the logical agent a stable identity. Task, session, process instance and model
version are separate identifiers: changing a model does not silently replace the
agent's history. Sponsor identity is separate from agent identity too.

## Recording Shearline outcomes

The decision record should retain:

- A stable decision ID, task/request ID, agent and delegation references.
- The exact request and admitted input snapshot, or durable references sufficient to
  reconstruct it; policy text and ordered rule-set identities, engine version and bounds.
- A distinct `allow`, `deny` or `error` outcome. A denial records its absence explanation;
  an allowance records its proof. Preserve the original structured payload.
- On success, Decision.Steps/Bytes where supported. On error, structured error details
  and available TooLarge counters; do not present these as a completed evaluation.
- References to subsequent attempts/results, which may arrive later or remain unknown.

Do not reduce denied and failed decisions to silence. They show attempted scope
expansion, unreliable evidence sources and availability problems. Do not reduce an
allow to a successful action. One decision can have multiple attempts, each recorded.

Preserve old rule and input versions. A rule-set hash identifies content but cannot
recreate content that was discarded. Keep ordered inputs/options when exact proof
reproduction depends on them, and preserve the original receipt alongside any replay.

## Two clocks make the investigation useful

Anam distinguishes when a fact applied from when it learned the fact. This enables
separate questions: what applied at the time, and what was recorded as known then?

Example: a revocation is effective at 10:00, a decision allows an action at 10:02,
and Anam receives the revocation at 10:05. Preserve all three events. Do not rewrite
the 10:02 decision when the revocation arrives. An as-of read before 10:05 and a later
read filtered to 10:02 should expose the difference.

However, what Anam knew is not automatically what the agent or policy evaluator
consumed. Pin the actual decision input bundle and, where used, the Anam ledger ID,
watermark and served projection receipt. Record externally supplied inputs too.

Show three distinct operations:

1. Historical inspection: read the recorded judgment, decision and outcome.
2. Decision replay: run the retained policy against the actual retained inputs.
3. Counterfactual: rerun with corrected evidence, new policy or new judgments; label
   the result as a new simulation, never a correction to what actually happened.

An Anam receipt identifies what a read served. A Shearline proof/absence explains a
policy result. An execution receipt supports an action outcome. Link them by ID;
do not merge their meanings or imply that any one proves the other two.

## Failure and trust boundaries

Use a trusted host's durable event/outbox path with stable event identities, followed
by an importer through Anam's admission gate. Retries must not duplicate events.
For this example's simulated send, require a durably captured authorization and action
intent before dispatch. If capture fails, do not dispatch. Later Anam ingestion can
be retried from the durable source; make pending ingestion visible.

There is no implied transaction spanning Shearline, Anam and an external tool. If the
host crashes after dispatch but before receiving a result, record an unknown/pending
outcome and reconcile. Do not retry an irreversible action merely because its success
record is absent. Use the target's idempotency support where available, with the same
logical action ID across retries. Recording is not an exactly-once execution guarantee.

Append-only history does not prove completeness, source honesty or resistance to a
privileged administrator replacing storage. Distinguish missing evidence from evidence
of no action. Authority revocation still needs fresh checks at the execution boundary;
a historical allowance must not reauthorize a suspended agent.

Read projections should serve appropriate evidence to the agent, sponsor or reviewer.
Enforce that boundary in the host; do not assume a projection alone denies access to
the underlying timeline. Avoid retaining credentials. Define retention/redaction and
access rules for sensitive inputs before importing them into an immutable record.

## Fit with Anam today, and the adaptation decision

The inspected Anam checkout already describes immutable facets, Definition-governed
admission, source identities, idempotent commits, two clocks, watermark-based reads,
replay and projection receipts. These are a strong foundation for agent history.

It also explicitly partitions by `person_id` and associates admitted facts with that
person. Whole-person reads are the default. Agent tenancy and sponsor-specific read
permissions are not established by giving a bot a person's name or choosing a projection.

Before integrating, decide whether to generalize the record subject, create an agent
record variant, or represent agent activity in an explicitly human-owned ledger. The
last option is not equivalent to an independent agent-owned record. Keep that decision
with Anam's maintainers; do not silently change its person-binding contract here.

Verify registered Definition provenance for replay: Anam's documented receipt gap says
its Definition-set hash covers bundled manifests, not every registered Definition.
Retain the exact registered Definitions used by the adapter rather than assuming the
existing receipt pins them all. Recheck this limitation against the integration revision.

Source references: [Anam README](https://github.com/mattjoyce/anam/blob/3e30fd3c271f186e3fdc1e6b79d0d9e5b2cebbd0/README.md),
[architecture](https://github.com/mattjoyce/anam/blob/3e30fd3c271f186e3fdc1e6b79d0d9e5b2cebbd0/docs/ARCHITECTURE.md),
[receipt implementation](https://github.com/mattjoyce/anam/blob/3e30fd3c271f186e3fdc1e6b79d0d9e5b2cebbd0/internal/receipt/receipt.go), and
[registered-Definition provenance gap](https://github.com/mattjoyce/anam/blob/3e30fd3c271f186e3fdc1e6b79d0d9e5b2cebbd0/docs/explainer/bundle/gaps/receipts-do-not-pin-registered-definitions.md).

## A bounded follow-on demonstration

Keep classifiergate runnable without Anam. Add an optional integration after the subject
and Definition contracts are settled: one agent, one task, recorded assessments, an allow,
a simulated send, a denial, an evaluation error, and a later correction/revocation.
Use a real local Anam instance for that integration; fixtures alone demonstrate the event
contract, not Anam integration.

Acceptance: resending importer events creates no duplicates; replay rebuilds the same
history; late evidence does not rewrite prior decisions; different read views disclose
only authorized evidence; a crash leaves an explicit incomplete attempt, not invented
success; suspending the agent prevents a new action despite an earlier allow.

Hickey's lens keeps observations, judgments, permission and execution as separate
immutable values. Armstrong's lens asks which component durably captured each event
and what recovery does after a crash. Anam makes the history inspectable; the host still
has to make that history reliable and enforce current authority.
