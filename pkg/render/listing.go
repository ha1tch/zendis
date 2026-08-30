package render

import (
	"fmt"
	"io"

	"github.com/ha1tch/zendis/pkg/disasm"
)

// WriteListing renders the default address+hex+mnemonic form to w.
// Hints-marked data records are chunked into 4-byte rows -- the same
// width as the widest instruction -- so the hex column stays aligned
// throughout the listing; the trailing truncated case keeps its
// original one-line-however-long form unchanged. hints may be nil
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
		for i := 0; i < len(rec.Bytes); i += 4 {
			end := i + 4
			if end > len(rec.Bytes) {
				end = len(rec.Bytes)
			}
			chunk := rec.Bytes[i:end]
			chunkAddr := rec.Address + uint16(i)
			line := fmt.Sprintf("%04X:  %-11s DB %s", chunkAddr, hexDump(chunk), hexList(chunk))
			if i == 0 {
				if c, ok := hints.Comments[rec.Address]; ok {
					line += "  ; " + c
				}
			}
			fmt.Fprintln(w, line)
		}
	}
}
