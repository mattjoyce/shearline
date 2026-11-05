# 007 Deny properties, checked over the request space

Owner: hrchat. Needs: Shearline `Compare` (shearline card 009, built). Written in ASD-STE100.

## Why this is useful

Today the policy's assurance is 39 scenarios. A scenario shows one request is denied. It
does not show a class of request is denied. If a person asks "can anyone read the CFO's bank
details?", the answer today is "not in the cases we wrote". This card makes the answer "no
request in the space we enumerate, and here is the why-not for each one we checked".

## Method

Each property is a `deny/3` rule that says the thing that must never be allowed. Load the
policy plus the property as a second policy. `Compare` the two over the request space. A diff
is a request the policy allows and the property denies: a hole. No diff is evidence that the
property holds over that space.

Because a property is a rule in the same language, a reviewer reads the property, not the test
code. And because each diff carries both decisions, a hole explains itself.

## The properties

Write each one as an ASD-STE100 sentence, then as a rule. Start with these:

1. Nobody reads or writes high data of a person above them in their own reporting line.
   `deny(P, A, M) :- principal(P), action(A), resource(M), up(P, M), level(A, "high").`
   (This is already a rule in hrchat/1. As a property it checks that no later rule, and no
   entitlement fact, widens past it.)
2. Nobody approves a pay change they raised. `deny(P, "approve.salary", S) :- ... context("requester", P).`
3. Nobody approves a pay change with no requester recorded.
4. No high action without MFA. `deny(P, A, R) :- ..., level(A, "high"), not context("mfa", true).`
5. No high write without a confirmation.
6. The chat channel never carries write.salary, write.status or write.tfn.
7. A manager reaches a direct report only: no path to a report's report.
8. Payroll reaches bank, TFN, payslip and salary of others, and nothing else of others.
9. Your own record: only the self path. No role or relationship widens what you do to yourself.
10. A flagged guardrail verdict allows nothing.

## The request space

Not every (principal, action, resource, context): 1001 × 30 × 1001 × contexts is too many.
Enumerate by role instead:

- Principals: the CEO, one director, one manager, one IC per division, every HR partner, every
  payroll officer, and one person with no role. About 30.
- Resources: for each principal, self, manager, manager's manager, one direct report, one
  report's report, one peer, one person in another division, one HR partner, one payroll
  officer. About 10 each.
- Actions: all 27 in the catalogue.
- Contexts: a grid over channel (2), guardrail (2), mfa (2), confirmed (2), requester (none,
  self, subject). 48.

About 30 × 10 × 27 × 48 = 388,800 requests per property. Measured 2026-11-04: 600 decisions
over seed 1 at about 145 µs each (M-series Mac, no race detector), so about one minute per
property per policy, and twice that for a Compare. Ten properties is about 20 minutes. Too
slow for every `go test`; fine for a nightly.

So: a `go test` tag `properties` runs the full space; the default run takes a fixed sample of
2,000 requests (seeded, the same every run). CI runs the sample. A nightly or a release runs
the full space.

## Deliverable

1. `hrpolicy/properties.dl`: the properties, each with its ASD-STE100 sentence as the comment
   above it. One rule set `hrchat-properties/1`.
2. `hrpolicy/properties_test.go`: loads the policy, loads the policy plus the properties, builds
   the request space, runs `Compare`, fails on any diff with `Explain` of both sides.
3. `cmd/hrchat properties [--full]`: the same, as a command, printing the count checked, the
   time, and each diff. The README's assurance section cites its output.
4. README: replace "39 scenarios" with the two numbers (scenarios, properties × requests).

## Acceptance

1. Every property passes over the sample on `go test ./...`.
2. Every property passes over the full space on `go test -tags properties ./...`. Record the
   count and the time in this card.
3. Mutation: remove each of the three `deny` rules from hrchat/1 in turn; at least one property
   fails for each, over the sample. Record which property catches which rule.
4. Mutation: remove `four_eyes` from `alsoNeeds`; property 2 and 3 fail.
5. A reviewer who does not read Go can read `properties.dl` and say what each property promises.

## Out of scope

- Proof over all possible organisations. That is a solver (Cedar SymCC). This is evidence over
  one organisation and an enumerated space.
- Properties about allow ("a manager can always read a report's leave"). Those are the
  scenarios. A later card can add allow properties the same way if the scenarios feel thin.
