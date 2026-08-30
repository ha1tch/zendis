package render

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/ha1tch/zendis/pkg/disasm"
)

func TestWriteAssemblySource_LabelDeclaration(t *testing.T) {
	// Plain format's whole point is that "Main:" must appear, or a
	// re-assembler has nothing to resolve "JP Main" against. JP's
	// target must actually land on HALT: JP is 3 bytes (0x8000-0x8002),
	// so HALT is at 0x8003, not 0x8004 -- an earlier version of this
	// test got that off by one and both label assertions passed for
	// the wrong reason (the label substitution doesn't care whether a
	// record exists at the target address; only the declaration check
	// below does).
	code := []byte{0xC3, 0x03, 0x80, 0x76} // JP 0x8003; HALT at 0x8003
	hints := &disasm.Hints{Labels: map[uint16]string{0x8003: "Main"}}
	records := DecodeBuffer(code, 0x8000, disasm.TargetZ80, hints)
	labelFor := ResolveLabels(records, hints, true)

	var buf bytes.Buffer
	WriteAssemblySource(&buf, records, labelFor, hints, 0x8000, DialectZenas, false, io.Discard)
	out := buf.String()

	wantLines := []string{"ORG 0x8000", "JP Main", "Main:", "HALT"}
	for _, want := range wantLines {
		if !strings.Contains(out, want) {
			t.Errorf("plain output missing %q:\n%s", want, out)
		}
	}
	// The label declaration must come before the instruction it labels.
	if strings.Index(out, "Main:") > strings.Index(out, "HALT") {
		t.Errorf("Main: declared after HALT, not before:\n%s", out)
	}
}

func TestWriteAssemblySource_UserLabelWinsOverAuto(t *testing.T) {
	code := []byte{0xC3, 0x03, 0x80, 0x76} // JP 0x8003; HALT at 0x8003
	hints := &disasm.Hints{Labels: map[uint16]string{0x8003: "Entry"}}
	records := DecodeBuffer(code, 0x8000, disasm.TargetZ80, hints)
	labelFor := ResolveLabels(records, hints, true) // autoLabels=true, but hints.Labels should still win

	var buf bytes.Buffer
	WriteAssemblySource(&buf, records, labelFor, hints, 0x8000, DialectZenas, false, io.Discard)
	out := buf.String()

	if !strings.Contains(out, "JP Entry") {
		t.Errorf("user-supplied label 'Entry' did not win over generated 'L8003':\n%s", out)
	}
	if strings.Contains(out, "L8003") {
		t.Errorf("generated label leaked through despite a user override:\n%s", out)
	}
}

func TestWriteAssemblySource_PasmoZ80NFallback(t *testing.T) {
	// MUL D,E (ED 30) has no pasmo representation; DialectPasmo must
	// fall back to a DB line, not emit a mnemonic pasmo would reject.
	code := []byte{0xED, 0x30, 0x76} // MUL D,E; HALT
	hints := &disasm.Hints{}
	records := DecodeBuffer(code, 0x8000, disasm.TargetZ80N, hints)
	labelFor := map[uint16]string{}

	var buf bytes.Buffer
	WriteAssemblySource(&buf, records, labelFor, hints, 0x8000, DialectPasmo, false, io.Discard)
	out := buf.String()

	if strings.Contains(out, "MUL") {
		t.Errorf("pasmo output still contains the Z80N mnemonic MUL:\n%s", out)
	}
	if !strings.Contains(out, "DB 0xED,0x30") {
		t.Errorf("pasmo output missing the DB fallback for MUL D,E's bytes:\n%s", out)
	}
	if !strings.Contains(out, "HALT") {
		t.Errorf("pasmo output missing the following HALT instruction:\n%s", out)
	}
}

func TestWriteAssemblySource_NonPasmoKeepsZ80NMnemonic(t *testing.T) {
	// The same instruction under DialectZenas must keep its real
	// mnemonic -- the fallback is pasmo/snasm-specific.
	code := []byte{0xED, 0x30}
	hints := &disasm.Hints{}
	records := DecodeBuffer(code, 0x8000, disasm.TargetZ80N, hints)
	labelFor := map[uint16]string{}

	var buf bytes.Buffer
	WriteAssemblySource(&buf, records, labelFor, hints, 0x8000, DialectZenas, false, io.Discard)
	out := buf.String()

	if !strings.Contains(out, "MUL D,E") {
		t.Errorf("non-pasmo output lost the Z80N mnemonic:\n%s", out)
	}
}

