# BLUE17 independent render review

Scope: `datalog/render.go`, snapshot 5fe3a78. Read proof/absence/constant public contracts and existing render tests for context. No production edits. No red reports or tests read.

## Safeguards already present

- Proof text tracks `(relation, tuple)` before descending, suppressing shared subtrees and terminating cycles created through mutable Body slices. Tuple keys encode kind and lengths, so embedded separators inside constants do not alias ordinary tuple identities.
- String constants use strconv.Quote; tuple, goal, bindings and failed literal values escape control characters. Bindings sort names deterministically.
- Rule JSON exposes only set/line/text, avoiding mutable parser AST leakage. JSON uses encoding/json for escaping and Const for scalar types. Nil rules serialize as null; absent bound maps normalize to {}; absent absence rules normalize to []; open goal positions become null.
- Engine-generated Proof values are acyclic; engine-generated absence Because recursion is bounded and deduplicated before rendering. Those are upstream guarantees, not renderer limits.

## Reachable gaps

1. **High: JSON expansion of legitimate shared proofs is exponential (184–194; also 227–229).** json() recursively materializes every occurrence before encoding. A chain `p0(X) :- base(X). p1(X) :- p0(X), p0(X). ... pn(X) :- pn-1(X), pn-1(X).` has O(n) derivation records / shared Proof storage and O(2^n) jsonProof allocations/output. Around 25–30 levels can exhaust process memory despite tiny evaluation input. Proof.String already avoids this shape. The full-tree format is explicitly documented, so changing it needs a format decision; nevertheless the current error-returning marshaler has no resource bound, context, or safe failure path. A node/output budget returning an error preserves ordinary format while preventing catastrophic expansion. Absence.Blocked inherits the same issue even though WhyNot has its own walk limits.

2. **Medium: oneLine rewrites string values (73, used 96/132).** strings.Fields collapses whitespace everywhere, including inside quoted constants. A valid rule `p("a  b") :- q("a  b").` is displayed as `p("a b") :- q("a b").`; literal tabs and Unicode whitespace are also lost. The tuple itself and JSON rule.text retain the original, leaving contradictory human-readable evidence. Normalize only whitespace outside quoted strings; account for escapes and comments. This is reachable through ordinary parsed rules, without malformed public values.

3. **Medium hardening: cyclic public graphs recurse without detection in JSON and Absence.String (184–188, 140–142, 224–225).** Exported slices permit `p.Body=make([]Proof,1); p.Body[0]=p`, or `a.Rules=make([]RuleFailure,1); a.Rules[0].Because=[]Absence{a}`. Proof.String terminates, but both JSON paths and absence text recurse until fatal stack exhaustion; encoding/json cycle detection cannot run because conversion happens first. Engine-generated graphs exclude these cycles, so classify as public-value hardening rather than an engine-produced failure. Use active traversal identities plus limits, not just semantic tuple identity, when rejecting cycles.

4. **Low hardening: Absence.String panics on a nil RuleFailure.Rule (132).** `Absence{Rules: []RuleFailure{{}}}.String()` dereferences nil in ruleRef; equivalent JSON safely emits rule:null. Nil/zero Proof and Absence values otherwise render successfully. Public mutation can introduce this; generated RuleFailure values have nonnil rules. Add a placeholder or explicit validation if renderers are intended to tolerate caller-built values.

5. **Resource caveat: deep unshared text proofs have quadratic output.** Per-node indentation grows with depth (86), so a linear chain generates O(depth²) text even with deduplication; there is no depth/output bound. This follows from the chosen indented format and is weaker than finding 1, but merits a stress test and optional render budget.

## Proposed regression tests

- Compile a double-reference chain at 12–16 levels; assert distinct derivation size is small and demonstrate emitted JSON node count 2^(n+1)-1. A resource-limit fix should instead return a deterministic error once a configured/safe budget is crossed. Include the same proof as RuleFailure.Blocked.
- Render parsed rules containing two spaces, tab, and Unicode whitespace inside string constants; assert the displayed quoted values remain exact, while outside whitespace can fold. Assert JSON rule.text remains exact.
- Test public cyclic Proof and Absence graphs only in a subprocess with timeout/resource limits until defensive errors exist: current failures can kill the test process and cannot be recovered as ordinary panics.
- Test `Absence{Rules: []RuleFailure{{}}}` for intended nil-rule handling; test zero values, nil/empty tuples and slices explicitly to lock down null versus [] decisions.
- Test a diamond proof with same tuple appearing twice, a self-cycle, repeated relation names with distinct tuples, and mixed scalar kinds to preserve text deduplication safeguards.

No runtime stress tests were run; exponential growth and cycle behavior above follow directly from recursive conversion/traversal. Existing tests cover the ordinary shared text proof, basic JSON shape, and one open goal; they do not cover these boundaries.
