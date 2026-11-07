# Conversational explanations

Shearline's structured proof and why-not receipts can be supplied to an LLM to explain
a decision conversationally. This applies to any consumer, not just a worked example.

For example, a recorded failure to find a matching approval can be rendered as:
“Your deployment grant is present, but I could not find approval for this artifact.”
Each part of that sentence must be supported by the supplied evidence.

The LLM is a presentation layer: it does not decide, change policy or create evidence.
Keep the original receipt available and fall back to the existing renderer if needed.
Pass only evidence the viewer may inspect, and treat free text in facts as data rather
than instructions. Preserve any truncation marker: why-not is not necessarily an
exhaustive blocker list, and a suggested remedy is not a guarantee of authorization.

If the explanation discusses accountability, supply the corresponding sponsor,
delegation and action records explicitly. A policy receipt alone does not establish
who owns the agent or whether a permitted action was executed successfully.

An [Anam agent record](AGENT-RECORD.md) could supply that history for questions such as
“Why was this allowed yesterday but refused today?” Select the actual recorded decisions
and their input versions; distinguish later corrections from evidence available then.
