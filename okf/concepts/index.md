# Concept

* [Constants](constants.md) - The engine's values: three scalar kinds, a UTF-8 string domain, finite float64 numbers with -0 folded to 0, and no exact integers beyond 2^53.
* [Declared context](declared-context.md) - Every context key a request may carry is declared with its kind in Config.Context, or the request is refused; the declaration is also the schema check on context/2.
* [Fail-closed decision](fail-closed-decision.md) - Every error is a deny: Decide never returns Allow=true with an error, and an absent allow/3 tuple is a deny with a nil error.
* [Per-request evaluation](per-request-evaluation.md) - The bounds on an evaluation are per request, so the bundled rules are seeded by the request; the 38k-pair cliff is the story of why.
* [Prefix grants](prefix-grants.md) - Grants are plain string prefixes with no path semantics; deny is exact; the consumer ends prefixes with its separator and canonicalises names first.
* [Proof](proof.md) - A proof is the first derivation of a tuple, shared as a DAG of values, deterministic, and naming its rule by set, line and text.
* [Rule-set identity](rule-set-identity.md) - A rule set is a named, versioned program with a sha256 over its text; Verify re-parses so the rules that ran are the rules that were hashed.
* [Stratified negation](stratified-negation.md) - What the engine's Datalog is and is not: no function symbols, built-ins as guards, stratified negation, and a hook mechanism for declared-but-undefined relations.
* [Value and builder](value-and-builder.md) - A relation is a builder while evaluation runs and a sealed value afterwards, never both; Base and Result hold values only, so reads are safe by construction.
* [Why-not provenance](why-not-provenance.md) - How a refusal explains itself: the farthest literal each candidate rule reached, a Truncated flag for the bounded walk, and Blocked for a negation that held.
