# RED TEAM: datalog/builtins.go

Reviewed commit `5fe3a789d230a28d59b16a03f3c6ac7753ac1a8c`. Scope: `datalog/builtins.go`; compiler, ordering, evaluation and constants inspected only to establish reachable preconditions. No production changes or new tests.

## Findings

No confirmed defect in the assigned file. No severity assigned.

## Confirmed behavior and nonfindings

- Lines 19–22: all three registered guards have arity two and the correct Go string predicate. Lines 31–34 reverse the DSL's `(pattern, subject)` arguments exactly once. Empty patterns match, matching is case sensitive, and no glob, normalization or locale semantics are promised. UTF-8 and embedded NUL strings use ordinary Go string semantics; valid UTF-8 inputs cannot accidentally match half a multibyte code point using a valid UTF-8 pattern.
- Lines 31–32: either non-string argument makes every predicate false, including both-number and boolean cases. Line 51 flips that result for negation. Thus `not prefix("1", 1)` succeeds intentionally; the file documents this behavior explicitly.
- Lines 30–34 assume two arguments; lines 46–51 assume a known builtin, nonnil atom and fully bound variables. These assumptions hold for public compiled plans: `checkAtom` rejects wrong arities (`plan.go:237`), `checkGuard` rejects anonymous/unbound arguments, `checkRule` excludes builtins from positive binders, and `order`/`ready` schedule guards only after binding atoms. Constants need no binder. Calling these private helpers directly with a short slice could panic, but is not a reachable public-input finding.
- Lines 19–23 and 40–51 share one private registry. Production code has no registration API or map writes after initialization. Compiler checks reserve the names against schemas, declarations and rule heads. The sequential hook test temporarily adds an entry and removes it before parallel tests execute; no demonstrated concurrent production registry mutation exists. Missing-entry/nil-function panics require internal map mutation and are not findings.
- Guard dispatch runs before relational negation in `eval.go:231`, so negated builtins do not incorrectly consult a nonexistent base relation. No builtin generates constants or participates as a recursive relation.

## Cost boundary / unconfirmed concern

Lines 47–49 allocate and resolve two constants per guard call. String comparisons inspect input bytes; `contains` may scan a long subject. These guards neither count bytes examined nor poll cancellation during a string predicate. The neighboring evaluator charges candidate tuple visits, not guard operations (`eval.go:279`), so `MaxSteps` is not a bound on string bytes or number of guards. This is a confirmed limitation, but not elevated to a defect in this file: the documented step unit is tuples visited, and comparisons have the same general boundary. A claim of exploitable denial of service would need explicit input-size/resource guarantees and a measured workload; none was established in this bounded review.

## Verification

`env GOCACHE=/private/tmp/red17-builtins-gocache go test ./datalog -run '^TestBuiltins_' -count=1` passed. Existing focused tests cover registration hooks, parameter order, empty strings, case sensitivity, positive/negative predicates, mixed kinds, malformed arities, unsafe variables, reserved names and recursive use. Initial use of the default Go cache failed due filesystem permissions; the isolated writable cache resolved it.
