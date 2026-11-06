# 010 Query: Leveson x Hoare (safety x correctness), code audit

Artifact: `git diff master..feat/010-query` at 635bcf5. Reviewer works alone.
All "I ran" claims were run in throwaway copies under `/tmp/lum-hl/` (`git archive` of the branch, then mutated). The repository was not edited; the only file written there is this one.

## 1. Hoare: the contract of `Query` (policy/query.go:54)

Notation. B = standing base, Pi = compiled plan, K = declared context schema. For a request (p,a,r,c): `Decide` allows iff `allow(p,a,r)` is in Eval(Pi, B + {principal(p), action(a), resource(r)} + ctx(c)). For a Space S: E_k = Eval(Pi, B + {principal(x) for x in P} + {action(y) for y in A} + {resource(z) for z in R} + ctx(C_k)).

Precondition (what must hold for the result to mean anything):
- Pre1 (syntax, checked): ctx live; P, A, R non-empty sets of non-empty strings (query.go:104, tarski `With`); every bag conforms to K (query.go:76).
- Pre2 (semantic, NOT checked, the whole safety claim): for every (p,a,r) in P x A x R and every k, `allow(p,a,r)` in E_k iff `Decide(p,a,r,C_k).Allow`. Card item 3 offers a syntactic condition ("head carries the request variable; prefix-free or separator-terminated") as the sufficient reason. Section 3 shows it is neither necessary nor sufficient.
- Pre3 (resource): each evaluation fits `maxDecisionSteps` / `maxDecisionBytes` (checked at run time; failure is `TooLarge`, fail-closed).

Postcondition on `err == nil`:
- Q1 `Allows` = { (p,a,r,k) in P x A x R x [0, max(1,|C|)) : Decide(p,a,r,C_k).Allow }, no duplicates, sorted (Bag, P, A, R).
- Q2 every `Allow.Proof` is a derivation of exactly `allow(p,a,r)`.
- Q3 `Steps`, `Bytes` = max over evaluations; `RuleSets` = private copy.
- Q4 absence of (p,a,r,k) means denied only if (p,a,r) was in S and k was a bag of S.
Postcondition on `err != nil`: `Allows == nil`, `Steps == Bytes == 0`, `RuleSets` filled.

Invariants: `p.base` and `p.plan` are never mutated (tarski `With` returns a new Base; `maps.Clone` of every bag, query.go:67-70); `out.Allows` only ever receives a whole bag's list (query.go:86), so the error paths cannot leak a partial bag.

What I could argue (not just test):
- Q1 on error: every `return` after `zero := ...` (query.go:57,61,65,74,77,81) returns `zero`; `out` is not reachable from any error return. Whole-or-absent holds by construction. Positive control.
- Q3: `Result.Cost()` is a stored counter and "does not depend on Options.Proofs" (tarski eval.go:36), so reading it after `Proof` calls (query.go:146-149) is not a defect, unlike the doc worry in Decide's comment.
- `base.With` of request facts when standing facts share a relation: `Load` refuses any standing fact in principal/action/resource/context (policy.go:173-178), so those four relations are empty in `p.base`; tarski `With` rebuilds a touched relation as prior + added (base.go:43-60), so Query's two-stage `With` (set facts, then bag facts, query.go:63,120) is an additive union of disjoint relations. No defect. The invariant lives in `Load` only and nothing in query.go records that it is relied on.
- Aggregates are refused at Load (policy.go:195-200), so no `count`-over-a-set rule can exist. This removes the worst class of set-sensitive rule. Positive control.

What I could not argue, only test: Pre2 => Q1 (section 3).

## 2. Leveson: the unsafe state and its controls

Unsafe state H: Query reports a triple as allowed, or leaves it absent, where `Decide` would decide otherwise, and a property check or access review passes on it.

Control structure as built:
1. Rule author -> rule text. Control: none in the rule files. rbac.dl:1-3 and hrchat.dl:8 still teach the Decide model ("every rule is seeded by the request's principal ... one decision derives one requester's world"; hrchat.dl:47 "computed for the requester alone"). The contract exists only in Query's godoc (query.go:49-53). README / DESIGN / CHANGELOG edits are not done (card stopped at acceptance 12).
2. Load -> plan. Control: refuses aggregates and standing request facts. Nothing checks keying (card defers to a tarski card).
3. Query -> Matrix. Control: membership filter, zero-on-error. Feedback unused: `Allow.Proof` is computed for every allow and never inspected.
4. Matrix -> consumer (007, 008). Control: none; the Matrix does not say what Space or what bags it answers (F4).
5. Tests. Controls: `TestQuery_Agreement` (rbac only), `TestQuery_HRChat*` (section 3).

