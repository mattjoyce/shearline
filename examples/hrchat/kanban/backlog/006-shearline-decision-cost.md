# 006 Shearline: show what a decision cost, and let the consumer set the bounds

Owner: hrchat (consumer). Ask for: shearline-lead. Written in ASD-STE100.

## Why this is useful

Shearline puts two bounds on each decision: 4M steps and 1 MB derived. A decision
that goes past a bound fails closed. That is correct, but the consumer sees the bound
only when it fails. Before a failure, the consumer cannot see how near a decision is to
the bound.

A policy that reads consumer relations (v0.3.0) moves near the bound when the data
changes, with no change to the code. An organisation adds a layer, a team becomes a
long chain, a role gets more grants. Each change makes the derivation larger. The
v0.3.0 changelog says this itself: a 2,000-person single chain goes past the byte
bound. If the consumer can read the cost of each decision, it can see the margin go
down over weeks. Without the cost, the first sign is an outage: a valid user is refused
and the error says "too large".

tarski already counts the cost (`Result.Cost()`). `Decide` discards it.

The cost alone is not sufficient. The bounds are constants in `policy/policy.go`
(`maxDecisionBytes = 1 << 20`, `maxDecisionSteps = 1 << 22`), and the comment says "not
a knob: a consumer that needs more has a policy that should be narrowed". That was true
for `rbac/1`, where a decision reads one principal's roles. It is not always true after
v0.3.0. An org tree is consumer data, and a correct manager rule over a deep tree
derives more than a role lookup. Today the consumer has two remedies when a correct
policy goes near a bound: fork the module, or change the policy until it is wrong. Both
are bad. The consumer must be able to set the bounds, and to see the cost that the
bounds apply to.

## Use case

The hrchat gate writes each decision to a ledger. The audit reads the ledger. With the
cost on each row, the audit can:

1. Find the principals whose decisions use the most steps and bytes. hrchat no longer
   derives downward over the org (a manager reaches the direct team only), so its worst
   decision is 135 steps. hrchat is not near a bound now. The card is for the next
   consumer whose correct policy recurses over its data, and for hrchat if a rule of
   that kind comes back.
2. Alert when a decision uses more than 25% of a bound. The operator then has time to
   narrow the rules, or to raise the bound in `Config` as a decision that is reviewed,
   or to wait for demand evaluation.
3. Show that a rule change or an org change made decisions more expensive, by a
   comparison of the cost before and after.

hrchat has this today only because it copies `Decide`. Card 002 removes the copy, and
the cost goes with it.

## Job story

When a policy decision is near a bound, I want to see its cost on the decision and to
set the bound in my configuration, so that I can act before a valid request is refused
and I do not fork Shearline to do it.

## User story

As the operator of an HR chatbot gate, I want each `policy.Decision` to give the steps
and bytes of its evaluation, and I want to set the bounds in `policy.Config`, so that my
ledger and my audit can watch the margin and my configuration can move it.

## Proposed change

Two parts. Part 1 is necessary. Part 2 is the reason part 1 has a remedy.

**Part 1: the cost.** Add two fields to `Decision`. Set them from `Result.Cost()`
after a successful evaluation.

```go
type Decision struct {
    Allow    bool
    Proof    tarski.Proof
    Why      tarski.Absence
    RuleSets []string
    Steps    int // evaluation work; zero when evaluation did not complete
    Bytes    int // derived bytes; zero when evaluation did not complete
}
```

**Part 2: the bounds.** Add one field to `Config`. A zero value keeps the current
constants. `Policy.Bounds()` gives the bounds that the policy uses, so that a consumer
can compute the margin and does not copy the numbers.

```go
type Config struct {
    Standing  []tarski.Fact
    Sets      []tarski.RuleSet
    Context   map[string]tarski.Kind
    Relations map[string]Relation
    Bounds    Bounds // zero fields keep the defaults
}

type Bounds struct{ MaxSteps, MaxBytes int }

func (p Policy) Bounds() Bounds
```

`Load` must:

