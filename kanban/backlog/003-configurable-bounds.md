# 003 Let a policy set its decision bounds

Asked for by: hrchat (`examples/hrchat/kanban/backlog/006-shearline-decision-cost.md`, part 2).
Owner: shearline-lead. Status: PARKED. Written in ASD-STE100.

## Trigger

Do not start this card until one of these is true:

1. A consumer shows, from `Decision.Steps` / `Decision.Bytes` (card 002), a correct policy
   whose decisions use more than 25% of a bound on real data. Today hrchat's most expensive
   principal uses about 0.1% of the steps bound and 2.5% of the bytes bound.
2. Demand evaluation has shipped and a correct policy still goes near a bound.

When the trigger fires, choose between this card and demand evaluation with those
measurements. Record the measurements here first.

Measured 2026-11-07 (card 010, Query): one wide Space over hrchat seed 1, 30 principals × 27
actions × 62 resources under 4 context bags, 200,880 triples in one evaluation per bag:
Steps 2,317,129 (55.2 % of the bound), Bytes 446,191 (42.6 %). A narrow Space of one
principal stays under 0.5 %. The trigger fires for the wide shape only; the remedy today is
"split the Space", so the card stays parked until a consumer needs the wide shape in one call.

## Why it is parked

The bounds are a defence. A knob makes the defence easy to remove, and a consumer that meets
a bound raises it before it narrows the policy. Without demand evaluation, the cost of a
recursive consumer relation comes from the standing data, not the request, so card 002's
cost is enough to watch it grow. A ceiling chosen now ("16 times the default") is a guess,
not a measured safe capacity.

## Proposed change (from hrchat 006, to re-check at the trigger)

```go
type Bounds struct{ MaxSteps, MaxBytes int }
// Config gains: Bounds Bounds // zero fields keep the defaults
func (p Policy) Bounds() Bounds
```

`Load`: a zero field is the default (never "no bound"); a negative field is `ErrInvalid`; a
field above a ceiling constant is `ErrInvalid`, and the error names the ceiling. `Decide` and
`Permitted` use the same bounds.

## Open questions for the trigger

1. The ceiling: measure representative workloads and concurrent decisions before choosing it.
2. Bounds are policy: a change to them changes which requests fail. Where does a reviewer see
   them? (`Policy.Bounds()`, the `TooLarge` message.)
3. Ledger meaning across changes: a consumer comparing cost over time needs the effective
   bounds and the engine version beside each row. Decide whether Shearline exposes the engine
   version or leaves it to the consumer.
