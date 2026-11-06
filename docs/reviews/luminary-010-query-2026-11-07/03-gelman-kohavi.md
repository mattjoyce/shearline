# 03 Gelman x Kohavi (inference x decision): card 010, acceptance 12

Branch feat/010-query @ 635bcf5. Read: `kanban/doing/010-query.md` (lines 1-366), `examples/hrchat/hrpolicy/query_test.go`, `query_space_test.go`. The two post-stop tests are not in the tree; judged from the card's description only.

## Verdict

Supported claim, stated in the unit the pre-data rule used: on org seed 1, hrchat/2, Go 1.27.3, M1, Query ran the card's 007 workload in 9.8-12.3 s against 35.5-41.0 s for the 388,800-decision loop. That is 3.3x to 3.9x faster (ratio 0.256-0.299 over three runs). The bar was 0.1. Acceptance 12 FAILED as written, and the failure is not noise: the margin is 2.6x-3x, the run-to-run spread is about 15%.

The 14x figure is a true statement about a different quantity: cost per answered triple. About 3/4 of the answered triples are not requests the loop (or card 007) ever needed. It does not rescue the criterion, and it is not a measurement that was ever going to be independent of the first one: 0.277 / 4 = 0.0693, and the reported 0.0695 is that arithmetic plus one confirmation that Decide scales linearly.

The 25 % bound criterion is the one genuinely independent, deterministic check, and it passes with large headroom on the card's shape (0.44 % steps, 1.20 % bytes). It fails on the wide shape (55.2 % / 42.6 %). Both facts are real and should both stay in the card.

## Raw data (all runs, one table)

| Run | Who | Conditions | Query | Loop | Loop work | Ratio |
|---|---|---|---:|---:|---|---:|
| 1 | Sol | overlapped golden + coverage checks | 9.903 s | 38.631 s | 388,800 | 0.2564 |
| 2 | Sol | "isolated" (builder's own commands only) | 9.837 s | 35.492 s | 388,800 | 0.2772 |
| 3 | this review | shared machine: load 1.9 -> 4.7, other agents live; `GOCACHE=/tmp/lum-gk-gocache`; test 53.2 s | 12.267 s | 40.959 s | 388,800 | 0.2995 |
| 4 | Splendid, throwaway | not in tree | 9.80 s | 140.9 s | 1,555,200 | 0.0695 |

Steps/Bytes in runs 1-3: Query 18,421 / 12,595, Decide 170 / 621, identical every run (deterministic, machine-independent).

Derived (arithmetic on the rows above):
- Loop per decision: 99.4, 91.3, 105.3 (run 3), 90.6 (run 4) us. Card 007's "145 us" is higher than all four.
- Query per answered triple: 6.37, 6.33, 7.89 (run 3), 6.30 us. Query per evaluation: 9.84 s / 5,760 = 1.71 ms.
- Run 3 vs run 2: Query +25 %, loop +15 %. Ratio moves 8 % in the loop-favouring direction under load.
- Ratio under three denominators, run 2 Query time as numerator:
  - work the loop needs, 388,800: 0.277 (fail)
  - answered triples, 1,555,200 (loop 90.6 us): 0.0695 (observed; pass)
  - distinct answered triples, 1,425,600 (see 🟠 duplicate bags; loop at 90.6 us = 129 s): about 0.076 (pass)

## Findings

### 🔴 F1. The "equal work" restatement is a forking path, not a correction. It is a legitimate amendment only if recorded as one.

Locus: card lines 346-355 (Equal work), 365-366 (Decision pending); card lines 157-162.

1. The unequal work was not discovered after the data; it was written down before it. Lines 157-162: "at most 5,760 evaluations of 270 triples per side, in place of 388,800 decisions". 5,760 x 270 = 1,555,200. The bag scheme was chosen knowing Query answers 4x the cells (the Why at lines 27-29 names the bag list as "the one axis that multiplies"), and line 160 says the extra cells are "a correct answer and is kept". So the pre-data rule measured the system including its known multiplier. "Credits the loop with a quarter of the work" re-describes a designed cost as a spec error.
2. The loop's 388,800 is what card 007 needs (`requestAt` in `query_space_test.go:127-134` maps exactly 48 variants per cell; the 007 property loop is that). The 1,166,400 extra cells are a byproduct of expressing "requester = resource" as a bag. If 007 or 008 truly need the off-diagonal requester cells, then the right fix is to restate the baseline (the loop would be 1.555M, and card 007's "a minute each side" is wrong), and that must come from 007's requirement, not from the ratio result. The card does not say that.
3. Denominator choice was made after the result and in the only direction that flips the outcome. Candidates available: needed requests (fail), answered triples (pass), distinct triples (pass), allows (not reported), per evaluation vs per decision (not reported). One of the several defensible choices was selected; the selector (Splendid) is a third party to the rule writer (Fable) and the builder (Sol), but saw the failing number first. That is the garden: no one cheated, the data was visible when the path was chosen.
4. Run 4 adds almost no information. 9.80 s Query is the same measurement as runs 1-2; the loop at 90.6 us/decision matches run 2's 91.3 us. It confirms linear scaling of Decide in request count. It is not a replicate of the Query side and not a second seed.

