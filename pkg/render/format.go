package render

import (
	"fmt"
	"strings"

	"github.com/ha1tch/zendis/pkg/disasm"
)

// resolvedText returns in's text with any BranchTarget substituted for
// its resolved label, if one applies -- shared by every emitter in
// this package. Substituting the exact "0xNNNN" substring BranchTarget
// resolved is safe and unambiguous: that substring appears at most once
// per Operands string, and only when BranchTarget itself reports ok.
func resolvedText(in *disasm.Instruction, labelFor map[uint16]string) string {
	text := in.Text()
	if t, ok := in.BranchTarget(); ok {
		if label, named := labelFor[t]; named {
			text = strings.Replace(text, fmt.Sprintf("0x%04X", t), label, 1)
		}
	}
	return text
}

// hexDump renders a byte slice as space-separated uppercase hex pairs,
// e.g. "DD 21 34 12", for the listing's byte column.
func hexDump(b []byte) string {
	out := make([]byte, 0, len(b)*3)
	for i, c := range b {
		if i > 0 {
			out = append(out, ' ')
		}
		out = append(out, hexDigit(c>>4), hexDigit(c&0xF))
	}
	return string(out)
}

func hexDigit(n byte) byte {
	if n < 10 {
		return '0' + n
	}
	return 'A' + (n - 10)
}

// hexList renders bytes as a comma-separated 0x list, for DB lines.
func hexList(b []byte) string {
	out := ""
	for i, c := range b {
		if i > 0 {
			out += ","
		}
		out += fmt.Sprintf("0x%02X", c)
	}
	return out
}

func toIntSlice(b []byte) []int {
	out := make([]int, len(b))
	for i, c := range b {
		out[i] = int(c)
	}
	return out
}