func TestSjasmplusHalfRegisterFix(t *testing.T) {
	// Confirmed against the real sjasmplus binary: it rejects mixed-case
	// "IXh"/"IXl" ("Label not found: IXh" -- parsed as an undefined
	// label, not the register) but accepts all-caps "IXH"/"IXL". Real
	// pasmo accepts the mixed-case form fine, so this fix is sjasmplus-
	// specific, never applied to the other dialects.
	cases := []struct{ in, want string }{
		{"INC IXh", "INC IXH"},
		{"DEC IXl", "DEC IXL"},
		{"LD B,IXh", "LD B,IXH"},
		{"LD IXh,IXl", "LD IXH,IXL"},
		{"ADD A,IYh", "ADD A,IYH"},
		{"NOP", "NOP"}, // untouched when no half-register is present
	}
	for _, tc := range cases {
		if got := sjasmplusHalfRegisterFix(tc.in); got != tc.want {
			t.Errorf("sjasmplusHalfRegisterFix(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSnasmLiteralFix(t *testing.T) {
	// Confirmed against the real SNASM binary: 0x8000 fails outright
	// ("Label next to number without white space"); $8000 is the
	// required form. Also confirms the fix itself: an earlier version
	// used ReplaceAllString with a "$${1}" replacement, which Go's
	// regexp expansion syntax parses as a literal "$" (from "$$")
	// followed by literal text "{1}" -- not a group reference at all
	// -- producing "${1}" verbatim in the output instead of the hex
	// digits. Caught by testing against the real binary, not by
	// reading the Go regexp docs closely enough the first time.
	cases := []struct{ in, want string }{
		{"LD A,0x2A", "LD A,$2A"},
		{"JP 0x9CF4", "JP $9CF4"},
		{"LD HL,0x1234", "LD HL,$1234"},
		{"DB 0x00,0x01,0xFF", "DB $00,$01,$FF"},
		{"NOP", "NOP"}, // untouched when no literal is present
		{"ORG 0x8000", "ORG $8000"},
	}
	for _, tc := range cases {
		got := snasmLiteralFix(tc.in)
		if got != tc.want {
			t.Errorf("snasmLiteralFix(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if strings.Contains(got, "${") {
			t.Errorf("snasmLiteralFix(%q) = %q contains a literal \"${\" -- the ReplaceAllString escaping bug is back", tc.in, got)
		}
	}
}

func TestSnasmMulFix(t *testing.T) {
	// Confirmed against the real binary: SNASM's Z80N MUL has no
	// operand syntax at all (fixed on D*E->DE); "MUL D,E" fails even
	// under "opt illegal". Every other Z80N form checked matched
	// zendis's existing rendering with no change needed -- this is the
	// one exception.
	cases := []struct{ in, want string }{
		{"MUL D,E", "MUL"},
		{"NEXTREG 0x07,0x03", "NEXTREG 0x07,0x03"}, // untouched
		{"NOP", "NOP"},
	}
	for _, tc := range cases {
		if got := snasmMulFix(tc.in); got != tc.want {
			t.Errorf("snasmMulFix(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestWriteAssemblySource_SnasmOptPreamble(t *testing.T) {
	code := []byte{0x00} // NOP
	hints := &disasm.Hints{}
	records := DecodeBuffer(code, 0x8000, disasm.TargetZ80, hints)
	labelFor := map[uint16]string{}

	var buf bytes.Buffer
	WriteAssemblySource(&buf, records, labelFor, hints, 0x8000, DialectSnasm, false, io.Discard)
	if !strings.HasPrefix(buf.String(), "OPT Z80\n") {
		t.Errorf("classic-target snasm output should start with \"OPT Z80\\n\", got:\n%s", buf.String())
	}

	buf.Reset()
	WriteAssemblySource(&buf, records, labelFor, hints, 0x8000, DialectSnasm, true, io.Discard)
	if !strings.HasPrefix(buf.String(), "OPT ZXNEXT\n") {
		t.Errorf("z80n-target snasm output should start with \"OPT ZXNEXT\\n\", got:\n%s", buf.String())
	}

	// Confirm no other dialect gets a preamble at all.
	buf.Reset()
	WriteAssemblySource(&buf, records, labelFor, hints, 0x8000, DialectZenas, false, io.Discard)
	if strings.HasPrefix(buf.String(), "OPT") {
		t.Errorf("zenas output should have no OPT preamble, got:\n%s", buf.String())
	}
}

func TestWriteAssemblySource_SnasmFallbackAndLiteralFix(t *testing.T) {
	// RLC (IX+3),B: must fall back to DB for SNASM (silent-wrong if
	// emitted as-is), and the DB line's bytes must use $-prefixed hex,
	// not 0x -- exercising the fallback and the literal fix together,
	// since the fallback path builds its own DB line independently of
	// resolvedText.
	code := []byte{0xDD, 0xCB, 0x03, 0x00}
	hints := &disasm.Hints{}
	records := DecodeBuffer(code, 0x8000, disasm.TargetZ80, hints)
	labelFor := map[uint16]string{}

	var buf bytes.Buffer
	WriteAssemblySource(&buf, records, labelFor, hints, 0x8000, DialectSnasm, false, io.Discard)
	out := buf.String()

	if strings.Contains(out, "RLC") {
		t.Errorf("snasm output still contains the unrepresentable RLC copy-form:\n%s", out)
	}
	if !strings.Contains(out, "DB $DD,$CB,$03,$00") {
		t.Errorf("snasm output missing the $-formatted DB fallback:\n%s", out)
	}
	if strings.Contains(out, "0x") {
		t.Errorf("snasm output still contains 0x-prefixed hex somewhere:\n%s", out)
	}
}
