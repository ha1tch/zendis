// Package api is zendis's own importable entry point: everything
// cmd/zendis's own main() does to turn an input source into a
// disassembly -- resolving whichever kind of source was given into
// code+org via pkg/load, then decoding and rendering it via
// pkg/disasm/pkg/render -- wrapped into one call, so a caller
// embedding zendis (a tracer, a build tool, anything wanting a
// disassembly without shelling out to the CLI) doesn't have to
// duplicate that orchestration. cmd/zendis's own main() is now a thin
// wrapper around exactly this.
package api

import (
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/ha1tch/zendis/pkg/disasm"
	"github.com/ha1tch/zendis/pkg/load"
	"github.com/ha1tch/zendis/pkg/render"
	"github.com/ha1tch/zentools/pkg/symfile"
)

// SourceKind identifies which pkg/load function Options.Image should
// be resolved through. Not content-sniffing auto-detection -- the
// underlying formats aren't reliably distinguishable by content alone
// without also knowing which one the caller means, the same way the
// CLI's own --tape/--snapshot/--recording flags require the caller to
// state the kind rather than guessing from the file itself.
type SourceKind int

const (
	// SourceRaw treats Image as an already-assembled binary: code
	// starts at Org (0 if Org is nil) and runs Length bytes (or the
	// whole of Image if Length is nil).
	SourceRaw SourceKind = iota
	// SourceTape resolves Image via load.FromTape (.tap/.tzx/.pzx).
	SourceTape
	// SourceSnapshot resolves Image via load.FromSnapshot
	// (.sna/.z80/.szx, 48K or 128K-family).
	SourceSnapshot
	// SourceRecording resolves Image via load.FromRZX (the first
	// embedded snapshot in an RZX input-recording file).
	SourceRecording
	// SourceNEX resolves Image via load.FromNEX: a Spectrum Next
	// executable. Bank, Page and BankAt choose a part of it.
	SourceNEX
)

// DefaultTarget is the CPU a source of this kind is for when the caller
// has no reason to say otherwise: the Z80N for a NEX file, which runs on
// a Spectrum Next only, and the Z80 for the rest. Options.Target's zero
// value is the Z80 whatever the kind; this is what the command uses for
// --target when it is not given.
func DefaultTarget(kind SourceKind) disasm.Target {
	if kind == SourceNEX {
		return disasm.TargetZ80N
	}
	return disasm.TargetZ80
}

// ErrOnePart is WriteParts' error for a source that has nothing to list.
var ErrOnePart = errors.New("api: this kind of input is one part; Org and Length (and Bank for a 128K snapshot) choose what is taken of it")

// WriteParts lists the parts of an input that has several, with what
// each of the options that choose one takes: a tape's blocks
// (BlockIndex, BlockName) and a NEX file's banks (Bank, Page, BankAt).
// It is the command's --blocks. load.TapeBlocks and load.DescribeNEX
// return the same as data. The other kinds give ErrOnePart.
func WriteParts(w io.Writer, kind SourceKind, image []byte) error {
	switch kind {
	case SourceTape:
		return load.WriteTapeBlocks(w, image)
	case SourceNEX:
		return load.WriteNEXBanks(w, image)
	}
	return ErrOnePart
}