What would make the change legitimate (any of these; the first is mandatory):
- (a) Amend, do not replace. Keep acceptance 12 as written with its recorded verdict FAIL (0.26-0.30). Add a new item 12b dated and labelled "post hoc, written after seeing runs 1-3". Disclose that the data was seen.
- (b) Justify the new unit independently of the result, from what 007/008 need (point 2).
- (c) Confirm on data that did not pick the rule: other org seeds (see Extra run). Seed 1 chose the rule; seeds 2-4 may test it.
- (d) Make the decision, not the bar, the thing recorded (see Restatement below).

### 🔴 F2. The decision the bar gates is mis-specified: wall time is a proxy; the card's own value statement is not speed.

Locus: card lines 23-31 (Why), 201-205 (accept. 12), 231-232 (stop rule).

The Why says the value is "the derived allow relation ... each tuple with its proof", set-valued, with TooLarge an error and never a shorter list. Acceptances 1-11 test that and pass (card 273-286). Speed was never the OEC; it is a guardrail ("Query must not be a regression"). A guardrail has one natural bar: not worse than the thing it replaces, plus a budget the consumer needs. At 0.28 Query is 3.6x faster than the loop on the work 007 needs and saves about 26 s per side per property in an absolute sense; 007's own text says the loop is "about a minute each side" and both fit. The 0.1 bar (an order of magnitude) has no stated derivation. The consequence of missing it ("007 keeps its Decide loop") throws away a feature that passes every correctness criterion because a proxy fell short of an unmotivated threshold, and the response (restate the bar) fixes the symptom in the same unprincipled way.

### 🟠 F3. n = 2 (plus one contaminated run), no variance reporting, no A/A.

Locus: card lines 251-258; `query_test.go:107-150`.

- Single org seed, single machine, single Go version, ordered Query-then-loop in one process with no repetition and no warm-up. Run 3 shows +25 % Query / +15 % loop under ordinary shared load; the card's "isolated" means only that the builder ran nothing else, on a machine shared with other agents.
- No A/A: nobody ran loop-vs-loop, or Query-vs-Query in swapped order, to learn the noise floor of the ratio in this harness. Run-to-run spread (0.256-0.299, about 15 %) is the only estimate of it and it is not on a quiet machine.
- The fixed order (Query first, loop second, 35-140 s later) confounds order with thermal/GC state. The linearity of loop per-decision cost across runs 2 and 4 (91.3 vs 90.6 us) is mild evidence against throttling, but the harness would not detect it.
- The `matrices` slice of maps (line 115) stays live into the loop timing, so loop GC runs with a heap Query built. Not measured; direction unknown.

### 🟠 F4. Type M on the 14x: it is a one-workload number with an unmodelled cost structure.

Locus: card lines 350-363; `query_space_test.go:114-124`.

- Steps per unit: Query's max steps per evaluation is 18,421 for 270 triples = about 68 per triple; Decide is 170 per decision. That is a 2.5x gap in a deterministic metric (max-vs-max, upper bound for Query), against a 14x gap in wall time. Most of the wall-time gap is therefore not the engine doing less derivation; it is per-call overhead in Decide amortised over 270 triples. That is a real benefit, but it is different from "set-valued evaluation is 14x cheaper", and it predicts the ratio will move when the loop's per-decision overhead changes (a leaner `Decide`, a reused base) without any change in Query.
- The card's own numbers imply the cost per answered triple moves 5x with shape: 6.3 us (main), 7.1 us (wide), and the narrow "same work" row (0.27 s for 200,880 triples) is 1.34 us. A 5x swing in the denominator of the headline ratio means 14x is a point on a curve, not a property. Also (unverified hypothesis from the card's numbers only): 1.7 ms/evaluation at 270 triples and 0.27 s / 120 evaluations = 2.25 ms at 1,674 triples would put most of Query's time in a roughly 1.5 ms fixed cost per evaluation, so the lever is the bag count (5,760) not triple count. If true, "equal work" is the wrong axis for Query entirely.
- Only max Steps/Bytes are recorded; totals would give a machine-independent cost ratio. Fix the logging (F6).

