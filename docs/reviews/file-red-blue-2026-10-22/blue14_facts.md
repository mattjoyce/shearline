# Blue 14: policy/facts.go

Reviewed snapshot 5fe3a78 independently. Scope: policy/facts.go, with policy load/decision boundaries, bundled RBAC, datalog base validation, and ordinary policy tests read to establish how its guarantees are enforced. No red-team artifacts or review reports inspected. No production edits.

## Conclusion

No confirmed security defect in facts.go. Constructors deliberately produce data without returning errors; validation occurs when facts enter a Base through Load or Decide. Treating the constructors alone as validators would be misuse of the API, not a bypass of policy enforcement.

## Existing defenses

- Each fact constructor allocates a new Tuple and stores immutable Const values. Separate calls do not share tuple storage. These exported facts remain mutable by their owner until ingestion; NewBase/With copy tuples before holding them.
- Schema returns fresh arity/check maps. Mutating a public Schema does not weaken a later Load. Load independently constructs its schema and copies the context declaration.
- All principal/action/resource/role/inherits/grants/denied positions require nonempty string constants. This rejects an empty grant prefix, which would otherwise match everything. Rule variables intentionally pass: compile-time schema checks cannot establish their eventual values.
- Arity and unknown-relation rejection precede the relation checks in datalog Base. contextCheck returning nil for the wrong arity therefore is not an ingestion bypass.
- Base rejects invalid UTF-8 string facts. Valid Unicode is accepted literally; no case folding, normalization, whitespace stripping, path resolution or identifier authentication is implied.
- Public Schema declares no context keys. Load declares only Config.Context keys; constant rule keys and values are checked against those declarations. Decide also checks every supplied key and kind before facts are built.
- Request facts are local to each decision; context keys are sorted, giving reproducible fact ordering. Load rejects standing principal/action/resource/context facts, preventing persistent request impersonation.
- Grant is explicitly a literal prefix, Deny explicitly an exact resource, and Inherits flows from role to parent. The helper encodings agree with bundled RBAC.

## Meaningful limits and consumer responsibilities

1. **Context types do not establish provenance.** A caller-controlled `context("trusted", true)` is schema-valid when declared boolean. Applications must derive security-sensitive context from authenticated server state, and must decide which keys are mandatory. Missing context is allowed; a deny rule requiring a missing key does not fire. This is a policy/application design boundary, not a constructor bug.
2. **Empty context string values are valid.** Feeding one into prefix/2 yields a universal prefix. Load documents this explicitly. A consumer using context as a resource prefix must reject empty values or guard the rule.
3. **Unicode and resource spelling remain exact.** Whitespace, control characters, Unicode confusables, canonically equivalent Unicode spellings, dot segments, and separators are not canonicalized by these helpers. A consumer must use the same canonical naming scheme at grant creation and request evaluation. Invalid UTF-8 is rejected at ingestion, not in the helper.
4. **Schema check callbacks are atom checks, not full validators.** Calling Schema().Checks directly does not perform arity checks or UTF-8 validation. The supported validation path is Compile/NewBase/Load/Decide.
5. **Variable context keys are intentionally not statically type-checked.** contextCheck returns early when the key is a variable, so an atom such as context(K, 42) may compile even if no declared key is numeric. Request ingestion still enforces actual keys/types; the rule simply cannot match. This can hide a consumer rule mistake but does not admit undeclared request facts.
6. **Trusted rule sets can override intended RBAC policy.** Constructor semantics and schema checks do not ensure arbitrary consumer rules preserve deny precedence or prevent derived values in base-named relations. Load explicitly treats rule sets as trusted policy. Do not expose rule-set creation as untrusted request input.

## Regression coverage to retain or add

Existing ordinary tests cover fresh Schema maps, malformed standing values, standing context refusal, declared context keys/kinds and copied declarations, empty requests, context ordering, inherited grants, and exact Deny behavior.

Useful focused additions (no production change required):

- Table-drive all exported constructors through Load, placing empty and invalid UTF-8 strings in every argument; require ErrInvalid.
- Mutate a fact's Tuple after Load and verify the original decision remains unchanged; verify separate helper calls do not alias.
- Check Unicode composed/decomposed strings remain distinct and whitespace-only identifiers remain literal accepted strings, documenting the naming boundary.
- Verify a declared string context value of "" is accepted, and demonstrate a consumer prefix rule matches all resources unless it guards emptiness.
- Compile a variable-key context atom with a mismatched constant kind and assert it cannot match well-typed input; protect the distinction between permissive rule checking and strict request ingestion.

Validation: source and existing test inspection only; no new executable regression tests added.
