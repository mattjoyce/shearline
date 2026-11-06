# Collation: card 010 Query, branch feat/010-query at 635bcf5 (2026-11-07)

The panel reviewed the diff `master..feat/010-query`: the new `Policy.Query` API in
`policy/query.go`, its tests, the hrchat/2 rule change, the deprecation of `Permitted`, and the
card's acceptance-12 measurement with its two after-the-stop re-measurements. Three pairs ran
blind to each other: Ousterhout × Liskov on the module boundary (surface × substitution),
Leveson × Hoare on the contract and the unsafe state (safety × correctness), Gelman × Kohavi on
the measurement claim (inference × decision). The question put to the panel: is this diff ready
to finish and release as v0.6.0, and is the card's stop at acceptance 12 the right reading?

## Verdict table

| Pair | Axis | One-line verdict |
|---|---|---|
| **Ousterhout × Liskov** | surface × substitution | Query's "agrees with Decide" promise is conditional on the caller's Space as well as the rules, and a violation adds an allow instead of raising an error. |
| **Leveson × Hoare** | safety × correctness | The keying contract is the wrong precondition: three rule sets that satisfy its text still diverge, and the shipped suite cannot see any of them. |
| **Gelman × Kohavi** | inference × decision | Acceptance 12 failed as written (0.26 to 0.30 over three runs); the "equal work" ratio of 0.07 is a post-hoc change of unit, legitimate only if recorded as one and confirmed on seeds that did not choose it. |

## The shared meta-finding

**The agreement-with-Decide contract is prose, is not the true condition, fails open, and nothing
in the tree can detect its violation.** Ousterhout reached it as a dependency invisible at the
interface (the caller's resource names and the rule author's patterns interact through
`prefix/2`); Liskov as a postcondition no caller can check; Hoare by constructing three rule sets
that meet the contract's wording and still diverge (`01`, O1/L1; `02`, F1 a to c); Leveson as a
hazardous state with no control, where the only feedback that could detect it, the proofs, is
computed and discarded (`02`, F2). Gelman × Kohavi reached the same locus from the other side:
the card's own Why says the value is the relation with proofs, so correctness is the goal and
speed is a guardrail, which makes the unverifiable correctness claim the only finding that
matters (`03`, F2).

## Where the pairs converge

**C1 🔴 Respecify the invariant and make it executable.** `01` O1/L1, `02` F1, F2, F3. Three
counterexamples beyond the card's `met/1`: an unjoined keyed relation (`direct(_, S)` at
hrchat.dl:58, head still carries P, 9 wrong allows, whole suite passes); a prefix leak that is a
property of the caller's Space, not the rules (`doc/1` in the Space makes `doc/10` allowed);
and a leak under `not` that removes allows, which a "no hole" property check cannot see.
Unified fix, in order:

1. Restate the contract in the card and `query.go` doc as Hoare words it: every positive use of
   a request-seeded relation is joined to the request variable of its axis, and a pattern derived
   from a request fact is compared only with that request's own resource. Say "fail-open".
2. Add Leveson's runtime control inside Query: a proof of `allow(p, a, r)` whose body cites
   `principal(q)`, `action(b)` or `resource(z)` for another value is a derivation Decide could
   not make. Prototyped: 0 of 23,760 hrchat/2 allows flagged, all 9 wrong allows flagged. Return
   an error (whole-or-absent) naming the tuple and the foreign fact; the doc says a flag means
   "run Decide on this tuple". It catches spurious allows only; spurious absences stay with the
   static check (a tarski card) or a Decide sample.
3. Add a table test of deliberately violating rule sets (principal-unkeyed, action-unkeyed,
   unjoined keyed relation, prefix collision, negation) asserting Query disagrees with Decide,
   and that the runtime control catches the allow-side ones.
4. Add a differential test over multi-member Spaces on every axis against the Decide loop.
   `TestQuery_HRChatAgreement` runs single-principal Spaces and so cannot see a principal-axis
   leak; it passed with hrchat/1's `met/1` restored (`02`, F1 d). Record that tuple on the card:
   `emp/E0950 approve.salary emp/E0002 requester=emp/E0950`.