### 🟠 F5. Internal inconsistency in the wide-Space paragraph.

Locus: card lines 358-363.

"Per triple the cost is the same (7.1 us against 8.4 us); the wide shape is not faster." 1.43 s / 200,880 = 7.1 us, fine. The narrow comparator is stated as 0.27 s, which is 1.34 us/triple, not 8.4 us (8.4 us would be 1.69 s). One of the two numbers is wrong, and they point opposite ways: 0.27 s says wide is 5.3x slower than narrow; 8.4 us says it is slightly faster. The conclusion "narrow Spaces are the right shape" survives either; the sentence "not faster" does not survive 0.27 s. Reconcile before this paragraph is cited as the card 003 trigger evidence.

### 🟠 F6. Harness does not log enough to replay, and the replayable part is opt-in.

Locus: `query_test.go:108-109, 145, 147-148`.

- Skipped unless `SHEARLINE_QUERY_MEASURE=1` (line 108): CI never runs it, so the measurement and the 25 % bound check are not a regression gate for anything. They ran twice, by hand, once under overlap.
- The `t.Logf` line records seed, counts, times, ratio, Steps/Bytes. It omits: Go version (card has it by hand only), GOOS/GOARCH, `GOMAXPROCS`, CPU model, git commit, wall-clock date, load, whether `-race` was on, and Query/loop order. The card's prose supplies some of this; the test output alone cannot replay.
- Line 147 folds four conditions (`count != 388800`, `ratio > 0.1`, steps/4, bytes/4) into one boolean and one message ("measurement misses the card's bar"); the reader must reparse the log to learn which criterion failed. Report each separately; the independent 25 % criterion should be its own assertion.
- `count != 388800` and `ratio > 0.1` are hard-coded. The restated rule has no harness: the equal-work experiment is not in the tree, so the claim "met at 0.0695" is a statement about code that no longer exists.
- Timing on the Query side includes `matrixKeys` map construction (`query_test.go:123`); the loop side includes map lookups (line 134). Small relative to 91 us, but asymmetric and unmeasured.

### 🟠 F7. Sample ratio: the denominators contain duplicate work on both sides (read from code, not run).

Locus: `query_space_test.go:56-62, 97-99`; `query_test.go:131-133`.

`propertyResources` adds `p.ID` first, so `resources[0] == principal`. `propertyBags` ranges over requesters `["", principal, resources...]`, so `requester == principal` appears twice: 12 requesters, 11 distinct, 16 duplicate bags per principal. Consequences: Query evaluates 30 x 16 = 480 duplicate bags (129,600 duplicate triples; distinct answered = 1,425,600, not 1,555,200). The loop's variant 1 and 2 for resource 0 are the same request: 30 x 27 x 16 = 12,960 duplicate decisions (distinct needed = 375,840 of 388,800). The ratio is insensitive to this (about 3 % and 8 % respectively) but "1,555,200 triples" and "388,800 decisions" are counts of evaluated cells, not of distinct questions, and the two sides duplicate in different proportions. The card says `TestQuery_Bags` "checks repeated bag indices"; it does not say the measurement space has them by construction.

### 🟡 F8. Twyman on the 14x: agree it is not a bug, but only because correctness was checked.

The exciting number is trustworthy as arithmetic because every Query answer was compared to Decide (`query_test.go:134`, card 255, 355), in every run, including 1.555M cells in run 4. That check is the reason 14x is not a logging artifact. What Twyman still flags is the interpretation: the ratio is large because the numerator and denominator count different things (F1) and the fixed per-call overhead dominates (F4). The check that would catch a bug (disagreement) passed; the check that would catch a misread (unit) was not run.

### 🟡 F9. The 25 % bound criterion deserves more credit and less weight.