1. Use the default for each zero field.
2. Refuse a negative field with `ErrInvalid`. Zero is "default", never "no bound": a
   policy has no way to remove a bound.
3. Refuse a field above a hard ceiling (proposal: 16 times the default) with
   `ErrInvalid`. The ceiling is a constant, and the error names it.
4. Use the same bounds in `Decide` and `Permitted`.

## Pitfalls

1. **The cost is not a stable contract.** A new tarski version can change the step count
   for the same answer (an optimisation, demand evaluation). The doc comment must say
   this. The changelog must not list a cost change as an answer change, and golden
   tests must not pin the cost.
2. **A consumer must not make a decision from the cost.** "Refuse when Steps > N" puts a
   second bound outside the policy, and that bound changes when tarski changes. The cost
   is for observation only. The doc comment must say this.
3. **The cost is not a hash input.** The answer hash and the proof bytes must stay the
   same as v0.3.0. If the cost goes into a hash, a tarski upgrade changes every ledger
   hash.
4. **The error path.** On a `*TooLarge`, `Decide` returns an error. The cost is in
   `TooLarge.Steps` and `TooLarge.Bytes`, and the consumer can get it with `errors.As`.
   Do not copy it into the zero `Decision`. One place for the cost on an error is
   sufficient, and a cost with `err != nil` can look like a completed evaluation.
5. **Bytes is approximate.** tarski counts approximately the JSON text of the derived
   tuples. It is not the memory that Go uses. The doc comment must say "derived bytes",
   not "memory".
6. **Permitted.** `Permitted` uses the same bounds (part 2), but do not change its
   signature in this card. If a consumer needs its cost, that is a different card.
7. **The bound is a defence, and a knob makes it easy to remove.** Each decision
   evaluates under the request's context, and a request is input. A high bound lets one
   bad request use more CPU and memory. A consumer that meets a bound will raise it
   before it narrows the policy. The doc comment must keep the current message: "a
   policy that needs more is usually wrong, not large; narrow it first". The ceiling
   (Load rule 3) stops a typo or a panic fix from removing the defence.
8. **The bounds are policy.** A change to `Config.Bounds` changes which requests fail.
   It must be reviewed like a rule change. Shearline cannot enforce this, but the
   bounds must appear where a review can see them. Proposal: `Policy.Bounds()`, and the
   bounds in the `TooLarge` error message, which tarski gives already.
9. **The knob is not the fix.** For a recursive consumer relation, demand evaluation
   makes the cost follow the question, not the size of the org. Part 2 is for the time
   before demand evaluation, and for a consumer that has a correct large policy. The
   doc comment must point to demand evaluation.

## Acceptance

- [ ] `Decision.Steps` and `Decision.Bytes` are set on each successful `Decide`, allowed
      or denied, and are zero when `err != nil`.
- [ ] A test shows that a larger fixture gives a larger cost.
- [ ] A test shows that a `*TooLarge` error gives its steps and bytes through `errors.As`.
- [ ] A zero `Config.Bounds` gives the current constants, and `Policy.Bounds()` shows
      them.
- [ ] A raised bound lets a fixture through that the default refuses, in `Decide` and
      in `Permitted`.
- [ ] A negative bound and a bound above the ceiling each have a test with the error
      they return.
- [ ] Answers, proofs, rule-set hashes and golden tests do not change.
- [ ] The doc comments say pitfalls 1, 2, 5, 7 and 9, and the "not a knob" comment
      on the constants is replaced.
- [ ] `go test -race -shuffle=on ./...` and the go-quality gate pass.
- [ ] hrchat reads the cost from `policy.Decision`, and the deepest-decision test in
      `hrpolicy/policy_test.go` and the README cost figures come back.

## Out of scope

- A ledger hash. hrchat makes its row hash in the gate, from the rule-set IDs and the
  proof or the why-not. That is the consumer's choice of evidence, not Shearline's.
- Demand evaluation. That is the fix for the cost. This card makes the cost visible
  and the bound adjustable until then.
- A bound for each request. One bound for each policy is sufficient. A bound that a
  request sets is a bound that the input sets.
