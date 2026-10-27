# 002 Show what a decision cost

Asked for by: hrchat (`hrchat/kanban/backlog/006-shearline-decision-cost.md`, part 1).
Owner: shearline-lead. Built by: opencode via _POINTDEV. Release: v0.4.0 (new fields, additive).
Part 2 of the request (configurable bounds) is card 003, parked. Written in ASD-STE100.

## Now

`Decide` (`policy/policy.go`, `func (p Policy) Decide`) calls `tarski.Evaluate` and discards
`res.Cost()`. A consumer sees a bound only when a decision fails with `*tarski.TooLarge`. With
consumer relations (v0.3.0) the cost grows with the consumer's standing data, so a consumer must
be able to watch the margin before a valid request is refused.

## Change

1. Add two fields to `Decision`, after `RuleSets`:
   ```go
   Steps int // evaluation steps (Result.Cost); zero when err != nil
   Bytes int // derived bytes, approximate (Result.Cost); zero when err != nil
   ```
2. In `Decide`, set them from `res.Cost()` only on a return with `err == nil`, allowed or denied.
   The return after `WhyNot` fails (`"is neither proved nor absent"`) is an error: leave both zero
   there. Every early return before `Evaluate` leaves both zero.
3. Do not copy the cost into the `Decision` on a `*TooLarge`. It stays reachable through
   `errors.As(err, &tl)` as `tl.Steps` and `tl.Bytes` (`evalError` wraps with `%w`); test that.
4. Doc comment on the two fields (and on `Decision`), in these words or near them:
   - The cost is the engine's evaluation work only: it excludes request preparation, proof
     construction and why-not processing. It is not latency and not memory; Bytes approximates
     the encoded size of derived tuples.
   - It is for observation (logs, ledgers, alerts on the margin to the bounds). Do not decide on
     it: it can change with any tarski version, with no change to the answer.
   - It is in no hash, and a changelog does not list a cost change as an answer change.
5. Replace the "It is not a knob" sentences on `maxDecisionBytes` and `maxDecisionSteps` with:
   a policy that goes past a bound is usually wrong, not large, so narrow it first; a correct
   policy over large consumer data can also go past the current budget; `Decision.Steps` and
   `Decision.Bytes` show the margin; card 003 (configurable bounds) and demand evaluation are the
   planned remedies. Keep the constants' values.
6. `Permitted` does not change.

## Acceptance

- [ ] `Decision.Steps > 0` and `Decision.Bytes > 0` on a successful allow, and on a successful
      deny, in tests.
- [ ] Both are zero on each error path: a malformed request (`ErrRequest`), a cancelled context,
      and a `*TooLarge`. Use the v0.3.0 consumer-relations fixture of a 2,000-person single chain
      for the `*TooLarge` (it exceeds the byte bound).
- [ ] On that `*TooLarge`, `errors.As(err, &tl)` works and `tl.Bytes > tl.MaxBytes`.
- [ ] A larger fixture gives a larger cost: the same consumer `manages` policy over a 100-person
      and a 1,000-person binary tree, same request shape, and `Steps` and `Bytes` both larger for
      1,000. Assert the order, not exact numbers.
- [ ] Same request twice gives the same `Steps` and `Bytes` (deterministic within one tarski version).
- [ ] No test pins an exact cost number. Answers, proofs and `rbac/1 sha256:0bf5a121...` do not
      change; every existing test passes unedited.
- [ ] Docs: `CHANGELOG.md` Unreleased entry (at the top, above v0.3.0); `docs/DESIGN.md` Decision
      block; `okf/api/policy-decide.md`; `okf/concepts/per-request-evaluation.md` (a paragraph on
      watching the margin); `okf/log.md` one `* **Change**:` line. Okf validator CONFORMANT.
- [ ] Gate passes. Do not tag. Do not merge.

## Out of scope

- Configurable bounds (card 003). Cost for `Permitted`. An engine version on `Decision`.

## Review 1 (shearline-lead, 2026-10-28): code approved, docs changes requested

`policy.go` and `cost_test.go` are approved: two mutations (drop the cost on the deny path; copy
the `TooLarge` cost into the Decision) each fail a test. The docs repeat the card's sentences
verbatim in four places. The card's "in these words" was mine; fix it now. Change only these
files, and only as follows:

1. `policy/policy.go`, the two constant comments. No kanban card numbers in code. Use exactly:
   - `maxDecisionBytes`: "maxDecisionBytes bounds what one decision may derive. A policy that goes
     past it is usually wrong, not large: narrow it first. A correct policy over large consumer
     data can also go past it; Decision.Bytes shows the margin. Configurable bounds and demand
     evaluation are the planned remedies."
   - `maxDecisionSteps`: "maxDecisionSteps bounds the join work of one decision; the deadline alone
     would let a wide product of duplicates run until it fires. Past it, as for
     maxDecisionBytes: narrow first; Decision.Steps shows the margin."
2. `okf/concepts/per-request-evaluation.md`: restore the two table rows to their v0.3.0 text
   (`git show origin/master:okf/concepts/per-request-evaluation.md`). Replace the sentence "Both are
   named constants, invariants and not knobs." with "Both are named constants. A correct policy over
   large consumer data can go past them; see Cost for observation." In "# Cost for observation",
   delete the last sentence (from "A policy that goes past a bound"), and use no em-dash.
3. `okf/log.md`: move the Decision-cost line to the end of the list, after the v0.3.0 line, and
   shorten it to: "* **Change**: `Decision.Steps` and `Decision.Bytes` give the evaluation cost of a
   successful decision, for observation; zero on error. Answers, proofs and `rbac/1` unchanged."
4. `CHANGELOG.md` Unreleased entry, replace the bullet with:
   "- **Decision cost.** `Decision` gains `Steps` and `Bytes`, the evaluation cost from
   `tarski.Result.Cost()`, on every successful decision, allowed or denied. They are zero when
   `Decide` returns an error; on a `*TooLarge` the cost is on the error (`errors.As`). They are for
   observation, in no hash, and may change with any tarski version. Answers, proofs and `rbac/1`
   are unchanged."

## Review 2 (shearline-lead, 2026-10-28): approved

Commit 9c0b1bc, same opencode session resumed. All four Review 1 items match the given text
exactly; the log line is last; gate and okf validator pass. Ready for PR.

## Review 3 (shearline-lead, 2026-10-28): two additions (from GPT's review of PR #9)

1. Test gap: swapping `Steps` and `Bytes` in `Decide` passes every cost test. In
   `policy/cost_test.go`, add `TestDecide_CostForwardsResultCost`: for an allow and for a deny,
   build the same base `Decide` builds (`p.base.With(req.facts()...)`), call `tarski.Evaluate` with
   `tarski.Options{MaxBytes: maxDecisionBytes, MaxSteps: maxDecisionSteps, Proofs: true}`, and
   assert `d.Steps == steps` and `d.Bytes == bytes` from `res.Cost()`. Pick a fixture where steps
   and bytes differ, and assert `steps != bytes` so a swap cannot pass by coincidence.
2. Read-only bounds, so a consumer can compute the margin without copying private constants.
   In `policy/policy.go`, add exactly:
   ```go
   // Bounds are the limits one decision is evaluated under. Decide and Permitted use the
   // same bounds. Read-only: a policy cannot change them.
   type Bounds struct{ MaxSteps, MaxBytes int }

   // Bounds returns the bounds this policy's decisions are evaluated under, for computing
   // the margin from Decision.Steps and Decision.Bytes.
   func (p Policy) Bounds() Bounds { return Bounds{MaxSteps: maxDecisionSteps, MaxBytes: maxDecisionBytes} }
   ```
   Add `TestPolicy_Bounds` asserting the two values equal the constants. Do not add a `Config`
   field (that is card 003).
3. Docs, one line each, no other change:
   - `CHANGELOG.md` Unreleased: add the bullet "- **Bounds.** `Policy.Bounds()` returns the
     `MaxSteps` and `MaxBytes` a decision is evaluated under, read-only, so a consumer can compute
     the margin from `Decision.Steps` and `Decision.Bytes`."
   - `okf/concepts/per-request-evaluation.md`, at the end of `# Cost for observation`, add: "The
     margin is the cost over `Policy.Bounds()`."
   - `okf/log.md`, a last line: "* **Change**: `Policy.Bounds()` exposes the decision bounds
     read-only."

## Review 4 (shearline-lead, 2026-10-28): approved

Commit 27c10d7, built by a Sonnet subagent (opencode free tier exhausted). Review 3 items match;
swapping `Steps` and `Bytes` in `Decide` fails `TestDecide_CostForwardsResultCost` (allow and
deny); gate and okf validator pass. Ready for merge.
