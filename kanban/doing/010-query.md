# 010 Query: what is permissible

Asked for by: Matt (2026-11-07), from the question Shearline could not answer: "who may do X",
"what may P do", the access matrix.
Owner: shearline-lead. Status: DOING. Release: v0.6.0, additive (one method, three types;
`Permitted` deprecated). Written in ASD-STE100. Drafted by Fable (read-only), built by Sol,
reviewed by Fable.

## Job story

When I want to know what a policy permits over the principals, actions and resources I know,
I want one call that returns every allowed triple with its proof, so a property check, a
multi-author diff or an access review reads a relation and not a loop of 388,800 decisions.

## Now

`Decide` injects one principal, one action, one resource and one context bag, and reads one
`allow` tuple. `Permitted` injects a principal and an action and reads `permit/3` prefixes:
no `deny`, no context, patterns and not resources. `docs/DESIGN.md` § Decided 5 says the
listing is a loop of `Decide`. hrchat card 007 plans that loop: 388,800 requests per property
per side, about a minute each side at 145 µs a decision.

## Why

Datalog is set-valued. The engine holds every fact of a relation at once and joins them in one
bounded evaluation. Principals, actions and resources are each a relation, so each can be a set
in one evaluation. Context is one bag per evaluation, because a rule reads `context(K, V)` as
the state of this request, so a list of bags is the one axis that multiplies. Resources have no
finite domain (opaque strings, prefix grants), so the caller names the candidates. The result
is the derived `allow` relation over the sets given, each tuple with its proof, under the same
bounds, with `TooLarge` an error and never a shorter list.

## Change

All new code is in `policy/query.go` and `policy/query_test.go`. `policy/policy.go` is 441
lines, past the 400-line guideline; it gains the `Deprecated` line on `Permitted` and nothing
else.

```go
// Space is the sets one Query ranges over. Principals, Actions and Resources are sets: order
// does not matter, a duplicate counts once, and an empty set is ErrRequest. Contexts is a
// list, not a set: Query evaluates once per bag, and an Allow names its bag by index. A nil
// or empty Contexts is one evaluation with no context. Query keeps none of the caller's
// slices or maps.
type Space struct {
	Principals []string
	Actions    []string
	Resources  []string
	Contexts   []map[string]tarski.Const
}

// Allow is one tuple of allow/3 that held, with its proof and the bag it held under.
type Allow struct {
	Principal, Action, Resource string
	Bag                         int // index into Space.Contexts; 0 when Contexts was empty
	Proof                       tarski.Proof
}

// Matrix is the derived allow relation over a Space: every allowed triple, under each bag.
// A Matrix is whole or absent: on any error Query returns a zero Matrix with RuleSets filled.
type Matrix struct {
	Allows   []Allow  // sorted by Bag, then Principal, Action, Resource, bytewise
	RuleSets []string // as Decision.RuleSets
	Steps    int      // the largest Steps of any one evaluation; the bounds are per evaluation
	Bytes    int      // the largest Bytes of any one evaluation
}

// Query holds every principal, action and resource of s as facts at the same time, evaluates
// once per bag under the decision bounds with proofs on, and returns the allow/3 tuples
// inside s.
func (p Policy) Query(ctx context.Context, s Space) (Matrix, error)
```

1. **Meaning.** A triple (p, a, r) is in the Matrix under bag k exactly when
   `Decide(Request{p, a, r, bag k})` allows it, for a rule set that keeps the keying contract
   (item 3). `deny/3` applies: Query reads `allow/3`, and rbac/1's allow rule ends in
   `not deny(P, A, R)`. This is the difference from `Permitted`.
2. **Absence.** The Matrix is over the standing facts plus the Space and nothing else. A
   triple absent from the Matrix is denied only if it was in the Space. A resource the caller
   did not name is not denied: it was not asked, and the policy may allow it. The doc comment
   says this in these words or near them.
