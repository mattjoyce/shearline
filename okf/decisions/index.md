# Decision

* [Decision 1: resource matching](1-resource-matching.md) - Grants stay plain string prefixes and a resource name is an opaque string; no glob, no canonicalisation; under/3 is the planned answer to segmented names.
* [Decision 2: a proof names its rule set](2-proof-names-its-rule-set.md) - Rule gains Set and Text so a proof says which set, which line and what text; the hash stays in Decision.RuleSets.
* [Decision 3: a refusal explains itself](3-a-refusal-explains-itself.md) - Decide fills Decision.Why with engine-level why-not provenance, deterministic and RBAC-blind, with the blocking deny's proof when deny/3 fired.
* [Decision 4: context keys are declared](4-context-keys-declared.md) - Load takes Config.Context mapping each key to its kind; an undeclared key or wrong kind is ErrRequest, and the declaration is the schema check on context/2.
* [Decision 5: the inverse listing is the loop](5-inverse-listing-is-the-loop.md) - Who may do A on R is answered by calling Decide once per known principal; no Principals call and no second rule set is planned.
* [Decision 6: a relation is a value or a builder](6-relation-is-value-or-builder.md) - During evaluation every relation is a builder; when Evaluate returns each is sealed into a value with no write method, so reads are safe by construction.
