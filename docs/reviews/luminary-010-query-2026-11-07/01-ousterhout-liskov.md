# Luminary 01: Ousterhout x Liskov (surface x substitution)

Artifact: `git diff master..feat/010-query` at 635bcf5 (Query, Space/Allow/Matrix, Permitted deprecated, hrchat/2, tests).
Read: `policy/query.go`, `policy/policy.go`, `policy/compare.go`, `policy/facts.go`, `policy/rbac.dl`, `docs/DESIGN.md` Decided 5, the card, the hrchat query tests.
Status of the branch: card STOPPED at acceptance 12 (README, DESIGN, CHANGELOG untouched; mutation and review items 11/13/15 unrun).

Evidence I ran (throwaway copy at /private/tmp/lum1/sl, outside the tree; the Shearline tree is unchanged apart from this file; the copy was not deleted because the delete was denied):

- E1. Consumer rule `permit(P, A, S) :- principal(P), action(A), resource(S), owner(P, S).` over RBAC(), standing `owner("bo","u1")`, Space `{bo} x {read} x {u1, u10}`. Query allows `bo read u1` AND `bo read u10`. `Decide(bo, read, u10)` is false. Output: `Query allows bo read u10` / `Decide u10 -> false`.
- E2. hrchat.dl restored to master's hrchat/1 (met/1, unmet/1, cleared/1). `TestQuery_HRChatAgreement` (the 2,000-sample agreement test) PASSES. `TestQuery_HRChatPrincipalKey` FAILS: `emp/E0950 approve.salary emp/E0002 requester=emp/E0950: Query=true Decide=false`.
- E3. `go doc ./policy Policy.Permitted` prints "...number in it. Deprecated: use Query; removed in v0.7.0." as the tail of the previous paragraph.
- E4. Proofs on vs off over 8 hrchat property Spaces (23,760 allows): 2.53 s / 2.77 s on, 2.14 s / 2.15 s off. About 15-22 % of Query time. Not the dominant cost.
- E5. Bag 2 of 3 invalid: error is `policy: query: bag 2: policy: malformed request: tarski: invalid program: the request carries 1 context keys, but Config.Context declares 0`, raised after bags 0 and 1 were evaluated (query.go:72-78).

---

## Ousterhout's eye (surface)

### Is Query deep?

Medium. The signature is one method and three structs, and behind it sits a lot: set-valued evaluation, membership filtering, proof extraction, sorting, copying, per-bag bounds. That part is good, and Space is plain data that is the request pluralised, not a relocation.

But the interface a caller must hold in their head is not the 12 fields. It is the doc comment at query.go:42-53, which carries three obligations the type does not:

1. the keying contract on rule sets (query.go:49-52),
2. the absence rule ("denied only if it was in the Space", query.go:45),
3. the bounds rule ("On TooLarge, split the Space", query.go:53).

A module whose interface cost is mostly prose obligations on other people's artifacts is shallower than its line count says. The 160 lines of implementation do not hide the hard part; the hard part (is this rule set safe to ask set-wise) is handed to the caller.

### 🔴 O1. The keying contract leaks the engine's evaluation model into every rule author, and clause 2 is a property of the caller's Space, not of the rules

query.go:49-52: "A pattern joined to resource(R) by prefix/2 ends with the separator, or the resource names are prefix-free." Read that as an interface sentence: it says the correctness of `Space.Resources` depends on what the rule sets do with resource names. E1 shows it with a rule that obeys clause 1 (every head carries P, A, S) and still breaks: `u1` is a pattern by rbac.dl:19 (`allow(P, A, R) :- permit(P, A, Pat), resource(R), prefix(Pat, R)`), and `u10` is in the Space.

The caller of Query chose `u10`. The rule author wrote a natural rule (hrchat.dl:62-65 is the same shape). Neither alone broke anything. The complexity is a cross-product between two parties, which is exactly what Ousterhout calls a dependency that is not visible at the interface. Nothing in `Space`, `Matrix` or the error surface tells either party. hrchat is safe only because `org.go:24` makes IDs fixed-width, which is a fact about hrchat's data that Shearline cannot see.

