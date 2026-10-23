# Directory Update Log

## 2026-10-22
* **Creation**: Established the bundle: [datalog](/packages/datalog.md) and [policy](/packages/policy.md) packages, ten concepts, eight API entries, the [RBAC rule set](/rulesets/rbac.md), six decisions, three reviews and the [engineering standards](/standards/engineering.md).
* **Note**: Written from the code as committed at 3c287fb; docs/DESIGN.md was reconciled to the code the same day.

## 2026-10-23
* **Addition**: [logsvc](/examples/logsvc.md), a worked example of embedding Shearline with YAML-defined roles and tokens; CI now builds and tests it as its own module.
* **Addition**: [Red/blue file audit](/reviews/file-red-blue-2026-10-22.md), resolved on master. `Answer` is now immutable with accessors; `Plan.Defines` is new; proof JSON writes a shared sub-proof once; string literals read JSON or Go escapes; `Number` is JSON's grammar.
