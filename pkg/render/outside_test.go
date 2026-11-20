package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ha1tch/zendis/pkg/disasm"
)

// A branch to an address that no line of the source starts at needs its
// label defined, or the source does not assemble: a ROM routine, and the
// middle of an instruction.
func TestBranchTargetsOutsideTheSourceGetAnEQU(t *testing.T) {
	code := []byte{
		0xCD, 0x9B, 0x22, // 8000 CALL 0x229B   outside the code
		0x18, 0x02, //       8003 JR 0x8007     into the middle of the next instruction
		0x21, 0x00, 0xC9, // 8005 LD HL,0xC900
		0xC3, 0x00, 0x80, // 8008 JP 0x8000     a line of the source
		0xCD, 0x9B, 0x22, // 800B CALL 0x229B   the same routine again
	}
	hints := &disasm.Hints{Labels: map[uint16]string{0x229B: "BORDER", 0x0010: "PRINT_A"}}
	for _, dialect := range []Dialect{DialectPlain, DialectPasmo, DialectSjasmplus, DialectSnasm} {
		var out bytes.Buffer
		NewDisassembly(code, 0x8000, disasm.TargetZ80, hints, true).WriteAssemblySource(&out, dialect, nil)
		src := out.String()
		hex := "0x"
		if dialect == DialectSnasm {
			hex = "$"
		}
		for _, want := range []string{"BORDER EQU " + hex + "229B\n", "L8007 EQU " + hex + "8007\n", "CALL BORDER", "JR L8007", "L8000:\n", "JP L8000"} {
			if !strings.Contains(src, want) {
				t.Errorf("%s: no %q in:\n%s", dialect, want, src)
			}
		}
		if n := strings.Count(src, "BORDER EQU"); n != 1 {
			t.Errorf("%s: BORDER is defined %d times", dialect, n)
		}
		if strings.Contains(src, "L8000 EQU") {
			t.Errorf("%s: a label the source declares on a line also has an EQU:\n%s", dialect, src)
		}
		if strings.Contains(src, "PRINT_A") {
			t.Errorf("%s: a name nothing branches to is in the source:\n%s", dialect, src)
		}
		if strings.Index(src, "BORDER EQU") > strings.Index(src, "ORG") || strings.Index(src, "BORDER EQU") > strings.Index(src, "L8007 EQU") {
			t.Errorf("%s: the EQUs are not ahead of ORG in address order:\n%s", dialect, src)
		}
	}
}