The failure direction is an extra allow in an authorisation library. Error-defining-out would be a runtime tripwire rather than a paragraph. Options, cheapest first:

- Name the cross-resource hazard in the doc with the u1/u10 example, and put the example in `Example_` so it is executable.
- A deterministic sampled self-check inside Query: for k Allows and k absent triples, call the Decide path and return an error on disagreement. This hides the contract behind the interface instead of exporting it. It costs k evaluations, not 388,800.
- Reject, at `Space` build, a Resources set in which one name is a strict prefix of another, with an opt-out. Too blunt for hierarchical names like `orders/12` and `orders/12/items`; I list it only to say why I would not.

### 🟠 O2. "split the Space" relocates a Query-internal problem to every caller

query.go:128-130 wraps `*TooLarge` with `; split the Space`. The remedy is real (card measurement: wide Space 55.2 % of the step bound, narrow under 0.5 %), but:

- On failure `Steps` and `Bytes` are zero (query.go:55 `zero`), so the caller has no number to size a split by. `Bounds()` gives the ceiling but there is no consumption figure on the error path.
- Cards 007 and 008 will each write the same splitter, over `Principals`, which is the one axis along which a split is always sound under the keying contract.
- The deep-module move is for Query to bisect `Principals` itself on TooLarge and merge. Interface unchanged, error defined out for the common case, `Steps/Bytes` stay "largest of any one evaluation".

The tension with Liskov is in the convergence section below.

It also duplicates `evalError` (policy.go:414-420) instead of reusing it, and the two now give different remedies for the same `*TooLarge`: Decide says "narrow the grants reachable from this principal", Query says "split the Space". `maxDecisionBytes`'s own comment (policy.go:14-17) says a bound hit is "usually wrong, not large: narrow it first". Query's remedy text drops that guidance, which is the more important half when the cause is a runaway rule (the TestQuery_TooLarge `wide` rule is exactly that).

### 🟠 O3. Space is product-shaped, and the first real consumer pays a 4x tax to fit it

`requestAt` (query_space_test.go:126-133): `bag = 32 + resource*16 + grid`, and `propertyBags` (query_space_test.go:96-110) appends one requester per candidate resource. Requester-equals-subject depends on the resource, but Contexts is resource-independent, so card 007 builds 192 bags and Query answers 1,555,200 triples to decide 388,800 questions (card Result, "Equal work"). That is the reason acceptance 12 failed on the card's own wording and was later restated. The interface shape caused the measurement miss; this is the clearest sign in the diff that the abstraction is fitted to the sets question and not to the property-check question it was built for.

Not asking for a change in this release. It is the decision for the DA: either accept the product (and say so in the doc, with the 007 example as the cost warning), or open a card for a `[]struct{Context; Resources}` variant. The doc comment currently suggests Space fits any question.

### 🟠 O4. Matrix is not self-describing; the index leaks the caller's slice

Two places where the output cannot be read without the input:

- `Allow.Bag` is an index into `Space.Contexts` (query.go:29). The code already holds an immutable snapshot (`bags[i] = maps.Clone(bag)`, query.go:67-70), then throws it away and hands back an index into the caller's mutable slice. `TestQuery_ContextSnapshot` (query_test.go:237-251) is precisely the scenario where `s.Contexts[1]` changes during the call: after it returns, `Allow.Bag == 1` points at a bag the allow did not hold under. The test pins that Query is correct while its result is not interpretable.
- Absence has meaning only against the Space (query.go:45). A Matrix persisted or hashed by a consumer (the card, item 8, says the consumer hashes the receipt) records neither the sets nor the bags, so absent-because-denied and absent-because-unasked are indistinguishable.

