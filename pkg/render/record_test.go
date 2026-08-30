package render

import (
	"testing"

	"github.com/ha1tch/zendis/pkg/disasm"
)

func TestDecodeBuffer_DataRange(t *testing.T) {
	code := []byte{
		0x00, 0x01, 0x02, 0x03, // 8000: hints-marked data
		0x76, // 8004: HALT
	}
	hints := &disasm.Hints{Data: []disasm.DataRange{{Start: 0x8000, Length: 4}}}
	records := DecodeBuffer(code, 0x8000, disasm.TargetZ80, hints)

	if len(records) != 2 {
		t.Fatalf("got %d records, want 2 (one data range, one HALT)", len(records))
	}
	if records[0].Instr != nil || len(records[0].Bytes) != 4 || records[0].Truncated {
		t.Errorf("record 0 = %+v, want a 4-byte non-truncated data record", records[0])
	}
	if records[1].Instr == nil || records[1].Instr.Mnemonic != "HALT" {
		t.Errorf("record 1 = %+v, want HALT", records[1])
	}
}

func TestDecodeBuffer_TruncatedTail(t *testing.T) {
	code := []byte{0xDD} // a lone DD prefix byte: can't decode a full instruction
	records := DecodeBuffer(code, 0x8000, disasm.TargetZ80, nil)
	if len(records) != 1 || !records[len(records)-1].Truncated {
		t.Errorf("got %+v, want exactly one record with Truncated=true", records)
	}
}

func TestDecodeBuffer_NilHints(t *testing.T) {
	// hints may be nil; must not panic and must decode normally.
	code := []byte{0x00} // NOP
	records := DecodeBuffer(code, 0x8000, disasm.TargetZ80, nil)
	if len(records) != 1 || records[0].Instr == nil || records[0].Instr.Mnemonic != "NOP" {
		t.Errorf("got %+v, want a single NOP record", records)
	}
}

func TestResolveLabels_HintsSeededUnconditionally(t *testing.T) {
	// A hints label naming an address that is NOT a branch target must
	// still appear in the map -- this is the exact bug a real pasmo
	// round-trip test once caught: a hints-only label was in --sym
	// output but never declared in the accompanying assembly source,
	// because label resolution only seeded hints names that happened
	// to coincide with a branch target.
	code := []byte{0x00} // NOP, no branches at all
	hints := &disasm.Hints{Labels: map[uint16]string{0x9000: "DataTable"}}
	records := DecodeBuffer(code, 0x8000, disasm.TargetZ80, hints)

	labelFor := ResolveLabels(records, hints, true)
	if labelFor[0x9000] != "DataTable" {
		t.Errorf("labelFor[0x9000] = %q, want \"DataTable\" (hints labels must be seeded unconditionally, not just branch targets)", labelFor[0x9000])
	}
}

func TestResolveLabels_AutoLabelsRespected(t *testing.T) {
	code := []byte{0xC3, 0x03, 0x80, 0x76} // JP 0x8003; HALT at 0x8003

	records := DecodeBuffer(code, 0x8000, disasm.TargetZ80, nil)

	withAuto := ResolveLabels(records, nil, true)
	if withAuto[0x8003] != "L8003" {
		t.Errorf("autoLabels=true: labelFor[0x8003] = %q, want \"L8003\"", withAuto[0x8003])
	}

	withoutAuto := ResolveLabels(records, nil, false)
	if _, ok := withoutAuto[0x8003]; ok {
		t.Errorf("autoLabels=false: labelFor[0x8003] should be absent, got %q", withoutAuto[0x8003])
	}
}