Hazard H is reachable with every component meeting its local spec: the rule set is valid Datalog, Load accepts it, Decide agrees with its own oracle, Query computes the correct fixpoint of the program it was given. The program just means something different over a set. That is the component-interaction accident.

## 3. Findings

### F1 🔴 The keying contract is the wrong precondition, and the shipped suite cannot see the difference

Card item 3 / query.go:49-52 say: a derived relation that reads a request fact "and does not carry that variable in its head" leaks; patterns joined by `prefix/2` must end in the separator or names be prefix-free. I built three rule sets that satisfy that text and still diverge, plus one the text does cover. All four are reproducible.

(a) Unjoined use of a keyed relation, head carries P. Mutation M3 of hrchat/2, hrchat.dl:58:
```
permit(P, A, S) :- principal(P), action(A), resource(S), direct(_, S), manager_may(A), cleared(P, A).
```
(`direct(P, S)` replaced by `direct(_, S)`). Under `Decide` the two are equal (only one principal is loaded, so `direct` only holds for it). Under `Query` anyone in the Space gets the manager's permission on S. I ran `go test ./examples/hrchat/hrpolicy` on M3: `ok` (the full shipped suite, Query tests included, passes). A probe Space {manager E0004, unrelated E0001} x all 27 actions x {E0005, E0004's report}: 9 divergences, e.g. `emp/E0001 approve.leave emp/E0005 Query=true Decide=false`. Fail-open. The head-carries-variable test passes on this rule; the defect is the body discarding the key.

(b) Prefix leak that is not a rule-set property. Rule `permit(P, A, S) :- principal(P), action(A), resource(S), owner(P, S).` with `owner(bo, "doc/1")`, Space resources {"doc/1","doc/10"}. Decide: doc/1 allow, doc/10 deny. Query: both allowed (`bo read doc/10`), because `permit(bo,read,"doc/1")` is a pattern and rbac.dl:19 runs `prefix(Pat, R)` against every resource in the Space. Every head carries P, A, S. The contract puts the obligation on "the resource names" which is the caller's Space at call time, not the rule set. hrchat/2's four `permit` rules (hrchat.dl:57-60) are this shape. It is safe today only because of `fmt.Sprintf("E%04d", n)` (examples/hrchat/org/org.go:108): at 10,000 employees `emp/E1000` is a prefix of `emp/E10000`. The org has about 1,000, so latent, not live.

(c) Negation inverts the failure direction. `blocked(A) :- principal(P), action(A), banned(P).` + `allow(P,A,R) :- principal(P), action(A), resource(R), not blocked(A).` with banned(eve), Space {eve, bo}: Decide says eve deny, bo allow; Query returns 0 allows. Here the leak removes an allow. The card's only worked example (met/1) leaks toward allow; a leak under `not` leaks toward absence, which a "no hole" property check cannot see.

(d) The case the text does cover (old met/1): I restored hrchat/1's `met/1`, `unmet/1`, `cleared/1` in a copy (acceptance 11's mutation, which the card says was not run). Result: `TestQuery_HRChatPrincipalKey` fails with `emp/E0950 approve.salary emp/E0002 requester=emp/E0950: Query=true Decide=false` (a self-approval, as the card predicted). `TestQuery_HRChatAgreement` PASSES under the same leak. Record that tuple on the card (acceptance 11 asks for it).

Why the suite is blind to (a): `TestQuery_HRChatAgreement` (hrpolicy/query_test.go:28) runs 30 single-principal Spaces (query_space_test.go:114-124 builds `Principals: []string{principal}`). A one-member principal axis cannot expose principal-axis keying, whatever the rules say. `TestQuery_HRChatPrincipalKey` (query_test.go:75-105) is the only multi-principal check: one HR partner and one subject per department, 3 actions, 2 resources, one bag. Mutations M1 and M2 (`up(P, M)` -> `up(_, M)`, hrchat.dl:72) die there; M3 does not.

Severity: 🔴 because the Query doc promises "Under this contract Query agrees with Decide", the Matrix is the input to a security property check, and the stated contract is not the true condition. The fix is in the specification before it is in code: state the invariant as "every positive use of a request-seeded relation is joined to the request variable of the same axis, and any pattern derived from a request fact is compared only with that request's own resource", or drop the claim.

