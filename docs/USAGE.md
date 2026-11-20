# zendis — usage

The command, option by option. The [README](../README.md) has the short form;
[LIBRARY.md](LIBRARY.md) has the same things as Go calls.

## The listing

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
couldn't resolve to a mnemonic under the active target: a Z80N
instruction under `--target=z80`, or bytes that are no instruction. A truncated instruction at the end of the buffer is emitted as
a `DB` line of raw bytes rather than causing a hard failure, so a
partial or misaligned binary still produces a listing for the bytes
that do decode.

## Tape and snapshot input

Real ZX Spectrum software is overwhelmingly distributed as tape images
or machine snapshots, not pre-extracted raw binaries -- `--tape` and
`--snapshot` load directly from either instead of requiring that
extraction as a separate step first:

```
./zendis --tape=game.tap --format=listing
./zendis --tape=game.tzx --block-name=CODE --format=listing
./zendis --snapshot=game.sna --format=pasmo
./zendis --snapshot=game.z80 --org=0x8000 --length=0x2000 --format=listing
```

Exactly one input source is required: a positional raw-binary path,
`--tape=FILE` (`.tap`, `.tzx` or `.pzx`, auto-detected from the file's
own bytes), `--snapshot=FILE` (`.sna`, `.z80` v1/v2/v3 or `.szx`, 48K
or 128K-family -- see below), `--recording=FILE` (the first snapshot
in an `.rzx`), or `--nex=FILE` (a Spectrum Next executable -- see
[NEX input](#nex-input)).

An input with more than one part needs to be told which part is meant.
`--blocks` lists the parts and stops, with what each option takes:

```
$ zendis --tape=game.tap --blocks
4 block(s). --block=N takes a block by its number here; --block-name=NAME a header by its name.
  0  header  "game"     Program, 44 bytes, runs from line 10
  1  data       44 bytes, flag 0xFF
  2  header  "game"     Code, 6912 bytes, loads at 0x8000
  3  data     6912 bytes, flag 0xFF
```

`--org` and `--length` apply uniformly across all three input kinds
via `--org`/`--length` overriding whatever the source itself provides
-- a tape block's own load address, or a snapshot's own `PC` -- rather
than requiring it. Omitted, `--org` defaults to 0 for a raw binary, or
the source's own answer for `--tape`/`--snapshot`; `--length` defaults
to everything the source provides (the whole file, the whole tape
block, or up to the top of the 64K address space for a snapshot).

For `--tape`, `--block=N` selects a block by its position in the
tape's own block list (not a header+data pair count -- some real
tapes, confirmed by inspecting an actual commercial release, have a
single header for a small BASIC loader followed by several bare data
blocks with no headers at all, for a custom loading routine that
already knows their addresses; `--block` can reach those too, but
needs `--org` supplied explicitly since there's no header to default
one from). `--block-name=NAME` selects by exact header name instead,
taking precedence over `--block`. Neither given, the first block whose
header type is Code is used -- a reasonable default, not a guarantee
of finding the "main" code block on a tape with several Code blocks
(a screen or data buffer can legitimately come first; override with
`--block`/`--block-name` when it does).

`--snapshot` handles 128K-family snapshots too, auto-detected from the
file the same way 48K ones are -- no separate flag needed. Banks 5 and
2 are always mapped at `0x4000`/`0x8000` in every normal 128K/+2/+2A/+3
mode; only the top quarter, `0xC000`-`0xFFFF`, varies. By default that
means whichever bank the snapshot's own paging state has selected (a
"current view" -- reconstructing exactly what the CPU would see if
resumed, `--org` defaulting to the snapshot's own `PC` as for 48K).
`--bank=N` (0-7) overrides that, mapping bank `N` at `0xC000` instead,
for inspecting a bank that isn't currently paged in -- `--org`, if also
given, must then fall in `0xC000`-`0xFFFF`, the only range a bank can
ever occupy; omitted, it defaults to `0xC000`.

Two things are deliberately out of scope, refused with a clear error
rather than guessed at: the +3's special all-RAM paging mode (a
genuinely different memory layout selected by port `0x1FFD`'s own bit
0, which `zentools` carries as a raw, undecoded byte -- no foundation
here to reconstruct it correctly on top of), and any `--org` below
`0x4000` (ROM, never captured in a snapshot of any kind).

