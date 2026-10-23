# Blue 15: policy/policy.go independent boundary review

Snapshot requested: `5fe3a78`. Scope: `policy/policy.go` and public README/design/spec only. No other review reports or production implementations were read. No production changes or tests added. Conclusions about datalog internals rely on its documented contracts rather than a second implementation audit.

## Assessment

No direct authorization bypass is established inside this file for a successfully loaded Policy, trusted Config, caller-owned request, and valid Go context. The code explicitly treats extensions as trusted authorization code, not a constrained deny-only plugin system. Most plausible confused-deputy failures occur when a consumer assumes stronger semantics than this API supplies.

### B15-1: public listing language overpromises equivalence (documentation, medium integration risk)

README says Permitted comes from the same rules so listing and gate cannot disagree. In this file lines 184–199 explicitly define a different operation: prefix grants, no concrete resource, no request context, and no deny application. For bundled RBAC with grant `timeline/` and denial `timeline/secret`, Permitted includes `timeline/` while Decide denies the secret. For a trusted extension that conditions permit on context, Permitted supplies no context and may omit a prefix that Decide can use. An extension may also add allow rules unrelated to permit (expressly permitted at lines 95–99).

This is not a bug in the implementation's stated Permitted contract. It is an actionable public documentation mismatch that can cause a consumer to authorize enumeration or reads by prefix alone. Replace the equivalence claim with an explicit candidate-discovery contract: enumerate candidate resources under returned prefixes, then Decide for every actual resource with the same authenticated identity and trusted context. Direct allow extensions also mean Permitted need not be complete candidate discovery; consumers choosing that pattern must constrain their own rule vocabulary.

### B15-2: declared context validates shape, not authority or presence (consumer responsibility; documentation opportunity)

Lines 263–278 validate only supplied keys and kinds. A declared key is optional; an empty context is accepted. This is coherent with the public Config contract, but a security-relevant key such as MFA state, source network, current time, or tenant cannot safely be taken from an arbitrary caller simply because its type is correct. A rule relying on absence through negation may allow when such a key is omitted. Empty string context is deliberately accepted and may become a universal prefix (lines 108–109).

Consumer must authenticate Principal; bind Action and Resource to the operation actually executed; construct privileged context from trusted observations; enforce required fields before Decide or positively require them in rules; and avoid mixing attacker-supplied scope prefixes with authorization scope. Config.Context is a schema, not attestation or a required-field declaration. No implicit engine fix is warranted: rejecting every omitted declared key would change the stated API.

### B15-3: IDs attest rule text, not the complete authorization state (consumer responsibility)

Load verifies every supplied RuleSet and records its ID in order (125–130); Decide returns an independent slice of those IDs (153). This is a useful integrity safeguard. It does not authenticate the author or authorize the source of a replacement set: a hostile caller can create and hash hostile rules. Nor do IDs include Standing or Config.Context, so the same RuleSets list can describe opposite decisions after a role/grant change. Public wording correctly calls them rule-set IDs, but audit receipts need the authenticated request and a consumer-owned version/hash of standing facts and context policy, plus relevant observed context, if replay or complete policy identity matters.

Extensions may add unconditional allow, redefine the effective permit vocabulary, or bypass deny checks by adding another allow rule. That is expressly documented at lines 95–99 and is not a bypass of a promised deny-overrides sandbox. Consumer must restrict who supplies Sets and review the combined policy.

### B15-4: bounds do not promise an end-to-end request deadline (hardening opportunity, low under bounded consumer input)

Decide checks ctx at entry, validates/sorts supplied context and builds request facts before Evaluate, then constructs Proof or WhyNot without passing ctx (154–176). Permitted similarly filters and sorts its result after Evaluate (203–219). Therefore maxDecisionBytes/maxDecisionSteps constrain evaluation, not all input processing, proof expansion, explanation, output sorting, or serialized receipt size. The public WhyNot contract has its own finite candidate/depth bounds, which helps, but does not make it context-cancellable.

A request with many undeclared context keys is fully collected and sorted before the first rejection (263–272), despite Config already bounding the number of legitimate keys. Consumer should bound request bytes, string sizes and context entry count before invoking this library and supply a deadline. Optional inexpensive safeguard: reject a context map larger than the declaration before allocating/sorting keys. Preserve the distinction between evaluator tuple/work limits and total process memory/latency guarantees. No measured resource-exhaustion claim is made here.

## Safeguards present

- Standing request relations are rejected before building the base (116–120), preventing ambient identity/resource/context facts from speaking for every request.
- Context declaration is privately copied and constrained to recognized kinds (247–258); request checks are deterministic by sorted key (263–278).
- Compile and Base construction errors stop Load; required allow/permit arities are checked before serving (132–143, 281–295).
- Decide starts with Allow=false, sets it only after successful evaluation and matching proof, and every explicit error path returns the false value. Error and absence are distinct; absence carries Why, errors do not.
- Permitted filters exact principal/action, verifies returned prefix values, sorts and deduplicates; it does not accidentally list other principals' derived permit tuples (207–219).
- RBAC returns a cloned Program; sortedFacts leaves the caller's outer fact slice unchanged; declared context and returned ID slices are copied. Public datalog contracts additionally promise deep-copy compilation and immutable bases. The shallow tuple copy in sortedFacts is not itself a demonstrated alias bug because ownership passes through documented NewBase construction.

## Operational ownership requirements

Only publish a Policy after Load succeeds; replace policies atomically rather than mutating inputs during Load. Treat Config as trusted, stable input throughout the call. Do not mutate Request.Context concurrently with Decide; it is read directly twice through validation and fact construction and Go maps do not permit concurrent writes. A loaded Policy may be shared concurrently under the stated immutable-engine contracts. Returned RuleSets mutation cannot alter later decisions.

Use a non-nil context, as required by normal Go context conventions: ctx.Err is called directly. A nil context panics rather than yielding a denied Decision, so the word “total” is scoped to valid API use, not arbitrary malformed Go values. Do not use a zero Policy or discard a failed Load. Treat every Decide error as denial and do not fall back to Permitted. Decide supplies no authentication, resource canonicalization, resource existence check, atomicity with the subsequent side effect, receipt confidentiality, or trust partition between context keys. Those belong to the consumer. Canonicalize opaque resource names and use explicit path separators in grants before invoking policy, as the public design already requires.

## Validation limits

Static control-flow and public-contract review only. No test run was needed to establish the documentation mismatch or caller trust obligations. Potential engine copying, cancellation granularity, proof allocation, and hash/AST verification defects are outside this file and were not assumed absent beyond their published contracts.
