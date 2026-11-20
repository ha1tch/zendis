package render

import (
	"io"

	"github.com/ha1tch/zendis/pkg/disasm"
	"github.com/ha1tch/zentools/pkg/symfile"
)

// Disassembly bundles a decoded record stream with its label
// resolution and the inputs that produced them -- the "decode once,
// render many times" unit every Write* function in this package
// operates on. Fields are unexported: org and target in particular
// exist so a caller can't pass WriteAssemblySource a z80nTarget that
// doesn't match what DecodeBuffer actually decoded with, which was
// possible when a caller threaded records/labelFor/hints/org/target
// through every call by hand.
//
// The package-level DecodeBuffer/ResolveLabels/Write*/CollectSymbols
// functions remain available directly for callers who need something
// more custom than this type provides -- Disassembly is the
// convenient, encapsulated path for the common case, not a
// replacement for them.
type Disassembly struct {
	records  []Record
	labelFor map[uint16]string
	hints    *disasm.Hints
	org      uint16
	target   disasm.Target
}

// NewDisassembly decodes code (T-05-aware, honoring hints's marked
// data ranges) and resolves its labels in one step. hints may be nil
// (treated as empty). autoLabels controls whether branch targets
// without a hints-file name get a generated Lxxxx label -- see
// ResolveLabels.
func NewDisassembly(code []byte, org uint16, target disasm.Target, hints *disasm.Hints, autoLabels bool) *Disassembly {
	if hints == nil {
		hints = &disasm.Hints{}
	}
	records := DecodeBuffer(code, org, target, hints)
	return &Disassembly{
		records:  records,
		labelFor: ResolveLabels(records, hints, autoLabels),
		hints:    hints,
		org:      org,
		target:   target,
	}
}

// TruncatedTail reports whether decoding ended in a partial,
// undecodable tail rather than a clean instruction boundary -- the
// CLI-style signal for a non-zero exit code, without a caller needing
// to reach into Records() and check the last element directly.
func (d *Disassembly) TruncatedTail() bool {
	return len(d.records) > 0 && d.records[len(d.records)-1].Truncated
}

// Records returns the decoded record stream. The returned slice
// shares Disassembly's own backing array -- a caller that needs an
// independently mutable copy should copy it.
func (d *Disassembly) Records() []Record {
	return d.records
}

// LabelFor returns the resolved branch-target label map, keyed by
// address. Shares Disassembly's own backing map.
func (d *Disassembly) LabelFor() map[uint16]string {
	return d.labelFor
}

// Hints returns the hints this Disassembly was built with.
func (d *Disassembly) Hints() *disasm.Hints {
	return d.hints
}

// WriteListing renders the default address+hex+mnemonic form to w.
// See the package-level WriteListing for the exact format and warnOut
// semantics.
func (d *Disassembly) WriteListing(w io.Writer, warnZ80N bool, warnOut io.Writer) {
	WriteListing(w, d.records, d.labelFor, d.hints, warnZ80N, warnOut)
}

// WriteAssemblySource renders dialect's re-assemblable form to w.
// z80nTarget is d's own decode target, not a separately-supplied
// value -- see the package-level WriteAssemblySource for the exact
// per-dialect behavior.
func (d *Disassembly) WriteAssemblySource(w io.Writer, dialect Dialect, warnOut io.Writer) {
	WriteAssemblySource(w, d.records, d.labelFor, d.hints, d.org, dialect, d.target == disasm.TargetZ80N, warnOut)
}

// WriteJSON renders the machine-readable JSON form to w.
func (d *Disassembly) WriteJSON(w io.Writer) error {
	return WriteJSON(w, d.records, d.labelFor, d.hints)
}

// Symbols returns the full name->address symbol set (CollectSymbols),
// ready for WritePasmoSym or WriteSjasmplusSym.
func (d *Disassembly) Symbols() map[string]uint16 {
	return CollectSymbols(d.labelFor, d.hints)
}

// WriteCSpectMap writes the same symbols as a CSpect map file, the kind
// its debugger loads with -map: every one a label, with the physical
// address the Next's memory gives it when a program starts
// (symfile.NextPhys).
func (d *Disassembly) WriteCSpectMap(w io.Writer) error {
	return symfile.WriteCSpectMap(w, symfile.FromLabels(d.Symbols()))
}
