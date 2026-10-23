# ductile: Ductile's token scopes as a Shearline policy

SPEC.md step 7 said: load Ductile's tokens and config as standing facts, ask requests as
`Request`, and see whether an assumption shaped by Anam leaks. This directory is that
exercise, with the answer pinned as a test.

```
go run .                                       # every token against every route
go run . readonly POST '/pipeline/{pipeline}'  # one deny, with the reason
go run . operator POST '/pipeline/{pipeline}'  # one allow, with the proof
go test .
```

## The shape

Ductile ([`internal/auth`](https://github.com/mattjoyce/ductile)) holds a flat scope set per
token and a hand-written implication table; each route lists the scopes that reach it.
Here:

- a **scope is a role**, and the implication table is `Inherits` facts (`scopes.go`,
  `Implies`), the wildcard a role that inherits the four read-write scopes;
- a **route is a grant** of an action (`read` or `write`) on a capability (`jobs/status`,
  `plugin/invoke`) to the narrowest scope that reaches it; the wider scopes arrive by
  inheritance, so the table is written once, not once per scope;
- a **token is `Role` facts**, one per scope it holds;
- a **request is PARC** with no context: the router already knows the route, and whether a
  plugin command is read-class or write-class, before it asks.

The resource is a capability, not a path, because paths are prefixes of each other
(`/job/{jobID}` of `/job/{jobID}/tree`) and capabilities are not. Ductile's per-field
response shaping (its D1 rule) is a capability too, `jobs/result`, decided once per
response.

## What the exercise found

- Nothing Anam-shaped leaks. The bundled `rbac.dl` expresses the lattice, the grants and
  the wildcard without a consumer rule set, and `TestParityWithDuctile` holds the model to
  a transcription of Ductile's own `normalizeScopes` and `HasAnyScope` on every scope,
  every pair of scopes and every route: 154 tokens, 19 rows, no disagreement.
- Ductile's request shape (SPEC open question 5) is principal, action, resource, and no
  context key. `Config.Context` is unused.
- A denial comes back with the reason. Ductile's 403 today says "insufficient scope"; the
  `Why` for the same request names the rule, the literal it stopped at, and the fact that
  was missing.
- A scope no route names is refused at load (`TestFacts_RefusesUnknownScope`). In Ductile it
  is a scope that silently grants nothing.
