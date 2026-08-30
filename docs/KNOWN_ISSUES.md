# zendis — Known Issues / Dormant Guards

Updated: 2026-08-30

Intentional limits and invariant boundaries live here; the dormant-guards
table tracks verification that doesn't run in the default test invocation
(`go test ./...` from the module root), per this project's tracking
conventions. A guard's specification (the check exists) and its execution
record (it ran, when, where, on what) are different facts — only the
second is evidence.

## Dormant guards

### G-01. zenas round-trip verification (`tools/roundtrip`)

- **Gate condition:** own `go.mod` (separate module, so zenas's dependency
  tree — including `zentools` — never touches this module's own
  `go.mod`/`go.sum`), not reachable via `go test ./...` from the zendis
  module root.
- **Hardware requirements:** none. Network only on first `go mod tidy`
  (to fetch zenas); fully offline afterward.
- **Canonical invocation:** `cd tools/roundtrip && go run .`
- **What it checks:** assembles Z80/Z80N source with zenas, disassembles
  the binary with zendis, reassembles zendis's own output text with
  zenas again, and confirms the two binaries are byte-identical — a
  verification axis the zen80 execution oracle (decode_oracle_test.go,
  decode_ddfd_oracle_test.go) can't cover, since that path never touches
  zenas's parser at all.
