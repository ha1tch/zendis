package disasm

// T-3 extension: oracle checks for the DD/FD-specific substitution
// paths not covered by decode_ddfd_oracle_test.go's boundary-case
// suite -- the 16-bit forms (rpSub/rp2Sub tables: LD IX,nn, ADD IX,rp,
// PUSH/POP IX, JP (IX), LD SP,IX, EX (SP),IX) and 8-bit ALU-with-
// (IX+d), all genuinely DD/FD-specific logic distinct from the shared
// unprefixed-table code the general-case substitution otherwise
// delegates to.

import (
	"testing"

	"github.com/ha1tch/zen80/z80"
)

func TestDDFD_Oracle_SixteenBitForms(t *testing.T) {
	t.Run("LD IX,nn", func(t *testing.T) {
		bytes := []byte{0xDD, 0x21, 0x34, 0x12} // LD IX,0x1234
		in, err := Decode(bytes, 0, 0x8000)
		if err != nil {
			t.Fatalf("zendis decode: %v", err)
		}
		if in.Text() != "LD IX,0x1234" {
			t.Fatalf("zendis decoded %q, want \"LD IX,0x1234\"", in.Text())
		}
		cpu, _ := newPlainOracle(bytes, 0x8000)
		cpu.Step()
		if cpu.IX() != 0x1234 {
			t.Errorf("zen80 IX = %#04x, want 0x1234", cpu.IX())
		}
	})

	t.Run("ADD IX,DE", func(t *testing.T) {
		bytes := []byte{0xDD, 0x19} // ADD IX,DE
		in, err := Decode(bytes, 0, 0x8000)
		if err != nil {
			t.Fatalf("zendis decode: %v", err)
		}
		if in.Text() != "ADD IX,DE" {
			t.Fatalf("zendis decoded %q, want \"ADD IX,DE\"", in.Text())
		}
		cpu, _ := newPlainOracle(bytes, 0x8000)
		cpu.SetIX(0x1000)
		cpu.SetDE(0x0234)
		cpu.SetHL(0x9999) // sentinel: ADD IX,DE must not touch real HL
		cpu.Step()
		if cpu.IX() != 0x1234 {
			t.Errorf("zen80 IX = %#04x, want 0x1234", cpu.IX())
		}
		if cpu.HL() != 0x9999 {
			t.Errorf("zen80 HL changed to %#04x -- ADD IX,DE leaked into real HL", cpu.HL())
		}
	})

	t.Run("PUSH IX / POP IX round-trip", func(t *testing.T) {
		push := []byte{0xDD, 0xE5} // PUSH IX
		in, err := Decode(push, 0, 0x8000)
		if err != nil || in.Text() != "PUSH IX" {
			t.Fatalf("zendis decoded %q (err=%v), want \"PUSH IX\"", in.Text(), err)
		}
		pop := []byte{0xDD, 0xE1} // POP IX
		in2, err := Decode(pop, 0, 0x8000)
		if err != nil || in2.Text() != "POP IX" {
			t.Fatalf("zendis decoded %q (err=%v), want \"POP IX\"", in2.Text(), err)
		}

		cpu, mem := newPlainOracle(push, 0x8000)
		cpu.SetIX(0xBEEF)
		cpu.SP = 0xF000
		cpu.Step()
		if cpu.SP != 0xEFFE {
			t.Fatalf("zen80 SP after PUSH IX = %#04x, want 0xEFFE", cpu.SP)
		}
		// POP it right back on the same CPU/memory to confirm the value
		// round-trips through the stack correctly.
		cpu.PC = 0x8000
		mem.Load(0x8000, pop)
		cpu.IXH, cpu.IXL = 0, 0 // clear before popping back
		cpu.Step()
		if cpu.IX() != 0xBEEF {
			t.Errorf("zen80 IX after POP IX = %#04x, want 0xBEEF (round-trip through PUSH)", cpu.IX())
		}
	})

	t.Run("JP (IX)", func(t *testing.T) {
		bytes := []byte{0xDD, 0xE9} // JP (IX)
		in, err := Decode(bytes, 0, 0x8000)
		if err != nil || in.Text() != "JP (IX)" {
			t.Fatalf("zendis decoded %q (err=%v), want \"JP (IX)\"", in.Text(), err)
		}
		cpu, _ := newPlainOracle(bytes, 0x8000)
		cpu.SetIX(0x9500)
		cpu.Step()
		if cpu.PC != 0x9500 {
			t.Errorf("zen80 PC after JP (IX) = %#04x, want 0x9500", cpu.PC)
		}
	})

	t.Run("LD SP,IX", func(t *testing.T) {
		bytes := []byte{0xDD, 0xF9} // LD SP,IX
		in, err := Decode(bytes, 0, 0x8000)
		if err != nil || in.Text() != "LD SP,IX" {
			t.Fatalf("zendis decoded %q (err=%v), want \"LD SP,IX\"", in.Text(), err)
		}
		cpu, _ := newPlainOracle(bytes, 0x8000)
		cpu.SetIX(0xABCD)
		cpu.Step()
		if cpu.SP != 0xABCD {
			t.Errorf("zen80 SP = %#04x, want 0xABCD", cpu.SP)
		}
	})

	t.Run("EX (SP),IX", func(t *testing.T) {
		bytes := []byte{0xDD, 0xE3} // EX (SP),IX
		in, err := Decode(bytes, 0, 0x8000)
		if err != nil || in.Text() != "EX (SP),IX" {
			t.Fatalf("zendis decoded %q (err=%v), want \"EX (SP),IX\"", in.Text(), err)
		}
		cpu, mem := newPlainOracle(bytes, 0x8000)
		cpu.SetIX(0x1111)
		cpu.SP = 0x9000
		mem.Write(0x9000, 0x22) // low byte of the stack value: 0x2222
		mem.Write(0x9001, 0x22) // high byte
		cpu.Step()
		if cpu.IX() != 0x2222 {
			t.Errorf("zen80 IX after EX (SP),IX = %#04x, want 0x2222 (value that was on the stack)", cpu.IX())
		}
		lo, hi := mem.Read(0x9000), mem.Read(0x9001)
		if got := uint16(hi)<<8 | uint16(lo); got != 0x1111 {
			t.Errorf("zen80 stack after EX (SP),IX = %#04x, want 0x1111 (IX's original value)", got)
		}
	})
}

