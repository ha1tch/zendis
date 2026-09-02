# zendis — Resolved

Closed items, append-only, newest first. Full detail text preserved
verbatim as it stood at closure, stamped with the closing version and
date. `CHANGELOG.md` says what shipped; this says what was wrong and
how it was resolved — the two reference, never duplicate, each other.

## [0.3.6] T-15 — No RZX input-recording support -- can't disassemble code straight from a TAS/competition recording (v0.3.6, 2026-08-31)

Theme: formats · closed 0.3.6 · 2026-08-31
Scope: RZX input-recording input (`--recording=FILE.rzx`), extracting
code from the first embedded snapshot and applying the same
--bank/--org/--length handling a standalone .sna/.z80/.szx already
gets.

Status note: implemented in pkg/load/rzx.go (FromRZX). An RZX's own
defining content -- the frame-by-frame log of what every CPU IN
instruction returned during an actual recorded session -- is
deliberately irrelevant here: this package exists to get code in
front of a disassembler, and a recording's embedded snapshot is
exactly as disassemblable as if extracted from that snapshot
directly. A multiload recording can carry more than one embedded
Snapshot block; the first is used, matching FromTape's own
default-to-first-Code-block convention. An External snapshot block (a
filename reference, not embedded bytes) is rejected with a clear
error rather than silently producing nothing. Extension matching
(SNA/Z80/SZX) is case-insensitive, verified directly with a lowercase
"szx" fixture, not assumed from the switch statement alone.

Trigger: same as T-13. The most interesting of the three additions --
this is a genuinely new INPUT CATEGORY for zendis, not just another
format zentools happens to parse, since it lets someone disassemble
code straight from a TAS/competition recording without first manually
extracting the snapshot.

Related, filed as a follow-up to the already-closed T-10 rather than
reopening it: FromTZX now reads TZX blocks 0x11 (Turbo Speed Data) and
0x14 (Pure Data) in addition to 0x10 -- an earlier version silently
dropped every turbo-loaded block on any real TZX using one, a large
fraction of real commercial tapes (that's the entire reason turbo
loaders exist). Found and fixed for the identical reason in the
sibling zentools project's own TZX -> tape conversion path first; this
brings zendis's own loading up to the same standard.

Cross-ref: CHANGELOG 0.3.6.

## [0.3.6] T-14 — No SZX snapshot input -- zentools v0.8.0 added SZX support, zendis's own loader doesn't use it yet (v0.3.6, 2026-08-31)

Theme: formats · closed 0.3.6 · 2026-08-31
Scope: SZX (zx-state) snapshot input (`--snapshot=FILE.szx`,
auto-detected via the "ZXST" signature), 48K or 128K-family, with the
same --bank/--org/--length options SNA/Z80 already support.

Status note: implemented in pkg/load/szx.go (FromSZX). Genuinely
simple to add: szx.Decode produces the identical *snapshot.MachineState
type SNA/Z80 decoding already does, so this dispatches straight into
the existing snapshot48KToCode/snapshot128ToCode -- no new flattening
logic anywhere. One real doc-comment mistake caught before it shipped:
an early draft claimed szx.Decode itself rejects the +3's special
paging mode; verified directly against zentools's own source and it
doesn't -- it just stores Port1FFD's raw byte, uninterpreted. The
rejection genuinely happens, but one layer up, in this package's own
snapshot128ToCode (the same ErrSpecialPagingMode check a Z80/SNA
source in that state already triggers) -- confirmed by a real test
round-tripping a special-mode SZX file, not by trusting the corrected
comment either.

Peripheral state (AY sound, joysticks, disk controllers) a real .szx
file might carry is already dropped by szx.Decode before this
function ever sees the result -- irrelevant for disassembly, since
this package only ever wanted CPU/paging/memory, but named explicitly
so "SZX support" doesn't imply more fidelity than what's used.

Trigger: same as T-13.

Cross-ref: CHANGELOG 0.3.6.

## [0.3.6] T-13 — No PZX tape image input -- zentools v0.8.0 added PZX support, zendis's own loader doesn't use it yet (v0.3.6, 2026-08-31)

Theme: formats · closed 0.3.6 · 2026-08-31
Scope: PZX tape image input (`--tape=FILE.pzx`, auto-detected via the
"PZXT" signature, same as .tap/.tzx). Reuses selectBlock's existing
BlockIndex/BlockName/default-to-Code selection logic entirely -- a
PZX file's DATA blocks are decoded into the same []tap.Block shape
FromTAP/FromTZX already select from, via tap.DecodeBlock on each
block's own payload.