3. **Keying contract.** Under Query every member of a set is a fact at once, and a rule reads
   them all. A derived relation that reads `principal(P)`, `action(A)` or `resource(R)` and
   does not carry that variable in its head holds for every member when it holds for one.
   Under `Decide` the set has one member, so the two agree; under Query they do not. The
   contract for a rule set: a derived relation that depends on a request fact carries that
   fact's variable in its head, and a pattern joined to `resource(R)` by `prefix/2` ends with
   the separator or the resource names are prefix-free. rbac/1 keeps it (every rule carries P,
   A or R; patterns are the consumer's prefixes). hrchat/1 breaks it once, at
   `hrchat.dl:38`: `met("four_eyes") :- principal(P), context("requester", Q), Q != P.` With two
   principals in a Space, `met("four_eyes")` holds for both when it holds for one, `unmet` and
   `cleared` follow, and a principal may approve the pay change they raised. hrchat's names
   are prefix-free (fixed-width IDs, `org.go:24`), so its `permit(P, A, S)` column is safe
   under `prefix/2`. The fix is item 11. Shearline does not check the contract statically in
   this release; acceptance 1 and 11 are the check, and the `Query` doc states the contract.
4. **Validation.** Each element passes Decide's `ErrRequest` rules: the schema's checks on
   `Base.With` (a non-empty string), and `checkRequestContext` on each bag. An empty
   `Principals`, `Actions` or `Resources` is `ErrRequest`: a question over nothing is
   malformed, not an empty answer, since an empty answer passes a property check. An error
   in bag k names k. A duplicate element counts once (`NewBase` says so).
5. **Only the Space.** A rule set may derive `allow` tuples outside the sets, as it may derive
   `permit` tuples for anyone. Query lists a tuple only when each column is a string in its
   set (`Kind() == KindString` and membership), as `Permitted` keeps the caller's tuples.
6. **Bounds and cost.** Each evaluation runs under `maxDecisionSteps` and `maxDecisionBytes`,
   with `Options.Proofs` on. A `*TooLarge` is wrapped as `evalError` wraps it, with the
   remedy "split the Space". `Matrix.Steps` and `Matrix.Bytes` are the largest of any one
   evaluation, since the bounds are per evaluation; they are zero when `err != nil`. No
   separate result cap: `MaxBytes` bounds `allow/3` in each evaluation, and the caller bounds
   the count of evaluations with `len(Contexts)`. A cancelled `ctx` is an error at entry and
   before each evaluation, and the engine checks it every 1024 steps. On any error the Matrix
   is zero. Compare returns the diffs found before a cancel; Query does not, because a partial
   Matrix reads as a smaller allow relation, and a property check over it passes with nothing
   checked.
7. **Proofs.** Every `Allow.Proof` is `Result.Proof("allow", tuple)`. There is no why-not:
   absence over a Space is not one question. The doc says: for "why not (p, a, r)", call
   `Decide` on that request and read `Why`.
8. **Determinism.** The set facts are sorted before they are held (`sortedFacts`), and a bag's
   keys are sorted as `Request.facts` sorts them, so a proof is a function of the Space's
   content and not of the order given. `Allows` is sorted by Bag, Principal, Action, Resource.
   `RuleSets` is the policy's own copy. Matrix carries no hash: as for `Decision`, the
   consumer hashes the receipt it keeps. `encoding/json` on a Matrix gives tarski's frozen
   Proof bytes; a golden test pins it (acceptance 9).
9. **Demand.** Query does not use `Plan.Demand`. A demand plan binds a position to one
   constant per evaluation, and Query has a set at every position of `allow/3`, so the
   adornment would be `fff`, which tarski says can cost more than full evaluation. rbac/1 and
   hrchat/1 seed every rule by `principal`, `action` or `resource` (the comment at the top of
   `rbac.dl`), which is demand by hand. A one-element set is not a special case. `Decide`
   does not use demand either (`policy.go:229`).
10. **Permitted.** `Permitted` reads `permit/3` patterns; Query reads `allow/3` over named
    resources; no thin call exists between them. The use folds: a caller that took prefixes
    and filtered its own list now passes the list as `Resources` and gets exact allows with
    `deny` applied. `Permitted` gains `// Deprecated: use Query; removed in v0.7.0.` and keeps
    its tests until then. `requireRelations` keeps `permit/3`: rbac/1's allow rule reads it.
11. **hrchat/2.** `hrchat.dl` keys the attestation chain by principal, and nothing else changes:
    ```
    met(P, "mfa")       :- principal(P), context("mfa", true).
    met(P, "confirmed") :- principal(P), context("confirmed", true).
    met(P, "velocity")  :- principal(P), context("high_reads", N), N < 25.
    met(P, "four_eyes") :- principal(P), context("requester", Q), Q != P.
    unmet(P, A)   :- principal(P), action(A), needs(A, Req), not met(P, Req).
    cleared(P, A) :- principal(P), attested(A), not unmet(P, A).
    ```
    The four `permit` rules read `cleared(P, A)`. `Rules()` names the set `hrchat/2`; the hash
    changes and the CHANGELOG lists it.
12. **Build note.** Build the Space base once, `p.base.With(set facts...)`, then one
    `With(bag facts...)` per bag: the request relations hold no standing tuples (`Load`
    refuses them), so each `With` rebuilds only the relations it touches.
13. **Docs.** README: the bullet "List what a principal holds" becomes "Ask what is
    permissible", four sentences: the call, `deny` applies, an unnamed resource is not denied,
    `Decide` for one request. `examples/hrchat/README.md:206` says the matrix is one Query per
    principal. `docs/DESIGN.md` § Package policy gains the three types and the signature, and
    § Decided 5 gets a dated amendment under it: Query answers the listing in one evaluation
    per bag and agrees with the loop. CHANGELOG entry for v0.6.0. Card 007 says Query, as below.

### How card 007 uses Query

One Query per principal, 30 calls per side: `Principals` the one principal, `Actions` all 27,
`Resources` that principal's about ten candidates, `Contexts` the 32 bags with no requester
or requester = self, plus 16 bags with requester = s for each candidate s (about 192 bags).
Requester = subject depends on the resource, so it is a bag per candidate; under that bag the
other nine resources answer "another person raised it", which is a correct answer and is
kept. That is at most 5,760 evaluations of 270 triples per side, in place of 388,800 decisions
per side. A hole is a tuple in the policy's Matrix that is not in the policy-plus-property
Matrix; both are sorted, so the difference is one merge. The hole carries the policy's proof,
and `Decide` on the property side gives the Why for the report. The default `go test` run
groups 007's seeded 2,000-request sample by principal into the same calls.

### How card 008 uses Query

One Query per author over one Space. The symmetric difference of two Matrices' triples
(Proof ignored) is one cell of the diff matrix. A tuple in the difference carries a proof on
the side that allowed it; `Decide` on the other side gives its Why.

## Acceptance

1. Agreement: over `helpers_test.go`'s fixture and rbac/1, Space = {ana, bo, zed} × {read,
   write, delete} × {timeline/x, docs/y, timeline/secret, other/z}, no context. The Matrix
   equals the set of requests `Decide` allows, triple for triple.
