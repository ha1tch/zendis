package render

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/ha1tch/zendis/pkg/disasm"
)

func TestNewDisassembly_MatchesManualPipeline(t *testing.T) {
	// A Disassembly's Records()/LabelFor() must be identical to calling
	// DecodeBuffer/ResolveLabels by hand -- it's a convenience wrapper,
	// not a different code path.
	code := []byte{0xC3, 0x03, 0x80, 0x76} // JP 0x8003; HALT at 0x8003
	hints := &disasm.Hints{Labels: map[uint16]string{0x8003: "Main"}}

	wantRecords := DecodeBuffer(code, 0x8000, disasm.TargetZ80, hints)
	wantLabelFor := ResolveLabels(wantRecords, hints, true)

	d := NewDisassembly(code, 0x8000, disasm.TargetZ80, hints, true)

	if len(d.Records()) != len(wantRecords) {
		t.Fatalf("Records() len = %d, want %d", len(d.Records()), len(wantRecords))
	}
	for i := range wantRecords {
		if d.Records()[i].Address != wantRecords[i].Address {
			t.Errorf("Records()[%d].Address = %#04x, want %#04x", i, d.Records()[i].Address, wantRecords[i].Address)
		}
	}
	if d.LabelFor()[0x8003] != wantLabelFor[0x8003] {
		t.Errorf("LabelFor()[0x8003] = %q, want %q", d.LabelFor()[0x8003], wantLabelFor[0x8003])
	}
}

func TestNewDisassembly_NilHints(t *testing.T) {
	code := []byte{0x00} // NOP
	d := NewDisassembly(code, 0x8000, disasm.TargetZ80, nil, true)
	if len(d.Records()) != 1 || d.Records()[0].Instr == nil || d.Records()[0].Instr.Mnemonic != "NOP" {
		t.Errorf("Records() = %+v, want a single NOP record", d.Records())
	}
	if d.Hints() == nil {
		t.Error("Hints() = nil, want a non-nil empty *disasm.Hints (nil input should be normalized)")
	}
}

func TestDisassembly_TruncatedTail(t *testing.T) {
	clean := NewDisassembly([]byte{0x00}, 0x8000, disasm.TargetZ80, nil, true)
	if clean.TruncatedTail() {
		t.Error("TruncatedTail() = true for a cleanly-decoded NOP, want false")
	}

	truncated := NewDisassembly([]byte{0xDD}, 0x8000, disasm.TargetZ80, nil, true)
	if !truncated.TruncatedTail() {
		t.Error("TruncatedTail() = false for a lone DD prefix byte, want true")
	}
}

// TestDisassembly_OrgAndTargetCapturedCorrectly is the whole point of
// this type: WriteAssemblySource's org and z80nTarget can no longer
// drift from what DecodeBuffer actually used, because they're captured
// once at construction rather than re-supplied by the caller on every
// render call.
func TestDisassembly_OrgAndTargetCapturedCorrectly(t *testing.T) {
	code := []byte{0xED, 0x30} // MUL D,E -- only resolves under TargetZ80N
	d := NewDisassembly(code, 0x9000, disasm.TargetZ80N, nil, true)

	var buf bytes.Buffer
	d.WriteAssemblySource(&buf, DialectSnasm, io.Discard)
	out := buf.String()

	if !strings.Contains(out, "ORG $9000") {
		t.Errorf("output missing the org this Disassembly was built with (0x9000, rendered as $9000 under DialectSnasm):\n%s", out)
	}
	if !strings.HasPrefix(out, "OPT ZXNEXT") {
		t.Errorf("output should open with \"OPT ZXNEXT\" since this Disassembly was decoded under TargetZ80N, got:\n%s", out)
	}
	if !strings.Contains(out, "MUL") {
		t.Errorf("output missing the Z80N MUL instruction that only resolves under TargetZ80N:\n%s", out)
	}
}

func TestDisassembly_WriteMethodsMatchPackageFunctions(t *testing.T) {
	code := []byte{0xC3, 0x03, 0x80, 0x76}
	hints := &disasm.Hints{Labels: map[uint16]string{0x8003: "Main"}}
	d := NewDisassembly(code, 0x8000, disasm.TargetZ80, hints, true)

	var viaMethod, viaFunc bytes.Buffer
	d.WriteListing(&viaMethod, true, io.Discard)
	WriteListing(&viaFunc, d.Records(), d.LabelFor(), d.Hints(), true, io.Discard)
	if viaMethod.String() != viaFunc.String() {
		t.Errorf("WriteListing via method = %q, via package function = %q, want identical", viaMethod.String(), viaFunc.String())
	}

	viaMethod.Reset()
	viaFunc.Reset()
	d.WriteAssemblySource(&viaMethod, DialectZenas, io.Discard)
	WriteAssemblySource(&viaFunc, d.Records(), d.LabelFor(), d.Hints(), 0x8000, DialectZenas, false, io.Discard)
	if viaMethod.String() != viaFunc.String() {
		t.Errorf("WriteAssemblySource via method = %q, via package function = %q, want identical", viaMethod.String(), viaFunc.String())
	}

	symbolsViaMethod := d.Symbols()
	symbolsViaFunc := CollectSymbols(d.LabelFor(), d.Hints())
	if len(symbolsViaMethod) != len(symbolsViaFunc) || symbolsViaMethod["Main"] != symbolsViaFunc["Main"] {
		t.Errorf("Symbols() via method = %v, via package function = %v, want identical", symbolsViaMethod, symbolsViaFunc)
	}
}
