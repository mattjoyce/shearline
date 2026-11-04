# 005 Let a policy declare its own builtins

Asked for by: Matt, from the Cedar comparison (2026-11-04). Cedar has extension types for IP
ranges, decimals, dates and durations. tarski already has the mechanism (`Schema.Builtins`);
Shearline does not pass it through.
Owner: shearline-lead. Status: BACKLOG. Release: additive (one Config field).
Written in ASD-STE100.

## Job story

When a rule depends on a value that facts cannot express (an address is in a network, a date
is before another date, a duration is under a limit), I want to give Shearline a Go function
that answers it, so the rule can read it as a relation and the proof shows that it held.

## Now

`policy.Config` has `Standing`, `Sets`, `Context` and `Relations`. `schemaWithRelations`
(`policy/facts.go`) builds a `tarski.Schema` with `Arities` and `Checks` only. A rule can use
the three engine builtins (`prefix`, `suffix`, `contains`) and nothing else. A consumer that
needs "in network" today puts a precomputed fact in `Standing` for every (principal, network)
pair, or does the check outside Shearline, where the proof cannot see it.

## Change

1. Add to `Config`:
   ```go
   // Builtins are relations answered in Go, not by facts: guards a rule reads after its
   // arguments are bound. See tarski.Builtin for the contract (pure, total, deterministic,
   // safe for concurrent use). A nil map declares none.
   Builtins map[string]tarski.Builtin
   ```
2. `Load` copies it into the Schema that `schemaWithRelations` builds (`Schema.Builtins`).
3. `Load` refuses, with `ErrInvalid`: a builtin named like a PARC relation, a declared
   consumer relation, or an engine builtin; a builtin with no `Holds`; a builtin with no
   `Args`. The error names the builtin.
4. Ship four builtins in a new package `policy/builtins`, each a `tarski.Builtin` value the
   consumer puts in the map under a name of its choice:
   - `InCIDR` — `(Addr, Network)`: Addr is an IP in Network (CIDR). `net/netip`.
   - `Before` — `(A, B)`: A is before B, both RFC 3339. `time`.
   - `Within` — `(A, B, Duration)`: B minus A is at most Duration (Go duration text).
   - `DecimalLess` — `(A, B)`: A < B as decimals with no float; strings, since tarski
     numbers are exact already. Drop this one if tarski numbers already cover it: check first.
   Each has a `Check` that refuses a constant of the wrong shape at Load.
5. Doc comment on `Builtins`: a builtin binds nothing, so every argument must be bound by a
   positive atom first; the proof records the builtin's atom as a leaf that held; the
   why-not records it as the literal that failed.

## Acceptance

1. A rule `allow(P, A, R) :- permit(P, A, R), context("ip", IP), in_net(IP, "10.0.0.0/8").`
   with `in_net: builtins.InCIDR` allows from 10.1.2.3 and denies from 192.168.0.1, and the
   deny's why-not stops at `in_net`.
2. `Load` refuses a `Builtins` key of `prefix`, `context`, or a declared relation name.
3. `Load` refuses a builtin with a `Check` that rejects a constant written in a rule
   (`in_net(IP, "not-a-cidr")`), and the error names the line.
4. A policy with builtins gives the same answer and hash on repeated decides (determinism).
5. The four shipped builtins have table tests, with wrong-kind and malformed inputs false,
   never a panic.
6. README gains one line under "Extend with your own rules".

## Out of scope

- Builtins that bind (compute a value). tarski has none, and this card adds none.
- Builtins in `Permitted`. It lists grant prefixes and reads no context, so a builtin over
  context cannot apply there. Say so in the doc.

## Open questions

1. Is a builtin part of the policy's identity? The rule set hash covers the rule text, not the
   Go function. Two consumers with the same `.dl` and a different `in_net` get the same
   `RuleSets` IDs and different answers. Options: leave it and document it; or add the
   builtin names (not the functions) to the Decision. I lean to document it: the function is
   code, reviewed like code.