2. Deny applies: with `Deny("bo", "read", "timeline/secret")`, bo/read/timeline/secret is
   absent and bo/read/timeline/x is present. (`TestPermitted_IgnoresDeny` inverted.)
3. Bags: a consumer rule that reads `context("mfa", true)`, two bags (true, false). The tuple
   appears with `Bag == 0` only, and each `Allow.Bag` indexes the bag it held under.
4. Only the Space: a stray set deriving `allow("other", "read", "secret/")` adds nothing.
5. `ErrRequest`: an empty set; an element `""`, with the relation named; an undeclared key in
   bag 1, with "bag 1" in the message. A Space with "bo" twice gives the same Matrix as once.
6. Determinism: the same Space with every slice shuffled and every bag's keys inserted in
   another order gives byte-equal JSON, proofs included. `Allows` is in the stated order.
7. `TooLarge`: a rule `wide(P, A, R, Q) :- principal(P), action(A), resource(R), principal(Q).`
   over sets sized past `maxDecisionSteps`. The error wraps `*tarski.TooLarge`, `Allows` is
   nil, `Steps` and `Bytes` are zero.
8. Cancel: a context cancelled before the call is an error. A builtin whose `Holds` cancels
   the context during bag 2 of 3 gives `ctx.Err()` and a zero Matrix, not two bags of Allows.
9. Golden: `policy/testdata/matrix.golden.json`, the Matrix over the fixture plus one Deny and
   two bags. `TestGolden_MatrixJSON` fails on any byte change. No `-update` flag: a changed
   golden is edited on purpose and listed in the CHANGELOG.
10. hrchat/2 (item 11): the 39 scenarios and `compare_test.go` pass with the same outcomes.
11. hrchat agreement: over 007's sample, grouped into Query calls as above, every request is
    in the Matrix exactly when `Decide` allows it. Mutation: hrchat/1's old `met/1`, `unmet/1`
    and `cleared/1` put back; this test fails with a self-approval in the Matrix. Record the
    tuple here.
12. Measurement, org seed 1, in one test binary on one machine: 007's full space as the 30
    calls above, against the 388,800-decision loop. Record wall time per side, the largest
    `Steps` and `Bytes`, and the ratio. Bar: Query is at most one tenth of the loop's time,
    and no evaluation passes 25 % of either bound. Past 25 %, card 003's trigger fires:
    record the numbers there.
