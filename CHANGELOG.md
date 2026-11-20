# zendis — Changelog

## [0.8.2] — 2026-11-16

Documents, and the three lines of the flags' help that name one. The
program's output is 0.8.1's.

### Changed

- **The README is one page** (67 lines; it was 649): what zendis does, a
  table of it, and where to read on. What it held is in `docs/USAGE.md`
  (the command: the listing, each kind of input, NEX files, code and
  data, hints, symbol files, the output formats) and `docs/LIBRARY.md`
  (the packages and the library call for each option), moved as it was.
  The flags' help that said "see README" names `docs/USAGE.md`.
- The README said tapes were `.tap` and `.tzx` and snapshots `.sna` and
  `.z80`; `.pzx`, `.szx` and `.rzx` recordings are read too, and were
  before this release.

## [0.8.1] — 2026-11-16

The last pieces of 0.8.0 that were in the command only, as library
calls. The command's output is the same as 0.8.0's.

### Added

- `disasm.ParseDataRange`: `ADDR:LENGTH[:KIND]`, the form of `--data`.
- `api.DefaultTarget(kind)`: the Z80N for a NEX file, the Z80 for the
  rest; what `--target` is when it is not given.
- `api.WriteParts(w, kind, image)` and `api.ErrOnePart`: `--blocks` for
  whichever kind of input.
- A table in the README of each option and its library call.

### Known state

- Tried with zenas 0.16.0 in one process: a program assembled with
  tags, written as a NEX file by `artifact.Encode`, listed, disassembled
  with its own names and two data ranges, and that source assembled and
  encoded to the same NEX file.
- G-01 ran: 8 of 8. G-02 not run, as for 0.8.0.

## [0.8.0] — 2026-11-16

NEX files as input, and data written as data.

### Added

- **`--nex=FILE`**: disassemble a Spectrum Next executable. By default
  what is used of the banks in the address space when the program
  starts (5, 2 and the entry bank), from the bank the entry point is
  in; the entry point is labelled `nex_entry`; the Z80N instructions
  are on. `--bank=N`, `--page=N` and `--bank-at=ADDR` take any other
  bank or 8K page alone, where the program puts it. Library:
  `api.SourceNEX`, `Options.Bank`/`Page`/`BankAt`, `load.FromNEX`,
  `load.DescribeNEX`.
- **`--blocks`**: list the parts of an input and stop -- a tape's
  blocks with the numbers `--block` takes, a NEX file's banks with
  where each is and how much of it is used (`load.WriteTapeBlocks`,
  `load.WriteNEXBanks`, `load.TapeBlocks`).
