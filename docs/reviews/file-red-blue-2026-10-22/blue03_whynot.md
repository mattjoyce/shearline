# BLUE 03: datalog/whynot.go

Reviewed snapshot 5fe3a78; production scope only `datalog/whynot.go`. Supporting public types, matching helpers, proof ownership and ordinary why-not tests were read. No production edits. A broad symbol search accidentally returned filenames and brief matching lines from a red-prefixed test; that file was not opened or used as evidence.

## Confirmed correctness gap

**P1: repeated variables in recursive goals lose equality constraints (lines 289–307).** `unifyHead` treats every variable goal position as unconstrained. Although correct for independent existential variables, this drops the equality in a failed literal such as `p(X,X)`. Reproduction: base `edge("a","b")`; rules `p(A,B) :- edge(A,B). q("missing") :- p(X,X).` Evaluation correctly leaves q absent, but WhyNot's nested absence for p has a rule with zero Failed and `Truncated=false`: it walked the unrelated off-diagonal tuple successfully. This directly contradicts the documented invariant at lines 50–54 and produces an invalid explanation. Preserve variable equivalence classes when unifying the head, including equality propagated through constants and repeated head variables.

Evidence: `go test ./datalog -run '^TestBlue03' -count=1` fails in the unique `datalog/blue03_whynot_test.go` with `stopped at (no failing literal found)`.

**Related memoization defect (lines 325–338):** `goalKey` collapses `p(X,X)` and `p(X,Y)` (and anonymous independent positions) into the same key. These are different predicates on tuples, so fixing unification alone leaves incorrect suppression of distinct explanations. Canonicalize variable identities by first occurrence while keeping `_` independent. This follows directly from the key implementation; no additional executable reproduction was added.

## Defenses and deliberate tradeoffs

- Concrete public goals reject unknown relations, wrong arities, present tuples and proof-disabled results before walking. Constant head mismatches and repeated head-variable bindings against concrete goals are checked.
- Positive matching delegates to `bind`, which enforces repeated body variables and unwinds newly introduced bindings on mismatch; the walker also unwinds successful branch bindings. Failure bindings are cloned at capture, so later branches do not overwrite the selected evidence.
- Goals, rule ASTs, failed literals and blocked proofs are copied out; maps are per invocation. Internal sharing of copied rules/proof DAGs within an answer follows the Proof contract and does not expose the plan/result. Ordinary tests cover concurrent read use and determinism, but do not directly mutate an entire Absence to verify ownership.
- Depth truncation propagates to ancestors; candidate exhaustion propagates even when no failure has yet been found. First farthest failure is intentional representative evidence, not a complete list of all failed branches.
- Global visited-goal suppression, including across sibling failures, is explicitly documented and is not itself missing truncation. Repeated-variable memo collisions above exceed that permitted tradeoff.

## Budget limitations and useful tests

The step budget bounds attempted candidate bindings, not elapsed work, allocations, output size or proof expansion. `matching` is called before `spend` (lines 237 and 262): a derived relation can be fully scanned to produce zero candidates without consuming any steps. Repeating such probes across branches may perform substantial work with `Truncated=false`. Unbound matching also allocates a full candidate-index slice before the budget is consulted. `rulesFor` rescans the plan for every nested absence and neither rule visits nor guards consume steps. A blocking proof is constructed outside depth/step accounting (line 156). Thus the comment claiming a few milliseconds is not a robust adversarial resource guarantee. This is a contract limitation rather than evidence that the numeric candidate limit is incorrectly implemented. If a hard resource ceiling is required, charge scanning, rule visits and proof construction or give them separate budgets.

`maxDepth` has no upper clamp; nonpositive values select the default. Large caller depths can permit long recursive absence trees, while body recursion is independently proportional to rule length. Public WhyNot accepts Tuple constants, not arbitrary Terms, so malformed ASTs are outside this entry point's supported input; invalid UTF-8 String goals can still yield an absence because WhyNot does not run Base validation. No memory-safety issue was found from such goals, but rejection/acceptance should be explicit if goals are serialized or externally trusted.

Recommended targeted coverage: repeated goal aliases with distinct and constant heads; distinct alias shapes in the same call's memo; mutation isolation across all returned nested objects; no-match scans over large derived relations; a large blocked-proof DAG; and exact depth/step boundary behavior. Existing tests cover the normal recursion, depth propagation and broad candidate-exhaustion cases.
