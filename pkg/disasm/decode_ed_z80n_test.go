package disasm

import "testing"

// z80nCases is the full 29-slot Z80N table, byte sequences and their
// expected renderings. Byte values sourced from zen80/z80/prefix_ed.go's
// dispatch table; names cross-checked against zen80/z80/z80n_test.go's
// z80nOpcodes table. Operand bytes here are arbitrary but distinctive,
// chosen so a byte-order mistake changes the expected text.
var z80nCases = []struct {
	bytes []byte
	text  string
	hint  string // expected PossibleZ80N under TargetZ80
}{
	{[]byte{0xED, 0x23}, "SWAPNIB", "SWAPNIB"},
	{[]byte{0xED, 0x24}, "MIRROR A", "MIRROR A"},
	{[]byte{0xED, 0x27, 0x2A}, "TEST 0x2A", "TEST"},
	{[]byte{0xED, 0x28}, "BSLA DE,B", "BSLA DE,B"},
	{[]byte{0xED, 0x29}, "BSRA DE,B", "BSRA DE,B"},
	{[]byte{0xED, 0x2A}, "BSRL DE,B", "BSRL DE,B"},
	{[]byte{0xED, 0x2B}, "BSRF DE,B", "BSRF DE,B"},
	{[]byte{0xED, 0x2C}, "BRLC DE,B", "BRLC DE,B"},
	{[]byte{0xED, 0x30}, "MUL D,E", "MUL D,E"},
	{[]byte{0xED, 0x31}, "ADD HL,A", "ADD HL,A"},
	{[]byte{0xED, 0x32}, "ADD DE,A", "ADD DE,A"},
	{[]byte{0xED, 0x33}, "ADD BC,A", "ADD BC,A"},
	// Little-endian words: bytes 0x34 0x12 -> 0x1234.
	{[]byte{0xED, 0x34, 0x34, 0x12}, "ADD HL,0x1234", "ADD"},
	{[]byte{0xED, 0x35, 0x34, 0x12}, "ADD DE,0x1234", "ADD"},
	{[]byte{0xED, 0x36, 0x34, 0x12}, "ADD BC,0x1234", "ADD"},
	// PUSH's operand is BIG-endian -- the only one in the whole ISA.
	// Same byte sequence zen80's own TestZ80N_Push_BigEndianOperand
	// loads (ED 8A 12 34), asserted there to push 0x1234. A decoder
	// that wrongly reused the little-endian word reader would render
	// PUSH 0x3412 here.
	{[]byte{0xED, 0x8A, 0x12, 0x34}, "PUSH 0x1234", "PUSH"},
	{[]byte{0xED, 0x90}, "OUTINB", "OUTINB"},
	{[]byte{0xED, 0x91, 0x10, 0x05}, "NEXTREG 0x10,0x05", "NEXTREG"},
	{[]byte{0xED, 0x92, 0x10}, "NEXTREG 0x10,A", "NEXTREG"},
	{[]byte{0xED, 0x93}, "PIXELDN", "PIXELDN"},
	{[]byte{0xED, 0x94}, "PIXELAD", "PIXELAD"},
	{[]byte{0xED, 0x95}, "SETAE", "SETAE"},
	{[]byte{0xED, 0x98}, "JP (C)", "JP (C)"},
	{[]byte{0xED, 0xA4}, "LDIX", "LDIX"},
	{[]byte{0xED, 0xA5}, "LDWS", "LDWS"},
	{[]byte{0xED, 0xAC}, "LDDX", "LDDX"},
	{[]byte{0xED, 0xB4}, "LDIRX", "LDIRX"},
	{[]byte{0xED, 0xB7}, "LDPIRX", "LDPIRX"},
	{[]byte{0xED, 0xBC}, "LDDRX", "LDDRX"},
}

// TestZ80N_TargetZ80N_ResolvesAll29 sweeps every Z80N slot under
// TargetZ80N: correct text, correct length (== len of the byte
// sequence), Undefined false, no PossibleZ80N hint.
func TestZ80N_TargetZ80N_ResolvesAll29(t *testing.T) {
	if len(z80nCases) != 29 {
		t.Fatalf("z80nCases has %d entries, want 29 (the full Z80N set)", len(z80nCases))
	}
	for _, tc := range z80nCases {
		t.Run(tc.text, func(t *testing.T) {
			in, err := DecodeTarget(tc.bytes, 0, 0x8000, TargetZ80N)
			if err != nil {
				t.Fatalf("DecodeTarget(z80n): %v", err)
			}
			if in.Text() != tc.text {
				t.Errorf("text = %q, want %q", in.Text(), tc.text)
			}
			if in.Length != len(tc.bytes) {
				t.Errorf("length = %d, want %d", in.Length, len(tc.bytes))
			}
			if in.Undefined {
				t.Errorf("Undefined = true, want false")
			}
			if in.PossibleZ80N != "" {
				t.Errorf("PossibleZ80N = %q, want empty under TargetZ80N", in.PossibleZ80N)
			}
			if in.Prefix != PrefixED {
				t.Errorf("Prefix = %v, want ED", in.Prefix)
			}
		})
	}
}

