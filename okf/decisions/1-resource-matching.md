---
type: Decision
title: "Decision 1: resource matching"
description: "Grants stay plain string prefixes and a resource name is an opaque string; no glob, no canonicalisation; under/3 is the planned answer to segmented names."
tags: [decision, resources, prefix, matt]
timestamp: 2026-10-22T00:00:00Z
---

# Context

The original spec wrote grants as `"timeline/*"`. A prefix is not a glob, and an engine that knows nothing of vocabulary has no business knowing that `/` separates segments or that `..` climbs. Yet consumers do have segmented names, and prefix matching over them has sharp edges: `"timeline/health"` also matches `"timeline/healthy"`.

# Options considered

| Option | For | Against |
|---|---|---|
| Plain string prefix, consumer canonicalises | Smallest surface; engine stays vocabulary-free | Consumer must end prefixes with its separator and normalise names |
| `glob/2` built-in | Familiar to rule authors | Another matching language to specify and test; `*` semantics are vocabulary |
| Canonicalisation in `policy` | Fewer foot-guns | Names a separator and a case rule; Shearline would own vocabulary |
| `under(Sep, Pat, R)` now | Segment-aware | No consumer has shown segmented names yet |

# Decision

Matt, 2026-10-22: grants stay plain string prefixes and a resource name is an opaque string. The consumer ends a prefix with its separator and makes names canonical before `Decide`. No `glob/2`, no canonicalisation in `policy`. If a consumer shows segmented names, add `under(Sep, Pat, R)`: true when R equals Pat or starts with Pat then Sep, the separator the consumer's, so Shearline still names no vocabulary.

# Rationale

The engine's value is that the derivation is right, not that it knows what a name means. Deferring `under/3` costs nothing until a consumer needs it, and its shape is already fixed so it does not smuggle a separator in.

# Consequences

- `Grant` and `Deny` docs warn about prefix sharp edges; `Deny` is exact.
- The edge audit lists "canonicalise before evaluation, and execute the same target" as an integration edge.
- Grants use the `prefix/2` built-in; the original `"*"` suffix is gone (a deviation recorded in the contract).

See [prefix grants](/concepts/prefix-grants.md), [stratified negation](/concepts/stratified-negation.md) for built-ins, and [fact constructors](/api/policy-fact-constructors.md).

# Citations

[1] [DESIGN.md, Decided with Matt, item 1, and Deviations from the spec](../../docs/DESIGN.md)
[2] [Edge audit, integration edges](../../docs/reviews/edge-audit-2026-10-22.md)
