# 006 Say what Shearline is for

Asked for by: Matt (2026-11-04), after the Cedar comparison.
Owner: shearline-lead. Status: BACKLOG. Release: docs only.
Written in ASD-STE100.

## Why

The README says what Shearline is (PARC over tarski) and how to use it. It does not say when
to choose it, or when to choose Cedar. Matt's idea for Shearline: a standard way to have
flexible rules in the code base. The rules that change (who may do what, under what) are data
with a hash, loaded by one call, decided by one call, and each decision carries its reasons.
The code around them does not change when the rules do. The README must say this, and say
where Cedar is the better choice, so a reader does not look for Cedar features here.

## Change

1. A new section after "Why it is built this way": **When to use Shearline, and when not**.
   Four short paragraphs, each with one claim and one fact:
   - The rules that change live in the code base as data, not as if-chains. A rule set has a
     hash; a decision names it; a change to a rule is a change to the hash, reviewed like code.
   - A decision carries its reasons. The proof says which rules and facts allowed it; the
     why-not says which literal stopped each path. A receipt is the decision, not a log line.
   - Relationships are the policy. A rule may recurse over any relation (a reporting line, a
     delegation chain), not only a fixed hierarchy. `examples/hrchat` shows it.
   - Use Cedar when: you need to prove a property for every possible set of entities (SymCC);
     you need "what may this principal see" as a filter (partial evaluation); you need typed
     entities and schema checking of rule bodies; or you need a language with a large install
     base. Cedar has a Go library. Shearline and Cedar can both run in one program.
2. One sentence at the top of the README, after the first paragraph, that says the standard
   shape: `Load` once, `Decide` per request, a receipt on each decision.
3. `docs/` gets no new file. The section is the whole statement.

## Acceptance

1. A reader who knows Cedar can say from the README which to pick for an HR gate (Shearline,
   for the reasons on each refusal and the reporting-line rule) and for a document share
   (either; Cedar if they need SymCC).
2. No feature claim in the section is beyond what the code does today. Each claim names the
   function or the example that holds it.
3. ASD-STE100: one claim per sentence, present tense, no "powerful", "robust", "simple".

## Out of scope

- A feature comparison table. It goes stale, and it invites a race.
