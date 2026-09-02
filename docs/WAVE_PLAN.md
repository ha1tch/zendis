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


**Wave 5 — tape image input -- TAP and TZX (≈ 2.0d, added 2026-08-30).** T-10 is scoped to a single wave on its own, not merged into the same wave as the snapshot items (T-11/T-12), because tape and snapshot are genuinely different input shapes with different unknowns: a tape image needs block selection and per-format quirks (TZX's many pulse-level block types beyond the common 0x10 case), where a snapshot is a single machine-state blob. Landing this first since zentools's tap/tzx packages are the most directly reusable of the three -- Block.Param1 already gives a load address for a TypeCode header, no format-specific reconstruction needed the way snapshot paging requires.


**Wave 6 — snapshot input, 48K -- SNA and Z80 (≈ 1.5d, added 2026-08-30).** T-11 is the simpler half of snapshot support, kept separate from T-12's paged-memory case specifically because it genuinely is simpler: zentools's own state.go confirms a 48K snapshot's memory maps onto a flat 64K space with no ambiguity at all (banks 5/2/0 fixed at 0x4000/0x8000/0xC000, Paging left zero and never needing resolution). Sequenced after Wave 5 (tape) since it reuses the same CLI-surface shape (org override, range/length control) once that one's built, but before T-12 since 48K is the case with no open design questions -- T-12's bank-selection and paging-boundary questions are real enough to deserve their own wave rather than slow this one down.


**Wave 7 — snapshot input, 128K paged memory (≈ 2.5d, added 2026-08-30).** T-12 is the genuinely harder half of snapshot support, kept separate from Wave 6 rather than an extension of it: the zentools Memory struct always holds all eight 16K banks, and Paging.Port7FFD determines which bank is actually mapped to 0xC000 at the moment of the snapshot -- the other banks are real addressable memory not visible in any single flat view. Two real design questions have to be answered here that do not exist in the 48K case: what a current view even means (resolve Port7FFD, reconstruct what the CPU would see if resumed), and how a caller selects a bank that is not currently paged in at all. Sequenced last among the three format waves since it depends on nothing else finishing first, but is the one most likely to need a design pass before implementation starts.


**Wave 8 — tape image input, PZX (≈ 1.5d, added 2026-08-31).** zentools v0.8.0 added PZX support (Patrik Rak's simpler TZX successor); this closes the gap between what zentools can parse and what zendis can load. Extraction is deliberately more permissive than zentools's own PZX -> TAP reconstruction: no pulse-timing check applies, since a DATA block's payload is already resolved bytes regardless of the timing that got it onto tape.


**Wave 9 — snapshot input, SZX (≈ 1.0d, added 2026-08-31).** zentools v0.8.0 added SZX support. Genuinely simple to add: szx.Decode produces the same *snapshot.MachineState type SNA/Z80 decoding already does, so this needed no new flattening logic at all -- just a new decode entry point dispatching into what already existed.


**Wave 10 — input-recording support, RZX (≈ 1.5d, added 2026-08-31).** zentools v0.8.0 added RZX support. The most interesting of the three additions: a genuinely new input category, not just another format zentools happens to parse -- lets someone disassemble code straight from a TAS/competition recording without first manually extracting the snapshot. Filed alongside a related fix to the already-closed T-10: FromTZX now reads 0x11/0x14 blocks, not just 0x10, closing a real gap for any tape using a turbo loader.