**C2 🟠 Matrix must carry the question it answers.** `01` O4, `02` F4. `Bag` is an index into a
slice the caller can mutate, and absence means nothing without the Space. Echo a canonical form
of the Space (sorted deduplicated sets, each bag's sorted keys and values, or a digest) in the
Matrix, as `Diff.Request` does in compare.go; a future `Minus` refuses unequal Spaces.

**C3 🟠 Fail early.** `01` O5, `02` F12. Validate every bag before the first evaluation. An
invalid bag 2 today errors after bags 0 and 1 ran.

**C4 🟠 Empty `Contexts` is asymmetric.** `02` F5; `01` tension on empty sets. Empty principals,
actions or resources are refused; empty Contexts becomes one bare evaluation, and `Bag == 0`
then means "first bag" or "no bag". Decide one way and say so in the doc.

**C5 🟠 "split the Space" is the wrong remedy text.** `01` O2, `02` F6. It exports a splitter to
every caller, drops Decide's "usually wrong, not large", does not say which set or bag was over,
and splitting by principal turns a leaking rule set into one that looks fine. Name the sizes and
the bag, keep Decide's wording, and do not prescribe splitting.

**C6 🟠 The `Permitted` deprecation is not one.** `01` L2, `02` F7. No blank `//` line above
`Deprecated:`, so pkg.go.dev, gopls and staticcheck do not see it; acceptance 14 is met by grep
only. Semantically it removes a capability (open-world prefixes, no deny, no context) rather than
substituting one; DESIGN Decided 5 is not amended on the branch. Fix the comment, amend DESIGN.

**C7 🟠 Acceptance 12: the FAIL stands, and the bar was never derived.** `03` F1, F2; `01` O3.
Three runs give 0.26 to 0.30 against 0.1. The 0.07 "equal work" figure is 0.277 divided by the
known 4x multiplier the card's own bag scheme wrote down before any data (card lines 157 to 162),
so it is a change of unit after the result, chosen in the one direction that passes. Ousterhout
reads the same 4x as a product-shaped Space taxing the first consumer. Unified fix: keep 12 with
its verdict FAIL; add 12b, dated and labelled post hoc; derive the rule from what 007 needs (not
slower than the loop on the needed requests, under 25% of a bound on the 007 shape), and confirm
on org seeds 2 to 4 with interleaved order, an A/A loop-vs-loop noise floor, and a logged
environment. Fix the card's wide-Space sentence: the narrow comparator was 32,400 triples, not
"the same work", so 8.4 µs is right and the phrase is wrong (`03`, F5).

**C8 🟡 Cost numbers are being treated as contract.** `01` L3, O6; `03` F4. The golden pins
`Steps` and `Bytes`, which `Decision` calls observation-only and version-unstable; rename to
`MaxSteps`, `MaxBytes` and drop them from the golden. The 14x per-triple figure is a point on a
curve (per-evaluation overhead amortised over 270 triples), not a property of set evaluation.

**C9 🟡 Small loci all three touched.** `facts()[3:]` positional coupling (query.go:120); `proof, _ :=`
discards the ok flag (query.go:146); rbac.dl and hrchat.dl comments still teach the one-request
model; each Allow retains about 7.8 KiB of proof; `E%04d` IDs stop being prefix-free at 10,000.

## The genuine cross-pair tensions

**T1 Auto-split on `TooLarge`.** Ousterhout: bisect `Principals` inside Query and merge, so
callers never write a splitter. Liskov and Leveson: a bound hit is information, Decide's doc calls
it "usually wrong, not large", and splitting by principal is exactly the move that hides a
principal-axis leak (`02`, F6 and § 6). Owner's call. The panel's lean: never split; after C1's
runtime control exists, the hazard splitting hides is detected anyway, which weakens the
objection but not the "information" point.

**T2 Empty set: error or empty Matrix.** Ousterhout defines the error out (007's candidate list
can be empty). Liskov and Leveson keep it: a vacuous Matrix passes a property check with nothing
checked. The panel sides with keeping the error, and extends it to `Contexts` (C4).

**T3 Is speed an acceptance criterion at all.** Gelman: drop the 0.1 bar, report the interval and
the shape curve, no pass/fail. Kohavi: a decision must be made and its rule fixed before the
replicate, or "report the curve" defers it forever. 12b takes Kohavi's structure and Gelman's
content. If the owner wants the 0.1 bar to stand on the needed work, the honest outcome is the
card's own stop rule: Query ships, 007 keeps its loop.

**T4 The cheap shape is the blind shape.** Inside the card's own results: performance and the
TooLarge remedy push consumers to single-principal Spaces, which are exactly the Spaces on which
Query cannot be wrong on the principal axis; the feature's value, a whole Matrix over many
principals, is exactly where it can (`02`, § 6). C1's runtime control is the only thing that
lets the useful shape be the safe shape.

**T5 Prove or detect.** Hoare wants the precondition proved by static analysis of the compiled
plan (a tarski card) and reads a runtime check as a patch over a missing proof. Leveson will not
wait and wants the partial detector now. Both: detector now, proof as the second layer.

## Single-lens signals

- **Ousterhout × Liskov:** computing Query with proofs off was 15 to 22% faster, so a proof
  knob is not worth an interface; Query duplicates Decide's evaluate-and-cost block, extract one
  `p.evaluate`; the error text carries engine words and a double `policy:` prefix.
- **Leveson × Hoare:** three `query.go` mutations survive the suite (drop the final sort, drop
  either `ctx.Err()` check); whole-or-absent on error holds on every return path and the
  membership filter is sound; `base.With` is safe because `Load` forbids standing request facts.
- **Gelman × Kohavi:** both measurement sides run duplicate work (16 duplicate bags per
  principal); the harness logs no Go version, host or commit and fixes Query-then-loop order;
  the 25% bound check is the one deterministic, machine-independent criterion and it passed on
  the 007 shape with large headroom.

## Build verdict

**Hold.** The code is sound where it claims to be (whole-or-absent, membership, determinism,
bounds), and every lens credits that. It is not releasable while the doc promises "under this
contract Query agrees with Decide" and the contract is false, the failure direction is an extra
allow in an authorisation library, and no test or runtime check can see it.

Must-fix before v0.6.0: C1 (respecified contract, runtime proof-citation control, violating
rule-set tests, multi-axis differential test, the recorded mutation tuple), C6 (a real
deprecation and the DESIGN amendment), C7 (12 stays FAIL, 12b written before the seed 2 to 4
replicate runs, wide-Space sentence corrected). Should-fix in the same release: C2, C3, C4, C5,
C8. Decisions owed by the owner: T1, T3.
