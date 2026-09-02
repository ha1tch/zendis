# zendis — Changelog

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