### F2 🟠 The evidence to detect the fail-open half of F1 is already in the Result and is not used

Under Decide only one principal, one action and one resource exist, so a proof of `allow(p,a,r)` can cite only `principal(p)`, `action(a)`, `resource(r)` as request leaves. In a Matrix, a proof whose body cites `principal(q)`, `action(b)` or `resource(z)` for a different value is a derivation Decide could not have made. Prototype (test-side, 15 lines walking `Proof.Body`):
- hrchat/2, first 8 single-principal property Spaces: 23,760 allows, 0 flagged (no false positive).
- M3 probe Space: 20 allows, 9 flagged, exactly the 9 divergences, e.g. `emp/E0001 approve.leave emp/E0005 cites principal(emp/E0004)`.
- (b) prefix case: `doc/10` flagged `cites resource(doc/1)`; `doc/1` not; rbac fixture Space: 10 allows, 0 flagged.
Limits: it sees spurious allows only (not (c)); a tuple with both a legitimate and a leaky derivation may be flagged though Decide agrees, so a flag means "run Decide on this tuple", not "wrong". That is a cheap adjudication: cost is proportional to flagged tuples only.
Recommendation: Query returns `ErrDisagrees`-style error (whole-or-absent) or a per-Allow `Foreign bool` when any proof cites a foreign request fact, and the card's tarski static check becomes the second layer. For (c) the control is a Decide sample of the absent triples (or the static check); nothing at run time sees it for free.

### F3 🟠 Correctness claimed by test where an argument is owed

- `TestQuery_Agreement` (query_test.go:48) covers rbac/1 over the fixture: no context, no bag, no deny, no negation under sets, no overlapping resource names. It passes for any rule set whose derived relations happen to keep the contract; it is the fixture the card says "happens to keep it". There is no negative control anywhere in `policy/`: no test shows a leaking rule set being disagreed with. If the keying rule were removed from the docs tomorrow nothing here would notice.
- Also query_test.go:65: the failure message prints `!d.Allow` as the Query value regardless of which side is wrong (cosmetic, but the message is the only diagnostic).
- Acceptance 11 and 13 mutations were not run by the builder (card Result items 11, 13). I ran three: drop final sort (S1), drop the pre-bag `ctx.Err()` (S2, query.go:73), drop the post-evaluate `ctx.Err()` (S3, query.go:134). All three SURVIVE `go test ./policy`. S1: `Result.Relation` already returns sorted tuples, so the sort is dead code that no test pins (card flags this). S2 and S3 are redundant with each other for non-final bags, and `TestQuery_Cancel` cancels in bag index 1 of 3 so it never separates them; neither is pinned for the last bag.
- `TestQuery_HRChatAgreement` logs "2000 seeded requests agree across 30 Query calls" unconditionally (query_test.go:64); true but it is the agreement of a blind Space shape (F1).
- Suggested control, general: a differential test over multi-member Spaces on every axis (random subsets of principals x actions x resources x bags, compare against the `Decide` loop), run in each consumer's CI. 007's own shape (one principal per call, card "How card 007 uses Query") is exactly the shape that cannot exercise principal-axis keying, so 007's agreement test will pass on a leaking rule set. Say so in the card.

### F4 🟠 The Matrix does not carry the question it answers; absence is undecidable from the Matrix alone

