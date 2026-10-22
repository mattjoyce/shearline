# Go API

* [datalog.Compile and Schema](datalog-compile-schema.md) - Compile validates programs against a Schema and groups rules into strata; Schema names base relations and optional per-relation checks.
* [datalog.Evaluate and Options](datalog-evaluate-options.md) - Evaluate runs a plan over a base to its fixpoint, semi-naively, bounded by context, MaxBytes and MaxSteps, optionally recording proofs.
* [datalog.NewBase and Base.With](datalog-base-with.md) - NewBase holds facts under a schema; With adds facts as a new value, sharing untouched relations, and equals NewBase over all the facts.
* [datalog.Result, Proof and WhyNot](datalog-result-proof-whynot.md) - Result.Relation returns a hashed Answer; Result.Proof returns the first derivation; Result.WhyNot explains an absence; both render as text and JSON.
* [policy fact constructors and Schema](policy-fact-constructors.md) - Role, Inherits, Grant and Deny build standing facts without learning the engine's vocabulary; Schema is the PARC relations with non-empty-string checks.
* [policy.Config and policy.Load](policy-config-load.md) - Load verifies each rule set, compiles them over the PARC schema, holds the sorted standing facts, and refuses anything Decide or Permitted would fail on.
* [policy.Decide](policy-decide.md) - Decide answers one request with Allow, a Proof or a Why, and the loaded rule-set IDs; it is total, and an error is always a deny.
* [policy.Permitted](policy-permitted.md) - Permitted returns the grant prefixes a principal holds for an action, from the same rules that gate requests; it does not apply deny.
