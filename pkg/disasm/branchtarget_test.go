package disasm

import "testing"

func TestBranchTarget(t *testing.T) {
	cases := []struct {
		name       string
		bytes      []byte
		addr       uint16
		wantTarget uint16
		wantOK     bool
	}{
		{"JP nn", []byte{0xC3, 0x00, 0x90}, 0x8000, 0x9000, true},
		{"JP cc,nn", []byte{0xCA, 0x00, 0x90}, 0x8000, 0x9000, true}, // JP Z,nn
		{"JP (HL) has no encoded target", []byte{0xE9}, 0x8000, 0, false},
		{"CALL nn", []byte{0xCD, 0x00, 0x90}, 0x8000, 0x9000, true},
		{"CALL cc,nn", []byte{0xC4, 0x00, 0x90}, 0x8000, 0x9000, true}, // CALL NZ,nn
		{"JR d forward", []byte{0x18, 0x05}, 0x8000, 0x8007, true},     // 0x8000+2+5
		{"JR d backward", []byte{0x18, 0xFB}, 0x8000, 0x7FFD, true},    // d=-5: 0x8000+2-5
		{"JR cc,d", []byte{0x20, 0x05}, 0x8000, 0x8007, true},          // JR NZ,d
		{"DJNZ d", []byte{0x10, 0x05}, 0x8000, 0x8007, true},
		{"RST has no label-worthy target", []byte{0xFF}, 0x8000, 0, false}, // RST 0x38
		{"non-branching instruction", []byte{0x00}, 0x8000, 0, false},      // NOP
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in, err := Decode(tc.bytes, 0, tc.addr)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			target, ok := in.BranchTarget()
			if ok != tc.wantOK {
				t.Fatalf("%s: BranchTarget() ok = %v, want %v (decoded as %q)", tc.name, ok, tc.wantOK, in.Text())
			}
			if ok && target != tc.wantTarget {
				t.Errorf("%s: BranchTarget() = %#04x, want %#04x", tc.name, target, tc.wantTarget)
			}
		})
	}
}

// TestBranchTarget_DDFDPassThrough confirms BranchTarget still resolves
// correctly for JR/DJNZ/JP cc,nn/CALL cc,nn reached through a DD/FD
// prefix's "unaffected" pass-through path (decode_ddfd.go): the extra
// prefix byte only prepends the encoding, so "displacement is the last
// byte" / "address is the last two bytes" still hold.
func TestBranchTarget_DDFDPassThrough(t *testing.T) {
	in, err := Decode([]byte{0xDD, 0x18, 0x05}, 0, 0x8000) // DD-prefixed JR d
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	target, ok := in.BranchTarget()
	if !ok || target != 0x8008 { // 0x8000 + Length(3) + 5
		t.Errorf("DD-prefixed JR d: BranchTarget() = %#04x, ok=%v, want 0x8008, true", target, ok)
	}
}
