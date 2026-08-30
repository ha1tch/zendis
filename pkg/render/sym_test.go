package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ha1tch/zendis/pkg/disasm"
)

func TestWritePasmoSym_Format(t *testing.T) {
	// Real pasmo's own format (asm.cxx: dumpsymbol/tablabel), padding by
	// name length: 2 tabs under 8 chars, 1 tab for 8-15, a space at 16+.
	symbols := map[string]uint16{
		"Main":                0x8000, // 4 chars -> 2 tabs
		"GameLoop":            0x8010, // 8 chars -> 1 tab
		"VeryLongLabelName16": 0x9000, // 19 chars -> 1 space
	}
	var buf bytes.Buffer
	if err := WritePasmoSym(&buf, symbols); err != nil {
		t.Fatalf("WritePasmoSym: %v", err)
	}
	out := buf.String()

	wantLines := []string{
		"Main\t\tEQU 08000H",
		"GameLoop\tEQU 08010H",
		"VeryLongLabelName16 EQU 09000H",
	}
	for _, want := range wantLines {
		if !strings.Contains(out, want) {
			t.Errorf("pasmo sym output missing %q:\n%s", want, out)
		}
	}
	// Sorted by name.
	if strings.Index(out, "GameLoop") > strings.Index(out, "Main") {
		t.Errorf("pasmo sym output not sorted by name:\n%s", out)
	}
}

func TestWriteSjasmplusSym_Format(t *testing.T) {
	// Real sjasmplus's own format (sjio.cpp: WriteLabelEquValue/PrintHex32):
	// "NAME: EQU 0xVVVVVVVV", colon-then-EQU, 0x-prefix, 8 hex digits.
	symbols := map[string]uint16{"Main": 0x8000}
	var buf bytes.Buffer
	if err := WriteSjasmplusSym(&buf, symbols); err != nil {
		t.Fatalf("WriteSjasmplusSym: %v", err)
	}
	want := "Main: EQU 0x00008000\n"
	if buf.String() != want {
		t.Errorf("sjasmplus sym output = %q, want %q", buf.String(), want)
	}
}

func TestCollectSymbols_IncludesNonBranchTargetHints(t *testing.T) {
	// A hints-file label naming a data table's start address isn't a
	// branch target of anything in the listing, but it's still a real
	// symbol worth exporting.
	labelFor := map[uint16]string{0x8010: "Main"} // a real branch target
	hints := &disasm.Hints{Labels: map[uint16]string{
		0x8010: "Main",  // also in labelFor -- must not duplicate/conflict
		0x9000: "Table", // never a branch target
	}}
	symbols := CollectSymbols(labelFor, hints)
	if symbols["Main"] != 0x8010 {
		t.Errorf("symbols[Main] = %#04x, want 0x8010", symbols["Main"])
	}
	if symbols["Table"] != 0x9000 {
		t.Errorf("symbols[Table] missing or wrong -- got %#04x", symbols["Table"])
	}
	if len(symbols) != 2 {
		t.Errorf("len(symbols) = %d, want 2 (no duplicate for Main)", len(symbols))
	}
}
