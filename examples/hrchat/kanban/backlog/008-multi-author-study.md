# 008 Study: one English policy, many authors, compare the rule sets

Owner: hrchat. Needs: Shearline `Compare` (built); card 007's request space is useful but not
required. Written in ASD-STE100. This is a study, not a feature: its output is a note.

## Question

Is `hrchat.dl` the policy a reader gets from the English, or is it one of several? If several
models write a rule set from the same ASD-STE100 policy and they decide the same, the English
is unambiguous and the `.dl` is a faithful translation. Where they differ, the English has a
gap, or the translation has a slip, and the diff names the request.

## Method

1. Write the policy in ASD-STE100: one rule per sentence, present tense, one meaning. Start from
   the comments in `hrchat.dl` and `entitlements.go`. This document is the input; keep it in
   `docs/POLICY.md`. (It is also the document a non-engineer signs off. Card 007 wants it too.)
2. Give the English, the relation list (from `Relations()`), the context keys, and Shearline's
   `rbac/1` text to each author. Ask for `hrchat/1` as a rule set. Authors: four models from
   different vendors, plus the existing hand-written `hrchat.dl` as author zero. No author
   sees another author's output.
3. Load each one with the same facts. A rule set that does not load is a result: record the
   error.
4. `Compare` each pair over card 007's sample (or, before 007, over the 39 scenario requests
   and a grid like shearline card 009's hrchat test). Record the diff count per pair and each diff with
   both `Explain` lines.
5. For each diff, classify: English gap (two readings are both faithful), translation slip (one
   reading is not faithful), or Shearline gap (the English says something the language cannot).
6. Fix the English for each gap. Run again. Stop when every diff is a slip, or after three
   rounds.

## Deliverable

A note in `docs/MULTI-AUTHOR.md`: the English policy version used, the authors, the diff matrix
per round, each diff classified, and the English changes made. Plus `docs/POLICY.md` as the
signed-off policy. The rule sets themselves go in `docs/authors/` for the record, not in the
build.

## Acceptance

1. The matrix exists for at least one round with at least three authors.
2. Every diff is classified, and every English gap has a change.
3. The note says, in one sentence, what the study found about `hrchat.dl`'s readability.

## Out of scope

- Replacing `hrchat.dl` with a model's version. The study measures the English and the
  translation; the shipped rule set stays the reviewed one.
- A tool that runs this on demand. If the study is worth repeating, that is a later card.
