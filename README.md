# zendis

A disassembler for the Zilog Z80 and the Z80N (the Spectrum Next's
extended Z80), written in Go.

zendis is a new sibling to [zen80](https://github.com/ha1tch/zen80)
(a Z80/Z80N CPU core) and [zenas](https://github.com/ha1tch/zenas) (a
Z80 assembler). It doesn't depend on either at runtime: zen80 is an
execution engine with no mnemonic tables to reuse, so zendis's decoder
is a new implementation of the standard opcode decomposition, built to
follow the same `x/y/z/p/q` bit-field structure zen80 uses internally
for dispatch. (zen80 is pulled in as a test-only dependency, to
cross-check zendis's Z80N decode claims against zen80's own execution
— see `pkg/disasm/decode_oracle_test.go`.) Output uses `0x`-prefixed
hex, one of the numeric forms zenas already accepts as input, so a
zendis listing can be fed back into zenas without translation.

## Status

Unprefixed, CB-prefixed, ED-prefixed (classic Z80 and Z80N), and
DD/FD-prefixed (including the DDCB/FDCB sub-form) instructions all
decode to mnemonics. Z80N's own ED-prefixed extensions (`NEXTREG`,
`MUL D,E`, the `LDIX`/`LDDX`/`LDWS` block-copy family, and the rest —
all 29 opcodes) resolve under `--target=z80n`; the default
`--target=z80` leaves those slots undefined rather than risk
mislabelling real classic-Z80 code, and warns on stderr
(`--warn-z80n`, on by default) whenever an undefined opcode matches a
Z80N instruction. JP/JR/CALL/DJNZ targets are replaced with generated
labels (`Lxxxx`) by default. See `docs/TRACKING.md` for what's tracked
as open work.

## Usage

```
go build -o zendis ./cmd/zendis
./zendis --org=0x8000 game.bin
./zendis --org=0x8000 --target=z80n next-game.bin
```

`--target` selects `z80` (default) or `z80n`. Under `--target=z80`,
`--warn-z80n` (default: on) prints a stderr line whenever an undefined
opcode matches a Z80N instruction, as a nudge to rerun with
`--target=z80n` if the binary is actually Next code; `--warn-z80n=false`
silences it.

```
8000:  00          NOP
8001:  3E 2A       LD A,0x2A
8003:  DD 21 00 80 LD IX,0x8000
8007:  DD 7E 05    LD A,(IX+0x05)
800A:  CB 7C       BIT 7,H
800C:  ED B0       LDIR
800E:  C3 00 00    JP L0000
```

`--labels` (default: on) replaces the JP target above with a generated
`Lxxxx` label, one per unique referenced address, resolved in a first
pass over the whole buffer so forward references work — a branch to
an address decoded later still gets the same label a backward branch
to it would. `--labels=false` reverts to raw `0x`-prefixed addresses.
Only JP, JR, CALL, and DJNZ get labels; `JP (HL)`/`(IX)`/`(IY)`
(register-computed, no encoded target) and `RST` (a fixed, well-known
vector) are left alone.

A line prefixed with `*` instead of a space is an opcode zendis
couldn't resolve to a mnemonic under the active target (see Status
above). A truncated instruction at the end of the buffer is emitted as
a `DB` line of raw bytes rather than causing a hard failure, so a
partial or misaligned binary still produces a listing for the bytes
that do decode.

### Hints file

`--hints=<path>` loads a plain-text file of directives that let you tell
zendis things it can't infer on its own:

```
label 0x800B Main
comment 0x800B program entry point
data 0x8003 8
```

- `label <addr> <name>` names a branch-target address, overriding the
  generated `Lxxxx` form -- a user-supplied name always wins, whether
  or not `--labels` is set.
- `comment <addr> <text>` attaches a trailing `; text` comment to the
  instruction or data line at that address. Addresses match zenas's own
  comment syntax, so a comment survives round-tripping through
  `--format=plain` and back through zenas.
