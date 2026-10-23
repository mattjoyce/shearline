# Blue 18 — datalog/plan.go

Scope: snapshot 5fe3a78, plan.go only for implementation review. Read public AST definitions, README, docs/DESIGN.md and existing plan_test.go. No review reports or red-team files/history read. No production changes.

## Assessment

For ordinary caller-owned programs and observational, terminating schema callbacks, plan.go establishes the intended compiler boundary. No new unconditional high-severity defect found. Callback isolation is a real hardening opportunity: the immutable-plan guarantee depends on callback behavior, although the public contract does not currently spell that out.

## Code-owned defenses

- Compile clones each Program, checks its shape before relation/rule walks, then validates schema before collecting arities (46–64). A caller changing its original AST or schema maps after successful compilation cannot ordinarily reach the plan. The new normal-ownership test exercises nested body argument mutation, head mutation and schema arity mutation.
- Relation collection sees all programs before rule checks. Forward references work, empty declared hooks remain legal, and conflicting declarations/heads, base-derived collisions and reserved built-in names fail. Arity checks precede schema callbacks. Unknown relation diagnostics list known names in sorted order.
- Safety comes from positive relational atoms only; built-ins, negations and comparisons do not bind variables. Anonymous head/guard variables are refused, while anonymous negation positions are allowed. Safety is independent of written guard order; stratify is responsible for actual scheduling. New public test demonstrates a comparison and negation written before their binder evaluate correctly.
- Every rule atom is arity-checked, including negative occurrences; callbacks are invoked for schema relations. Error wrapping retains both ErrInvalid and a callback sentinel (covered by existing TestSchemaChecks_KeepTheirSentinel).
- accepts rejects any base relation colliding with derived relations, even if empty, and requires every compiled schema relation at the expected arity. Base extras are allowed unless derived. This is deliberately shape compatibility, not callback equivalence; docs/DESIGN.md explicitly permits different Checks. Treating that as a check bypass would contradict the documented trust boundary.

## Callback boundary: confirmed limitations, not attacker-code containment promises

1. **A check receives a mutable alias to compiled AST arguments** (checkAtom -> check(a), 248–249). Atom is copied by value, but Args is a slice of the plan-owned terms. A callback may retain it and mutate it after Compile returns, changing evaluation. New TestBlue18PlanCallbackRetainsMutableArguments reproduces this sequentially without a race: out(X) :- node(X) initially matches a present node; replacing the retained X with the constant "absent" makes out empty. The original caller Program remains unchanged, confirming this reaches the compiler's copy rather than defeating Clone. Concurrent callback-alias writes could also invalidate concurrency safety; no intentional race test added. Mutation during validation additionally happens after bound-variable collection (165–180), so the collected safety evidence need not describe the mutated atom.

   Defensive option: pass a fresh argument slice to each callback. Document checks as read-only and non-retaining if that is the intended caller obligation. Retained alias mutation should not be conflated with ordinary caller AST edits: those are isolated successfully.

2. **Callbacks can change the schema between two compiler snapshots** (relations at 61 versus base copy at 81–84). TestBlue18PlanCallbackChangesSchemaSnapshot adds extra/1 inside a node check: the returned plan reports no Arity("extra"), yet Evaluate refuses an otherwise matching base because accepts requires extra/1. No concurrent access needed. A callback closure can mutate maps because Schema's value copy retains caller map references.

   Defensive option: snapshot Arities and Checks before validation/callback execution and use that snapshot throughout. Concurrent map mutation during Compile remains a caller synchronization obligation; copying maps alone does not make concurrent reads/writes safe.

3. Panic, infinite loop, allocation, arbitrary side effects, and malicious Error methods from caller functions are not contained. Compile has no context or callback time budget. These functions execute with application authority; a hostile function is already executable caller code. Do not claim sandboxing or fail-closed behavior across that boundary. Returned ordinary callback errors are handled correctly.

## Lower-priority observations

- Error selection is not universally deterministic. checkSchema ranges unsorted over Arities/Checks; declaration collection ranges over Declared; accepts sorts base-derived collisions but ranges unsorted over missing/wrong-arity required relations (279). Multiple independent invalid entries can therefore name different first offenders. This does not alter successful answers or error classification. The sortedKeys comment describes a local helper, not a global guarantee. Sort remaining diagnostic map walks if stable error text is required.
- accepts requires even unused schema relations. This is stronger than the design's phrase "lacks a relation the plan reads," but consistent with compiling over an entire schema and the base-map comment. It can reject a base that would suffice for execution; no unsafe acceptance follows.
- Compilation has no explicit limits on program count, relation count, arity, AST size or diagnostic size. It clones before validating, sorts all names for an unknown-relation error, and delegates graph/order work to stratify. Evaluation Options do not bound compilation. Large caller-supplied ASTs/schema maps can consume memory/time; no tested asymptotic claim about Clone or stratify is made because those implementations are outside scope. Deployment accepting untrusted policy text needs load-side size limits. The public policy contract calls rule sets policy, never user input.
- Zero-value Plan has nil maps; Arity safely reports absent. Whether Evaluate should accept it is an evaluator/API-contract issue, not evidence of malformed-input panic in this file.

## Validation

Added datalog/blue18_plan_test.go with three passing public-API characterization/assurance tests. Ran:

- go test ./datalog -run '^TestCompile' -count=1 — pass.
- go test ./datalog -run '^TestBlue18' -count=1 — pass.
- go test -race ./datalog -run '^TestBlue18' -count=1 — pass.

Characterization tests intentionally assert existing callback exposure so this review is reproducible; they should be revised if callback isolation is implemented. They are not desired-behavior regression tests endorsing mutable plans.
