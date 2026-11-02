# 004 Compare two policies over a set of requests

Asked for by: Matt, after a comparison with Cedar (2026-11-02).
Owner: shearline-lead. Status: BACKLOG. Release: additive (new function and types).
Written in ASD-STE100.

## Job story

When I change a policy, or when I get one policy from more than one author, I want to see
each request where two policies give different decisions, so I can find the change or the
error before a person finds it in production.

## Why

Cedar has SymCC. It proves that two policy sets are equivalent, or that one never allows
more than the other, and it gives a counterexample when this is false. Shearline cannot do
proofs of this type. But `Policy` is a value and `Decide` is a pure function of (policy,
request), so Shearline can compare two policies over a set of requests that the consumer
gives. This is a small change, and it has three uses:

1. Policy change review. "This change only removes access" is true when no request is
   allowed by the new policy and denied by the old policy.
2. Policy tests. A deny property ("nobody reads high data of a person above them") is a
   comparison with a policy that has the deny rule twice. No diff means no request breaks
   it, over the requests that the test gives.
3. More than one author. Many models (or people) write `.dl` from one English policy.
   Compare each pair. Each diff is an ambiguity in the English or an error in one `.dl`.

Each diff has the two decisions, and each decision has its Proof or its Why. Thus a diff
tells why each policy decided as it did. A Cedar counterexample does not do this.

## Change

```go
// Diff is one request where a and b decide differently.
type Diff struct {
	Request Request
	A, B    Decision
	ErrA    error // nil when Decide on a succeeded
	ErrB    error
}

// Compare decides every request on a and on b, and returns each request where the two
// decisions are different, in request order.
func Compare(ctx context.Context, a, b Policy, reqs iter.Seq[Request]) ([]Diff, error)
```

1. Two decisions are different when `Allow` is different, or when one `Decide` returns an
   error and the other does not. Two errors are equal for this comparison. Proof, Why,
   RuleSets and cost are not part of the comparison.
2. `a` and `b` can have different rule sets, different standing facts, or both. Compare a
   new hire against the old org as easily as a new rule against the old rule.
3. `Compare` stops and returns `ctx.Err()` when the context is cancelled. It returns the
   diffs that it found before the cancel.
4. A request that is malformed for one policy (`ErrRequest`, for example an undeclared
   context key) is a diff, not a stop. A policy with a different context declaration
   refuses different requests, and the consumer must see this.
5. Doc comment on `Compare`, in these words or near them: "Compare checks only the requests
   that you give, over the standing facts that each policy holds. No diff is not a proof that
   the policies are equivalent for all requests or for other facts. It is evidence for the
   requests that you gave."

## Acceptance

1. A policy compared with itself gives no diffs, for every request in the `decide_test.go`
   tables.
2. A policy compared with the same policy minus one `deny/3` fact gives exactly the requests
   that the fact denied. Each diff has a Proof on one side and a Why on the other side.
3. Order: the diffs are in the order of `reqs`. Two runs give the same result.
4. Cancel: a cancelled context stops `Compare`, and the diffs found before the cancel are
   returned with the error.
5. An `ErrRequest` on one side only is a diff. On both sides, it is not a diff.
6. `examples/hrchat` gets one test: its policy compared with a mutant that does not have the
   `four_eyes` attestation (`hrpolicy/entitlements.go`, `alsoNeeds`). The diffs are all
   `approve.salary` requests where the requester is the approver or is not recorded. (This
   shows the multi-author use without models.)

## Out of scope

- Proofs for all possible facts (what SymCC does). This needs a solver. It is a different card.
- Making the requests. The consumer knows its principals, actions, resources and contexts. A
  helper for the cross product can come later, if two consumers write the same loop.
- Comparing proofs. Two policies that allow the same request for different reasons are equal
  here.

## Open questions

1. Is two errors equal correct? A `*TooLarge` on one side and an `ErrRequest` on the other side
   are both errors, but they mean different things. Possible answer: compare the error class.
2. Memory: hrchat has 1001 principals times its tools times its context combinations. Return a
   slice, or give `iter.Seq[Diff]` so a consumer can stop at the first diff?
