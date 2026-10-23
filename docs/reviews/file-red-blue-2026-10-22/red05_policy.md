# Independent review: policy/policy.go

Snapshot: 5fe3a78. Scope: this file; facts/schema, base ownership, compilation ownership, and WhyNot were inspected only to establish its seams. No other review reports/history read. No production edits.

## Confirmed findings

### Low — context validation drops the documented underlying ErrInvalid

Lines 272 and 275, reached through Decide at lines 156–158. ErrRequest's public documentation (lines 48–51) says the underlying ErrInvalid stays reachable. Empty principal/action/resource errors do wrap both errors, but undeclared context keys and wrong context kinds return only ErrRequest. Trigger: a loaded RBAC policy declaring k:string receives either unknown:"x" or k:true. errors.Is(err, ErrRequest) is true; errors.Is(err, datalog.ErrInvalid) is false. Callers relying on the documented common validation sentinel receive inconsistent classification. This is an API contract defect, not an authorization bypass: the decision remains denied. Either wrap both sentinels here or narrow the documentation. Confirmed by TestRed05ContextErrorChain.

### Low — required relation validation accepts declarations without definitions

Lines 281–295, especially 287. Load's contract says the plan must define allow/3 and permit/3, and the adjacent explanation says this prevents silently denying everything when RBAC is missing. requireRelations checks only Plan.Arity, which includes declarations. Trigger: NewRuleSet("empty", 1, "allow/3. permit/3.") followed by Load succeeds; Decide returns an ordinary denial. Consequently a consumer who accidentally ships only interface declarations gets no startup failure. This does not grant access; it defeats the advertised configuration guard and can cause an availability failure. Confirmed by TestRed05DeclarationsSatisfyRequiredRelations. If intentionally allowing explicit declaration-only deny-all policies, narrow the contract and validation comment instead of treating this as an authorization defect.

## Defenses and nonfindings

- RBAC() clones its Program. Load verifies hashes, Compile clones programs, and NewBase copies tuple slices. Config.Context is privately copied. Sequential mutation of caller-owned inputs after Load does not change the policy. Concurrent mutation while Load reads them is a caller data race, not a supported ownership promise.
- Standing request relations are refused before base construction. The fixed schema forbids derived rules writing principal/action/resource/context. Principal identity is caller-supplied by design; this library authenticates neither principals nor context. Consumers must supply trusted identity/context and trusted rule sets.
- Arbitrary rule sets can add allow rules and bypass bundled deny logic, explicitly documented in Load. Permitted intentionally lists prefixes without applying concrete-resource deny rules and correctly filters principal/action before validating and returning patterns.
- Required arities prevent t[:2]/t[2] panics in Permitted. Non-string/empty permit patterns for the requested identity return ErrInvalid. Results are sorted and deduplicated. sortedFacts owns its slice and base construction subsequently owns tuples; proof order is content-based.
- Decide initializes Allow=false and assigns true only after successful evaluation and proof membership. Evaluation errors, malformed requests and cancellation at entry fail closed. Decision.RuleSets is copied on each call.
- Context missing keys are optional by contract; undeclared/wrong-kind provided keys fail. Empty context strings are deliberately allowed and their wildcard risk is documented. UTF-8 and empty PAR identities are checked by Base.With/schema.
- Both evaluation entry points supply byte and step bounds. Request facts and context sorting occur before engine bounds; consumers must bound request input lengths/counts at their transport. WhyNot has its own depth/step limits, but proof/absence assembly is outside the context-aware evaluation call; no additional confirmed defect assigned here without a focused timing reproduction.
- nil context would panic at ctx.Err(), consistent with Go's general context convention requiring non-nil contexts. A zero Policy refuses request facts rather than granting; callers must successfully Load first.

## Focused probes

Added only policy/red05_policy_test.go. `go test ./policy -run TestRed05 -v` passed, confirming both behaviors above. Tests assert observed defects, not proposed fixes. No speculative exploit or privilege escalation claimed.