Status note: implemented in pkg/load/pzx.go (FromPZX, pzxBlocksFor).
Deliberately more permissive than the sibling zentools project's own
PZX -> TAP reconstruction: extraction only needs the resolved bytes a
DATA block already carries by the time PZX's own decoder produces
them, regardless of what pulse timing represented them physically on
tape (standard Spectrum ROM timing, a turbo loader, anything else) --
no timing check applies here the way pzxToTAP (in zentools) needs one
when reconstructing a genuinely re-loadable TAP file. Verified against
both standard-timing and deliberately non-standard (turbo-loader-
style) fixtures; the latter is the actual point of testing this --
confirming block selection doesn't silently depend on the source
using this project's own assumptions about timing.

Trigger: zentools v0.8.0 added PZX/SZX/RZX support; this closes the
gap between what zentools can now parse and what zendis can load for
disassembly. Filed and closed together with T-14/T-15 in the same
session, all three built on the same underlying zentools release.

Cross-ref: CHANGELOG 0.3.6.

## [0.3.5] T-12 — No snapshot input, 128K paged memory (SNA128/Z80 128K+) -- 48K snapshot support alone can't address banked RAM (v0.3.5, 2026-08-31)

Theme: formats · closed 0.3.5 · 2026-08-31
T-11 covers 48K snapshot loading, where memory maps onto a flat 64K space
with no ambiguity. That stops being true for the 128K family (128K, +2,
+2A, +3): zentools's own MachineState.Memory always holds all eight 16K
banks (RAM [8][16384]byte), and for these models Paging.Port7FFD (plus
Port1FFD for +3) determines which bank is actually mapped to the switchable
0xC000-0xFFFF region at the moment of the snapshot -- the other banks not
currently paged in are still real, addressable memory (routines, data
tables) that a reverse-engineer may well want to disassemble, but they
aren't part of any single flat 64K view.

This is a genuinely different, harder problem than T-11, not an extension
of it, which is why it's its own wave rather than folded into the same one:
  - A "current view" default (resolve Port7FFD, reconstruct exactly what
    the Z80 would see if execution resumed at this instant, org = CPU.PC as
    in T-11) is the natural equivalent of T-11's behaviour, and probably
    the right default here too.
  - But a caller may specifically want a bank that ISN'T currently paged
    in -- code sitting in bank 3 while bank 6 happens to be mapped at
    snapshot time, say. That needs an explicit bank-selection mechanism
    (address bank N as if mapped at 0xC000, independent of what Port7FFD
    actually says), which has no equivalent at all in the 48K case.
  - Whether disassembly should ever be allowed to span a paging boundary is
    a real, open design question, not just an implementation detail: code
    can genuinely bank-switch mid-execution on real hardware (an OUT to
    0x7FFD followed by falling through to the newly-mapped bank at
    0xC000), but a single static snapshot only ever shows one bank at that
    address at a time -- there's no way to statically know what was there
    a moment before or will be a moment after without a second snapshot or
    a trace.

Scope, tentative pending design: --snapshot=FILE (128K/+2/+2A/+3 models),
default org/memory = current Port7FFD-resolved view, --bank=N to select a
specific RAM bank directly regardless of current paging, --org override on
top of either.

Status note: implemented as pkg/load (FromSNA128/FromZ80128/
FromSnapshot128K, plus FromSnapshot -- a format- and size-auto-detecting
entry point covering 48K and 128K together, which --snapshot now uses
unconditionally rather than requiring the caller to already know which
shape a file has). The paging-boundary question flagged as open when this
was filed resolved cleanly once the actual layout was confirmed against
real +3 hardware documentation (Sinclair Wiki's Memory paging and ZX
Spectrum +3 pages, and the MrKWatkins/ZXSpectrumNextTests port reference,
independently agreeing): banks 5 and 2 are fixed at 0x4000/0x8000 in every
normal 128K/+2/+2A/+3 mode, so only the top quarter varies -- "current
view" and "org defaults to PC" both carry over from T-11 essentially
unchanged, --bank=N substitutes for Port7FFD's own selection. The +3's
special all-RAM paging mode (port 0x1FFD bit 0 set, four further
configurations selected by bits 1-2, confirmed against the same sources)
is explicitly out of scope, not attempted: zentools carries Port1FFD as a
raw, undecoded byte, giving no foundation to build a correct
reconstruction on top of, so a snapshot with that bit set is refused with
ErrSpecialPagingMode rather than silently producing a wrong memory map.
ROM addresses (org below 0x4000, never captured in any snapshot) are
similarly refused rather than read as zero.

