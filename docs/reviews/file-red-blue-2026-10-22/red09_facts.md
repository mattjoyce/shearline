# RED09: policy/facts.go

Focused review of `policy/facts.go` at `5fe3a78`. No confirmed security or correctness defect found in the assigned file. No production changes or new tests.

## Checked boundaries and nonfindings

- **Lines 14–29, schema ownership and relation names:** each call allocates new arity/check maps; exported Schema has no declared context keys. Load uses a private copied declaration map before constructing the internal closure. Mutating an exported schema or the caller's Config.Context after Load does not change policy validation. Base.NewBase separately copies schema maps.
- **Lines 37–53, string checks:** each constant position of principal/action/resource/role/inherits/grants/denied must be a nonempty string. Variables pass because these callbacks also inspect rule patterns. Arity is checked by the engine before facts are accepted; the callback need not duplicate it. Invalid UTF-8 is separately rejected by Base.group. No empty-prefix grant bypass found.
- **Lines 55–80, constructors:** Role maps `(principal, role)` to role/2; Inherits maps `(role, parent)` to inherits/2; Grant maps `(role, action, prefix)` to grants/3; Deny maps `(principal, action, resource)` to denied/3. Each allocates its own tuple; all inputs become string constants. Constructors deliberately return data rather than errors: malformed values are rejected when loaded. Base copies tuple slices before holding them. No argument swap, mutable input alias, or relation-name confusion found.
- **Lines 101–119, context checks:** constant keys must be declared nonempty strings and constant values match their declaration. A variable key intentionally skips static kind checking because its selected declaration is unknown; requests still pass checkRequestContext plus Base validation, so this does not admit an undeclared/wrong-kind request. Wrong arity returning nil here is not a validation bypass because the engine owns arity validation. A direct call to the exported callback with malformed Atom data is not a supported fact insertion path.
- **Lines 121–135, request conversion:** exactly one principal/action/resource fact is generated, independent of context names. Context cannot inject a role, grant, denied, or request relation; it becomes a context/2 key/value tuple. Sorting makes context insertion stable. Constants have private value fields, and generated tuples do not share the map's storage. Concurrent mutation of the caller's Request.Context map is not supported ownership; it is an ordinary Go data race, not a demonstrated policy authorization defect.

## Explicitly documented preconditions and misuse

- **Lines 69–72:** Grant is literal string-prefix authorization, not a path boundary or canonicalization operation. `Grant("reader", "read", "timeline/health")` also covers `timeline/healthy`; the comment explicitly warns about this and recommends separators. Traversal/URI normalization requires a consumer-defined resource naming convention. This is documented semantics, not a confirmed defect.
- **Lines 75–78:** Deny matches exactly one resource and depends on the bundled RBAC rule interpretation of denied/3; it is not a subtree denial. The comment explicitly supplies the consumer-rule alternative. The broader Load documentation warns that trusted custom rules may define allow directly, so “whatever it is granted” is not a promise against arbitrary policy programs.
- **Lines 98–119:** declarations specify accepted optional keys and types, not required presence or trusted provenance. Missing context stays absent. Empty string context values are permitted; Load explicitly documents their wildcard behavior if fed to prefix/2. Consumers must supply trusted identity/context and write deny-on-missing rules where required. No hidden validation guarantee was assumed.

## Bounded verification

Read the relevant Policy Load/Decide/declaration checks, Base insertion/ownership checks, Const representation, and existing focused tests only as boundary evidence. Ran:

`go test ./policy -run 'TestSchema_FreshEachCall|TestLoad_Rejects|TestContextDeclaration|TestRequestFacts_ContextSorted|TestDecide_EmptyContextValueIsAllowed|TestDecide$' -count=1`

Result: PASS. Some regex alternatives may match no tests; the named schema freshness, context declaration, sorted context, and empty-context-value tests exist and ran. No speculative concern was promoted to a finding.