// Options configures a single ResolveInput or Disassemble call.
// Mirrors the CLI flags cmd/zendis's own main() exposes; fields not
// meaningful for the chosen Kind are simply ignored, the same way the
// CLI's own flag validation only enforces them conditionally.
type Options struct {
	// Kind selects which pkg/load function resolves Image, or
	// SourceRaw to treat Image as an already-assembled binary
	// directly.
	Kind SourceKind
	// Image is the source bytes: a raw binary, a tape image, a
	// snapshot, or an RZX recording, matching Kind. Required. This
	// package never reads a file itself -- a caller with a path reads
	// it first (os.ReadFile), the same way pkg/load's own functions
	// already take bytes, not paths.
	Image []byte

	// Target selects the CPU dialect for decoding. Zero value is
	// disasm.TargetZ80.
	Target disasm.Target
	// Hints carries label/comment/data/entry annotations. A nil Hints
	// is treated as empty, not an error.
	Hints *disasm.Hints
	// AutoLabels replaces JP/JR/CALL/DJNZ target addresses with
	// generated labels (Lxxxx) where Hints doesn't already name one.
	AutoLabels bool
	// Symbols is the names of other tools' symbol files (a CSpect map, a
	// zenas or pasmo .sym, an sjasmplus --sym file, a z88dk .map), made
	// with symfile.NewTable and its AddFile. Its labels name addresses
	// that Hints does not already name; its constants name branch targets
	// that lie outside the code (a ROM routine the program's source
	// defined with EQU). nil is no symbols. The table is only read, so
	// one table can serve any number of calls: a debugger builds it once
	// and disassembles whatever part of memory it is showing.
	Symbols *symfile.Table

	// Org overrides the source's own load address/PC when non-nil.
	// Meaningful for every Kind: the explicit start address for
	// SourceRaw (0 if nil), an override for the others.
	Org *uint16
	// Length limits how many bytes are resolved, starting at the
	// resolved org, when non-nil. nil means everything the source
	// itself provides.
	Length *int

	// BlockIndex selects a tape block by its 0-based position in the
	// raw block list. SourceTape only; ignored if BlockName is set.
	BlockIndex *int
	// BlockName selects a tape header block by exact name, using its
	// following data block. SourceTape only; takes precedence over
	// BlockIndex when both are set.
	BlockName string

	// Bank selects a specific 128K-family RAM bank (0-7) to appear at
	// 0xC000-0xFFFF, overriding the source's own paging state.
	// SourceSnapshot/SourceRecording only; meaningless for a 48K
	// source. For SourceNEX it is one of the file's 16 KB banks
	// (0-111), taken alone, at BankAt.
	Bank *int
	// Page selects one 8 KB page (0-223) of a NEX file, taken alone, at
	// BankAt. SourceNEX only; not given with Bank.
	Page *int
	// BankAt is the address of the first byte of the Bank or Page of a
	// NEX file: a multiple of 0x2000. nil is 0xC000. SourceNEX only.
	BankAt *uint16

	// AsData, when non-nil, writes everything that Hints does not mark
	// as data of this kind, and decodes no instruction: for a block
	// that is a table, a screen, text. Data ranges of Hints keep their
	// own kinds inside it.
	AsData *disasm.DataKind
}

// ResolveInput extracts (code, org) from opts.Image per opts.Kind --
// the first half of the pipeline Disassemble (and cmd/zendis's own
// main()) runs, exported directly for a caller that wants the raw
// bytes without a full Disassembly (custom analysis, feeding a
// different decoder entirely, and so on).
func ResolveInput(opts Options) (code []byte, org uint16, err error) {
	switch opts.Kind {
	case SourceTape:
		lopts := load.TapeOptions{Options: load.Options{Org: opts.Org, Length: opts.Length}}
		if opts.BlockName != "" {
			lopts.BlockName = opts.BlockName
		} else {
			lopts.BlockIndex = opts.BlockIndex
		}
		return load.FromTape(opts.Image, lopts)

	case SourceSnapshot:
		lopts := load.SnapshotOptions{Options: load.Options{Org: opts.Org, Length: opts.Length}, Bank: opts.Bank}
		return load.FromSnapshot(opts.Image, lopts)

	case SourceRecording:
		lopts := load.SnapshotOptions{Options: load.Options{Org: opts.Org, Length: opts.Length}, Bank: opts.Bank}
		return load.FromRZX(opts.Image, lopts)

	case SourceNEX:
		lopts := load.NEXOptions{Options: load.Options{Org: opts.Org, Length: opts.Length}, Bank: opts.Bank, Page: opts.Page, BankAt: opts.BankAt}
		return load.FromNEX(opts.Image, lopts)

	case SourceRaw:
		code := opts.Image
		org := uint16(0)
		if opts.Org != nil {
			org = *opts.Org
		}
		if opts.Length != nil {
			if *opts.Length > len(code) {
				return nil, 0, fmt.Errorf("length %d exceeds the %d bytes available", *opts.Length, len(code))
			}
			code = code[:*opts.Length]
		}
		return code, org, nil

	default:
		return nil, 0, fmt.Errorf("api: unknown SourceKind %d", opts.Kind)
	}
}