- **Known, documented exclusions** (confirmed gaps in zenas's own
  encoder, not zendis defects — see the file's own header comment):
  DD/FD-prefixed CB instructions (rotate/BIT/RES/SET on `(IX+d)`/`(IY+d)`,
  undocumented copy-to-register forms included) and `EX (SP),IX`/`IY`.
  Two further known-lossy encodings (the undocumented ED-NOP duplicate,
  the undocumented `IM 0/1` duplicate) are checked separately and
  reported as expected mismatches, not failures.
- **Last exercised:** 2026-08-29, env: sandbox (this session) — 7/7 real
  cases passed (`unprefixed mix`, `CB rotates and bit ops`, `ED classic`,
  `Z80N extensions`, `DD/FD substitution`, `IXh/IXl and IYh/IYl matrix`,
  `rare-but-documented ED forms`); both known-lossy cases confirmed
  mismatching exactly as documented.

### G-02. pasmo/sjasmplus/snasm round-trip verification (`tools/dialectcheck`)

- **Gate condition:** own `go.mod` (separate module; zenas is a
  test-fixture-generation dependency here, not what's under test), not
  reachable via `go test ./...` from the zendis module root. Genuinely
  environment-dependent in a way G-01 isn't: needs `git`, `g++`, and
  (for sjasmplus) `make` to build pasmo and sjasmplus from source the
  first time; `mono` to run SNasm.exe (a Mono/.NET assembly, not a
  native binary). `bootstrap.go` checks `$PATH`, then a per-user build
  cache (`os.UserCacheDir()/zendis-dialectcheck/`), before building --
  a from-scratch run is slow (sjasmplus especially); a cached rerun is
  fast. SNASM is different from the other two in one important way:
  it's freely distributed but not open source, so `findSnasm` never
  attempts to fetch or build it -- it only checks `$PATH` (a `snasm`
  wrapper) and the cache dir, and if neither has it, the harness prints
  where to get it (https://mdf200.itch.io/snasm) and skips SNASM's
  checks rather than failing the whole run. pasmo/sjasmplus stay
  mandatory (they can always be built from source); SNASM is the one
  genuinely optional participant.
- **Hardware requirements:** the build toolchain above, plus `mono` if
  SNASM checks are wanted.
- **Canonical invocation:** `cd tools/dialectcheck && go run .`
- **What it checks:** the same byte-identity property G-01 checks for
  zenas, but against the real pasmo, sjasmplus, and (when available)
  SNASM binaries -- catches anything specific to how each actually
  parses its `--format` output that reading source or docs alone can
  miss. Findings so far:
  - Unindented directives at column 0 are parsed as labels by default
    in sjasmplus (`Options::syx.IsPseudoOpBOF`/`--dirbol`, confirmed
    in `sjasm/parser.cpp`'s `ParseLabel`) -- fixed by indenting
    non-label lines for that dialect.
  - Real sjasmplus rejects the mixed-case `IXh`/`IXl` spelling zendis
    shares with zenas and pasmo, accepting only all-caps `IXH`/`IXL`
    (confirmed in `z80.cpp`'s `GetRegister`/`GetRegister_3letter`) --
    fixed with `sjasmplusHalfRegisterFix`, sjasmplus-only.
  - The ten (now twelve) adversarial cases found that real pasmo also
    rejects the undocumented bare `IN (C)`/`OUT (C),0` forms, and
    DDCB/FDCB's undocumented copy-to-register forms (e.g.
    `RLC (IX+d),B`) -- neither was covered by the original Z80N-only
    fallback. `pasmoUnrepresentable` now covers all three categories.
  - The same adversarial pass found a real, dialect-independent bug: a
    hints-file label that wasn't also a branch target (e.g. naming a
    data table's start) was included in `--sym` output
    (`collectSymbols` always includes every hints label) but never
    actually declared as a `Name:` line in the accompanying
    assembly-source output. A real pasmo round-trip test caught this
    directly: `--sym` claimed a symbol the `.asm` never defined, so
    pasmo's own regenerated `.sym` for that file came back empty.
    Fixed by seeding label resolution with every hints-file label
    unconditionally.
  - SNASM needs `$`-prefixed hex (`0x` fails outright: "Label next to
    number without white space" -- SNASM's own changelog says this
    check was added in the tested release, V3.2.3, specifically for
    numbers glued to label-like tokens) and an `OPT Z80`/`OPT ZXNEXT`
    mode preamble no other dialect here needs.
  - SNASM's Z80N `MUL` has no operand syntax at all (fixed on
    D*E->DE); `MUL D,E` -- correct for every other dialect -- fails
    even under `opt illegal`. Found by testing the real binary
    directly, not by trusting SNASM's own docs (which turned out to
    have at least three stale lines once cross-checked: `ADD HL,nn`'s
    entry is described as "Add A to HL", `BSLA`/`BSRA`/`BSRL` are all
    listed as the same opcode, and `MIRROR DE` is still documented
    despite SNASM's own changelog recording its removal in 2018 --
    confirmed removed by testing it directly). Every other Z80N form
    checked (`SWAPNIB`, `OUTINB`, `PIXELDN`, `PIXELAD`, `SETAE`,
    `LDIX`/`LDWS`/`LDDX`/`LDIRX`/`LDDRX`, `MIRROR A`, the `ADD`
    HL/DE/BC forms, `PUSH` imm16, `TEST`, the five barrel-shift
    instructions, `NEXTREG`, `JP (C)`) matched zendis's existing
    rendering exactly, no fix needed.
  - SNASM's version of the DDCB/FDCB copy-to-register gap is worse
    than pasmo's for the rotate group specifically
    (`RLC`/`RRC`/`RL`/`RR`/`SLA`/`SRA`/`SLL`/`SRL`): it doesn't reject
    the syntax, it silently accepts it and emits the wrong
    (non-copying) opcode -- `0 Errors, 0 Warnings`, no diagnostic at
    all. Checked mechanically across all 8 rotate mnemonics x 7
    register targets (56 combinations), no exceptions. `RES`/`SET`'s
    copy-form fails safely instead (42/42 sampled combinations
    rejected) -- same category as pasmo. `snasmUnrepresentable`
    (sharing `undocumentedFormUnrepresentable` with
    `pasmoUnrepresentable`, minus the Z80N check SNASM doesn't need)
    covers both.
  - `-sym` and `-vice` genuinely produce no output at all in the
    tested release (V3.2.3.0) -- tried both single- and double-hyphen
    spellings, several filename/ordering variants, cleanly isolated
    each time. `-map` works and writes a real CSpect-format file
    (`[SYMBOLS]`/`[FILES]`/`[LINES]`/`[END]` sections, label names
    uppercased), but it's a different kind of artifact (source-line
    debug info, not just a symbol table) and isn't implemented here --
    `--sym` is rejected for `--format=snasm` with an explanation
    rather than silently producing nothing useful.
  - Also confirmed: `writePasmoSym` and `writeSjasmplusSym` output is
    byte-for-byte identical to each real assembler's own
    `--sym`/positional-symbol-file output, including at pasmo's own
    tab-padding boundary lengths (7/8/15/16-character names).
- **Corpus:** the original six cases (unprefixed mix, CB, ED classic,
  DD/FD substitution, IXh/IXl matrix, Z80N extensions, pasmo Z80N
  fallback) plus twelve adversarial cases: two unedited real-world
  extracts (a slice of `zexall.com`, Ian Bartholomew's Z80 instruction
  exerciser, via zen80's test ROM set; the real Sinclair ZX Spectrum
  48K ROM's reset routine and RST vector table), two real snippets from
  Sidewize by Steve Whetherill -- shared for technical discussion by
  MartianGirl (SpecIDE's author) via Mastodon
  (https://mas.to/@FromTheFourthPlanet/117185231157817130),
  independently cross-checked against a 2007 reverse-engineering
  writeup of the same routine
  (http://www.zxdesign.info/sidewizeTest.shtml), used with attribution
  -- a keyword-colliding label (`END`), IX/IY displacement at the
  signed-byte extremes, long label names at pasmo's tab-padding
  boundaries, SLL, DDCB/FDCB plain rotate/bit forms, DDCB/FDCB's
  undocumented copy-to-register form, the undocumented bare IN/OUT
  forms, and a synthesized-but-realistic ZX Spectrum Next idiom
  (NEXTREG palette/Layer 2 setup, MUL for a sprite-row-offset
  calculation) -- run against pasmo and sjasmplus (36 checks). SNASM,
  when available, runs the shared corpus, the adversarial cases, and
  its own dedicated MUL-syntax case (15 further checks; 52 total when
  all three assemblers are present). Each non-raw case's starting
  binary is assembled by zenas from readable source; the real-world/
  Sidewize cases and the four cases zenas itself cannot assemble (SLL,
  DDCB-with-displacement, the undocumented copy-to-register and IN/OUT
  forms -- all separately-documented zenas gaps) use verified raw
  bytes instead, decoded and cross-checked against zendis's own
  oracle-tested decoder before use, not hand-derived and trusted
  blind.
- **Last exercised:** 2026-08-30, env: sandbox (this session; pasmo,
  sjasmplus, and SNasm.exe all present -- pasmo/sjasmplus built from
  source into the session's cache, SNasm.exe placed there manually per
  its own non-open-source constraint, none persisted beyond the
  session) -- 52/52 checks passed after all fixes above.
