# Bonsai Design Philosophy

## Why bonsai exists

AI agents are non-deterministic. Given the same prompt and the same
context, a model may produce subtly different output on every run.
This is not a bug; it is a structural property of autoregressive
generation with temperature > 0 (and even at temperature 0, batching
and floating-point non-associativity introduce variance).

The consequence for governance is severe: you cannot rely on the model
"choosing" to comply with repository conventions. A rule stated in a
system prompt is obeyed probabilistically, not deterministically. The
larger and more complex the ruleset, the lower the probability that
every rule is satisfied on a single pass. Repeating instructions,
bolding them, or prefixing them with "IMPORTANT" shifts the
probability but does not eliminate the failure mode.

Bonsai exists because governance of AI-generated code requires
_external enforcement_, not self-policing. The model is the generator;
something outside the model must be the validator.


## Enforcement model

The single source of truth for any governance concern is a **SKILL.md**
file. Each skill encodes one validation domain: commit message format,
file-header compliance, dependency rules, and so on. SKILL.md is
written in structured prose that is both human-readable and
machine-parseable (criteria are extracted programmatically).

Bonsai applies two enforcement strategies depending on the command
type:

**Dual enforcement** (code-generating commands):
1. **Pre-injection.** Before the code-generating model is invoked,
   bonsai extracts the enforceable criteria from every relevant
   SKILL.md and injects them into the role prompt. The model sees the
   rules at generation time, which raises first-pass compliance
   probability.
2. **Post-hoc gate.** After generation, bonsai runs each relevant
   skill against the output. Skills return pass/fail with structured
   diagnostics. On failure, the diagnostics are fed back to the model
   for a corrective iteration, up to a configurable maximum.

**Single enforcement** (validation-only commands):
1. **Post-hoc only.** The gate runs skills against existing content
   (a diff, a branch, a working tree). There is no generation step,
   so pre-injection is inapplicable.

This dual-loop architecture means code-generating workflows get both
a "nudge" (pre-injection) and a "check" (post-hoc gate). Validation
workflows get the check alone.


## Token economics

Bonsai must cost fewer tokens than the two naive alternatives:

- **Verbose prompting:** Pasting entire SKILL.md documents into the
  system prompt. This bloats context with prose the model does not
  need for generation, wastes input tokens on every call, and still
  provides no enforcement guarantee.
- **Blind retry:** Running the generator, checking manually, and
  re-running on failure with the full conversation context growing
  each iteration.

Bonsai controls token spend at each stage:

**Pre-injection is compact.** Criteria are extracted from SKILL.md as
terse, enumerated rules (~500 tokens for a typical skill). The full
prose, rationale, and examples in SKILL.md are not injected. The model
receives only what it needs to shift first-pass compliance upward.

**Post-hoc runs cheapest skills first.** Skills declare a cost hint.
The gate sorts by ascending cost and applies fail-fast: if a cheap
skill (e.g., regex-based header check) fails, expensive skills
(e.g., LLM-judge coherence review) are never invoked. The failing
diagnostic is returned immediately.

**The gate loop maximizes first-pass success to minimize iterations.**
Because pre-injection already raises compliance, the post-hoc gate
typically passes on the first or second iteration. Each additional
iteration is a full generate-then-validate round-trip, so avoiding
even one saves the most tokens.


## Latency constraints

Every layer bonsai adds must justify its wall-clock cost:

**Pre-injection adds zero API calls.** Criteria are embedded text,
resolved at prompt-assembly time by string interpolation. There is no
network round-trip, no model call, no file fetch beyond reading the
local SKILL.md. The latency contribution is negligible (sub-millisecond).

**Post-hoc skills are cost-sorted.** Cheap deterministic skills
(regex, AST checks, line-count validators) run before expensive
non-deterministic skills (LLM-as-judge). Fail-fast means the
expected-case latency is dominated by the first failing skill, not
the sum of all skills.

**No protocol may add orders-of-magnitude time.** If a skill takes
10 seconds and the base generation takes 5 seconds, that is
acceptable (same order of magnitude). If a skill takes 5 minutes,
it must be restructured or made optional. Bonsai is not a CI pipeline;
it runs in the inner loop of an interactive agent session.


## Current limitations

The generate-then-validate loop is **probabilistic, not deterministic**.
Pre-injection raises compliance probability; the gate catches
violations and feeds back corrections. But there is no formal proof
that the model will converge to a compliant output within any finite
number of iterations. In practice, convergence is typical within 1-3
iterations for well-scoped skills, but pathological cases exist:
contradictory criteria, ambiguous natural-language rules, or model
capability gaps.

**There is no convergence guarantee within the maximum iteration
count.** When the gate loop exhausts its budget, bonsai reports
failure and surfaces the remaining diagnostics. It does not silently
accept non-compliant output.

Bonsai is, frankly, a **band-aid framework** given current model
capabilities. It compensates for the fact that models cannot
self-validate reliably by externalizing the validation. This is
necessary today but should not be necessary forever.


## Long-term vision

The correct architecture is **progressive validation**: the
equivalent of lint-on-save, applied during generation rather than
after it. Instead of letting the model produce a complete output
and then checking it, the validator would run incrementally as tokens
are emitted, flagging violations in real time and steering generation
before non-compliant patterns solidify.

This requires **agent backend integration** that does not exist in
current tooling. Possible substrates include:

- MCP (Model Context Protocol) hooks that expose a validation
  callback invoked on partial output.
- IDE-level integration where the agent's output buffer is
  continuously linted and violations are injected as inline context.
- Custom sampling loops that interleave generation and validation
  at the token or line level.

Progressive validation is the **only model that does not rely on
"hoping" the AI follows rules**. Pre-injection hopes; post-hoc
catches failures after the fact. Progressive validation structurally
prevents non-compliant output from completing, the same way a type
checker prevents ill-typed programs from compiling.

Until that infrastructure exists, bonsai's dual-enforcement loop is
the pragmatic best option: raise the probability with pre-injection,
catch the remainder with post-hoc gating, and iterate until compliant
or budget-exhausted.
