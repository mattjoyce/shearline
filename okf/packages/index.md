# Go Package

* [Engine (tarski)](datalog.md) - The engine, now the separate module github.com/mattjoyce/tarski (required at v0.3.2): what it owns and what the policy layer relies on.
* [Package policy](policy.md) - The PARC layer over the engine: Load, Decide, Permitted, fact constructors and the bundled RBAC rule set.