13. Mutations of `query.go`, each caught: drop the membership filter (4 fails); drop the sort
    (6 fails); return `Allows` found before an error (7 and 8 fail); `Bag` off by one (3
    fails).
14. `Permitted` carries the `Deprecated` line; `go vet ./...` is clean; the four documents in
    item 13 are changed; `policy.go` differs from v0.5.0 by the `Deprecated` line only.
15. Cross-vendor: Sol builds; Fable reviews, read-only. Each finding is reproduced, fixed and
    pinned by a test that fails on the old code, as in card 005's review.

## Out of scope

- Why-not over a Space (`WhyEmpty`, per-tuple absence). `Decide` answers it for one request.
- Demand evaluation (item 9). Re-check when a consumer asks with one principal and ten
  thousand resources.
- A static check of the keying contract. It is an analysis over the compiled plan, so it is a
  tarski card, after acceptance 1 or 11 has caught a leak the contract text did not prevent.
- A Matrix difference helper. 007 and 008 each write one merge over sorted `Allows`; when
  both exist, `Minus` is the next card.
- Making the sets, and a grid helper, as card 009 left them.
- Parallel bags inside Query. `Policy` is a value: a caller splits `Contexts` across
  goroutines and calls Query on each.
- `iter.Seq` results. A slice, as Compare; a caller bounds memory with `len(Contexts)`.
- Configurable bounds (card 003). Acceptance 12 feeds its trigger.

## Open questions

None for the builder. Two stop rules: if acceptance 11 finds a second leak in hrchat/1 beyond
`met/1`, stop and report it here, do not patch further. If acceptance 12 misses the bar,
record the numbers, leave the card open, and card 007 keeps its `Decide` loop for the full
space.

## Result

2026-11-07. Status: STOPPED at acceptance 12. The card stays open.
Card 007 keeps its `Decide` loop for the full space.
No implementation or migration changes follow the failed measurement.

### Measurement and stop finding

`SHEARLINE_QUERY_MEASURE=1 go test ./examples/hrchat/hrpolicy -run
'^TestQuery_HRChatMeasurement$' -count=1 -v -timeout=10m` runs both sides
in one test binary. The machine runs Go 1.27.3 on darwin/arm64.
The test uses org seed 1. It makes 30 Query calls and 5,760 evaluations.
The loop makes 388,800 decisions. Every loop decision agrees with its Matrix.
The test source fixes the principal selection and the ten candidates per principal.

| Run | Query wall time | Loop wall time | Query / loop |
|---|---:|---:|---:|
| Initial | 9.903451083 s | 38.631347792 s | 0.256358 |
| Isolated confirmation | 9.836787875 s | 35.491570500 s | 0.277158 |

The initial run overlaps the golden and package coverage checks at startup.
The confirmation run has no other verification command running.
Both runs report these per-evaluation maxima:

| Side | Largest Steps | Largest Bytes |
|---|---:|---:|
| Query | 18,421 | 12,595 |
| Decide loop | 170 | 621 |

The bounds are 4,194,304 steps and 1,048,576 bytes.
Query uses 0.4392% of the step bound and 1.2012% of the byte bound.
The 25% cost trigger does not fire. Card 003 needs no trigger entry.
The time ratio exceeds 0.1. Acceptance 12 fails. The stop rule applies.
No optimisation follows this finding.

### Evidence at the stop

1. `TestQuery_Agreement` passes over the specified fixture space.
2. `TestQuery_DenyApplies` passes.
3. `TestQuery_Bags` passes. It also checks repeated bag indices and maximum cost.
4. `TestQuery_OnlySpace` passes. It also excludes non-string columns.
5. `TestQuery_Validation` and `TestQuery_DuplicatesAndCopies` pass.
6. `TestQuery_Determinism` passes with equal JSON and ordered Allows.
7. `TestQuery_TooLarge` passes. A successful bag precedes the bounded bag.
8. `TestQuery_Cancel` passes at entry and during the second of three bags.
9. `TestGolden_MatrixJSON` passes against `policy/testdata/matrix.golden.json`.
10. The existing hrchat tests pass after the hrchat/2 key change.
    The rule-set identity assertion now expects hrchat/2.
11. `TestQuery_HRChatAgreement` passes over 2,000 seeded requests in 30 calls.
    `TestQuery_HRChatPrincipalKey` passes over multi-principal spaces.
    No second keying leak appears in these checks or in the full-space loop.
    The manual hrchat/1 mutation is not run before the stop.
    No observed mutation tuple is available.