House style disagrees with this: `Diff.Request` (compare.go:13) carries the request it decided, and compare.go:54-56 clones the context so the record is "the one that was decided". Matrix should carry the canonical Space (sorted, deduped sets plus the snapshotted bags) and `Bag` stays an index into that. Cost is one clone already made.

### 🟠 O5. Validation of bags is lazy; the error surface carries engine words

- query.go:72-78 validates bag k only when the loop reaches k. E5: bag 2 invalid after bags 0 and 1 were evaluated in full. Decide validates before it evaluates (policy.go:222-224). All bags can be checked, with the clone, before the first evaluation: same whole-or-absent result, same error, earlier. Whole-or-absent is honoured either way; this is a latency and wasted-work issue, and it makes the cheap error expensive.
- Error text, E5: `policy: query: ... policy: malformed request: tarski: invalid program: principal set is empty`. A bad Space is reported as an "invalid program" and the prefix `policy:` appears twice. The house already does this (`checkRequestContext`, policy.go:374), and `ErrRequest`'s doc blesses it ("The underlying ErrInvalid stays reachable"). But Query adds new errors on the same pattern and `TestQuery_Validation` (query_test.go:119-142) pins `errors.Is(err, tarski.ErrInvalid)` for an empty set, so the leak is now two sites deep and under test. Cheap to stop making it worse: a Query-level empty-set error that wraps `ErrRequest` only.
- Both empty-set sites report the first empty axis only. A Space with two empty axes sends the caller around twice. Minor.

### 🟡 O6. Smaller surface notes

