# zendis — Tracking

Version: 0.3.1 · Last reviewed: 2026-08-30

## Status

| ID | Summary | Theme | Priority | Status | Blocks |
|----|---------|-------|----------|--------|--------|
| T-3 | No round-trip verification harness against zenas | verification | P1 | ◐ | confidence in DD/FD table |
| T-05 | No code/data discrimination -- linear sweep decodes data regions as garbage instructions | traversal | P1 | ◐ | — |
| T-06 | No control-flow-guided disassembly -- no traversal from entry points following JP/CALL/JR targets | traversal | P2 | ☐ | — |
| T-07 | No symbol-file input or annotation mechanism for user-supplied names and comments | cli | P2 | ◐ | — |
| T-09 | No fuzzing or property-based tests for decoder correctness | verification | P1 | ☐ | — |

## cli

### T-07. No symbol-file input or annotation mechanism for user-supplied names and comments

Theme: cli · Priority: P2 · Status: ◐ · Blocks/after: —

Even once T-2 (label inference) resolves branch targets to generated labels, there's no way to supply zendis with externally-known symbol names, or attach comments/annotations to specific addresses in the listing. A complete disassembler workflow typically needs both: user-supplied names for known routines and inline notes that survive re-runs.

Status note: implemented as `pkg/disasm.Hints` (`hints.go`), a general
hints file (`label`/`comment`/`data`/`entry` directives) rather than a
label-only format -- deliberately scoped broadly so T-05's data-region
marking could consume the same mechanism, which it now does, and so a
future control-flow traversal item could consume `entry` for extra
entry points without a second, incompatible format (that directive is
parsed and stored today; nothing consumes it yet). Exported at the
library level, not CLI-only, so an embedder can reuse the same parser.
CLI wiring: `--hints=<path>`; a user-supplied label always overrides
the generated `Lxxxx` form, independent of `--labels`. Verified two
ways: unit tests (hints_test.go, main_test.go) and a permanent
CLI-level round-trip check in tools/roundtrip that builds the real
zendis binary, runs it with a hints file exercising all three
directives, and confirms the `--format=plain` output reassembles
through zenas to the exact original bytes.

Trigger: identified during the same review as T-05/T-06; distinct from T-2 in that T-2 is about generating labels, this is about consuming and preserving user-supplied ones.

## decoder

## traversal

### T-05. No code/data discrimination -- linear sweep decodes data regions as garbage instructions

Theme: traversal · Priority: P1 · Status: ◐ · Blocks/after: —

zendis disassembles linearly from byte zero as if every byte were code. A binary containing embedded data tables, string literals, or other non-code regions will have those bytes "decoded" as plausible-looking but meaningless instructions rather than flagged as data, since there's currently no mechanism -- heuristic or user-supplied -- for marking a byte range as non-code.

Status note: implemented via T-07's hints file (`data <addr> <length>` directive) -- the user-supplied half of the "heuristic or user-supplied" framing above; no heuristic (data inferred without a hint) was attempted. The CLI's decode loop checks every address against the hints file's data ranges before attempting to decode, and renders a match as DB lines chunked into 4-byte rows (matching the widest instruction, so the listing's hex column stays aligned) rather than mis-decoding it. Unit-tested (main_test.go's TestPrintListing_HintsIntegration) and proven to round-trip through zenas as part of T-07's CLI-level round-trip check (tools/roundtrip).

Trigger: identified while reviewing the decoder against what a complete disassembler needs, comparing current scope against the original T-1 to T-4 items filed during initial implementation scoping.

### T-06. No control-flow-guided disassembly -- no traversal from entry points following JP/CALL/JR targets

Theme: traversal · Priority: P2 · Status: ☐ · Blocks/after: —

The CLI performs a single linear sweep from the --org address; there's no traversal that follows JP/CALL/JR/DJNZ targets to discover reachable code from one or more entry points. Most complete disassemblers use this kind of control-flow-guided walk (often combined with T-05's data marking) to avoid decoding through unreachable or data bytes that a purely linear sweep can't distinguish from real instructions.

Trigger: same review as T-05; the two are closely related but separately scoped pieces of work -- T-05 is about marking known-data regions, this is about discovering code regions by traversal.

## verification

### T-3. No round-trip verification harness against zenas

Theme: verification · Priority: P1 · Status: ◐ · Blocks/after: confidence in DD/FD table

The DD/FD substitution rules (which opcodes get HL/H/L/(HL) replaced,
the two documented exceptions currently hard-coded — HALT and
EX DE,HL — and the LD H,(IX+d)/LD L,(IX+d) literal-register exception)
were previously verified only against hand-written unit tests. A
targeted oracle harness (pkg/disasm/decode_ddfd_oracle_test.go) now
cross-checks the substitution *decision* against zen80's real DD/FD
dispatch: the mechanical cases (half-register forms, indexed-memory
addressing, the DDCB rotate sub-form) confirm clean against the
currently-published zen80 v0.5.5. Both hard-coded exceptions are
independently correct in zendis (confirmed by decode-only assertions
in the same file) but their live oracle checks currently degrade
gracefully (recover-and-log for HALT, detect-and-skip for EX DE,HL)
rather than asserting, since v0.5.5 itself has confirmed bugs on both
-- fixed in the local zen80 clone, not yet in a published release (see
[[repoman]] memory / the conversation this was found in). Those two
checks become real assertions once zendis's go.mod bumps to a zen80
release carrying the fix.

Status note: substitution-decision boundary verified (exceptions +
half-register + indexed-memory + DDCB, both DD/IX and FD/IY paths).
Not attempted: an exhaustive sweep of the ~200 general-case substituted
opcodes, or the full DDCB/FDCB combinatorics T-4 also covers. A
zenas-based round-trip harness now also exists (tools/roundtrip,
registered as a dormant guard in docs/KNOWN_ISSUES.md) as a second,
independent verification axis alongside the zen80 execution oracle.

Trigger: flagged during initial implementation — this is the single
area of the opcode space where a subtle table error is most likely and
least likely to be caught by casual use, since wrong output still
"looks like" plausible Z80 assembly.

### T-09. No fuzzing or property-based tests for decoder correctness

Theme: verification · Priority: P1 · Status: ☐ · Blocks/after: —

Test coverage is currently 6 hand-written test functions against roughly 950 lines of decode logic across four opcode tables. T-3 and T-4 already flag that the riskiest tables (DD/FD, DDCB/FDCB) lack independent verification; the same underlying gap -- correctness asserted rather than verified -- extends to the decoder as a whole. There's no fuzzing or property-based testing (e.g. "length always matches bytes consumed", "decode never panics on arbitrary input", "every byte sequence produces either a valid instruction or a reported error") despite the decoder being exactly the kind of tabular state machine that benefits from it.

Trigger: identified during the same review as T-05 to T-08; theme matches T-3/T-4 rather than the traversal/cli groups since this is about decoder trust generally, not a specific missing feature.

