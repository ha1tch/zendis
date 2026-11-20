package api

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ha1tch/zendis/pkg/disasm"
	"github.com/ha1tch/zendis/pkg/render"
	"github.com/ha1tch/zentools/pkg/symfile"
)

// The embedder's case: one table for the life of a program, and any
// window of memory disassembled with its names.
func TestDisassembleWithSymbols(t *testing.T) {
	table := symfile.NewTable()
	if _, err := table.AddFile(strings.NewReader(
		"0000229B 0000029B 01 BORDER\n"+
			"00008000 00008000 00 start\n"+
			"00008005 00008005 00 loop\n"+
			"0000800F 0000800F 00 table\n"+
			"00008012 00008012 00 done\n"), symfile.FormatAuto); err != nil {
		t.Fatal(err)
	}
	code := []byte{
		0x21, 0x0F, 0x80, // 8000 LD HL,0x800F
		0x06, 0x03, //       8003 LD B,3
		0x7E,             //             8005 LD A,(HL)
		0xCD, 0x9B, 0x22, // 8006 CALL 0x229B
		0x23,       //             8009 INC HL
		0x10, 0xF9, //       800A DJNZ 0x8005
		0xC3, 0x12, 0x80, // 800C JP 0x8012
		0x01, 0x02, 0x03, // 800F data
		0xC9, //             8012 RET
	}
	org := uint16(0x8000)
	hints := &disasm.Hints{
		Labels: map[uint16]string{0x8000: "entry"}, // the hints' name for 8000 wins
		Data:   []disasm.DataRange{{Start: 0x800F, Length: 3}},
	}
	d, err := Disassemble(Options{Image: code, Org: &org, Hints: hints, AutoLabels: true, Symbols: table})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	d.WriteAssemblySource(&out, render.DialectPlain, nil)
	src := out.String()
	for _, want := range []string{"BORDER EQU 0x229B\n", "entry:\n", "loop:\n", "CALL BORDER\n", "DJNZ loop\n", "JP done\n", "table:\n", "done:\n"} {
		if !strings.Contains(src, want) {
			t.Errorf("no %q in:\n%s", want, src)
		}
	}
	if strings.Contains(src, "start:") {
		t.Errorf("the table's name replaced the hints' one:\n%s", src)
	}
	if len(hints.Labels) != 1 {
		t.Errorf("the caller's hints were changed: %v", hints.Labels)
	}

	// The map of what was disassembled.
	var m bytes.Buffer
	if err := d.WriteCSpectMap(&m); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"0000229B 0001E29B 00 BORDER\n", "00008000 00008000 00 entry\n", "00008012 00008012 00 done\n"} {
		if !strings.Contains(m.String(), want) {
			t.Errorf("no %q in the map:\n%s", want, m.String())
		}
	}

	// A second window with the same table: the part from the loop on.
	org2 := uint16(0x8005)
	d2, err := Disassemble(Options{Image: code[5:12], Org: &org2, AutoLabels: true, Symbols: table})
	if err != nil {
		t.Fatal(err)
	}
	var listing bytes.Buffer
	d2.WriteListing(&listing, false, nil)
	if !strings.Contains(listing.String(), "CALL BORDER") || !strings.Contains(listing.String(), "DJNZ loop") {
		t.Errorf("the second window:\n%s", listing.String())
	}

	// No symbols: as before.
	d3, _ := Disassemble(Options{Image: code, Org: &org, AutoLabels: true})
	out.Reset()
	d3.WriteAssemblySource(&out, render.DialectPlain, nil)
	if !strings.Contains(out.String(), "CALL L229B") {
		t.Errorf("without symbols:\n%s", out.String())
	}

	// WithSymbols with nothing to add still gives usable hints.
	if h := WithSymbols(nil, nil); h == nil || h.Labels == nil {
		t.Error("WithSymbols(nil, nil) is not usable")
	}
}