- query.go:120 `(Request{Context: bag}).facts()[3:]` depends on `Request.facts` (facts.go:157-172) emitting exactly three axis facts first; the comment on line 119 is the only guard. Add `contextFacts(c)` in facts.go and have both call it. Dependency by position.
- `Matrix.Allows` is a sorted flat slice with no lookup. Three call sites already built a key map (`got` in query_test.go:50-56, `matrixKeys` in hrchat query_test.go:17-23, the card's planned "one merge" per consumer). A `func (m Matrix) Allowed(p, a, r string, bag int) bool` over the sorted slice (binary search) pulls that complexity down once. The card scopes `Minus` out; lookup is smaller than `Minus`.
- `Proofs: true` always on, no knob. E4 says proofs cost 15-22 % of Query time. That is not enough to justify a knob; keep it off the surface. Say so in the doc so nobody adds one for a guess.
- `Matrix.Steps` is "the largest of any one evaluation" (query.go:38), `Decision.Steps` is the evaluation's steps. Same word, different aggregate. Right for comparing against `Bounds()`, wrong for anyone totting up cost. Rename to `MaxSteps`/`MaxBytes`, or add `Evaluations int`. Note that max also hides which bag was heaviest, and that is the bag a caller needs to split on.
- rbac.dl:1-3 still says "The bounds on an evaluation ... are per request, so the rules must be too ... a decision derives only this principal's world". Under Query one evaluation derives every principal's world. The comment is now false for Query and is the one place a rule author reads.
- `proof, _ := res.Proof("allow", tuple)` (query.go:146) drops the ok. Decide treats the same condition as an error (policy.go:244-246). A zero Proof in an Allow would be silent.

---

## Liskov's eye (substitution)

### What Query promises relative to Decide

Card item 1: (p, a, r) is in the Matrix under bag k exactly when `Decide(Request{p, a, r, bag k})` allows it, for a rule set that keeps the keying contract. As a behavioural subtype claim: Query's `Allows` is the family of Decide postconditions, evaluated at once.

Substitution test: can a program written against "loop of Decide" be rewritten as one Query without changing what it concludes? That holds only when the precondition holds, and Liskov's rule is that a subtype may not demand more of its callers. Query demands:

- a rule set that carries every request variable in every dependent head (query.go:49-52),
- a Space whose resource names are not prefix-related through any pattern-from-resource rule (E1),
- a Space small enough to fit the per-evaluation bounds, which Decide never asks of its caller (the caller of Decide never has to split anything).

Decide demanded none of this. So Query is not a drop-in; it is a stronger-precondition operation that is honest about two of three in the doc.

### 🔴 L1. The agreement postcondition fails open and the checks the diff ships cannot see it

E1: allow for `bo read u10` that Decide denies. E2: the main agreement test passes against the broken rule set.

`TestQuery_HRChatAgreement` (hrchat query_test.go:28-67) builds one Query per principal (`propertySpaces`, query_space_test.go:112-120: `Principals: []string{principal}`), so it can never exercise a cross-principal leak. The card says so in its own "Corrections and limits". Only `TestQuery_HRChatPrincipalKey` catches it, and that test breaks after the first subject per department (`break`, query_test.go:98-99, comment: "One subject per department suffices"), and covers one leak class (principal axis). Nothing in the tree tests the action axis or the resource axis with more than one member at a time except E1-style prefix collisions, which no test builds.

And `policy/query_test.go:TestQuery_Agreement` passes because RBAC's rules are all keyed. So the agreement tests agree with Decide precisely on the rule sets that keep the contract. It shows the contract is satisfiable, not that the failure is detected.

Fix, in order of cost:
- Add a generic test: a table of deliberately-violating rule sets (principal-unkeyed, action-unkeyed, resource-as-pattern) asserting Query disagrees with Decide, with a comment naming each as the documented hazard. That turns the contract into an executable specification (a regression guard if tarski ever changes semantics).
- Do the card's mutation (acceptance 11) in CI as a test that swaps in the old rule set, not as a manual step. I ran it by hand here; it catches it only through the ad hoc test.
- Make Query's doc say "fail-open". "Query does not check this contract" understates: a violation yields extra Allows, not an error.

### Whole-or-absent: do all paths honour it?

I traced them in query.go. They do.

| Path | Result |
|---|---|
| ctx cancelled at entry | `zero` (56-58) |
| empty set, empty string element | `zero` (59-66) |
| invalid bag k | `zero` (76-78) |
| TooLarge in bag k | `zero` (79-82) |
| ctx cancelled during bag k | engine error, or the check at 134, `zero` |
| ctx cancelled between bags | `zero` (73-75) |
| ctx cancelled during final sort | whole Matrix returned, fine |

One asymmetry: query.go:134 turns a ctx cancel that arrives after a completed evaluation into an error, where Decide (policy.go:229-249) returns its decision. Query is stricter than Decide; that is the safe side, and a caller who treats any error as deny is unaffected. Worth one sentence in the doc so nobody reads ctx behaviour as "same as Decide".

Empty Contexts: `max(1, len(s.Contexts))` (67): nil, `[]`, `[nil]` and `[{}]` all give one evaluation with Bag 0. Consistent. But `Allow.Bag` (query.go:29) is "0 when Contexts is empty", which is an index into nothing; `s.Contexts[a.Bag]` panics for a caller who iterates Allows and looks up the bag. Define it out: say Contexts nil is the same as `[]map{nil}` in the type doc, or use the carried-Space fix in O4 so the index always resolves.

Duplicate elements: set dedupe at query.go:109-112; `base.With` also dedupes. `TestQuery_DuplicatesAndCopies` pins it. Duplicate bags are kept (index preserved) and tested (query_test.go:99-103). Both are consistent with the doc.

Non-string allow columns: dropped by `queryMember` (query.go:153-160) via `Kind() == KindString`. Decide builds `want` from strings and asks `res.Proof("allow", want)` (policy.go:237-238), so it can only ever allow a string tuple. Equivalent. `TestQuery_OnlySpace` pins numbers and bools. Good.

Empty set: Decide's empty principal is ErrRequest (base.With rejects ""), so an empty set being ErrRequest is the consistent generalisation. Good, and the card's reason is sound (an empty answer passes a property check, a vacuous truth is the wrong default).

### 🟠 L2. Deprecating Permitted is not a substitution

`Permitted` answers an open-world question (which prefixes does P hold for A), context-free, deny-blind. `Query` answers a closed-world one (which of these named resources), per bag, deny-applying. Card item 10 argues the use folds because a caller "took prefixes and filtered its own list". That is true of `examples/logsvc/main.go:51` (it has `s.Endpoints`), and false for any caller that forwards patterns, shows "you hold grants on docs/", or has no finite candidate list. For those, Query is not a replacement; the capability is removed at v0.7.0 with no successor named in the doc.

Behavioural differences a migrating caller meets, none of them in the `Deprecated` line:
- deny now applies (a stronger postcondition on allows; for authorisation the safe direction, but a UI will show fewer rows),
- context now matters (Permitted reads none; its doc says builtins over context "never hold" there),
- Resources must be named.

Also E3: the notice is wrong as written (policy.go:259 is not separated by a blank `//` line), so `go doc` folds it into the previous paragraph and neither gopls nor staticcheck SA1019 treats `Permitted` as deprecated. The card's acceptance 14 ("carries the Deprecated line") passes literally and not in effect. Insert `//` above it. (Separately, logsvc pins shearline v0.2.0, so it cannot see this either way.)

DESIGN Decided 5 (docs/DESIGN.md:222-226) argued against exactly this: "the listing is the gate, so it cannot disagree with it" and "a second evaluation mode would be a thing to keep consistent". Query is that second mode, and the keying contract is the consistency burden that entry predicted. The amendment is listed in card item 13 but not made on the branch; until it is, the design doc and the code disagree about whether Shearline has a second evaluation mode.

### 🟠 L3. Callers that depend on behaviour Query never promised

Looking for it in the hrchat tests:

- `matrixKeys` (hrchat query_test.go:17-23) and the measurement test treat `Allow.Bag` as the index of a deduplicated bag list they built. That is promised.
- `TestQuery_HRChatMeasurement` (query_test.go:107-148) asserts `maxSteps > bounds.MaxSteps/4` using `m.Steps`. `Decision` documents Steps/Bytes as observation-only, "in no hash", "can change with any tarski version" (policy.go:94-97). Matrix has no such caveat, so the test (and card 003's trigger) read a number as a contract that the type never made one.
- `TestGolden_MatrixJSON` (query_test.go:253-264) pins `"Steps":35,"Bytes":118` in `matrix.golden.json`. A tarski upgrade that changes cost, not answer, breaks the golden, and the failure message says to list the change in CHANGELOG, contradicting the Decision rule that "a changelog does not list a cost change as an answer change". Either strip cost from the golden (marshal a view of Allows and RuleSets) or give Matrix the same "observation only" paragraph as Decision.
- The 2,000-sample test depends on Allow order only through sorting inside Query, promised and pinned.

### 🟡 L4. Query depends on Policy internals in a way Decide did not

`Policy.Query` reads `p.base`, `p.plan`, `p.ids`, `p.checkRequestContext` directly and duplicates the evaluate-and-cost block of Decide (policy.go:229-249). Two evaluation sites now have to stay consistent on bounds, proofs and ctx. One `p.evaluate(ctx, base)` used by Decide, Permitted (with its different Options), and queryBag would make "same bounds" (policy.go:25 "Decide and Permitted use the same bounds") a fact of the code and not of two copies. Also, Bounds' doc now needs Query added.

---

## Where the two converge

1. **The condition is in prose and should be in the type or the code.** Ousterhout sees the keying contract as interface cost exported to rule authors (O1); Liskov sees a postcondition whose precondition no caller can check and whose violation is an extra allow (L1). Both land on one change: move the condition into something executable. A table of violating rule sets as tests, a sampled Decide cross-check inside Query, and a doc that says fail-open.
2. **Matrix should carry what it was computed over.** O4 (leaky `Bag` index into a mutable slice; absence undefined without the Space) and the Liskov reading of "absent is denied only if in the Space" are the same defect. `Diff.Request` is the in-house precedent.
3. **Permitted's deprecation line is mechanically wrong and semantically a removal.** One paragraph of doc and one `//` fixes the first, the DESIGN amendment fixes the second.
4. **Cost numbers are being treated as contract** (L3 `Steps/Bytes` in golden and in the measurement gate, O6 naming). Both lenses say: say they are observation, or stop pinning them.
5. **Fail early, not late** (O5 lazy bag validation) is both a surface point (the cheap error is expensive) and a contract point (Decide validates before evaluating, Query should too).

## Where they pull apart

- **Auto-split on TooLarge.** Ousterhout (O2) wants Query to bisect `Principals` and merge: the error disappears for the common case and callers never write a splitter. Liskov says a `TooLarge` is information. Decide's own doc calls a bound hit "usually wrong, not large" (policy.go:14-17), and a Query that quietly splits will hide a runaway rule behind more evaluations, and weakens "each evaluation under the bounds" into "each of an unstated number of sub-evaluations". My call: split only when the failing Space is wide on `Principals` (len > 1) and surface the split count in the Matrix; never split a single-principal Space. That is the DA's decision, and it is the one that should go to the principal.
- **Empty set.** Ousterhout would define the error out: an empty set gives an empty Matrix, one less exception for every caller that builds Spaces dynamically (007's "about ten candidates" can be zero). Liskov keeps the error: a vacuous Matrix passes a property check, and the postcondition callers rely on ("absent means checked and denied") would be false. I side with Liskov here, because the cost is one `if len(...) == 0` in the caller and the benefit is that a hole-finder cannot pass with nothing checked. Record it in the doc as deliberate.
- **Surface growth.** Ousterhout resists new fields; Liskov wants Matrix to carry the Space and cost naming to change. The fields are cheap compared with the prose obligations they replace, so I take the larger Matrix.
- **`Steps` as a max.** Ousterhout accepts it as a simple single number that compares to `Bounds()`; Liskov wants the name to stop colliding with `Decision.Steps`. Rename (`MaxSteps`, `MaxBytes`) and the two agree.
- **Proof knob.** Ousterhout: no knob (E4: 15-22 %, not worth an interface). Liskov: no objection, provided the doc says Proofs are always present, because card 008 ignores them and a reader may assume they are optional.

## Findings by severity

| # | Sev | One line |
|---|---|---|
| O1 / L1 | 🔴 | Agreement with Decide is conditional on rule sets and on the caller's Space (E1, E2), fails open, and the shipped agreement tests are blind to it. |
| O2 | 🟠 | "split the Space" exports a splitter to every caller; no cost on the error path; drops "usually wrong, not large"; duplicates `evalError`. |
| O3 | 🟠 | Product-shaped Space cost card 007 a 4x work tax; decide: document or open a variant card. |
| O4 | 🟠 | Matrix is not self-describing: `Bag` is an index into a mutable slice already snapshotted; absence is undefined without the Space. |
| O5 | 🟠 | Bag validation lazy (E5); engine words in the error text; double `policy:` prefix. |
| L2 | 🟠 | `Permitted` deprecation is mechanically unrecognised (E3) and semantically a capability removal; DESIGN Decided 5 not amended. |
| L3 | 🟠 | Golden pins `Steps`/`Bytes`, which `Decision` calls observation-only; measurement gate reads them as contract. |
| O6 | 🟡 | `facts()[3:]` positional coupling; no `Allowed` lookup; `Steps` naming; stale rbac.dl comment; dropped `ok` at query.go:146. |
| L4 | 🟡 | Second evaluation site duplicates Decide's evaluate-and-cost block; extract one `p.evaluate`. |

Unverified: wall-time claims beyond E4; the O2 auto-split design (not prototyped); whether the sampled self-check in O1 catches the E1 shape at a small k (reasoned, not run).
