# zendis — Tracking

Version: 0.8.2 · Last reviewed: 2026-11-16

## Status

| ID | Summary | Theme | Priority | Status | Blocks |
|----|---------|-------|----------|--------|--------|
| T-3 | No round-trip verification harness against zenas | verification | P1 | ◐ | confidence in DD/FD table |
| T-05 | No code/data discrimination -- linear sweep decodes data regions as garbage instructions | traversal | P1 | ◐ | — |
| T-06 | No control-flow-guided disassembly -- no traversal from entry points following JP/CALL/JR targets | traversal | P2 | ☐ | — |
| T-07 | No symbol-file input or annotation mechanism for user-supplied names and comments | cli | P2 | ◐ | — |
| T-09 | No fuzzing or property-based tests for decoder correctness | verification | P1 | ☐ | — |
| T-16 | Symbols name only branch targets: data operands stay numbers | cli | P2 | ☐ | — |
| T-17 | Symbol files: physical addresses unused, two of CSpect's formats unread, CSpect and z88dk not tried | cli | P3 | ☐ | — |
| T-18 | LDIRSCALE (ED B6): zenas assembles it, zendis does not decode it | decoder | P3 | ☐ | — |
| T-20 | Data kinds: pasmo and SNASM get numbers only | cli | P3 | ☐ | — |
| T-21 | NEX input: one bank at a time; screens, palette and copper not read | cli | P3 | ☐ | — |

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
through zenas to the exact original bytes. 0.5.0 adds `--symbols`,
which reads other tools' symbol files into the same labels
(`pkg/symfile`); what that leaves out is T-16 and T-17.

Trigger: identified during the same review as T-05/T-06; distinct from T-2 in that T-2 is about generating labels, this is about consuming and preserving user-supplied ones.

### T-16. Symbols name only branch targets: data operands stay numbers

Theme: cli · Priority: P2 · Status: ☐

- **Trigger:** 0.5.0, `--symbols`. A symbol file names data as well as code (`table`, `SPRITEDATA`, a system variable), and the disassembly uses a name only where JP, JR, CALL or DJNZ go to it: `LD HL,0x800F` stays a number although `table` is at 0x800F and is declared on its own line.
- **Why it is not simply done:** a 16-bit operand is not always an address (`LD BC,0x8000` as a count), so naming every operand that equals a symbol would put wrong names in. In a re-assemblable format the bytes come out the same either way; in a listing it misleads.
- **Fix:** name the operands that are addresses by their form first (`LD A,(nn)`, `LD (nn),HL`, `LD HL,(nn)`), then immediates under an option, and only for symbols that came from a file or the hints, never generated ones. Port numbers and 8-bit constants from a map's EQUs are a further step.
- **Done when:** a program disassembled with its own symbol file shows its data labels in the operands that address them, and the round trip through zenas is still byte-identical.

### T-17. Symbol files: physical addresses unused, two of CSpect's formats unread, CSpect and z88dk not tried

Theme: cli · Priority: P3 · Status: ☐

- **Trigger:** 0.5.0, `--symbols` and `--map`.
- **Not there:** the physical address of a cspect map is read and kept (`symfile.Symbol.Phys`) and nothing uses it: a symbol in a page that is not in the default paging is taken by its 16-bit address alone, and two symbols in different pages at the same address collide (the first by name wins). `--map` writes every label with the default paging, whatever `--bank` or a snapshot's own paging says. The `[FILES]` and `[LINES]` sections of SNASM's maps (source lines for addresses) are skipped. Boriel BASIC's map and Pasta/80's listing, which CSpect also loads, are not read: no file of either was at hand to take the format from.
- **Not tried:** a map written by `--map` loaded into CSpect; a `.map` written by z88dk (the reader follows the documented shape); the `EQU` lines for outside branch targets in pasmo and SNASM (guard G-02 was not run at 0.5.0).
- **Done when:** a zendis map has been seen working in CSpect's debugger, the z88dk reader has read a real file, and G-02 has run with a case that has an outside branch target.

### T-20. Data kinds: pasmo and SNASM get numbers only

