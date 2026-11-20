# zendis — as a library

Every option of the command is a field or a function of these packages.
[USAGE.md](USAGE.md) describes what each one does.

## `pkg/api`: the recommended entry point

`pkg/api` is the whole pipeline `cmd/zendis` itself runs -- resolving
an input source (a raw binary, a tape, a snapshot, or an RZX
recording) into code+org, then decoding and rendering it -- wrapped
into one call, for a caller embedding zendis (a tracer, a build tool,
anything wanting a disassembly without shelling out to the compiled
binary) rather than duplicating that orchestration itself:

```go
import "github.com/ha1tch/zendis/pkg/api"

d, err := api.Disassemble(api.Options{
    Kind:       api.SourceTape,     // or SourceRaw/SourceSnapshot/SourceRecording/SourceNEX
    Image:      tapeBytes,          // already-read bytes, matching pkg/load's own
                                     // convention -- this package never touches disk
    Target:     disasm.TargetZ80N,
    AutoLabels: true,
})
d.WriteListing(os.Stdout, true, os.Stderr)
```

Every option of the command is a field or a function here:

| Command | Library |
|---------|---------|
| `--nex`, `--tape`, `--snapshot`, `--recording` | `Options.Kind` (`SourceNEX`, `SourceTape`, ...) with the file's bytes in `Image` |
| `--target` | `Options.Target`; `api.DefaultTarget(kind)` is what the command uses when it is not given (the Z80N for a NEX file) |
| `--org`, `--length`, `--block`, `--block-name` | `Options.Org`, `Length`, `BlockIndex`, `BlockName` |
| `--bank`, `--page`, `--bank-at` | `Options.Bank`, `Page`, `BankAt` |
| `--blocks` | `api.WriteParts(w, kind, image)`; `load.TapeBlocks` and `load.DescribeNEX` return the same as data |
| `--data=ADDR:LENGTH[:KIND]` | `disasm.ParseDataRange`, appended to `Hints.Data` (`disasm.DataRange{Start, Length, Kind}`) |
| `--as=KIND` | `Options.AsData` (`disasm.ParseDataKind` reads the name) |
| `--hints` | `disasm.ParseHints` into `Options.Hints` |
| `--symbols` | `symfile.Table` in `Options.Symbols` |
| `--format` | the `Disassembly`'s `WriteListing`, `WriteAssemblySource(w, dialect, warn)`, `WriteJSON` |
| `--sym`, `--map` | `render.WritePasmoSym`/`WriteSjasmplusSym` of `d.Symbols()`, `d.WriteCSpectMap` |

`render.DataRows` gives the lines of one data record in a dialect, for
a caller that draws its own listing.

With zenas's `pkg/artifact` the two meet in one process: assemble,
`artifact.Encode` a NEX file, `api.Disassemble` it with the program's
own names (`assembler.ExportSymbols` into a `symfile.Table`), and the
source that comes out assembles to the same file again.

`Options.Kind` states which `pkg/load` function resolves `Image` --
not content-sniffing auto-detection, since none of the formats are
reliably distinguishable by content alone without also knowing which
one the caller means, the same way the CLI's own
`--tape`/`--snapshot`/`--recording` flags require the caller to name
the kind. `api.ResolveInput` is also exported directly, for a caller
that wants the raw `(code, org)` pair without a full `Disassembly` --
custom analysis, or feeding a different decoder entirely.

`cmd/zendis`'s own `main()` is now a thin wrapper around exactly this:
flag parsing, reading the chosen file from disk, and exit codes live
there; `pkg/api.Disassemble` is the one call doing the actual work,
so a library caller and the CLI can never drift out of sync on what
"disassemble this" actually means.

A future GUI-based disassembler in zenzx is expected to reuse this
same package; see `docs/KNOWN_ISSUES.md`'s "Recorded decisions for
future consumers" section for the design choices already made with
that consumer in mind (`Options`'s permissiveness, why no `Validate`
was added, and so on) before re-deciding any of them.

## `pkg/disasm`: the decoder, as a library

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

## `pkg/render`: the loader and every output format, as a library

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

## Symbol files, as a library

The symbol-file code is zentools' `pkg/symfile`
(`github.com/ha1tch/zentools/pkg/symfile`), which zenas uses too.
`symfile.Table` is the names of one or more symbol files, kept for as
long as a program is loaded: `NewTable`, then `AddFile(r, format)` for
each file (a CSpect map, a pasmo or zenas `.sym`, an sjasmplus `--sym`
file, a z88dk `.map`), or `Add(symbols)`, in order of preference.
`Label(addr)`, `Constant(value)` and `Name(addr)` look a name up. Set
the table as `api.Options.Symbols` and `api.Disassemble` uses its labels
for addresses the hints do not name and its constants for branch
targets outside the code, which is all `--symbols` does. The table is
only read, so one serves any number of calls.
`(*render.Disassembly).WriteCSpectMap` is `--map`.

## With zenas in one process: an emulator's debugger

An emulator that embeds both can assemble a program, keep its names and
show any part of memory with them, with no files:

```go
import (
    "github.com/ha1tch/zenas/assembler"
    "github.com/ha1tch/zendis/pkg/api"
    "github.com/ha1tch/zentools/pkg/symfile"
)

result, _ := assembler.New().AssembleString(source)

names := symfile.NewTable()                        // built once
names.Add(assembler.ExportSymbols(result, assembler.DefaultNextPaging))
names.AddFile(romSymbols, symfile.FormatAuto)      // and the ROM's, if there are any

org := pc                                          // each time the view moves
d, _ := api.Disassemble(api.Options{
    Image: memory[org : org+64], Org: &org,
    Target: disasm.TargetZ80N, AutoLabels: true, Symbols: names,
})
d.WriteListing(view, false, nil)                   // CALL BORDER, DJNZ loop
```
