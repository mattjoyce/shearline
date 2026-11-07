# Deploygate: explain a deployment decision and preview policy changes

Status: implementation brief. There is no runnable example yet. The commands below
describe the interface the team should build.

## Job story

When a human or bot requests a production deployment, I want to see exactly which
policy facts permit or prevent it, and preview who a policy change affects, so I can
review access before changing production.

The opening question is **“Why can this bot deploy to production?”** The memorable
demonstration is one request that moves from denied, to allowed, to denied again as
approval and freeze facts change. Each answer is backed by Shearline's actual proof
or absence explanation.

This complements the organisational permissions example: service ownership, build
identity, commit-bound approvals and operational restrictions exercise a different
authorization boundary.

## Deliverable

Build a small Go CLI importing Shearline directly. Follow the existing examples'
module conventions; do not copy Load, Decide or evaluation logic. Include readable
Datalog, small local fixtures, tests and a README walkthrough that takes a reader
from a clean checkout to the interesting result in a few commands.

The default example must be deterministic, offline and require no credentials,
cloud account or model access. It evaluates deployment authorization; it does not
deploy software, grant credentials or claim to implement GitHub/AWS policy semantics.

Proposed interface, with final flags left to the implementer:

```text
go run . decide --scenario missing-approval
go run . decide --scenario approved
go run . decide --scenario incident-freeze
go run . preview --before fixtures/before --after fixtures/after --catalog fixtures/requests.json
```

Support human-readable output and structured JSON. Include the request, decision,
rule-set IDs, proof/absence, evaluation cost and effective bounds where available
in the selected Shearline version. Do not invent APIs or copy private limit constants.

## Policy and input model

Use consumer relations for trusted fixture facts. Suggested concepts:

- Service ownership and team membership, with a small inheritance hierarchy.
- The authenticated actor and that actor's service/environment deployment grant.
- A build record binding an artifact digest to a repository and commit.
- A trusted workflow identity and protected source reference.
- An approval binding reviewer, service, environment, commit and artifact digest.
- Change author and reviewer identity, to enforce independent approval.
- An active incident freeze for a service/environment.

These are example-domain concepts, not additions to tarski. Define one canonical
representation for service/environment resource IDs. Commit and artifact IDs match
exactly; a string-prefix grant must not accidentally cover a neighbouring service.

Every allow path must positively require the applicable grant, trusted build,
matching approval and reviewer independence, and respect the freeze denial. Adding
an allow rule to Shearline is additive: the bundled deny hook does not automatically
constrain that new rule. Make the final gate explicit in every allow rule.

The adapter supplies authenticated identities and authoritative records. A request
cannot attest to its own approval or trusted-build status. For the offline demo these
are labelled fixtures, not evidence of real authentication. Missing prerequisites
must prevent allowance. Do not model “not known unsafe” as “known safe”.

Use an immutable input snapshot for each decision. If freshness/expiry is included,
pass a trusted evaluation time as data; never read the clock in a builtin. A production
adapter would have to bind execution to the checked artifact and address changes
between authorization and execution. The example must state that boundary.

For bot actors, identify the human sponsor and the scoped delegation that authorizes
deployment. Record authorization separately from execution: a proof of permission is
not evidence of a completed deploy. See the classifier example's
[accountability scenarios](../classifiergate/README.md#accountability-who-entrusted-the-bot-with-this-action)
for the shared distinction between judgment, authority, decision and outcome.

## Required walkthrough

1. A bot has the correct deployment grant, but no matching approval: denied.
2. An independent reviewer approves the exact commit/artifact: allowed.
3. An incident freeze becomes active: denied despite grant and approval.
4. An approval exists for an earlier commit: denied for the new commit.
5. A rule or ownership change newly allows an unrelated actor: the preview exposes it.

The plain renderer should show which prerequisite failed, with a link or identifier
back to the supporting evidence. “Approval was for a81c; requested b92d” is appropriate
only if both records are supplied and checked. A WhyNot result alone may merely say
that no matching approval was found. Do not fabricate the stronger explanation.

## Policy-change preview

Evaluate the same finite request catalogue against before/after snapshots. Each
snapshot identifies its rule sets and standing facts. Sort the output and show:

- Newly allowed requests, with the new proof and previous absence explanation.
- Newly denied requests, with the previous proof and new absence explanation.
- Unchanged decisions, optionally suppressed in the display.
- Evaluation errors as a distinct category. Never turn an error into an ordinary
  denial in the comparison, or report it as a clean preview.

Identify whether rules, facts or both changed. State the number of requests examined
and print **“Coverage: supplied request catalogue only.”** This is not a universal
analysis of every possible principal, action, resource or context.

The preview manifest should identify the request catalogue, policy/fact snapshots,
engine version and effective bounds. Define any snapshot digests in the example's
own manifest; do not imply that an answer hash or rule-set ID identifies the whole
world state. Keep telemetry outside existing proof/hash contracts.

## Acceptance and pressure cases

- Table-driven scenarios cover all five walkthrough outcomes plus wrong service,
  wrong environment, wrong artifact, self-approval, missing trusted identity, revoked
  grant and a similarly named neighbouring service.
- Successful scenarios have only the intended authorization path. Remove a required
  fact/rule and demonstrate that the relevant test fails; no fallback RBAC grants.
- Test actual proof/absence structure, not just substrings in generated prose.
- Bound and cancellation failures return no allowance; the preview labels them errors.
- Reordered equivalent facts preserve decisions; canonical fixture ordering keeps
  the example's recorded explanations reproducible.
- Before/after tests exercise both grant expansion and revocation. An unchanged
  catalogue/snapshot gives an empty decision diff.
- Report cost/headroom without freezing exact step counts across engine versions.
  Explain that evaluation cost excludes explanation work and is not peak memory.
- Run the example's tests in CI. Verify against both the declared released dependency
  and the root checkout using the repository's established temporary replacement method.

## Related work

[Classifiergate](../classifiergate/README.md) is a separate worked-example brief for
classifier judgments evaluated by Shearline rules. It is independent of this example.
Conversational rendering of receipts is a small [Shearline-wide option](../../docs/LLM-EXPLANATIONS.md).
An optional [Anam agent record](../../docs/AGENT-RECORD.md) can retain bot delegations,
deployment decisions, attempts and outcomes across runs; permission and execution remain
separate records.

The main README should lead with a real transcript and finish with the trust
boundaries. A useful launch description is: “Explain deployment authorization and
preview policy changes in Go.” Claims must match the executed scenarios.