- **Data kinds.** A data range is written as `bytes` (`DB` of numbers,
  as before), `hex` (zenas's `DB 0x 3E 01`), `words` (`DW`), `text`
  (`DB` of quoted strings) or `fill` (`DS count,value`): a fourth word
  on a hints file's `data` line, `disasm.DataRange.Kind`.
- **`--data=ADDR:LENGTH[:KIND]`**, repeatable: a data range without a
  hints file.
- **`--as=KIND`** (`api.Options.AsData`): the whole of what is
  disassembled as data, for a block that holds no code. Ranges of the
  hints and of `--data` keep their kinds inside it.
- `render.DataRows`, `render.DataRow`, `render.EffectiveDataKind`,
  `disasm.ParseDataKind`, `(*disasm.Hints).DataAt`; `data_kind` in the
  JSON form.

### Changed

- **Labels inside a data range are declared.** A name from the hints or
  a symbol file that fell inside a data range was left out of the
  source; a row now ends at it and the label stands ahead of the next.
- **Data in re-assemblable source is 16 bytes a line.** A range was one
  `DB` line however long, 6912 numbers for a screen.
- **Overlapping data ranges**: the range given first wins, as it did,
  and the other now goes on after it; it used to be lost from the first
  one's start on.
- A comment on an address inside a data range is printed on the row
  that starts there; only the range's first address was looked at.

### Known state

- Checked by assembling: every kind back to the same bytes with zenas
  and with sjasmplus 1.24.1; `--nex` source of three NEX files (one
  built by zenas, two sjasmplus-built examples of one and two banks)
  back to the same bytes, and the zenas one back to the same NEX file
  with `zenas build --nex`.
- `--format=pasmo` and `--format=snasm` write every kind as bytes
  (T-20).
- A NEX file's screens, palette and copper code are not read, and
  banks are taken one at a time (T-21).
- G-01 ran: 8 of 8. G-02 (`tools/dialectcheck`) was not run: it builds
  pasmo and sjasmplus from source. Its sjasmplus half was covered by
  hand for the data kinds, as above; pasmo output is unchanged but for
  the 16-byte `DB` lines.

## [0.7.0] — 2026-11-16

### Changed

- **`pkg/symfile` is zentools' now**
  (`github.com/ha1tch/zentools/pkg/symfile`, zentools 0.15.0), and this
  module's copy is gone. zenas uses the same package from its 0.14.0,
  so `assembler.ExportSymbols` hands an assembled program's symbols
  straight to the `symfile.Table` a disassembly takes its names from;
  0.6.0 needed a map written to a buffer in between. Closes T-19.
  Anything that imported `github.com/ha1tch/zendis/pkg/symfile` changes
  the import path; the names in it are the same.
- **zentools v0.8.6 to v0.15.0.** The loaders' tests pass unchanged.

### Known state

- Tried with zenas 0.14.0 in one process, as the README shows it:
  assemble in memory, add the result's symbols to a table, disassemble
  the bytes with the program's names.
- G-01 ran: 8 of 8.

## [0.6.0] — 2026-11-16

The symbol-file features as a library, for a program that embeds zendis.
The command's output is the same as 0.5.2's.

### Added

- **`symfile.Table`**: the names of one or more symbol files, by
  address, built once and read as often as needed (`NewTable`,
  `AddFile`, `Add`, `Label`, `Constant`, `Name`, `Labels`, `Constants`,
  `Len`), and `symfile.Identifier`.
- **`api.Options.Symbols`**: a `*symfile.Table` for `api.Disassemble`,
  which now does what `--symbols` did in `cmd/zendis`: labels for
  addresses the hints do not name, constants for branch targets outside
  the code. `api.WithSymbols` is the first half on its own, for a caller
  that builds a `render.Disassembly` directly. Neither changes the
  caller's hints or the table.
- **`(*render.Disassembly).WriteCSpectMap`**: what `--map` writes.

### Changed

- `cmd/zendis` no longer has symbol logic of its own: it reads the
  files into a table and sets the option.

### Known state

- Tried with zenas 0.13.0 in one process: a program assembled in
  memory, its map written to a buffer, read into a table, and the
  assembled bytes disassembled with the program's own names and the
  name of a ROM routine it calls. The README has the code.
- `pkg/symfile` exists twice, here and in zenas, kept in step by hand;
  an embedder of both passes names between them as a map in a buffer
  (T-19).

## [0.5.2] — 2026-11-16

Documents and one message; no change to what is decoded or written.

### Fixed

- **`README.md` and the `--sym` error for `--format=snasm` still said
  SNASM's `-map` output was "a different kind of artifact" that zendis
  did not implement.** Since 0.5.0 `--map` writes that format's
  symbols. Both now say so.
- **`README.md`'s library section had no entry for `pkg/symfile`.**

### Known state

- `docs/WAVE_TRACKING.md` does not pass `repoman waveprogress --check`
  with gorepoman 0.16.0, and did not at 0.4.0 either: that version
  counts a partly done item as a half and writes its own summary line
  for each wave, where the document has hand-written ones that say why
  an item is open. It was left as it is: regenerating it replaces those
  lines.
- zenas assembles `LDIRSCALE` (`ED B6`) with `--next`; zendis does not
  decode it under `--target=z80n` and shows the two bytes as undefined
  (the re-assemblable formats write them as `DB`, so a round trip keeps
  the bytes). Which of the two is right about the Next has not been
  settled here (T-18).

## [0.5.1] — 2026-11-16

### Changed

- **A constant names a branch target outside the code.** With a symbol
  file that tells constants from labels (a CSpect map, a z88dk map), a
  `CALL` or jump to an address no line starts at takes the name of a
  constant with that value: a ROM routine is usually an `EQU` in the
  program's source, and came out as `L229B`. Constants still never name
  a line of the code itself. `symfile.Constants` is the library call.
- **`symfile.WriteCSpectMap` gives a constant without a physical address
  its offset in an 8K page**, which is what sjasmplus writes
  (`0000229B 0000029B 01 BORDER`), and not its whole value. zendis's own
  `--map` writes labels only and is unchanged.

### Known state

- zenas 0.12.0 writes the same map format with `--map`; `pkg/symfile` is
  the same code in both and is kept in step by hand. A program assembled
  by zenas with `--map`, disassembled with that map and assembled again
  is the same bytes, and a map zendis wrote is read back by zenas's
  `--symbols`.
- G-01 ran again: 8 of 8. G-02 was not run (as at 0.5.0).

## [0.5.0] — 2026-11-16

Symbol files of other tools, in and out.

### Added

- **`--symbols=<path>`**: names for addresses from a CSpect map (SNASM's
  `-map`, sjasmplus's `CSPECTMAP`), a pasmo or zenas `.sym`, an sjasmplus
  `--sym` file or a z88dk `.map`. The format is told from the file's
  first line; `--symbols-format` names it. May be given more than once.
  A hints-file label wins over a symbol file; constants a format marks
  as such are not used as labels; names are made fit for an assembler
  (`START@LOOP` becomes `START_LOOP`).
- **`--map=<path>`**: the disassembly's labels as a CSpect map file, for
  its debugger's `-map` option, with any `--format`. Physical addresses
  are those of the Next's memory as a program finds it, the same as
  sjasmplus writes for its `ZXSPECTRUMNEXT` device.
- **`pkg/symfile`**: the library behind both: `Parse`, `ParseFormat`,
  `Labels`, `WriteCSpectMap`, `FromLabels`, `NextPhys`.

### Fixed

- **A branch to an address outside the disassembled lines made the
  re-assemblable output fail to assemble**: `CALL L229B` (a ROM routine)
  or a jump into the middle of what was decoded as one instruction used
  a label the source never defined. Such labels are now defined with an
  `EQU` ahead of `ORG`, in address order, in all four re-assemblable
  dialects. It was there since generated labels were added and was not
  in the register; every program that calls the ROM was affected.
- **Guard G-01 (`tools/roundtrip`) could not build the zendis command**
  since zendis began to depend on zentools: the tool built it through
  its own module, whose `go.sum` has none of zentools' sums. It now
  builds it in zendis's module. The guard's seven decoder cases were not
  affected; its command-line case had not run since 2026-08-29.

### Known state

- A 17.8 KB Next program (ZenZX's gallop example) assembled with zenas
  0.11.2 `--sym`, disassembled with `--symbols` and `--format=zenas` and
  assembled again is the same file byte for byte; a small program with
  a ROM call and a data table is too, through zenas and through
  sjasmplus 1.24.1. The three text maps that come with CSpect 4.5.1.0
  are read (77, 121 and 156 labels).
- G-01 ran: 8 of 8. G-02 (`tools/dialectcheck`, pasmo, sjasmplus and
  SNASM) was not run for this release: it downloads and builds pasmo and
  sjasmplus. The new `EQU` lines were assembled by zenas and sjasmplus
  only; pasmo and SNASM have not seen them.
- A map written by `--map` has not been loaded into CSpect, and the
  z88dk reader has not read a file z88dk made (T-17).

## [0.4.0] — 2026-09-02

### Added

- **`pkg/api`, zendis's own importable entry point.** Until now,
  `zendis` was usable as a library only in pieces (`pkg/disasm`,
  `pkg/load`, `pkg/render` are all independently exported), with the
  orchestration tying them together -- resolving an input source into
  code+org, then decoding and rendering it -- living unexported inside
  `cmd/zendis/main.go`. `api.Disassemble(Options) (*render.Disassembly,
  error)` is that same pipeline, exported directly, for a caller
  embedding zendis (a tracer, a build tool, a future GUI) rather than
  shelling out to the compiled binary. `api.ResolveInput` is also
  exported standalone, for a caller that wants raw `(code, org)`
  without a full disassembly. Works on `[]byte`, never a file path --
  matching `pkg/load`'s own convention, so the library never touches
  disk itself.
- **`disasm.ParseTarget(string) (Target, error)`**, the reverse of the
  existing `Target.String()` -- found while auditing what else needed
  exporting: `Target` is an `int`, unlike the already string-convertible
  `render.Dialect`, so this parsing only existed as `cmd/zendis`'s own
  unexported inline switch until now.

### Changed

- `cmd/zendis/main.go` is now a genuinely thin CLI wrapper: flag
  parsing, file reading, and exit codes stay there; `pkg/api.Disassemble`
  does the actual work, and `disasm.ParseTarget` replaces the former
  inline `--target` switch. Verified behaviour-preserving end to end
  (raw binary, tape, and both former error paths) before trusting the
  refactor, not just by re-running the existing test suite.

### Documentation

- `README.md`'s Library section gets a new `pkg/api` subsection,
  positioned first as the recommended entry point.
- `docs/KNOWN_ISSUES.md` gets a new "Recorded decisions for future
  consumers" section: the design choices already made for a future
  GUI-based disassembler in zenzx reusing these packages (`Options`'s
  deliberate permissiveness, why no `Validate` helper was added), so
  they don't need re-deciding when that work actually begins.

## [0.3.9] — 2026-09-02

### Changed

- Bumped `github.com/ha1tch/zen80` to `v0.5.6` (from `v0.5.5`),
  verified against the real published module. Diffed the published
  commit against what had been locally prepared and tested before it
  was pushed: the only difference was an unrelated vendored-tooling
  cleanup, every actual code file byte-identical.

### Fixed

- **T-3's two hard-coded DD/FD exception checks (`HALT`, `EX DE,HL`)
  now assert directly against zen80's real dispatch**, instead of
  recovering from an expected panic (`HALT`) or logging a known
  divergence and skipping (`EX DE,HL`). Both were degraded specifically
  because zen80 v0.5.5 had confirmed bugs in both opcodes under a
  DD/FD prefix; v0.5.6 carries the fix for both, so both checks were
  upgraded to real, unconditional assertions and verified passing
  against the now-current dependency. T-3 stays `◐` (partial) --
  the exhaustive sweep of the ~200 general-case substituted opcodes
  this doesn't cover is still genuinely not attempted.

## [0.3.8] — 2026-09-02

### Changed

- Bumped `github.com/ha1tch/zentools` to `v0.8.6` (from `v0.8.5`),
  verified against the real published module -- full `build`/`vet`/
  `test`/`repoman gomod check` clean. zendis still never imports
  `pkg/basic`, so the keyword-spacing fixes in 0.8.6 don't change
  zendis's own behaviour; this is a currency bump, not a behaviour
  fix. Updated the one comment in `snapshot128.go` that names the
  currently-depended-on version to match.

## [0.3.7] — 2026-09-01

### Changed

- Dropped the local `replace github.com/ha1tch/zentools => ...` hook
  in `go.mod` -- zentools is now published on GitHub, so zendis
  depends on the real `github.com/ha1tch/zentools v0.8.3`, verified
  by fetching it through the actual module proxy (not the local
  clone) and running the full build/vet/test suite against it.
  `repoman gomod check` confirms clean.

### Fixed

- Two stale comments, both verified against the real published
  zentools v0.8.3 before being touched, not assumed stale: the
  `EncodeSNA128` bank-2/bank-5 aliasing bug this package's own
  `snapshot128.go` documented as an open upstream issue "as of
  v0.6.0" is confirmed fixed (verified with a direct test against the
  real module: it now refuses cleanly with an error instead of
  silently producing a corrupt image) -- comment updated to reflect
  this. `snapshot.go`'s `FromZ80` comment, which worked around a
  misleading claim in zentools's own `DecodeZ80` doc comment ("v2/v3
  is not yet supported", contradicted by the code, which does
  support them) is simplified now that the upstream comment itself is
  fixed -- see zentools's own v0.8.3 changelog.

## [0.3.6] — 2026-08-31

### Added

- **PZX tape input** (`--tape=FILE.pzx`, auto-detected via the "PZXT"
  signature same as `.tap`/`.tzx`). Reuses `selectBlock`'s existing
  index/name/default-to-Code selection entirely -- a PZX file's `DATA`
  blocks decode into the same `[]tap.Block` shape `FromTAP`/`FromTZX`
  already select from. Deliberately more permissive than
  `github.com/ha1tch/zentools`'s own PZX -> TAP reconstruction: no
  pulse-timing check applies here, since a `DATA` block's payload is
  already resolved bytes by the time PZX's own decoder produces it,
  regardless of what timing represented them physically on tape.
  Verified against both standard and deliberately non-standard
  (turbo-loader-style) timing. Closes T-13.
- **SZX snapshot input** (`--snapshot=FILE.szx`, auto-detected via the
  "ZXST" signature), 48K or 128K-family, with the same
  `--bank`/`--org`/`--length` options `.sna`/`.z80` already support.
  `szx.Decode` produces the identical `*snapshot.MachineState` type
  those formats already decode to, so no new flattening logic was
  needed -- just a new decode entry point into what already existed.
  Closes T-14.
- **RZX input-recording support** (`--recording=FILE.rzx`), extracting
  code from the first embedded snapshot with the same
  `--bank`/`--org`/`--length` handling a standalone snapshot gets. The
  most interesting of the three: a genuinely new input category, not
  just another format zentools happens to parse -- disassemble code
  straight from a TAS/competition recording without first manually
  extracting the snapshot. An `External` snapshot block (a filename
  reference, not embedded data) is rejected with a clear error rather
  than silently producing nothing; embedded-snapshot extension
  matching is case-insensitive. Closes T-15.

### Fixed

- **`FromTZX` now reads TZX blocks `0x11` (Turbo Speed Data) and `0x14`
  (Pure Data), not only `0x10`.** An earlier version silently dropped
  every turbo-loaded block on any real TZX using one -- a large
  fraction of real commercial tapes, which is the entire reason turbo
  loaders exist. Found and fixed for the identical reason in the
  sibling zentools project's own TZX conversion path first; this
  brings zendis's own loading up to the same standard. Filed as a
  follow-up alongside T-15 rather than reopening the already-closed
  T-10.

### Changed

- Now depends on `github.com/ha1tch/zentools` v0.8.0 (previously
  v0.6.0), for PZX/SZX/RZX parsing support.

## [0.3.5] — 2026-08-31

### Added

- **Tape and snapshot input.** `--tape=FILE` (`.tap`/`.tzx`, auto-detected
  from the file's own bytes) and `--snapshot=FILE` (`.sna`/`.z80`, v1/v2/v3,
  auto-detected across 48K and 128K-family alike) load code directly,
  instead of requiring pre-extraction to a raw binary as a separate step.
  Exactly one input source is required: a positional raw-binary path,
  `--tape`, or `--snapshot`.
  - `--tape`: `--block=N` selects a block by its position in the tape's own
    block list -- not a header+data pair count, since real tapes (confirmed
    against an actual commercial release's structure) commonly have bare
    data blocks with no header at all, for a custom loader that already
    knows their addresses; those are reachable via `--block` plus an
    explicit `--org`. `--block-name=NAME` selects by exact header name
    instead. Neither given, the first Code-type block is used.
  - `--snapshot`: org defaults to the snapshot's own `PC`. For a
    128K-family snapshot, banks 5 and 2 are always at `0x4000`/`0x8000` in
    every normal paging mode; only the top quarter varies by the
    snapshot's own paging state by default, or `--bank=N` (0-7) to select
    a specific bank directly, for inspecting one that isn't currently
    paged in. The +3's special all-RAM paging mode is refused with a
    clear error rather than guessed at.
  - `--org` and `--length` apply uniformly across all three input kinds,
    overriding whatever the source itself provides rather than requiring
    it.
  - New package `github.com/ha1tch/zendis/pkg/load`
    (`FromTAP`/`FromTZX`/`FromTape`, `FromSNA48K`/`FromZ80`/
    `FromSnapshot48K`, `FromSNA128`/`FromZ80128`/`FromSnapshot128K`,
    `FromSnapshot`), built on `github.com/ha1tch/zentools`'s format
    parsers -- exported at the library level, the same as `pkg/render`, for
    the same reason: an embedder gets the real thing, not a CLI-only
    capability.
  - Closes T-10, T-11, T-12.

### Changed

- **`pkg/render`: the loader and every output format, extracted as a
  real library package**, not CLI-internal logic. `cmd/zendis` is now a
  127-line, single-function wrapper over it -- the same relationship
  `pkg/disasm`'s `Decode`/`DecodeTarget` already has with the CLI.
  `render.Disassembly` bundles a decoded record stream with its label
  resolution and the inputs that produced them (unexported fields --
  genuine encapsulation, not just convenience): decode once, render as
  many times as needed, with `WriteAssemblySource`'s `org`/`z80nTarget`
  captured once at construction so they can no longer drift from what
  `DecodeBuffer` actually decoded with, the way two independently-supplied
  call arguments could. `cmd/zendis` itself now uses it.
- **CI**: `.github/workflows/test.yml` (native build/vet/test, Linux
  x86-64 and arm64), `cross-build.yml` (build verification across 17
  targets, `CGO_ENABLED=0`), and `release.yml` (build, package, and
  publish all 17 targets on a version tag) -- closely mirroring
  `github.com/ha1tch/zenas`'s own three workflows, adapted for
  `cmd/zendis`'s path.

### Fixed

- Found and fixed upstream, in `github.com/ha1tch/zentools` (not this
  project): `EncodeSNA128` silently produced a malformed, oversized image
  whenever the paged bank aliased bank 2 or bank 5 -- a genuine, valid
  hardware scenario. Reported and fixed in zentools v0.6.1.

## [0.3.1] — 2026-08-30

### Added

- **`pkg/render`**: the CLI's decode loop, label resolution, and all
  six output formats are now a real library package, not
  CLI-internal logic in `cmd/zendis`. `cmd/zendis` is now a thin
  wrapper (127 lines, a single `main` function) over
  `github.com/ha1tch/zendis/pkg/render`, the same relationship
  `pkg/disasm`'s `Decode`/`DecodeTarget` already has with the CLI.
  Exported: `Record`, `Dialect` (with named constants for all five
  re-assemblable targets), `DecodeBuffer`, `ResolveLabels`,
  `WriteListing`, `WriteAssemblySource`, `WriteJSON`,
  `CollectSymbols`, `WritePasmoSym`/`WriteSjasmplusSym`,
  `IsZ80NInstruction`, `PasmoUnrepresentable`, `SnasmUnrepresentable`.
- **`render.Disassembly`**: a wrapper bundling a decoded record
  stream with its label resolution and the inputs that produced
  them, so a caller decodes once and renders as many times as
  wanted without re-threading `records`/`labelFor`/`hints`/`org`/
  `z80nTarget` through every call. `cmd/zendis` itself now uses it.
  Genuine encapsulation, not just convenience: its fields are
  unexported, and `WriteAssemblySource`'s `org`/`z80nTarget` are
  captured once at construction, so they can no longer drift from
  what `DecodeBuffer` actually decoded with the way two
  independently-threaded call arguments could.

### Notes

- No CLI-observable behavior changed. Verified against the real
  pasmo, sjasmplus, and SNASM binaries after the refactor, including
  a full, unedited 6393-byte real Next program
  ([jarikomppa/specnext](https://github.com/jarikomppa/specnext)'s
  PlayFLX) still round-tripping byte-for-byte through the
  reorganized code path; `tools/roundtrip` and `tools/dialectcheck`
  both still pass in full (52/52) against the new structure.
- Register unchanged from 0.3.0 -- this was an architectural
  extraction, not work against a filed backlog item.

## [0.3.0] — 2026-08-30

### Added

- **Three more re-assemblable output dialects: `--format=pasmo`,
  `--format=sjasmplus`, `--format=snasm`**, alongside the existing
  `plain`/`zenas`. All three verified against the real assemblers, not
  just read from source — see `tools/dialectcheck`, a self-bootstrapping
  harness (dormant guard G-02) that builds pasmo and sjasmplus from
  their public source and, when SNASM's binary is supplied locally
  (it's freely distributed but not open source, so never auto-fetched),
  runs it via `mono`. 52 checks: the shared instruction-body corpus,
  twelve adversarial cases (real-world binary extracts included), and
  SNASM's own dedicated `MUL`-syntax case.
  - `pasmo`: falls back to `DB` bytes with a stderr warning for the
    three things pasmo can't represent — Z80N instructions, the
    undocumented bare `IN (C)`/`OUT (C),0` forms, and DDCB/FDCB's
    undocumented copy-to-register form.
  - `sjasmplus`: indents every non-label line (real sjasmplus parses
    an unindented column-0 directive as a label by default) and
    uppercases the `IXh`/`IXl` half-register spelling to `IXH`/`IXL`
    (the only case real sjasmplus accepts).
  - `snasm` (Mike Dailly's SNASM): converts every `0x` literal to `$`
    (its only accepted hex prefix), emits an `OPT Z80`/`OPT ZXNEXT`
    mode preamble no other dialect needs, rewrites `MUL D,E` to bare
    `MUL` (its Z80N `MUL` takes no operand syntax at all), and falls
    back to `DB` for the same three undocumented categories as
    pasmo — with the DDCB/FDCB rotate-group case being the more
    important one to catch here, since SNASM doesn't reject it, it
    silently emits the wrong opcode.
- **`--sym=<path>`** writes a symbol file matching whichever dialect
  `--format` names: `plain`/`zenas`/`pasmo` share pasmo's own
  convention (`NAME<tabs>EQU 0XXXXH`, verified byte-for-byte identical
  to real pasmo's own `--sym` output, including at its tab-padding
  boundary lengths); `sjasmplus` gets its own shape (`NAME: EQU
  0xVVVVVVVV`, also verified identical to sjasmplus's own output).
  Not available for `--format=snasm`: its own `-sym`/`-vice` flags
  produce no output at all in the tested release, and its working
  `-map` output is a structurally different artifact (source-line
  debug info) not implemented here.
- **`--format=json`**: one object per instruction or data record —
  `address`/`bytes`/`mnemonic`/`operands`/`text`/`length`/`undefined`/
  `possible_z80n`/`label`/`comment` — for feeding a listing into other
  tooling.

### Fixed

- A hints-file label naming an address that wasn't also a branch
  target (a data table's start, say) was included in `--sym` output
  but never actually declared anywhere in the accompanying
  assembly-source output — `--sym` could claim a symbol the `.asm`
  itself never defined. Label resolution now seeds every hints-file
  label unconditionally, not just ones that happen to be branch
  targets.

Closes T-08. Cross-ref: RESOLVED.md.

## [0.2.0] — 2026-08-29

### Added

- **Z80N support.** All 29 Z80N-repurposed ED opcode slots resolve to
  their real mnemonics under a new `Target` selector
  (`disasm.TargetZ80`/`disasm.TargetZ80N`, CLI `--target=z80|z80n`).
  The default target (`z80`) never guesses: undefined slots that match
  a Z80N instruction carry the name in `Instruction.PossibleZ80N`, and
  the CLI surfaces it as a stderr warning (`--warn-z80n`, on by
  default) rather than silently mislabelling real classic-Z80 code.
  Closes T-1.
- **CLI label resolution.** JP/JR/CALL/DJNZ targets are replaced with
  generated `Lxxxx` labels by default (`--labels`, on by default),
  resolved in a first decode pass so forward references work.
  `Instruction.BranchTarget()` derives the target address directly
  from an instruction's own encoded bytes. Closes T-2.
- **Public library surface beyond `Decode`/`DecodeTarget`:** the
  x/y/z/p/q bit-decomposition (`Decompose`), the register/condition/
  ALU/rotate naming tables (`RegisterName`, `RegisterPairName`,
  `RegisterPairNameAF`, `ConditionName`, `ALUName`, `ALUTakesA`,
  `RotateName`), the hex/displacement formatters (`HexByte`, `HexWord`,
  `DispText`), and the Z80N opcode table (`Z80NName`, `Z80NLookup`,
  `Z80NEntry`, `Z80NOperandKind`) — so an embedder (e.g. a live
  disassembly view over another project's own memory) can reuse
  zendis's own tables and text emitters instead of duplicating them.
  All are functions, not exported variables, specifically so an
  importer can't mutate the underlying tables out from under zendis's
  own decoder.
- **`tools/roundtrip`**, a standalone verification tool (own `go.mod`,
  not part of `go test ./...`): assembles Z80/Z80N source with zenas,
  disassembles it with zendis, reassembles zendis's own output with
  zenas again, and confirms the two binaries are byte-identical — an
  independent verification axis alongside the zen80 execution oracle,
  since it's the only one of the two that exercises zenas's parser.
  Registered as dormant guard G-01 (`docs/KNOWN_ISSUES.md`).

### Verification

- DD/FD substitution-decision boundary (the HALT and `EX DE,HL`
  exceptions, the `LD H,(IX+d)`/`LD L,(IX+d)` literal-register
  exception, half-register forms, indexed-memory addressing, 16-bit
  forms, and DDCB/FDCB including the undocumented copy-to-register
  forms) cross-checked against zen80's real execution
  (`pkg/disasm/decode_ddfd_oracle_test.go`,
  `decode_ddfd_t3_extra_oracle_test.go`,
  `decode_ddfd_t4_oracle_test.go`). Closes T-4.
- All 29 Z80N opcodes cross-checked against zen80's dispatch
  (`pkg/disasm/decode_oracle_test.go`).

### Known limitations

- T-3 (round-trip verification harness) stays open: the DD/FD
  exhaustive substitution sweep isn't attempted, and two of the
  oracle's exception checks (HALT, `EX DE,HL`) currently degrade
  gracefully rather than assert, pending a zen80 release that carries
  fixes for both (found and fixed in a local zen80 clone this cycle,
  not yet published). See `docs/TRACKING.md` for the full note.
- `tools/roundtrip` documents two confirmed gaps in zenas's current
  encoder (DD/FD-prefixed CB instructions, `EX (SP),IX`/`IY`) and two
  known-lossy undocumented encodings (the ED-NOP duplicate, the
  `IM 0/1` duplicate) that can't round-trip through mnemonic text at
  all — none are zendis defects; see the tool's own header comment.

Cross-ref: RESOLVED.md (T-1, T-2, T-4).
