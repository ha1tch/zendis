# zendis — Changelog

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