One real bug found in zentools itself along the way, not this package:
EncodeSNA128 (v0.6.0) produces a malformed, oversized image whenever
Port7FFD's selected bank equals 2 or 5 -- a genuine, real hardware
scenario, confirmed by isolated direct testing. DecodeSNA128's own strict
length check means this package's own round trip is never silently wrong
(it fails loudly on decode), but it's worth reporting upstream; documented
in pkg/load/snapshot128.go for now. Verified against zentools's own
encoders for every case (current view, fixed-bank reads at 0x4000/0x8000,
explicit Bank override, Bank+org range validation, ROM rejection,
out-of-range Bank, special-mode rejection via .z80 -- .sna's own 128K
trailer has no Port1FFD field at all, confirmed by reading zentools's own
source, so that path can only be exercised through .z80) and against the
real CLI binary for --snapshot/--bank end to end, including proving bank
selection actually changes content (identical code placed in two banks,
one currently paged and one not, both correctly reachable).

Trigger: same investigation as T-10/T-11; the paging complexity here was
specifically flagged as a concern before any of the three were scoped, and
turned out to genuinely warrant its own wave once zentools's Paging/Memory
structures were actually read rather than assumed simple.

Cross-ref: CHANGELOG 0.3.5.

## [0.3.5] T-11 — No snapshot input, 48K (SNA/Z80) -- can't load code from a machine-state snapshot at all (v0.3.5, 2026-08-31)

Theme: formats · closed 0.3.5 · 2026-08-31
zendis has no way to load code from a machine-state snapshot (.sna/.z80) at
all -- only a raw binary plus a manual --org. A snapshot is a genuinely
different, useful starting point from a tape image or raw binary: it
captures a real, running moment (full register file, not just a load
address), which is often exactly what a reverse-engineer wants to start
from -- "what does the CPU do next from here" rather than "what does this
file's payload contain."

github.com/ha1tch/zentools/pkg/snapshot already provides this via a neutral
MachineState, confirmed by reading its actual struct definitions:
  - DecodeSNA/DecodeZ80(image []byte) (*MachineState, error).
  - MachineState.CPU (full register file, including PC -- a real, meaningful
    default org: "start from what would execute next," not an arbitrary
    address) and MachineState.Memory (RAM [8][16384]byte, all eight
    possible 16K banks always present in the struct regardless of model).

