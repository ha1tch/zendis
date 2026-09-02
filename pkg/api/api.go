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
	"fmt"

	"github.com/ha1tch/zendis/pkg/disasm"
	"github.com/ha1tch/zendis/pkg/load"
	"github.com/ha1tch/zendis/pkg/render"
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
)

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
	// source.
	Bank *int
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
	hints := opts.Hints
	if hints == nil {
		hints = &disasm.Hints{}
	}
	return render.NewDisassembly(code, org, opts.Target, hints, opts.AutoLabels), nil
}
