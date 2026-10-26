# Directory Update Log

## 2026-10-22
* **Creation**: Established the bundle: [datalog](/packages/datalog.md) and [policy](/packages/policy.md) packages, ten concepts, eight API entries, the [RBAC rule set](/rulesets/rbac.md), six decisions, three reviews and the [engineering standards](/standards/engineering.md).
* **Note**: Written from the code as committed at 3c287fb; docs/DESIGN.md was reconciled to the code the same day.

## 2026-10-23
* **Addition**: [logsvc](/examples/logsvc.md), a worked example of embedding Shearline with YAML-defined roles and tokens; CI now builds and tests it as its own module.
* **Addition**: [ductile](/examples/ductile.md), SPEC step 7: Ductile's scopes as a policy, parity with Ductile's own semantics pinned as a test; answers open question 5 (no context key).
* **Addition**: [Red/blue file audit](/reviews/file-red-blue-2026-10-22.md), resolved on master. `Answer` is now immutable with accessors; `Plan.Defines` is new; proof JSON writes a shared sub-proof once; string literals read JSON or Go escapes; `Number` is JSON's grammar.

## 2026-10-26
* **Change**: The engine moved out to [tarski](/packages/datalog.md) v0.2.2 and the `datalog/` package was deleted. `policy.Load` now refuses rule sets that use aggregates. The engine concept and API entries carry a pointer note; code citations point at the tarski tag. Review citations were repointed to the tarski tag; their text is unchanged.
* **Change**: True-up after the merge. Citations and pointer notes now name tarski v0.2.2. The constants entry says numbers are exact decimals, and the evaluation entries say what a step is since v0.2.1 and v0.2.2 (a tuple visited, a rule applied, a built-in called). The stratified-negation entry says the engine has aggregates and policy refuses them. The logsvc entry explains the `replace ../..`, and the engineering standards list the example's CI step. The review entries are untouched.
* **Change**: Shearline v0.2.0 tagged on tarski v0.2.2. The [logsvc example](/examples/logsvc.md) now requires that release; its `replace ../..` is gone.
* **Addition**: `CHANGELOG.md` at the repo root, with entries for v0.1.0 and v0.2.0, and a Changes section in the README that points to it.