`Matrix` has `Allows`, `RuleSets`, `Steps`, `Bytes` (query.go:35-40). Item 2 makes absence meaningful only over the Space, and `Allow.Bag` is meaningful only against the caller's own `Contexts` order. Consequences for the declared consumers:
- 007: "A hole is a tuple in the policy's Matrix that is not in the policy-plus-property Matrix; both sorted, one merge." If the property-side Space is narrower or ordered differently, holes are missed (policy Space narrower: a hole outside it is never asked) or bags misalign (bag 3 means a different context on each side). Nothing in either Matrix reveals this.
- A caller that persists a Matrix (the card's own JSON/golden) loses the Space.
Recommendation: echo a canonical form (sorted deduplicated sets + each bag's sorted key/values, or a digest of it) in the Matrix, and have any future `Minus` refuse unequal Spaces. This is Leveson's process-model point: the consumer is a controller whose model of the process (what was asked) is not in the feedback it receives.
Related: a non-empty Space that matches nothing (typo in a resource name) returns `Allows == nil, err == nil`, indistinguishable from "everything denied". The empty-set refusal (item 4) covers only the syntactically empty set; counts of Space members that appear in any allow (or none) would close the near-miss. Minor.

### F5 🟠 The empty-set refusal is not applied to `Contexts`, and Bag 0 is ambiguous

query.go:67: `make([]map..., max(1, len(s.Contexts)))`. A builder bug that yields zero bags (a loop over a filtered list) becomes one evaluation with no context. The card's reason for refusing empty P/A/R ("an empty answer passes a property check") applies equally: for fail-closed rules the result is fewer allows (check passes with the intended bags unchecked); for rules that read a key's absence through negation it is more allows (Config doc, policy.go:112-115, says such rules hold when the caller omits the key). `Allow.Bag == 0` then means either "first bag" or "no bag". Options: refuse `Contexts == nil` unless an explicit `NoContext`/one nil element is given, or document the asymmetry next to the empty-set refusal. Decision for the DA: which.

### F6 🟠 `TooLarge` remedy hides F1, and the error carries too little to act on

- query.go:130: "split the Space". For a rule set that violates keying, splitting by principal makes Query agree with Decide on the principal axis. The remedy for the failure mode converts a leaking set into one that looks fine. The hrchat measurement already moves 007 to narrow Spaces (card "Wide Space": 55.2 % of steps for 30 principals at once), so production shape drifts toward the shape that hides leaks (F1, F3).
- A `TooLarge` can be caused by one bag (the test `wide` rule is gated on `context("wide", true)`, query_test.go:190); splitting principals/actions/resources does not help if the single bag alone is over. The message names `bag %d` (query.go:81) but not which set sizes were in play; the remedy text does not say "or split the bags".
- Error class: policy.go:61-62 defines ErrRequest = 4xx, everything else 5xx. A `TooLarge` from Query is nearly always caller-induced (Space size), but is wrapped like Decide's (not ErrRequest). A server mapping errors to status will report a broken engine.

### F7 🟠 The deprecation notice is not a deprecation notice

policy.go:259: `// Deprecated: use Query; removed in v0.7.0.` is appended directly under the previous paragraph with no blank `//` line. Go tooling (pkg.go.dev, gopls, staticcheck SA1019) recognises `Deprecated:` only at the start of its own paragraph. Acceptance 14 ("`Permitted` carries the `Deprecated` line") is satisfied by grep and not by the tool. Insert `//` before it.

### F8 🟡 Rule-file prose still teaches the model the contract forbids

rbac.dl:1-3 ("a decision derives only this principal's world, however many other principals are loaded"), rbac.dl:10-11, hrchat.dl:8 and :47. Under Query these are false as stated. A rule author never opens query.go. The contract belongs in rbac.dl's header and DESIGN.md (not yet amended), with the three forms in F1(a)(b)(c), because that is where `met/1` gets written. hrchat.dl:33's updated comment header is the one place already touched.

### F9 🟡 Positional coupling to `Request.facts`

query.go:120: `(Request{Context: bag}).facts()[3:]` assumes the first three facts are the request axes. A reordering of `facts()` (policy/facts.go:157-172) silently injects wrong relations with no test failing except by accident. Extract `contextFacts(c)` and have `facts()` call it; Decide and Query then share one builder, which is also the better argument that bag handling cannot drift.

### F10 🟡 Q2 rests on an ignored return value

query.go:146: `proof, _ := res.Proof("allow", tuple)`. tarski returns `ok == false` when proofs were off or the tuple is absent. Today neither can happen, so Q2 holds, but the guard is a comment in tarski, not in the body; a future Options change would emit `Allow{Proof: Proof{}}` with a nil error. Make `!ok` an error (whole-or-absent). `TestQuery_Agreement` checks only `Proof.Relation` and tuple (query_test.go:55), not that the proof has a rule or body.

### F11 🟡 Matrix size is unbounded in a library call

Measured on hrchat Space 0 (192 bags): 2,892 allows retained 22.7 MiB (about 7.8 KiB per Allow; spaces 5 and 12 identical). `MaxBytes` bounds each evaluation, not the sum. The wide Space in the card has 14,152 allows, so about 110 MB by the same ratio (extrapolated, not measured). The card's answer (caller bounds `len(Contexts)`) is a documented control with no feedback to the caller; `Matrix.Bytes` reports the largest single evaluation and so understates retained size by the bag count.

### F12 🟡 Validation after evaluation

query.go:72-82 checks bag k's schema inside the loop, after bags 0..k-1 were evaluated. A malformed bag at index 5,000 costs 5,000 evaluations before the ErrRequest; a `TooLarge` in bag 1 masks a schema error in bag 4, so the caller gets "split the Space" (F6) for a request that was malformed. Validate all bags before the first evaluation (a loop over a map-key check, microseconds). Order of precedence then matches Decide (validate, then evaluate).

### F13 🟡 Doc drift on the cap

`Matrix.Allows` doc says "sorted ... bytewise" (query.go:36); the comparison is field-wise (`slices.Compare` of []string{P,A,R}), equal to the joined-key order the test builds (query_test.go:182) unless a field contains NUL. Not wrong for the data seen; say "field by field".

## 4. Specific answers to the brief

- Allow outside the Space passing `queryMember`: not found. `queryMember` (query.go:153-160) requires `KindString` and set membership for all three columns; `TestQuery_OnlySpace` covers number/bool columns.
- A proof missing: not reachable today; unguarded (F10).
- Steps/Bytes wrong: not found (argued above).
- Error returns partial Matrix: not found; all error returns are `zero`. Cancel mid-final-bag after Evaluate returns the whole Matrix, which is correct.
- `base.With` with shared relations: argued safe, relies on `Load` alone (section 1).
- Test of agreement covers the keying contract?: no (F1, F3). It covers the fixtures that keep it.
- rbac/1's `prefix(Pat, R)` under multiple resources: rbac's own patterns come from standing `grants`, so under multiple resources it is per-R and agrees (fixture Space has `timeline/x` and `timeline/secret`, neither a prefix of another; `docs` vs `docs/y` would not matter because grants end in "/"). The hazard begins when a rule derives the pattern from a resource, F1(b), hrchat/2 does.
- hrchat/2 remaining rules under sets: checked line by line, hrchat.dl:28-72. `needs`, `attested`, `channel_forbids`, entitlements and holds-everyone are keyed by A, or standing, or by P. `attested(A)` is context-only (bags are per evaluation, not a set), safe. `direct`/`up`/`permit`/`deny` carry P and joined correctly in the shipped text. No live defect in hrchat/2; the exposure is that M3-style edits would pass the suite (F1a, F3).
- A consumer who writes met/1 after reading the docs: they read rbac.dl and hrchat.dl, which still say the Decide model (F8); the contract is in Query's godoc and nothing runs on their rule set at Load (F2 gives a run-time check).
- Empty-set refusal: sound for P/A/R, inconsistent for Contexts (F5).
- TooLarge mid-bag: fail-closed and whole-or-absent, correct; remedy and error class are weak (F6, F12).

## 5. Reproducers

Everything under `/tmp/lum-hl/`: `sl/` (branch copy), `m1/` (old met/unmet/cleared), `M2/`, `M3/` (wildcards; `M3/examples/hrchat/hrpolicy/probe_test.go` holds the divergence probe and the proof-citation probe), `S1..S3` (query.go mutations). `GOCACHE=/private/tmp/lum-hl-gocache go test ./examples/hrchat/hrpolicy -count=1` in `m1` / `M2` / `M3` gives: fail (PrincipalKey) / fail (PrincipalKey) / ok. Not verified by me: the full 388,800-decision measurement, golden bytes, Fable review, and the CHANGELOG/README/DESIGN changes (not in the diff).

## 6. Convergence and tension

Convergence. Both lenses land on one locus: Pre2, the unchecked agreement-with-Decide precondition. Hoare: the precondition is stated in prose, is not the true condition (F1 a-c), and is "established" by tests whose Spaces cannot reach it (F3). Leveson: the only control is documentation read at the wrong time by the wrong person (F8), the feedback that could detect the unsafe state (proofs) is computed and discarded (F2), and the consumer's model of what the Matrix covers is not in the Matrix (F4). Whole-or-absent on error is genuinely sound and both lenses credit it; the accident that remains needs no failed component, only a valid rule set that means something else over a set.

Tension. Hoare wants Pre2 proved (static analysis of the compiled plan, a tarski card) and holds that a run-time check is a patch over a missing proof; Leveson will not wait for the proof and wants a detector of the hazardous state now, accepting that it is partial (spurious allows only; spurious absences stay uncaught) and that a flag must be adjudicated by Decide. A second tension is inside the card's own results: performance pushes consumers toward single-principal Spaces (and TooLarge's remedy is "split"), which are exactly the Spaces on which Query cannot be wrong on that axis, while the feature's value, a whole Matrix over many principals, is exactly where it can. The safe-looking usage hides the hazard; the useful usage carries it.