Built on [`github.com/ha1tch/zentools`](https://github.com/ha1tch/zentools)'s
format parsers (`pkg/tap`, `pkg/tzx`, `pkg/snapshot`) -- `pkg/load`'s
own job is picking which piece of a tape or snapshot to disassemble
and resolving its load address, not re-parsing bytes zentools already
understands. Exported at the library level (`load.FromTape`/`FromTAP`/
`FromTZX`/`FromSnapshot48K`/`FromSNA48K`/`FromZ80`), the same as
`pkg/render`, for the same reason: an embedder gets the real thing,
not a CLI-only capability.

## NEX input

`--nex=FILE` reads a Spectrum Next executable. The Z80N instructions
are on (`--target=z80` turns them off).

A NEX file is 16 KB banks, and a bank is whole in the file however
little of it the program uses. Three banks are in the address space
when the program starts: bank 5 at `0x4000`, bank 2 at `0x8000`, and
the header's entry bank (0 unless the file says otherwise) at `0xC000`.
With no other option that view is what is disassembled, and of it what
is used: from the first byte that is not zero in the bank the entry
point is in (or the entry point, if it is lower) to the last such byte
of the view. The entry point gets the label `nex_entry` when nothing
else names it. `--org` and `--length` move the two ends.

Any other bank is paged in by the program itself, wherever it chooses,
so it is taken alone:

| Option | Takes |
|--------|-------|
| `--bank=N` | bank N (0-111), 16 KB |
| `--page=N` | page N (0-223), 8 KB: pages 2n and 2n+1 are the halves of bank n, as the MMU numbers them |
| `--bank-at=ADDR` | where the bank's or page's first byte is: a multiple of `0x2000`, `0xC000` if not given |

```
$ zendis --nex=game.nex --blocks
NEX V1.2, entry point 0x8000, stack 0xBFFE, bank 0 at 0xC000. 2 bank(s):
  bank   2  used 0x0004-0x02BC (  697)  at 0x8000 when the program starts
  bank  20  used 0x0000-0x00FF (  256)  paged in by the program (--bank, --bank-at)

$ zendis --nex=game.nex --format=zenas > game.asm      # the program
$ zendis --nex=game.nex --bank=20 --as=bytes           # a bank of data
$ zendis --nex=game.nex --page=41 --bank-at=0x6000     # a page, where the program puts it
```

The source of the first command assembles back to the same bytes, and
`zenas build game.asm --nex --start nex_entry` makes a NEX file of it
again (checked with files zenas built and with sjasmplus-built examples
of one and two banks). What a NEX file holds besides banks -- loading
screens, a palette, copper code -- is not read; `zx next nex` of
zentools extracts those.

## Code and data

Everything is decoded as instructions unless it is said to be data.
Three ways say so, and they combine:

- `--data=ADDR:LENGTH[:KIND]`, any number of times, for a range;
- a `data` line of a [hints file](#hints-file), the same thing kept in
  a file;
- `--as=KIND` for the whole of what is being disassembled -- a block
  or a bank that is a table, a screen, text. Ranges given with `--data`
  or in the hints keep their own kind inside it.

The kind is which of zenas's data directives the bytes are written
with:

| Kind | Written as | For |
|------|-----------|-----|
| `bytes` (the default) | `DB 0x3E,0x01,0xC9` | anything |
| `hex` | `DB 0x 3E 01 C9` (zenas's compact form) | long binary data, in less space |
| `words` | `DW 0x8000,0x8010`; a last odd byte is a `DB` | address tables, 16-bit values |
| `text` | `DB "SCORE:",0x00`: runs of two or more printable characters quoted, the rest numbers, a line for each string | messages, names |
| `fill` | `DS 40,0x00` for a run of four or more equal bytes, `DB` between runs | buffers, padding |

```
$ zendis --org=0x9000 --data=0x9000:10:text --data=0x900A:40:fill --data=0x9032:8:words game.bin
9000:  53 43 4F 52 45 3A 00 DB "SCORE:",0x00
9007:  48 49 D3    DB "HI",0xD3
900A:  00 00 00 00.. DS 40,0x00
9032:  00 80 10 80 DW 0x8000,0x8010
9036:  34 12 FF FF DW 0x1234,0xFFFF
```

A row never runs across a labelled address: a name from the hints or
from a symbol file that falls inside a data range is declared ahead of
its own row, so a table keeps its labels. Every kind assembles back to
the bytes it was made from in zenas and in sjasmplus (which gets `bytes`
for `hex`, having no such form). `--format=pasmo` and `--format=snasm`
write every kind as `bytes`: their `DW`, `DS` and strings were not tried
against those assemblers. The JSON form has one record for a range, with
its kind in `data_kind`.

## Hints file

`--hints=<path>` loads a plain-text file of directives that let you tell
zendis things it can't infer on its own:

```
label 0x800B Main
comment 0x800B program entry point
data 0x8003 8
data 0x9000 32 text
```

- `label <addr> <name>` names a branch-target address, overriding the
  generated `Lxxxx` form -- a user-supplied name always wins, whether
  or not `--labels` is set.
- `comment <addr> <text>` attaches a trailing `; text` comment to the
  instruction or data line at that address. Addresses match zenas's own
  comment syntax, so a comment survives round-tripping through
  `--format=plain` and back through zenas.
- `data <addr> <length> [kind]` marks a byte range as data rather than
  code. The decode loop checks every address against these ranges before
  attempting to decode, and writes a match with data directives instead
  of mis-decoding it as instructions. `kind` is `bytes`, `hex`, `words`,
  `text` or `fill` (see [Code and data](#code-and-data)); `bytes` if
  there is none. Where two ranges overlap, the one given first wins
  and the other goes on after it.
- `entry <addr>` is parsed and stored but not yet consumed by
  anything -- reserved for a future control-flow-guided traversal mode
  that would use it as an additional starting point beyond `--org`.

Addresses and lengths accept `0x`-prefixed hex or plain decimal. Blank
lines and lines starting with `#` are ignored.

## Symbol files

`--symbols=<path>` reads names for addresses from another tool's symbol
file, so a program assembled elsewhere is disassembled with its own
labels. Four formats are read, told apart by the file's first line
(`--symbols-format` names one when that fails):

| Format | Written by | A line |
|---|---|---|
| `cspect` | SNASM's `-map`, sjasmplus's `CSPECTMAP`, zenas's `--map`; what CSpect's debugger loads with `-map` | `00008000 00008000 00 START` |
| `pasmo`, `zenas` | pasmo, and zenas with `--sym` | `start		EQU 08000H` |
| `sjasmplus` | sjasmplus with `--sym` | `start: EQU 0x00008000` |
| `z88dk` | z88dk's `.map` | `_main = $8000 ; addr, public, ...` |

```
zenas assemble game.asm game.bin --sym
zendis --org=0x8000 --symbols=game.sym --format=zenas game.bin
```

The option can be given more than once (a ROM's symbols and the
program's). A hints-file `label` for an address wins over a symbol file,
and an earlier file over a later one. What a file marks as a constant
(a cspect type of 01 or 02, a z88dk `const`) is not used as a label, with
one exception: where the code branches to an address outside itself and a
constant has that value, the constant is the name (`CALL BORDER` for a
ROM routine the source defined with `EQU`). The
pasmo and sjasmplus formats do not say which is which, so every value
there that fits in 16 bits names the address equal to it. Names are made
fit for an assembler: `START@LOOP` (cspect's local label) and
`start.loop` (sjasmplus's) become `START_LOOP` and `start_loop`. As with
generated labels, a name replaces the target of JP, JR, CALL and DJNZ
only; other operands stay numbers (`docs/TRACKING.md`, T-16).

`--map=<path>` writes the disassembly's labels as a cspect map, with any
`--format`. The physical address of each is the one the Next's memory
has when a program starts (the page in the address's 8K slot times 8192,
plus the offset), which is what sjasmplus writes for a `ZXSPECTRUMNEXT`
device whose paging the source leaves alone:

```
zendis --org=0x8000 --hints=game.hints --map=game.map game.bin
CSpect.exe -map=game.map game.nex
```

The formats were taken from files the tools wrote themselves (the maps
that come with CSpect 4.5.1.0, sjasmplus 1.24.1, zenas 0.11.2); z88dk's
is from its documented shape. A map zendis wrote has not been loaded
into CSpect. The library form is zentools' `pkg/symfile`; see
[Symbol files, as a library](LIBRARY.md#symbol-files-as-a-library).

## Output formats

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
    (V3.2.3.0, checked both single- and double-hyphen forms). Its `-map`
    output does work and is the map CSpect loads: `--map` writes the
    symbols of that format with any `--format`, this one included (see
    [Symbol files](#symbol-files)); the source-line sections SNASM adds
    after the symbols are not written.

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