12. The measurement above fails its time bar.
13. The hand-run query mutations are not run before the stop.
14. `Permitted` has the requested Deprecated line.
    `policy.go` differs from v0.5.0 by that line only.
    The migration documents are not changed before the stop.
15. Fable's read-only review does not run before the stop.

### Corrections and limits

Card 007 has no existing sample or executable space generator.
The hrchat test files supply a deterministic 2,000-request sample and a full generator.
The sample groups requests by principal as this card specifies.
A one-principal Query cannot expose the old cross-principal met/1 leak.
`TestQuery_HRChatPrincipalKey` adds the necessary multi-principal check.
The mutation and its observed tuple remain unverified at the stop.

New hrchat test code lives in `hrpolicy/query_test.go` and
`hrpolicy/query_space_test.go`. Those tests need the consumer's private context and
entitlement helpers. New production policy code stays in `policy/query.go`.
The Matrix golden has no update flag. Its initial bytes come from the fixture,
one Deny and two empty bags. The CHANGELOG entry remains unfinished at the stop.

The normal Go build cache is outside the writable sandbox.
Verification uses `GOCACHE=/private/tmp/shearline-go-cache`.
Lint uses a temporary `GOLANGCI_LINT_CACHE`.
The helper reports `Co-Authored-By: gpt-6.1-sol <noreply@openai.com>`.
Explicit `git add` fails with `Unable to create '.git/index.lock': Operation not permitted`.
The sandbox permits no approval escalation. No commit or push occurs.
The caller's pre-existing untracked paths remain unchanged.


### Verification of the stopped snapshot

| Command | Actual result |
|---|---|
| `gofmt -l .` | No output. Exit 0. |
| `go vet ./...` | No output. Exit 0. |
| `go test -race -shuffle=on -count=1 ./...` | All eight test packages pass. The hrchat command package has no tests. Exit 0. |
| `cd examples/logsvc && go test ./...` | `ok github.com/mattjoyce/shearline/examples/logsvc 0.711s`. Exit 0. |
| `golangci-lint run ./...` | `0 issues.` Exit 0. |
| `gosec ./...` | 18 files, 2,980 lines, 1 Nosec, 0 issues. Exit 0. |
| `go test -cover ./policy/` | `coverage: 94.7% of statements`. Exit 0. |

The full measurement test is opt-in through `SHEARLINE_QUERY_MEASURE=1`.
The normal test gate skips it. Its two explicit runs fail as recorded above.
The receipt's CHANGELOG entry, the README migrations, DESIGN amendment, card 007
migration, hand-run mutations and cross-vendor review remain unfinished under the stop rule.

One mutation needs review before a later build resumes.
`Result.Relation` already sorts string tuples. Query visits bags in index order.
Removing only the final Allows sort appears to preserve the required order.
This is a source-level observation. The manual mutation is not run at the stop.

### Two measurements after the stop (Splendid, 2026-11-07)

Both are throwaway tests over org seed 1, Go 1.27.3, darwin/arm64, not kept in the tree.

**Equal work.** Acceptance 12 compares unequal work. The card's bag scheme for 007 puts
all ten candidate resources under each of the 192 bags, so Query answers 1,555,200 triples
while the loop decides 388,800. The loop over the same 1,555,200 requests Query answered:

| Side | Wall time | Per triple |
|---|---:|---:|
| Query (30 calls, 5,760 evaluations) | 9.80 s | 6.3 µs |
| Decide loop (1,555,200 decisions) | 140.9 s | 90.6 µs |

Ratio 0.0695. Every answer agrees. The engine meets the 0.1 bar; the measurement as
written in acceptance 12 does not, because it credits the loop with a quarter of the work.

**Wide Space.** One Space of all 30 principals, 27 actions and the 62-resource union, under 4
no-requester bags: 200,880 triples, 14,152 allows, 1.43 s, Steps 2,317,129 (55.2 % of the
bound), Bytes 446,191 (42.6 %). The same work as 30 narrow calls: 0.27 s. Per triple the
cost is the same (7.1 µs against 8.4 µs); the wide shape is not faster, and it passes the
25 % trigger. Card 003 has the numbers. Narrow Spaces are the right shape, and "split the
Space" is a real remedy.

Decision pending: restate acceptance 12 as equal work (met at 0.0695), then finish items 11,
13, 14 and 15; or keep the bar as written and leave the card open.