- `data <addr> <length>` marks a byte range as data rather than code.
  The decode loop checks every address against these ranges before
  attempting to decode, and renders a match as `DB` lines instead of
  mis-decoding it as instructions.
- `entry <addr>` is parsed and stored but not yet consumed by
  anything -- reserved for a future control-flow-guided traversal mode
  that would use it as an additional starting point beyond `--org`.

Addresses and lengths accept `0x`-prefixed hex or plain decimal. Blank
lines and lines starting with `#` are ignored.

### Output formats

`--format` selects one of:

- `listing` (default): the address/hex/mnemonic form shown above.
- `plain` / `zenas` / `pasmo` / `sjasmplus` / `snasm`: an `ORG`
  directive followed by one instruction or `DB` line per record, no
  address/hex columns, and a `Name:` declaration at every labeled
  address -- genuinely re-assemblable, not just readable. All five
  share one renderer: the instruction/`DB`/label/comment text needs no
  per-dialect transformation for most of them (`0x` hex, `DB`/`DEFB`,
  an optional trailing colon on labels, and `;` comments are all
  pre-registered forms in real pasmo and sjasmplus, confirmed by
  reading their source). Per-dialect divergences, each confirmed
  against the real assembler, not assumed:
  - **pasmo**: a Z80N instruction, an undocumented bare
    `IN (C)`/`OUT (C),0`, or DDCB/FDCB's undocumented copy-to-register
    form (e.g. `RLC (IX+d),B`) -- pasmo has none of these -- falls
    back to a `DB` line of raw bytes with a stderr warning naming the
    reason.
  - **sjasmplus**: indents every non-label line (an unindented
    directive at column 0 is parsed as a label by default; labels
    themselves stay unindented) and uppercases the `IXh`/`IXl`
    half-register spelling zendis shares with zenas and pasmo (real
    sjasmplus only accepts all-caps `IXH`/`IXL`).
  - **snasm** ([Mike Dailly's SNASM](https://mdf200.itch.io/snasm),
    freely distributed but not open source): converts every `0x`
    literal to `$` (SNASM's only accepted hex prefix -- `0x8000`
    fails outright); emits an `OPT Z80`/`OPT ZXNEXT` mode directive
    first, which SNASM requires and no other dialect here does;
    rewrites `MUL D,E` to bare `MUL` (SNASM's Z80N `MUL` takes no
    operand syntax at all -- every other Z80N form checked matched
    zendis's rendering unchanged); and falls back to `DB` for the same
    three undocumented categories as pasmo -- `IN (C)`/`OUT (C),0`
    fail the same way, but DDCB/FDCB's copy-to-register form is worse
    for the rotate group specifically (`RLC`/`RRC`/`RL`/`RR`/`SLA`/
    `SRA`/`SLL`/`SRL`): SNASM silently accepts the syntax and emits
    the wrong (non-copying) opcode with zero warning, checked
    mechanically across all 56 mnemonic x register combinations, no
    exceptions. `--sym` isn't available for `snasm`: SNASM's own
    `-sym`/`-vice` flags produce no output at all in the tested release
    (V3.2.3.0, checked both single- and double-hyphen forms), and its
    working `-map` output is a different kind of artifact (source-line
    debug info, not just a symbol table).

  All four re-assemblable targets beyond `plain`/`zenas` are verified
  against the real assemblers, not just read from source or tested
  against zenas: `tools/roundtrip` covers zenas continuously (it's a Go
  dependency); pasmo, sjasmplus, and snasm are covered by
  `tools/dialectcheck`, a self-bootstrapping harness (dormant guard
  G-02, `docs/KNOWN_ISSUES.md`) that finds pasmo/sjasmplus on `$PATH`,
  in a build cache, or downloads and compiles them from source --
  it isn't part of the automated `go test` suite (needs
  `git`/`g++`/`make`, not just Go), but `cd tools/dialectcheck && go
  run .` is a real, repeatable check, not a one-off manual
  reproduction, with twelve adversarial cases (real-world binary
  extracts included) beyond its original six. snasm is checked the
  same way but treated as optional, never auto-fetched: it's not open
  source, so the harness only looks for it locally (`$PATH` or its
  cache dir) and skips those checks with a clear message -- pointing
  at where to get it -- if it's not there, rather than failing the
  whole run. This harness is what caught every divergence listed
  above, plus a real, dialect-independent bug it found along the way:
  a hints-file label that wasn't also a branch target was in `--sym`
  output but never actually declared in the accompanying source --
  fixed by seeding label resolution with every hints-file label
  unconditionally, not just branch targets.
- `json`: one object per instruction or data record --
  `address`/`bytes`/`mnemonic`/`operands`/`text`/`length`/`undefined`/
  `possible_z80n`/`label`/`comment` -- for feeding a listing into other
  tooling.

`--sym=<path>` writes a symbol file alongside any of the four
re-assemblable formats (not `listing` or `json`), in whichever format
matches: `plain`/`zenas`/`pasmo` share pasmo's own convention
(`NAME<tabs>EQU 0XXXXH`, 4-digit hex, tab-padded to align by name
length, matching pasmo's own real output byte-for-byte); `sjasmplus`
gets its own, unrelated shape (`NAME: EQU 0xVVVVVVVV`, 8 hex digits,
`0x`-prefixed -- also matching sjasmplus's own real `--sym` output
byte-for-byte). The symbol set includes every resolved label plus every
hints-file label, whether or not it's actually referenced by a branch
in the listing.

## Library

The decoder is usable directly as a library:

```go
import "github.com/ha1tch/zendis/pkg/disasm"

in, err := disasm.Decode(code, offset, addr)
// Decode is a TargetZ80 shorthand. For Z80N resolution:
in, err = disasm.DecodeTarget(code, offset, addr, disasm.TargetZ80N)
// in.Address, in.Bytes, in.Mnemonic, in.Operands, in.Length, in.Prefix
// in.PossibleZ80N is set when Undefined, Target was TargetZ80, and the
// opcode is one Z80N repurposes -- naming what it would have resolved to.
```

`disasm.Decode` decodes exactly one instruction starting at
`code[offset]` and reports `addr` on the result — callers step
`offset` and `addr` forward by `in.Length` between calls. Truncated
input, and opcodes zendis hasn't resolved to a mnemonic, are reported
through the error return and the `Instruction.Undefined` flag
respectively, so a caller can always tell the two apart from a
successfully decoded instruction.

Beyond full decoding, the tables and formatting a decoder needs are
exported in their own right, for a caller that wants to build its own
lighter-weight view without duplicating them — a live disassembly
panel over another project's own memory (zenzx), for instance:

```go
f := disasm.Decompose(op)                 // x/y/z/p/q bit fields
disasm.RegisterName(f.Z)                  // "B".."A", "(HL)" at index 6
disasm.RegisterPairName(f.P)              // "BC","DE","HL","SP"
disasm.RegisterPairNameAF(f.P)            // same, PUSH/POP's AF-at-3 form
disasm.ConditionName(f.Y)                 // "NZ","Z","NC","C","PO","PE","P","M"
disasm.ALUName(f.Y)                       // "ADD".."CP"
disasm.ALUTakesA(f.Y)                     // does ALUName(f.Y) take "A," explicitly
disasm.RotateName(f.Y)                    // "RLC".."SRL" (CB group)
disasm.HexByte(n)                         // "0x2A"
disasm.HexWord(nn)                        // "0x8000"
disasm.DispText(d)                        // "+0x05" / "-0x03"
disasm.Z80NName(op)                       // "MUL D,E" etc., or ok=false
disasm.Z80NLookup(op)                     // the full Z80NEntry (Mnemonic/Pre/Kind/Post)
```

All of these are the exact tables and functions the decoder itself
uses internally, not a parallel copy kept in sync by hand — the field
accessors (`RegisterName` and friends) and `Z80NLookup`/`Z80NName` are
functions rather than exported variables specifically so an importer
can't mutate the underlying tables out from under zendis's own
decoder; every field accessor also masks its argument to the table's
valid range, so an out-of-range byte gets a defined answer rather than
a panic.

### `pkg/render`: the loader and every output format, as a library

Everything `cmd/zendis` itself does beyond raw decoding -- the
data-range-aware decode loop, label resolution, and all six output
forms -- is `github.com/ha1tch/zendis/pkg/render`, not CLI-internal
logic. `cmd/zendis` is a thin wrapper over it, the same relationship
`pkg/disasm`'s `Decode`/`DecodeTarget` already has with the CLI. This
is also why `tools/roundtrip` and `tools/dialectcheck` no longer need
to shell out to a compiled `zendis` binary to test the dialect-specific
logic -- they still do, since that's also a faithful test of the CLI
wrapper itself (an unrelated regression in flag handling or exit-code
logic wouldn't show up in a library-level call), but the package
underneath is now directly callable too.

The recommended entry point is `render.Disassembly`, which
`cmd/zendis` itself uses -- decode once, render as many times as you
want from the same result, without re-threading `records`/`labelFor`/
`hints`/`org`/`z80nTarget` through every call by hand:

```go
import (
	"github.com/ha1tch/zendis/pkg/disasm"
	"github.com/ha1tch/zendis/pkg/render"
)

hints := &disasm.Hints{} // or disasm.ParseHints(r)
d := render.NewDisassembly(code, org, disasm.TargetZ80N, hints, true /* autoLabels */)

var buf bytes.Buffer
d.WriteAssemblySource(&buf, render.DialectSnasm, os.Stderr /* warnings */)
d.WriteListing(os.Stdout, true, os.Stderr)
symbols := d.Symbols()
```

This is more than convenience: `d.WriteAssemblySource` always uses the
`org`/`target` `d` was actually decoded with, since they're captured
once at construction rather than re-supplied by the caller on every
call -- with the package-level functions below, nothing stops passing
a `z80nTarget` that doesn't match what `DecodeBuffer` actually decoded
with.

`Disassembly`'s fields are unexported; `Records()`/`LabelFor()`/
`Hints()` give read access (sharing `Disassembly`'s own backing
slice/map, not a defensive copy) for a caller that wants to do
something the five `Write*`/`Symbols` methods don't cover.

The package-level functions `Disassembly` is built from remain
available directly, for anything more custom -- resolving labels with
a different strategy, inspecting intermediate state between decoding
and rendering, or the sort of exploratory calls this section's earlier
example showed:

```go
records := render.DecodeBuffer(code, org, disasm.TargetZ80N, hints)
labelFor := render.ResolveLabels(records, hints, true)
render.WriteAssemblySource(&buf, records, labelFor, hints, org,
	render.DialectSnasm, true /* z80nTarget */, os.Stderr)
```

`render.Record` is the decode/data-range union every `Write*` function
(and `Disassembly`) walks. `render.Dialect`
(`DialectPlain`/`DialectZenas`/`DialectPasmo`/`DialectSjasmplus`/
`DialectSnasm`) selects `WriteAssemblySource`'s target -- see its own
doc comment for the exact, source-verified divergence each one needs.
`render.WriteListing` and `render.WriteJSON` cover the other two
formats; `render.CollectSymbols` plus
`render.WritePasmoSym`/`WriteSjasmplusSym` cover `--sym`.
`render.IsZ80NInstruction`, `render.PasmoUnrepresentable`, and
`render.SnasmUnrepresentable` are exported in their own right too --
the specific, hard-won knowledge of what each real assembler can't
safely represent (the DDCB/FDCB copy-to-register matrix particularly)
is arguably the most valuable part of this package, not just a detail
of how the CLI's fallback works.

## Requirements

- Go 1.25 or later

## Licence

Apache 2.0. See `LICENSE`.
