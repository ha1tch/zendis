# zendis — Resolved

Closed items, append-only, newest first. Full detail text preserved
verbatim as it stood at closure, stamped with the closing version and
date. `CHANGELOG.md` says what shipped; this says what was wrong and
how it was resolved — the two reference, never duplicate, each other.

## [0.3.0] T-08 — Only one fixed output format -- no alternate assembler dialects or machine-readable output (v0.3.0, 2026-08-30)

Theme: cli · closed 0.3.0 · 2026-08-30
zendis currently emits exactly one fixed listing format (the address/hex/mnemonic columns shown in the README). There's no way to select an alternate output style -- a different assembler's syntax dialect, a machine-readable format for tooling, or a bare-mnemonic form without the address/hex columns.

Status note: all three named forms now implemented, plus symbol-file
export. `--format=listing|plain|zenas|pasmo|sjasmplus|json` --
investigating zenas's, pasmo's, and sjasmplus's own source directly
(not just testing against zenas, and not assuming from docs) found that
the instruction/DB/label/comment text needs no per-dialect
transformation at all: `plain`/`zenas`/`pasmo`/`sjasmplus` share one
renderer. `--sym=<path>` writes a symbol file matching whichever of the
four dialects `--format` names (`pasmo`/`zenas`/`plain` share pasmo's
own convention; `sjasmplus` gets its own, a genuinely different shape
-- colon-then-EQU, `0x`-prefix, 8 hex digits vs pasmo's tab-padded
4-digit `H`-suffix form). `json` is the machine-readable form for
tooling (one record per instruction or data run, with
address/bytes/mnemonic/operands/label/comment fields).

Four real divergences turned up once verification moved from ad hoc
manual checks to a real automated harness (`tools/dialectcheck`),
extended with ten adversarial cases (real-world binary extracts,
boundary conditions, undocumented forms) beyond its original six --
none were papered over:
- pasmo has zero Z80N support (source grep for nextreg/z80n/zxnext --
  no matches).
- sjasmplus parses an unindented directive at column 0 as a label by
  default (`ParseLabel`/`IsPseudoOpBOF`) -- caught only by testing
  against the real compiled binary, reading its source alone missed
  it.
- sjasmplus rejects the mixed-case `IXh`/`IXl` spelling zendis shares
  with zenas and pasmo, accepting only all-caps `IXH`/`IXL`
  (`GetRegister`/`GetRegister_3letter`).
- pasmo *also* rejects the undocumented bare `IN (C)`/`OUT (C),0`
  forms and DDCB/FDCB's undocumented copy-to-register form -- found
  by the adversarial pass, not the original six cases.

