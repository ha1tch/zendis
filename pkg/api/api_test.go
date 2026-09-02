package api

import (
	"strings"
	"testing"

	"github.com/ha1tch/zendis/pkg/disasm"
	"github.com/ha1tch/zentools/pkg/tap"
)

func u16Ptr(n uint16) *uint16 { return &n }
func intPtr(n int) *int       { return &n }

// TestDisassemble_SourceRaw confirms the exact pipeline cmd/zendis's
// own main() runs -- ResolveInput then render.NewDisassembly -- works
// end to end through the one exported call a library caller actually
// wants.
func TestDisassemble_SourceRaw(t *testing.T) {
	code := []byte{0x21, 0x00, 0x80, 0x3E, 0x05, 0xC9} // LD HL,0x8000; LD A,5; RET
	d, err := Disassemble(Options{
		Kind:  SourceRaw,
		Image: code,
		Org:   u16Ptr(0x8000),
	})
	if err != nil {
		t.Fatalf("Disassemble: %v", err)
	}
	var b strings.Builder
	d.WriteListing(&b, false, &b)
	got := b.String()
	for _, want := range []string{"LD HL,0x8000", "LD A,0x05", "RET"} {
		if !strings.Contains(got, want) {
			t.Errorf("listing = %q, want it to contain %q", got, want)
		}
	}
}

// TestDisassemble_SourceTape confirms the tape path resolves the
// header's own load address, matching what a caller would get from
// zendis's --tape flag -- verified against a real, zentools-encoded
// tape image, not a hand-built one, mirroring pkg/load's own test
// convention (fixtures built with tap.EncodeCode directly, not static
// testdata files).
func TestDisassemble_SourceTape(t *testing.T) {
	code := []byte{0x3E, 0x2A, 0xC9} // LD A,0x2A; RET
	image := tap.EncodeCode("TESTCODE", code, 0x8000)

	d, err := Disassemble(Options{
		Kind:  SourceTape,
		Image: image,
	})
	if err != nil {
		t.Fatalf("Disassemble: %v", err)
	}
	var b strings.Builder
	d.WriteListing(&b, false, &b)
	got := b.String()
	if !strings.Contains(got, "8000") || !strings.Contains(got, "LD A,0x2A") {
		t.Errorf("listing = %q, want the header's own 0x8000 org and LD A,0x2A", got)
	}
}

// TestResolveInput_Standalone confirms ResolveInput is independently
// usable without a full Disassemble call -- the exact use case a
// caller wanting raw code+org for its own analysis needs.
func TestResolveInput_Standalone(t *testing.T) {
	code := []byte{0xC9} // RET
	gotCode, gotOrg, err := ResolveInput(Options{
		Kind:  SourceRaw,
		Image: code,
		Org:   u16Ptr(0x9000),
	})
	if err != nil {
		t.Fatalf("ResolveInput: %v", err)
	}
	if gotOrg != 0x9000 {
		t.Errorf("org = %#04x, want 0x9000", gotOrg)
	}
	if string(gotCode) != string(code) {
		t.Errorf("code = % X, want % X", gotCode, code)
	}
}

// TestResolveInput_LengthExceedsAvailable confirms the SourceRaw path
// still catches an over-length request, the same invariant
// cmd/zendis's own former resolveInput enforced, now surfaced without
// a CLI flag name or file path baked into the message (this package
// never reads a file itself).
func TestResolveInput_LengthExceedsAvailable(t *testing.T) {
	_, _, err := ResolveInput(Options{
		Kind:   SourceRaw,
		Image:  []byte{0x01, 0x02, 0x03},
		Length: intPtr(10),
	})
	if err == nil {
		t.Fatal("expected an error for length exceeding available bytes, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error = %q, want it to mention the length exceeding available bytes", err.Error())
	}
}

// TestResolveInput_UnknownSourceKind confirms an invalid Kind value
// (not one of the four exported constants) fails clearly rather than
// falling through to unintended behaviour.
func TestResolveInput_UnknownSourceKind(t *testing.T) {
	_, _, err := ResolveInput(Options{
		Kind:  SourceKind(99),
		Image: []byte{0x00},
	})
	if err == nil {
		t.Fatal("expected an error for an unknown SourceKind, got nil")
	}
}

// TestDisassemble_NilHintsIsEmpty confirms a nil Hints (the zero value
// of Options) is treated as empty, not an error -- the common case for
// a caller that has no annotations to supply.
func TestDisassemble_NilHintsIsEmpty(t *testing.T) {
	d, err := Disassemble(Options{
		Kind:  SourceRaw,
		Image: []byte{0xC9}, // RET
		Hints: nil,
	})
	if err != nil {
		t.Fatalf("Disassemble with nil Hints: %v", err)
	}
	if d == nil {
		t.Fatal("Disassemble returned a nil *Disassembly with no error")
	}
}

// TestDisassemble_Z80NTarget confirms the Target option is actually
// threaded through to the decoder, not silently defaulted.
func TestDisassemble_Z80NTarget(t *testing.T) {
	// ED 24 is MIRROR A, a genuine Z80N-only opcode -- undefined under
	// plain Z80.
	code := []byte{0xED, 0x24}
	d, err := Disassemble(Options{
		Kind:   SourceRaw,
		Image:  code,
		Target: disasm.TargetZ80N,
	})
	if err != nil {
		t.Fatalf("Disassemble: %v", err)
	}
	var b strings.Builder
	d.WriteListing(&b, false, &b)
	got := b.String()
	if !strings.Contains(got, "MIRROR") {
		t.Errorf("listing = %q, want it to contain MIRROR (Z80N target not threaded through)", got)
	}
}
