// Package render turns a decoded []Record stream into the output
// formats zendis's own CLI produces -- the listing form, four
// re-assemblable dialects (zenas/pasmo/sjasmplus/snasm), JSON, and two
// symbol-file shapes -- as a library, not just CLI-internal logic.
//
// This package was originally all unexported code inside
// cmd/zendis/main.go. It moved here so an embedder can call the same
// loader and emitters zendis's own CLI does, rather than either
// shelling out to the compiled binary (which is what tools/roundtrip
// and tools/dialectcheck had to do before this existed) or
// reimplementing the dialect-specific knowledge from scratch -- the
// hardest-won part of this package, given how much of it (the DDCB
// copy-to-register matrix, SNASM's literal/mode-preamble/MUL
// requirements, sjasmplus's indentation and half-register rules) only
// came from testing real assembler binaries directly.
//
// cmd/zendis is now a thin CLI wrapper over this package, the same
// relationship pkg/disasm's Decode/DecodeTarget already has with it.
package render

import (
	"fmt"

	"github.com/ha1tch/zendis/pkg/disasm"
)

// Record unifies the two things a linear sweep produces: a decoded
// instruction, or a run of bytes that isn't one -- either a
// hints-marked data range or a trailing can't-decode tail. Every
// Write* function in this package walks the same []Record rather than
// re-deriving this split itself.
type Record struct {
	Address   uint16
	Bytes     []byte
	Instr     *disasm.Instruction // nil for a data record
	Truncated bool                // data record only: true for the trailing case, false for a marked range
	Kind      disasm.DataKind     // data record only: how the range asked to be written
}

// DecodeBuffer performs a linear sweep of code, starting at org under
// target. Before decoding at each position, it checks whether one of
// hints's data ranges starts there or already covers it (T-05); if so
// it emits the whole range as one data Record and skips past it
// without attempting to decode through it. This is a linear sweep, not
// control-flow-guided -- hints's data ranges are the only thing that
// can tell it "don't decode here" short of walking into an Undefined
// opcode. hints may be nil (treated as empty).
//
// The last Record has Truncated set to true if code ended
// mid-instruction. DecodeBuffer itself never reports this as an error
// value -- a truncated tail is a meaningful, valid partial result, not
// a failure -- so a caller that wants a CLI-style exit code should
// check that field on the final record directly.
func DecodeBuffer(code []byte, org uint16, target disasm.Target, hints *disasm.Hints) []Record {
	if hints == nil {
		hints = &disasm.Hints{}
	}
	var records []Record
	addr := org
	offset := 0
	for offset < len(code) {
		if dr, ok := hints.DataAt(addr); ok {
			length := int(dr.Length)
			if length <= 0 {
				length = 1 // malformed/zero-length range: don't spin forever
			}
			if offset+length > len(code) {
				length = len(code) - offset
			}
			records = append(records, Record{Address: addr, Bytes: code[offset : offset+length], Kind: dr.Kind})
			addr += uint16(length)
			offset += length
			continue
		}
		in, err := disasm.DecodeTarget(code, offset, addr, target)
		if err != nil {
			records = append(records, Record{Address: addr, Bytes: code[offset:], Truncated: true})
			break
		}
		records = append(records, Record{Address: in.Address, Bytes: in.Bytes, Instr: in})
		addr += uint16(in.Length)
		offset += in.Length
	}
	return records
}

// ResolveLabels builds the branch-target label map for records: every
// hints-file label is seeded first, unconditionally -- not just ones
// that happen to be branch targets. A label naming a data table's
// start, say, is a real symbol worth declaring in
// WriteAssemblySource's output even though nothing jumps to it, and
// CollectSymbols always includes every hints label regardless of
// branch-target status -- seeding only branch-target hints here would
// leave --sym claiming a symbol the assembly-source output never
// actually declares (a real pasmo round-trip test once caught exactly
// that inconsistency: --sym claimed a symbol the .asm never defined,
// so pasmo's own regenerated .sym for that file came back empty).
//
// Auto-generated Lxxxx labels then fill in for branch targets that
// don't already have a hints-file name, only when autoLabels is set,
// so an explicit hints label always applies even with autoLabels
// false. hints may be nil (treated as empty).
func ResolveLabels(records []Record, hints *disasm.Hints, autoLabels bool) map[uint16]string {
	if hints == nil {
		hints = &disasm.Hints{}
	}
	labelFor := map[uint16]string{}
	for addr, name := range hints.Labels {
		labelFor[addr] = name
	}
	for _, rec := range records {
		if rec.Instr == nil {
			continue
		}
		if t, ok := rec.Instr.BranchTarget(); ok {
			if _, named := labelFor[t]; !named && autoLabels {
				labelFor[t] = fmt.Sprintf("L%04X", t)
			}
		}
	}
	return labelFor
}