`pasmoUnrepresentable` now covers all three pasmo-specific categories
(Z80N, bare IN/OUT, DDCB/FDCB copy-to-register), each falling back to
a `DB` line with a stderr warning naming the reason;
`sjasmplusHalfRegisterFix` handles the `IXh`/`IXl` spelling, applied
only for that dialect. The adversarial pass also caught a real,
dialect-independent bug: a hints-file label that wasn't also a branch
target (e.g. naming a data table) was included in `--sym` output but
never actually declared in the accompanying assembly-source output --
a real pasmo round-trip test caught the inconsistency directly (`--sym`
claimed a symbol the `.asm` never defined, so pasmo's own regenerated
`.sym` came back empty). Fixed by seeding label resolution with every
hints-file label unconditionally. All of it verified against the real
pasmo and sjasmplus binaries, including two unedited real-world
extracts (a slice of `zexall.com`, the Z80 instruction exerciser; the
real Sinclair 48K ROM's reset routine) -- see dormant guard G-02
(docs/KNOWN_ISSUES.md) for the full corpus and every finding.

A fourth dialect, `--format=snasm` (Mike Dailly's SNASM,
freely-distributed, not open source), landed afterward. Real
compatibility work, not a config-only add: SNASM requires `$`-prefixed
hex (`0x` fails outright -- `snasmLiteralFix`), an `OPT Z80`/`OPT
ZXNEXT` mode preamble no other dialect here needs, and one Z80N
mnemonic fix (`MUL D,E` -> bare `MUL`; every other Z80N form checked
matched zendis's rendering unchanged) -- found the same way as the
sjasmplus/pasmo divergences, by running the real binary rather than
trusting its docs (which have at least three confirmed-stale lines:
`ADD HL,nn`'s own opcode table entry describes it as "Add A to HL",
`BSLA`/`BSRA`/`BSRL` are all listed as the same opcode, and `MIRROR
DE` is still documented despite SNASM's own changelog recording its
removal in 2018 -- confirmed removed by testing it directly).
`snasmUnrepresentable` shares its three undocumented-form categories
with `pasmoUnrepresentable` (refactored into
`undocumentedFormUnrepresentable`) but omits the Z80N check entirely
-- SNASM has full, correct Z80N support, confirmed by round-tripping
an entire real, unedited 6393-byte Next program
([jarikomppa/specnext](https://github.com/jarikomppa/specnext)'s
PlayFLX, Unlicense) byte-for-byte on the first try. The one category
SNASM handles worse than pasmo: DDCB/FDCB's copy-to-register form on
the rotate group (`RLC`/`RRC`/`RL`/`RR`/`SLA`/`SRA`/`SLL`/`SRL`)
doesn't just fail, it's silently accepted and emits the wrong
(non-copying) opcode with zero warning -- checked mechanically across
all 56 mnemonic x register-target combinations, no exceptions; `RES`/
`SET`'s copy-form fails safely instead, same as pasmo. `--sym` isn't
implemented for snasm: its own `-sym`/`-vice` flags (tried with both
single- and double-hyphen spellings) produce no output at all in the
tested release, and its working `-map` output is a structurally
different artifact (source-line debug info) not attempted here.
Because SNASM isn't open source, `tools/dialectcheck` never
auto-fetches it the way it does pasmo/sjasmplus from their public
repos -- `findSnasm` only checks `$PATH` and the harness's own cache
dir, and treats it as optional (skip with a clear message pointing at
where to get it) rather than failing the whole run if it's missing.
Same real-binary discipline as the other two once it's found: all
twelve adversarial cases plus the shared corpus round-trip through the
actual `SNasm.exe`, via `mono` (it's a Mono/.NET assembly, not a
native binary).

Trigger: same review as T-05 to T-07; grouped under the same cli theme as T-2 and T-07 since all three concern what the CLI can produce or consume beyond the current single hardcoded listing format.

Cross-ref: CHANGELOG 0.3.0.

## [0.2.0] T-4 — DDCB/FDCB undocumented copy-to-register forms unverified against a second source (v0.2.0, 2026-08-29)

Theme: verification · closed 0.2.0 · 2026-08-29
The DDCB/FDCB "copy result to register" undocumented forms (e.g.
`DD CB d 00` = `RLC (IX+d),B`) were implemented per the standard
reference decomposition but not cross-checked against a second
independent source or a real Z80/Z80N test run.

Status note: cross-checked against zen80's real execution
(pkg/disasm/decode_ddfd_t4_oracle_test.go) across all three
copy-producing groups -- rotate/shift, RES, SET -- plus confirmation
that BIT correctly has no copy target, and one FD/IY case.

Trigger: same session as T-3; split out separately since it's a
smaller, self-contained piece of the same underlying confidence gap.

Cross-ref: CHANGELOG 0.2.0.

## [0.2.0] T-2 — No label/symbol table support in the CLI listing (v0.2.0, 2026-08-29)

Theme: cli · closed 0.2.0 · 2026-08-29
`cmd/zendis` previously printed raw hex addresses for every
JP/JR/CALL/DJNZ target; there was no symbol file or label-inference
pass, so a listing of non-trivial code was harder to read than it
needed to be.

Status note: implemented. `Instruction.BranchTarget()`
(pkg/disasm/branchtarget.go) derives the target address directly from
an instruction's own encoded bytes (no core decode changes needed);
the CLI does a first decode pass to collect every referenced address,
then a second pass substituting generated `Lxxxx` labels, so forward
references resolve correctly. `--labels` (default on) can disable it.
Unit-tested (branchtarget_test.go) including the DD/FD pass-through
case and RST's deliberate exclusion.

Trigger: cut from the initial CLI to keep the first working version
scoped to the decoder itself.

Cross-ref: CHANGELOG 0.2.0.

## [0.2.0] T-1 — Z80N ED-prefixed extended opcodes not resolved to mnemonics (v0.2.0, 2026-08-29)

Theme: decoder · closed 0.2.0 · 2026-08-29

Z80N (github.com/ha1tch/zenzx's target) repurposes a number of the
classically-undefined ED opcode slots (0x00-0x3F and 0xC0-0xFF) for its
own instructions: `NEXTREG`, `MUL D,E`, `SWAPNIB`, `MIRROR A`, `TEST n`,
`OUTINB`, `PIXELDN`/`PIXELAD`, the `BSLA`/`BSRA`/`BSRL`/`BSRF`/`BRLC`
group, and the `LDIX`/`LDDX`/`LDWS`/`LDPIRX` block-copy family among
others. A `Target` selector (`TargetZ80`/`TargetZ80N`) now resolves all
29 slots under `TargetZ80N`; under the default `TargetZ80` they stay
`Undefined`, with `Instruction.PossibleZ80N` naming what they would have
been, and the CLI's `--warn-z80n` surfaces that on stderr.

Trigger: noted while scoping the initial ED table — the classic table
was straightforward to encode in full; the Z80N additions are a
distinct, separately-sized piece of work best done as its own pass with
its own tests against known Z80N opcode encodings.

Status note: implemented and tested — unit tests for all 29 slots under
both targets, plus an oracle harness cross-checking decode Length and
key operand claims (PUSH's big-endian operand, NEXTREG's port writes,
the three ADD rr,nn forms) against zen80 (github.com/ha1tch/zen80,
test-only dependency), a real, independently-cross-checked Z80N
execution core.

Cross-ref: CHANGELOG 0.2.0.
