package api

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/ha1tch/zendis/pkg/disasm"
	"github.com/ha1tch/zendis/pkg/render"
	"github.com/ha1tch/zentools/pkg/nex"
)

func nexImage(t *testing.T) []byte {
	t.Helper()
	f := &nex.File{PC: 0x8002, SP: 0xFF00, Screen: map[uint8][]byte{}, Banks: map[int][]byte{}}
	// 8000: JR 8002; 8002: NEXTREG 7,3; 8006: RET
	f.Banks[2] = append([]byte{0x18, 0x00, 0xED, 0x91, 0x07, 0x03, 0xC9}, make([]byte, nex.BankLen-7)...)
	f.Banks[20] = append([]byte("TEXT\x00"), make([]byte, nex.BankLen-5)...)
	img, err := nex.Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// TestDisassembleNEX: the view the program starts with, with the entry
// point named.
func TestDisassembleNEX(t *testing.T) {
	d, err := Disassemble(Options{Kind: SourceNEX, Image: nexImage(t), Target: disasm.TargetZ80N, AutoLabels: true})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	d.WriteAssemblySource(&out, render.DialectZenas, nil)
	want := "ORG 0x8000\nJR nex_entry\nnex_entry:\nNEXTREG 0x07,0x03\nRET\n"
	if out.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", out.String(), want)
	}
}

// TestDisassembleNEXBankAsData: another bank, written as text; no entry
// label there though the entry point's address is in its range.
func TestDisassembleNEXBankAsData(t *testing.T) {
	kind := disasm.DataText
	bank, at := 20, uint16(0x8000)
	d, err := Disassemble(Options{Kind: SourceNEX, Image: nexImage(t), Bank: &bank, BankAt: &at, AsData: &kind, AutoLabels: true})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	d.WriteAssemblySource(&out, render.DialectZenas, nil)
	if got := out.String(); got != "ORG 0x8000\nDB \"TEXT\"\n" || strings.Contains(got, NEXEntryLabel) {
		t.Errorf("got:\n%s", got)
	}
}

// TestAsDataKeepsHintRanges: a range of the hints keeps its kind inside
// a block that is data as a whole.
func TestAsDataKeepsHintRanges(t *testing.T) {
	kind := disasm.DataBytes
	org := uint16(0x9000)
	hints := &disasm.Hints{Data: []disasm.DataRange{{Start: 0x9002, Length: 2, Kind: disasm.DataWords}}}
	d, err := Disassemble(Options{Image: []byte{1, 2, 0x00, 0x80, 3}, Org: &org, Hints: hints, AsData: &kind})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	d.WriteAssemblySource(&out, render.DialectZenas, nil)
	want := "ORG 0x9000\nDB 0x01,0x02\nDW 0x8000\nDB 0x03\n"
	if out.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", out.String(), want)
	}
	if len(hints.Data) != 1 {
		t.Errorf("the caller's hints were changed: %+v", hints.Data)
	}
}

func TestDefaultTargetAndWriteParts(t *testing.T) {
	if DefaultTarget(SourceNEX) != disasm.TargetZ80N || DefaultTarget(SourceTape) != disasm.TargetZ80 {
		t.Error("the default targets are wrong")
	}
	var out bytes.Buffer
	if err := WriteParts(&out, SourceNEX, nexImage(t)); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"entry point 0x8002", "bank   2", "bank  20"} {
		if !strings.Contains(out.String(), s) {
			t.Errorf("the listing lacks %q:\n%s", s, out.String())
		}
	}
	if err := WriteParts(&out, SourceRaw, []byte{0}); !errors.Is(err, ErrOnePart) {
		t.Errorf("a raw binary: %v", err)
	}
}
