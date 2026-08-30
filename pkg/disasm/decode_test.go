package disasm

import "testing"

func decodeText(t *testing.T, code []byte, addr uint16) string {
	t.Helper()
	in, err := Decode(code, 0, addr)
	if err != nil {
		t.Fatalf("Decode(% X) failed: %v", code, err)
	}
	if in.Length != len(code) {
		t.Errorf("Decode(% X) consumed %d bytes, want %d", code, in.Length, len(code))
	}
	return in.Text()
}

func TestUnprefixed(t *testing.T) {
	cases := []struct {
		code []byte
		addr uint16
		want string
	}{
		{[]byte{0x00}, 0, "NOP"},
		{[]byte{0x01, 0x34, 0x12}, 0, "LD BC,0x1234"},
		{[]byte{0x7E}, 0, "LD A,(HL)"},
		{[]byte{0x80}, 0, "ADD A,B"},
		{[]byte{0x90}, 0, "SUB B"},
		{[]byte{0xB8}, 0, "CP B"},
		{[]byte{0x76}, 0, "HALT"},
		{[]byte{0x3E, 0x2A}, 0, "LD A,0x2A"},
		{[]byte{0xC3, 0x00, 0x80}, 0, "JP 0x8000"},
		{[]byte{0xCD, 0x00, 0x80}, 0, "CALL 0x8000"},
		{[]byte{0xC9}, 0, "RET"},
		{[]byte{0xC0}, 0, "RET NZ"},
		{[]byte{0xFF}, 0, "RST 0x38"},
		{[]byte{0x18, 0x02}, 0x8000, "JR 0x8004"},   // JR d: target = addr+2+d
		{[]byte{0x10, 0xFE}, 0x8000, "DJNZ 0x8000"}, // DJNZ -2: tight loop back to self
		{[]byte{0x20, 0x05}, 0x8000, "JR NZ,0x8007"},
		{[]byte{0xEB}, 0, "EX DE,HL"},
		{[]byte{0xE9}, 0, "JP (HL)"},
		{[]byte{0xF9}, 0, "LD SP,HL"},
		{[]byte{0xE3}, 0, "EX (SP),HL"},
	}
	for _, c := range cases {
		if got := decodeText(t, c.code, c.addr); got != c.want {
			t.Errorf("Decode(% X @ %#04x) = %q, want %q", c.code, c.addr, got, c.want)
		}
	}
}

func TestCB(t *testing.T) {
	cases := []struct {
		code []byte
		want string
	}{
		{[]byte{0xCB, 0x00}, "RLC B"},
		{[]byte{0xCB, 0x06}, "RLC (HL)"},
		{[]byte{0xCB, 0x7C}, "BIT 7,H"},
		{[]byte{0xCB, 0x86}, "RES 0,(HL)"},
		{[]byte{0xCB, 0xFF}, "SET 7,A"},
	}
	for _, c := range cases {
		if got := decodeText(t, c.code, 0); got != c.want {
			t.Errorf("Decode(% X) = %q, want %q", c.code, got, c.want)
		}
	}
}

func TestED(t *testing.T) {
	cases := []struct {
		code []byte
		want string
	}{
		{[]byte{0xED, 0x47}, "LD I,A"},
		{[]byte{0xED, 0x57}, "LD A,I"},
		{[]byte{0xED, 0xB0}, "LDIR"},
		{[]byte{0xED, 0xB8}, "LDDR"},
		{[]byte{0xED, 0xA1}, "CPI"},
		{[]byte{0xED, 0x78}, "IN A,(C)"},
		{[]byte{0xED, 0x79}, "OUT (C),A"},
		{[]byte{0xED, 0x44}, "NEG"},
		{[]byte{0xED, 0x45}, "RETN"},
		{[]byte{0xED, 0x4D}, "RETI"},
		{[]byte{0xED, 0x46}, "IM 0"},
		{[]byte{0xED, 0x56}, "IM 1"},
		{[]byte{0xED, 0x5E}, "IM 2"},
		{[]byte{0xED, 0x42}, "SBC HL,BC"},
		{[]byte{0xED, 0x4A}, "ADC HL,BC"},
	}
	for _, c := range cases {
		if got := decodeText(t, c.code, 0); got != c.want {
			t.Errorf("Decode(% X) = %q, want %q", c.code, got, c.want)
		}
	}

	in, err := Decode([]byte{0xED, 0x00}, 0, 0)
	if err != nil {
		t.Fatalf("Decode(ED 00) failed: %v", err)
	}
	if !in.Undefined {
		t.Errorf("Decode(ED 00) = %+v, want Undefined=true", in)
	}
}

