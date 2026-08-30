# zendis — Wave Plan

Updated: 2026-08-28

Short rationale paragraphs for each staged wave, in the order waves
are added. Per-item detail and status live in `docs/WAVE_TRACKING.md`;
this document exists to record *why* a wave was sequenced the way it
was, not to duplicate its item table.

**Wave 1 — baseline features (≈ 4.0d, added 2026-08-29).** The four items filed while scoping the initial decoder are the baseline the decoder needs before it can be called functionally complete and independently trustworthy: full Z80N coverage (T-1), a usable CLI listing (T-2), and cross-checking the two riskiest tables against something other than this decoder's own author's memory (T-3, T-4). Grouped as one wave rather than four unrelated items since none of them is urgent alone, but the decoder is not really 'done' until this wave closes.


**Wave 2 — traversal -- code/data classification and control-flow discovery (≈ 3.0d, added 2026-08-29).** T-05 and T-06 are the two halves of the same underlying gap: zendis currently has no notion of what is and isn't code. Data marking (T-05) is the simpler, more urgent half on its own -- it stops obviously-wrong output on data regions even without traversal. Control-flow-guided walking (T-06) is the larger feature and is what actually lets T-05's marking be discovered automatically rather than hand-supplied for every binary. Sequenced as one wave since neither is very useful shipped alone: marking without traversal still requires the user to know where the data is by hand, and traversal without marking still walks into the data it finds.


**Wave 3 — cli input/output flexibility -- symbol annotation and alternate output (≈ 2.5d, added 2026-08-29).** T-07 and T-08 are grouped with T-2 under the cli theme, but are their own wave rather than folded into Wave 1 since Wave 1 was scoped and closed off during initial implementation and these two were identified later, in a separate review of the project against what a complete disassembler needs. Both concern what the CLI can take in or put out beyond the current single hardcoded listing; T-07 (symbol input) is the more immediately useful of the two since it's what makes a real reverse-engineering session usable across multiple runs, so it's listed first.


**Wave 4 — decoder verification rigor -- fuzz and property testing (≈ 2.0d, added 2026-08-29).** T-09 shares its theme with T-3 and T-4 (both already open in Wave 1) but is scoped separately as its own wave: T-3/T-4 are about cross-checking two specific tables against an independent reference, while T-09 is a general-purpose harness that exercises the whole decoder, including tables T-3/T-4 don't touch. Kept as a single-item wave rather than merged into Wave 1 since Wave 1 was closed off at initial-implementation scope and this was identified later; it can still land before or after Wave 1 closes -- there's no ordering dependency between them.