func TestDDFD_Oracle_ALUWithIndexedMemory(t *testing.T) {
	t.Run("ADD A,(IX+d)", func(t *testing.T) {
		bytes := []byte{0xDD, 0x86, 0x04} // ADD A,(IX+4)
		in, err := Decode(bytes, 0, 0x8000)
		if err != nil {
			t.Fatalf("zendis decode: %v", err)
		}
		if in.Text() != "ADD A,(IX+0x04)" {
			t.Fatalf("zendis decoded %q, want \"ADD A,(IX+0x04)\"", in.Text())
		}
		cpu, mem := newPlainOracle(bytes, 0x8000)
		cpu.SetIX(0x9000)
		cpu.A = 0x10
		mem.Write(0x9004, 0x05)
		cpu.Step()
		if cpu.A != 0x15 {
			t.Errorf("zen80 A after ADD A,(IX+4) = %#02x, want 0x15", cpu.A)
		}
	})

	t.Run("CP (IX+d) leaves A unchanged", func(t *testing.T) {
		bytes := []byte{0xDD, 0xBE, 0x01} // CP (IX+1)
		in, err := Decode(bytes, 0, 0x8000)
		if err != nil {
			t.Fatalf("zendis decode: %v", err)
		}
		if in.Text() != "CP (IX+0x01)" {
			t.Fatalf("zendis decoded %q, want \"CP (IX+0x01)\"", in.Text())
		}
		cpu, mem := newPlainOracle(bytes, 0x8000)
		cpu.SetIX(0x9000)
		cpu.A = 0x50
		mem.Write(0x9001, 0x50) // equal -- Z flag should set, A untouched
		cpu.Step()
		if cpu.A != 0x50 {
			t.Errorf("zen80 A after CP (IX+1) = %#02x, want unchanged 0x50", cpu.A)
		}
		if cpu.F&z80.FlagZ == 0 {
			t.Errorf("zen80 Z flag not set after CP with equal operands (F=%#02x)", cpu.F)
		}
	})
}