func TestIndexedIX(t *testing.T) {
	cases := []struct {
		code []byte
		addr uint16
		want string
	}{
		{[]byte{0xDD, 0x21, 0x34, 0x12}, 0, "LD IX,0x1234"},
		{[]byte{0xDD, 0x34, 0x05}, 0, "INC (IX+0x05)"},
		{[]byte{0xDD, 0x7E, 0xFD}, 0, "LD A,(IX-0x03)"},
		{[]byte{0xDD, 0xE9}, 0, "JP (IX)"},
		{[]byte{0xDD, 0xF9}, 0, "LD SP,IX"},
		{[]byte{0xDD, 0xE3}, 0, "EX (SP),IX"},
		{[]byte{0xDD, 0xEB}, 0, "EX DE,HL"}, // documented exception: unaffected
		{[]byte{0xDD, 0x76}, 0, "HALT"},     // documented exception: unaffected
		{[]byte{0xDD, 0x19}, 0, "ADD IX,DE"},
		{[]byte{0xDD, 0x29}, 0, "ADD IX,IX"},
		{[]byte{0xDD, 0x66, 0x0A}, 0, "LD H,(IX+0x0A)"}, // dest is real H, only src substituted
		{[]byte{0xDD, 0x6C}, 0, "LD IXl,IXh"},           // both operands are half-registers
		{[]byte{0xDD, 0x36, 0x05, 0x7F}, 0, "LD (IX+0x05),0x7F"},
		{[]byte{0xDD, 0xCB, 0x05, 0x06}, 0, "RLC (IX+0x05)"},
		{[]byte{0xDD, 0xCB, 0x05, 0x46}, 0, "BIT 0,(IX+0x05)"},
		{[]byte{0xDD, 0xCB, 0x05, 0x00}, 0, "RLC (IX+0x05),B"}, // undocumented copy-to-register form
	}
	for _, c := range cases {
		if got := decodeText(t, c.code, c.addr); got != c.want {
			t.Errorf("Decode(% X) = %q, want %q", c.code, got, c.want)
		}
	}
}

func TestIndexedIY(t *testing.T) {
	cases := []struct {
		code []byte
		want string
	}{
		{[]byte{0xFD, 0x21, 0x34, 0x12}, "LD IY,0x1234"},
		{[]byte{0xFD, 0xE9}, "JP (IY)"},
		{[]byte{0xFD, 0xCB, 0x02, 0x86}, "RES 0,(IY+0x02)"},
	}
	for _, c := range cases {
		if got := decodeText(t, c.code, 0); got != c.want {
			t.Errorf("Decode(% X) = %q, want %q", c.code, got, c.want)
		}
	}
}

func TestTruncated(t *testing.T) {
	// LD BC,nn needs 3 bytes; give it only the opcode.
	if _, err := Decode([]byte{0x01}, 0, 0); err == nil {
		t.Error("Decode(01) with no operand bytes should error, got nil")
	}
	// DD prefix with nothing following.
	if _, err := Decode([]byte{0xDD}, 0, 0); err == nil {
		t.Error("Decode(DD) with nothing following should error, got nil")
	}
}