This item is deliberately scoped to 48K only: a 48K machine's memory maps
onto a flat 64K space with no ambiguity at all -- banks 5, 2, and 0 are
always mapped at 0x4000, 0x8000, and 0xC000 respectively (confirmed in
zentools's own state.go: "BankFor48K returns the RAM bank mapped to a 48K
address region"). MachineState.Paging is left zero for a 48K snapshot and
never needs resolving. That flat-mapping property is exactly what makes
this the right first snapshot item to land, and exactly what stops being
true the moment a 128K/+2/+3 snapshot's paging enters the picture -- see
T-12, filed separately specifically because that part is a different,
harder problem, not an extension of this one.

Scope: --snapshot=FILE for 48K .sna/.z80, default org = the snapshot's own
CPU.PC (override with --org), default range = the whole 64K (override with
an explicit range).

Status note: implemented as pkg/load (FromSNA48K/FromZ80/FromSnapshot48K),
plus --snapshot/--org/--length wiring, shared with T-10's flags. Two things
confirmed directly from zentools's own source before relying on them, not
assumed: DecodeSNA already resolves the classic .sna quirk (PC isn't a
header field -- it's pushed onto the stack at save time, so CPU.PC in the
returned MachineState is already the correctly-popped value, no extra logic
needed); and DecodeZ80's own doc comment claims v2/v3 "is not yet
supported", which is stale relative to the code -- decodeZ80Extended is
fully implemented and DecodeZ80 already dispatches to it whenever the v1
header's PC field is zero, confirmed by a test encoding a real v3 image via
zentools's own EncodeZ80v3 and decoding it back through FromZ80. A 128K
snapshot (either format) is rejected with a clear, specific error rather
than silently misreading it or falling through to a confusing failure --
including the one real edge case that needed its own handling: a 128K .sna
is exactly 131103 bytes, which DecodeSNA (48K-only) would reject only as
"wrong size", so FromSnapshot48K checks for that exact length upfront and
reports ErrSnapshot128K directly instead. Verified against zentools's own
encoders for every case (SNA, Z80 v1, Z80 v3, org/length overrides, both
128K rejection paths, unrecognized input) and against the real CLI binary
for --snapshot end to end.

Trigger: user request, alongside T-10 (tape loading) and T-12 (128K paged
snapshot loading) -- these three were filed together as one investigation,
each scoped to its own wave since they turned out to have genuinely
different shapes of complexity once zentools's actual API was read rather
than assumed.

Cross-ref: CHANGELOG 0.3.5.

## [0.3.5] T-10 — No tape image input (TAP/TZX) -- can't load code directly from a tape image, only raw binary + manual --org (v0.3.5, 2026-08-31)

Theme: formats · closed 0.3.5 · 2026-08-31
zendis currently only accepts a raw binary file plus a manually-specified
--org; there's no way to point it directly at a .tap or .tzx tape image and
disassemble the code block(s) it contains. Real ZX Spectrum software is
overwhelmingly distributed as tape images, not pre-extracted raw binaries --
pulling the payload and its load address out by hand, as a separate step
outside zendis entirely, is exactly the kind of friction a disassembler aimed
at this ecosystem shouldn't require.

github.com/ha1tch/zentools already has real, ready-to-use parsers for both
formats -- confirmed by reading the actual source, not assumed from the
module name:
  - pkg/tap: Decode([]byte) ([]Block, error). A Block's Param1 is the load
    address for a TypeCode header (exactly the "file-specified org" this
    item needs); Data holds a data block's payload.
  - pkg/tzx: Decode([]byte) ([]Block, error). Block ID 0x10 (Standard Speed
    Data) wraps literal TAP-block bytes in its own Data field -- directly
    reusable via tap.DecodeBlock on that slice, so the common case doesn't
    need separate handling. Other block IDs (0x11 Turbo Speed Data, 0x12
    Pure Tone, 0x13 Sequence Of Pulses, 0x14 Pure Data, 0x15 Direct
    Recording, 0x19 Generalized Data) are pulse/timing-level constructs with
    no clean "load address" concept the way a standard header carries one --
    genuinely out of scope for org/block-selection purposes, not just
    deferred for convenience.

Scope: --tape=FILE plus a way to select which block (default: the first
TypeCode block found; override by index or name), --org overriding the
block's own Param1 when given, and control over how much of the block to
disassemble (default: the whole block; override with an explicit length).

Status note: implemented as pkg/load (FromTAP/FromTZX/FromTape, exported
alongside pkg/render for the same embedding reasons), plus --tape/--block/
--block-name/--length/--org wiring in the CLI. One real design change from
the original scope, found only by testing against a real commercial tape
image's actual structure (not assumed): block selection indexes the raw,
flat block list, not a derived header+data "pairing" -- a common real-world
pattern is a single header for a small BASIC loader followed by several
bare data blocks with no headers at all, for a custom loading routine that
already knows their addresses from its own code. Indexing into a derived
pairing would have made those bare blocks completely unreachable; selecting
one now works via --block (an index), but requires --org explicitly (there's
no header to default a load address from). Verified against zentools's own
encoders for every case (block selection by index/name, org/length
overrides, TAP and TZX, a synthetic bare-data-block tape reproducing the
real pattern above) and against the real CLI binary for --tape end to end.

Follow-up: a TZX with no 0x10 (Standard Speed Data) blocks at all -- only
turbo-loading or other pulse-level types -- originally fell through to the
generic ErrNoCodeBlock, the same error as "0x10 blocks existed but none had
a Code header". Split into a distinct ErrNoStandardDataBlocks so the two
genuinely different situations (nothing to search, versus searched and
found nothing) don't read as the same diagnosis.

Trigger: user request, prompted by wanting to load code directly from real
tape/snapshot files rather than pre-extracted raw binaries -- filed
alongside T-11/T-12 (snapshot loading) as the natural next step now that the
T-08 pkg/render split makes it clean to add a new input source without
touching the decoder itself.

Cross-ref: CHANGELOG 0.3.5.

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