Passes narrow (0.44 % / 1.20 %), fails wide (55.2 % / 42.6 %). It is deterministic, so it is the cleanest evidence in the card. But the bar says "no evaluation passes 25 %" for the 007 shape only; the wide result is not a failure of acceptance 12, it is a shape-dependence finding. Record both in the card as the bound-versus-shape curve rather than as one pass and one trigger. It is also a dichotomy (25 %/not) on a quantity that has a measured slope; a third point between narrow and wide would make the "split the Space" remedy quantitative.

### 🟡 F10. Dichotomising the ratio at 0.1.

0.277 fails and 0.0695 passes, but the difference between the two is the denominator, not evidence. The interesting object is a cost model: Query time = a x (evaluations) + b x (triples), Loop time = c x (decisions). Fit a, b, c from the shapes already measured (30 principals x any bag count is free variation) and report the breakeven bag count at which Query stops being cheaper. That replaces a bar that moves with the unit.

## Restating acceptance 12 honestly

Leave the original and its verdict. Add, in the card, as a new item dated 2026-11-07 and labelled post hoc:

> 12 (as written, unchanged). Measured 0.256-0.299 over three runs on seed 1. FAIL. Bound criterion: PASS (0.44 % steps, 1.20 % bytes).
>
> 12b (post hoc; written after the 12 result was known; proposed by Splendid; not pre-registered). The decision this serves is whether card 007 runs its full space with Query or with the Decide loop. Rule: Query is chosen if (i) it is not slower than the loop on the requests 007 needs (met: ratio 0.28, a 3.6x speedup), (ii) no evaluation passes 25 % of either bound on the 007 shape (met), and (iii) on org seeds 2, 3, 4 (not used to choose this rule) the per-principal ratio of Query to loop on the needed requests has median below 0.5 and the per-answered-triple ratio below 0.1. Equal-work ratio 0.0695 on seed 1 is reported as exploratory and carries no weight in (iii).

The 0.1 bar is moved from the headline to the per-triple secondary only because the card's own Why makes speed secondary; if Matt instead wants the 0.1 bar to stand on the needed work, the honest outcome is the card's own stop rule: leave the card open, 007 keeps its loop, Query ships without the 007 migration.

## The one extra run that settles it

One test invocation, one process, new tests committed behind the same env var, no change to `policy/`:
1. Org seeds 2, 3, 4 (not 1).
2. Per principal, interleave: for even-indexed principals run Query then the 3 x 48 needed-request loop; for odd run loop then Query. Record per-principal wall time and ratio (n = 30 per seed, 90 total), and report median, IQR and range per seed.
3. In the same run, an A/A: the loop run twice per principal in swapped order, to give the noise floor of the ratio in this harness.
4. Record total Steps and Bytes for both sides, not just maxima.
5. Log Go version, GOOS/GOARCH, GOMAXPROCS, CPU, git sha, date, uptime load, and what else was running.
6. Report three denominators side by side: needed requests, answered triples, distinct answered triples. Assert each criterion separately.

The 12b rule above is written into the card before this runs. If the needed-request ratio median is under 0.5 on all three seeds and the per-triple ratio is under 0.1, 12b passes and the amendment stands. If the A/A shows the harness noise exceeds about 20 %, the 0.28 vs 0.1 failure is still a failure but any narrower claim is not supported.

## Convergence

Gelman and Kohavi converge on the central point: the original rule was fixed, the result was seen, the rule's unit was changed in the one direction that passes, and that cannot be the evidence for passing. Both want n > 1 on a seed that did not choose the rule, a logged and replayable harness, and the 12b rule written before the replicate. Both read the 14x as real arithmetic over an unlike denominator, not a bug.

## Tension

Gelman would drop the 0.1 bar entirely and fit the cost model (F10): report the interval on a continuous ratio, model the shape dependence, no pass/fail. Kohavi insists a decision be made and that its rule be fixed in advance: 007 must go one way or the other, and "report the curve" is how that decision gets deferred forever. The proposed 12b takes Kohavi's side on structure (a rule, a decision, written first) and Gelman's on content (a guardrail tested on fresh seeds with a noise floor, not a single threshold on one org). Where they stay apart: Gelman would not run the "median below 0.5" gate at all; Kohavi would, because a flat or failed result must have a pre-agreed consequence.
