# 004 Acting managers and dated delegations

## Job story
When a manager is on leave, I want `acting_for(P, M, From, To)` facts and a `today` context key, so the policy carries the delegation for its window and not after.

## Notes
- Datalog compares; the app supplies today as a number (days since epoch) in context.