// TestZ80N_TargetZ80_UndefinedWithHint sweeps the same slots under
// TargetZ80: two-byte Undefined regardless of the operand bytes that
// follow, with PossibleZ80N naming the Z80N instruction.
func TestZ80N_TargetZ80_UndefinedWithHint(t *testing.T) {
	for _, tc := range z80nCases {
		t.Run(tc.text, func(t *testing.T) {
			in, err := DecodeTarget(tc.bytes, 0, 0x8000, TargetZ80)
			if err != nil {
				t.Fatalf("DecodeTarget(z80): %v", err)
			}
			if !in.Undefined {
				t.Fatalf("Undefined = false, want true under TargetZ80")
			}
			if in.Length != 2 {
				t.Errorf("length = %d, want 2 (ED + opcode only)", in.Length)
			}
			if in.PossibleZ80N != tc.hint {
				t.Errorf("PossibleZ80N = %q, want %q", in.PossibleZ80N, tc.hint)
			}
		})
	}
}

// TestZ80N_NonZ80NSlot_NoHint: a classically-undefined slot Z80N does
// not repurpose stays Undefined under both targets, with no hint.
func TestZ80N_NonZ80NSlot_NoHint(t *testing.T) {
	for _, target := range []Target{TargetZ80, TargetZ80N} {
		in, err := DecodeTarget([]byte{0xED, 0x00}, 0, 0, target)
		if err != nil {
			t.Fatalf("DecodeTarget(%v): %v", target, err)
		}
		if !in.Undefined {
			t.Errorf("target %v: Undefined = false, want true", target)
		}
		if in.PossibleZ80N != "" {
			t.Errorf("target %v: PossibleZ80N = %q, want empty", target, in.PossibleZ80N)
		}
		if in.Length != 2 {
			t.Errorf("target %v: length = %d, want 2", target, in.Length)
		}
	}
}

// TestZ80N_DefaultDecodeIsTargetZ80: the compatibility wrapper keeps
// its pre-Target behaviour, plus the new hint.
func TestZ80N_DefaultDecodeIsTargetZ80(t *testing.T) {
	in, err := Decode([]byte{0xED, 0x30}, 0, 0)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !in.Undefined || in.Length != 2 {
		t.Errorf("Decode(ED 30) = %q len %d undefined=%v, want ??? len 2 undefined=true",
			in.Text(), in.Length, in.Undefined)
	}
	if in.PossibleZ80N != "MUL D,E" {
		t.Errorf("PossibleZ80N = %q, want \"MUL D,E\"", in.PossibleZ80N)
	}
}

// TestZ80N_TruncatedOperands: a Z80N instruction cut off mid-operand
// reports an error under TargetZ80N (the caller's DB fallback path),
// but is a complete two-byte Undefined under TargetZ80.
func TestZ80N_TruncatedOperands(t *testing.T) {
	truncated := [][]byte{
		{0xED, 0x27},       // TEST missing its immediate
		{0xED, 0x91, 0x10}, // NEXTREG n,n missing the value byte
		{0xED, 0x8A, 0x12}, // PUSH missing its low byte
		{0xED, 0x34, 0x34}, // ADD HL,nn missing the high byte
	}
	for _, code := range truncated {
		if _, err := DecodeTarget(code, 0, 0, TargetZ80N); err == nil {
			t.Errorf("TargetZ80N %X: want truncation error, got nil", code)
		}
		in, err := DecodeTarget(code, 0, 0, TargetZ80)
		if err != nil {
			t.Errorf("TargetZ80 %X: unexpected error %v", code, err)
			continue
		}
		if !in.Undefined || in.Length != 2 {
			t.Errorf("TargetZ80 %X: want 2-byte Undefined, got %q len %d",
				code, in.Text(), in.Length)
		}
	}
}