// Disassemble resolves opts.Image (via ResolveInput) and decodes it
// into a full *render.Disassembly, ready for any of its own output
// methods (WriteListing, WriteAssemblySource, WriteJSON, Symbols).
// This is the one call a library caller actually wants; cmd/zendis's
// own main() is a thin wrapper around exactly this, plus flag parsing
// and file reading.
func Disassemble(opts Options) (*render.Disassembly, error) {
	code, org, err := ResolveInput(opts)
	if err != nil {
		return nil, err
	}
	hints := WithSymbols(opts.Hints, opts.Symbols)
	if opts.Kind == SourceNEX && opts.Bank == nil && opts.Page == nil {
		// Only in the view the program starts with: another bank at the
		// same address is not where the entry point is.
		nameEntryPoint(hints, opts.Image, code, org)
	}
	if opts.AsData != nil && len(code) > 0 {
		// Last in the list: a range of the hints is found first.
		length := len(code)
		if length > 0xFFFF {
			length = 0xFFFF
		}
		hints.Data = append(append([]disasm.DataRange{}, hints.Data...),
			disasm.DataRange{Start: org, Length: uint16(length), Kind: *opts.AsData})
	}
	d := render.NewDisassembly(code, org, opts.Target, hints, opts.AutoLabels)
	if opts.Symbols == nil {
		return d, nil
	}

	// Branch targets that no line starts at and nothing has named: where a
	// constant has that value, it is the name. Then once more, with those.
	starts := map[uint16]bool{}
	for _, rec := range d.Records() {
		starts[rec.Address] = true
	}
	named := map[string]bool{}
	for _, name := range hints.Labels {
		named[name] = true
	}
	added := false
	for _, rec := range d.Records() {
		if rec.Instr == nil {
			continue
		}
		t, ok := rec.Instr.BranchTarget()
		if !ok || starts[t] {
			continue
		}
		name, isConstant := opts.Symbols.Constant(t)
		if _, taken := hints.Labels[t]; taken || !isConstant || named[name] {
			continue
		}
		hints.Labels[t] = name
		named[name] = true
		added = true
	}
	if added {
		d = render.NewDisassembly(code, org, opts.Target, hints, opts.AutoLabels)
	}
	return d, nil
}

// NEXEntryLabel is the name the entry point of a NEX file gets when
// neither the hints nor a symbol file has one for it.
const NEXEntryLabel = "nex_entry"

// nameEntryPoint labels the address a NEX file starts at, when it is in
// what is being disassembled and has no name yet.
func nameEntryPoint(hints *disasm.Hints, image, code []byte, org uint16) {
	info, err := load.DescribeNEX(image)
	if err != nil || info.PC == 0 {
		return
	}
	if int(info.PC) < int(org) || int(info.PC) >= int(org)+len(code) {
		return
	}
	if _, named := hints.Labels[info.PC]; named {
		return
	}
	for _, name := range hints.Labels {
		if name == NEXEntryLabel {
			return
		}
	}
	hints.Labels[info.PC] = NEXEntryLabel
}

// WithSymbols returns hints with the labels of a symbol table added: a
// copy, so neither argument is changed. A label the hints already have
// for an address stays, and a table name the hints use for another
// address gets its address appended. Either argument may be nil.
// Disassemble calls this itself for Options.Symbols; it is exported for
// a caller that builds a render.Disassembly directly.
func WithSymbols(hints *disasm.Hints, table *symfile.Table) *disasm.Hints {
	out := &disasm.Hints{Labels: map[uint16]string{}}
	if hints != nil {
		*out = *hints
		out.Labels = make(map[uint16]string, len(hints.Labels))
		for addr, name := range hints.Labels {
			out.Labels[addr] = name
		}
	}
	if out.Comments == nil {
		out.Comments = map[uint16]string{}
	}
	if table == nil {
		return out
	}
	named := map[string]bool{}
	for _, name := range out.Labels {
		named[name] = true
	}
	labels := table.Labels()
	addrs := make([]int, 0, len(labels))
	for addr := range labels {
		addrs = append(addrs, int(addr))
	}
	sort.Ints(addrs)
	for _, a := range addrs {
		addr := uint16(a)
		if _, taken := out.Labels[addr]; taken {
			continue
		}
		name := labels[addr]
		if named[name] {
			name = fmt.Sprintf("%s_%04X", name, addr)
		}
		named[name] = true
		out.Labels[addr] = name
	}
	return out
}