Theme: cli · Priority: P3 · Status: ☐

0.8.0 writes a data range as words (DW), text (DB of strings) or a fill (DS count,value) for zenas and sjasmplus, where each kind was assembled back to the same bytes. For --format=pasmo and --format=snasm every kind is written as DB of numbers (render.EffectiveDataKind): neither assembler was at hand to try its DW, DS and string syntax, and a wrong guess assembles to wrong bytes without an error. To do: try them with tools/dialectcheck (pasmo) and with SNASM, then let EffectiveDataKind pass the kinds that hold.

Trigger: found while adding data kinds for 0.8.0.

### T-21. NEX input: one bank at a time; screens, palette and copper not read

Theme: cli · Priority: P3 · Status: ☐

--nex (0.8.0) disassembles the view a NEX program starts with, or one bank or 8K page at an address given with --bank-at. Not there: a view made of several banks the program pages in (an MMU map given on the command line, a bank per slot); the file's loading screens, palette and copper code, which load.DescribeNEX does not list; physical addresses of a CSpect map to name things in the bank they are in (T-17).

Trigger: scope left out of --nex in 0.8.0.

## decoder

### T-18. LDIRSCALE (ED B6): zenas assembles it, zendis does not decode it

Theme: decoder · Priority: P3 · Status: ☐

- **Trigger:** 0.5.2, tidying zenas and zendis together. zenas (0.11.x on) assembles `LDIRSCALE` as `ED B6` under `--next` and lists it among 30 Z80N instructions; zendis decodes 29 and leaves `ED B6` undefined under `--target=z80n`.
- **What is known:** sjasmplus 1.24.1 rejects `LDIRSCALE`; CSpect 4.5.1.0's ReadMe, which lists the Z80N opcodes, does not have it; zen80 0.5.6, the CPU core, does not execute it. zenas's own note says it decodes as a plain repeating copy on the machine. No Next core source or hardware was consulted for this item.
- **Effect:** none on bytes: a re-assemblable format writes `DB 0xED,0xB6` and the round trip is exact. A listing shows `???` where zenas's source had a mnemonic.
- **Done when:** it is settled whether current Next cores have the instruction, and zenas, zendis and zen80 agree: either zendis decodes it and zen80 executes it, or zenas stops offering it.

## traversal

### T-05. No code/data discrimination -- linear sweep decodes data regions as garbage instructions

Theme: traversal · Priority: P1 · Status: ◐ · Blocks/after: —

zendis disassembles linearly from byte zero as if every byte were code. A binary containing embedded data tables, string literals, or other non-code regions will have those bytes "decoded" as plausible-looking but meaningless instructions rather than flagged as data, since there's currently no mechanism -- heuristic or user-supplied -- for marking a byte range as non-code.

Status note: implemented via T-07's hints file (`data <addr> <length>` directive) -- the user-supplied half of the "heuristic or user-supplied" framing above; no heuristic (data inferred without a hint) was attempted. The CLI's decode loop checks every address against the hints file's data ranges before attempting to decode, and renders a match as DB lines chunked into 4-byte rows (matching the widest instruction, so the listing's hex column stays aligned) rather than mis-decoding it. Unit-tested (main_test.go's TestPrintListing_HintsIntegration) and proven to round-trip through zenas as part of T-07's CLI-level round-trip check (tools/roundtrip). 0.8.0 adds the kinds a range is written in (bytes, hex, words, text, fill), `--data` for a range without a hints file and `--as` for a whole block; still no heuristic.

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
currently-published zen80 v0.5.6. Both hard-coded exceptions are
independently correct in zendis (confirmed by decode-only assertions
in the same file), and now their live oracle checks assert directly
too, not just decode-only: zen80 v0.5.6 carries the fix for both (DD
76 panicking, DD EB substituting IX into EX DE,HL) that v0.5.5 had --
zendis's go.mod bumped to it, and both checks turned from
recover-and-log/detect-and-skip into real assertions, verified passing
against the real published module, not the local clone this was
originally found and fixed against.

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

## formats

