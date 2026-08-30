package render

import (
	"testing"

	"github.com/ha1tch/zendis/pkg/disasm"
)

func TestIsZ80NInstruction(t *testing.T) {
	cases := []struct {
		name  string
		bytes []byte
		want  bool
	}{
		{"MUL D,E (Z80N slot)", []byte{0xED, 0x30}, true},
		{"NEXTREG (Z80N slot)", []byte{0xED, 0x91, 0x10, 0x05}, true},
		{"PUSH nn (reuses classic PUSH opcode 0x8A slot)", []byte{0xED, 0x8A, 0x12, 0x34}, true},
		{"classic LDIR (ED-prefixed, not a Z80N slot)", []byte{0xED, 0xB0}, false},
		{"non-ED instruction", []byte{0x00}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in, err := disasm.DecodeTarget(tc.bytes, 0, 0x8000, disasm.TargetZ80N)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got := IsZ80NInstruction(in); got != tc.want {
				t.Errorf("IsZ80NInstruction(%q) = %v, want %v", in.Text(), got, tc.want)
			}
		})
	}
}

func TestPasmoUnrepresentable(t *testing.T) {
	cases := []struct {
		name       string
		bytes      []byte
		target     disasm.Target
		wantUnrep  bool
		wantReason string
	}{
		{"MUL D,E (Z80N)", []byte{0xED, 0x30}, disasm.TargetZ80N, true, "Z80N instruction"},
		{"IN (C) bare", []byte{0xED, 0x70}, disasm.TargetZ80, true, "undocumented bare IN (C)"},
		{"IN A,(C) documented, unaffected", []byte{0xED, 0x78}, disasm.TargetZ80, false, ""},
		{"OUT (C),0 bare", []byte{0xED, 0x71}, disasm.TargetZ80, true, "undocumented bare OUT (C),0"},
		{"OUT (C),A documented, unaffected", []byte{0xED, 0x79}, disasm.TargetZ80, false, ""},
		{"RLC (IX+d),B copy-to-register", []byte{0xDD, 0xCB, 0x03, 0x00}, disasm.TargetZ80, true, "undocumented DDCB/FDCB copy-to-register form"},
		{"RLC (IX+d) plain, unaffected", []byte{0xDD, 0xCB, 0x03, 0x06}, disasm.TargetZ80, false, ""},
		{"BIT 0,(IX+d) z!=6 duplicate, unaffected (no copy target at all)", []byte{0xDD, 0xCB, 0x03, 0x41}, disasm.TargetZ80, false, ""},
		{"plain ADD A,B, unaffected", []byte{0x80}, disasm.TargetZ80, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in, err := disasm.DecodeTarget(tc.bytes, 0, 0x8000, tc.target)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			gotUnrep, gotReason := PasmoUnrepresentable(in)
			if gotUnrep != tc.wantUnrep {
				t.Fatalf("PasmoUnrepresentable(%q) = %v, want %v", in.Text(), gotUnrep, tc.wantUnrep)
			}
			if gotReason != tc.wantReason {
				t.Errorf("reason = %q, want %q", gotReason, tc.wantReason)
			}
		})
	}
}

func TestSnasmUnrepresentable(t *testing.T) {
	cases := []struct {
		name      string
		bytes     []byte
		target    disasm.Target
		wantUnrep bool
	}{
		{"IN (C) bare", []byte{0xED, 0x70}, disasm.TargetZ80, true},
		{"IN A,(C) documented, unaffected", []byte{0xED, 0x78}, disasm.TargetZ80, false},
		{"OUT (C),0 bare", []byte{0xED, 0x71}, disasm.TargetZ80, true},
		{"RLC (IX+d),B copy-to-register", []byte{0xDD, 0xCB, 0x03, 0x00}, disasm.TargetZ80, true},
		{"RLC (IX+d) plain, unaffected", []byte{0xDD, 0xCB, 0x03, 0x06}, disasm.TargetZ80, false},
		{"RES 1,(IX+d),B copy-to-register", []byte{0xDD, 0xCB, 0x03, 0x89}, disasm.TargetZ80, true},
		// Unlike pasmo, SNASM has full Z80N support (confirmed: an
		// entire real 6393-byte Next program round-tripped exactly) --
		// so no Z80N instruction should ever be flagged unrepresentable.
		{"MUL D,E (Z80N, SNASM supports it via a different syntax, not a fallback)", []byte{0xED, 0x30}, disasm.TargetZ80N, false},
		{"NEXTREG (Z80N, supported)", []byte{0xED, 0x91, 0x10, 0x05}, disasm.TargetZ80N, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in, err := disasm.DecodeTarget(tc.bytes, 0, 0x8000, tc.target)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			gotUnrep, _ := SnasmUnrepresentable(in)
			if gotUnrep != tc.wantUnrep {
				t.Errorf("SnasmUnrepresentable(%q) = %v, want %v", in.Text(), gotUnrep, tc.wantUnrep)
			}
		})
	}
}
