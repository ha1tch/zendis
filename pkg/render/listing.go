package render

import (
	"fmt"
	"io"

	"github.com/ha1tch/zendis/pkg/disasm"
)

// WriteListing renders the default address+hex+mnemonic form to w.
// Hints-marked data records are written by their kind (DataRows), in
// zenas's directives: numbers in 4-byte rows -- the same width as the
// widest instruction -- so the hex column stays aligned, strings in
// rows of up to 16 bytes, a fill as one DS row. The trailing truncated
// case keeps its original one-line-however-long form unchanged. hints may be nil
// (treated as empty).
//
// When warnZ80N is set, an undefined opcode that matches a Z80N
// instruction is reported to warnOut (pass io.Discard to suppress it
// while still passing true, or false to skip the check entirely).
func WriteListing(w io.Writer, records []Record, labelFor map[uint16]string, hints *disasm.Hints, warnZ80N bool, warnOut io.Writer) {
	if hints == nil {
		hints = &disasm.Hints{}
	}
	if warnOut == nil {
		warnOut = io.Discard
	}
	for _, rec := range records {
		if rec.Instr != nil {
			in := rec.Instr
			marker := " "
			if in.Undefined {
				marker = "*"
			}
			if warnZ80N && in.PossibleZ80N != "" {
				fmt.Fprintf(warnOut, "zendis: WARN %04X: %s undefined under z80 target, matches Z80N %s -- rerun with --target=z80n if this is Next code\n",
					in.Address, hexDump(in.Bytes), in.PossibleZ80N)
			}
			line := fmt.Sprintf("%04X:%s %-11s %s", in.Address, marker, hexDump(in.Bytes), resolvedText(in, labelFor))
			if c, ok := hints.Comments[in.Address]; ok {
				line += "  ; " + c
			}
			fmt.Fprintln(w, line)
			continue
		}
		if rec.Truncated {
			fmt.Fprintf(w, "%04X:  %-11s DB %s\n", rec.Address, hexDump(rec.Bytes), hexList(rec.Bytes))
			continue
		}
		for _, row := range DataRows(rec, DialectZenas, true, nil) {
			line := fmt.Sprintf("%04X:  %-11s %s", row.Address, rowHex(row), row.Text)
			if c, ok := hints.Comments[row.Address]; ok {
				line += "  ; " + c
			}
			fmt.Fprintln(w, line)
		}
	}
}
