package disasm

// T-4 oracle: cross-checks zendis's DDCB/FDCB undocumented
// copy-to-register decoding against zen80's real execution
// (executeDDFDCBOperation): for the rotate/shift, RES, and SET groups
// (x==0, x==2, x==3), z!=6 also copies the memory result into r[z] --
// not just BIT (x==1), which has no result to copy at all. One
// representative case per group, plus one under FD/IY; the addressing
// mechanism itself is already oracle-verified in
// decode_ddfd_oracle_test.go, so this targets the copy-to-register
// behaviour specifically.

import "testing"

func TestDDFD_Oracle_DDCB_CopyToRegister(t *testing.T) {
	t.Run("RLC (IX+3),B", func(t *testing.T) {
		bytes := []byte{0xDD, 0xCB, 0x03, 0x00} // x=0(RLC) y=0 z=0(B)
		in, err := Decode(bytes, 0, 0x8000)
		if err != nil {
			t.Fatalf("zendis decode: %v", err)
		}
		if in.Text() != "RLC (IX+0x03),B" || in.Length != 4 {
			t.Fatalf("zendis decoded %q len %d, want \"RLC (IX+0x03),B\" len 4", in.Text(), in.Length)
		}
		cpu, mem := newPlainOracle(bytes, 0x8000)
		cpu.SetIX(0x9000)
		mem.Write(0x9003, 0x81) // 1000_0001 -> RLC -> 0000_0011
		cpu.Step()
		if got := mem.Read(0x9003); got != 0x03 {
			t.Errorf("zen80 memory[IX+3] = %#02x, want 0x03", got)
		}
		if cpu.B != 0x03 {
			t.Errorf("zen80 B (undocumented copy target) = %#02x, want 0x03 -- same value RLC wrote to memory", cpu.B)
		}
	})

	t.Run("RES 0,(IX+3),C", func(t *testing.T) {
		bytes := []byte{0xDD, 0xCB, 0x03, 0x81} // x=2(RES) y=0 z=1(C)
		in, err := Decode(bytes, 0, 0x8000)
		if err != nil {
			t.Fatalf("zendis decode: %v", err)
		}
		if in.Text() != "RES 0,(IX+0x03),C" || in.Length != 4 {
			t.Fatalf("zendis decoded %q len %d, want \"RES 0,(IX+0x03),C\" len 4", in.Text(), in.Length)
		}
		cpu, mem := newPlainOracle(bytes, 0x8000)
		cpu.SetIX(0x9000)
		mem.Write(0x9003, 0xFF)
		cpu.Step()
		if got := mem.Read(0x9003); got != 0xFE {
			t.Errorf("zen80 memory[IX+3] = %#02x, want 0xFE (bit 0 cleared)", got)
		}
		if cpu.C != 0xFE {
			t.Errorf("zen80 C (undocumented copy target) = %#02x, want 0xFE", cpu.C)
		}
	})

	t.Run("SET 7,(IX+3),D", func(t *testing.T) {
		bytes := []byte{0xDD, 0xCB, 0x03, 0xFA} // x=3(SET) y=7 z=2(D)
		in, err := Decode(bytes, 0, 0x8000)
		if err != nil {
			t.Fatalf("zendis decode: %v", err)
		}
		if in.Text() != "SET 7,(IX+0x03),D" || in.Length != 4 {
			t.Fatalf("zendis decoded %q len %d, want \"SET 7,(IX+0x03),D\" len 4", in.Text(), in.Length)
		}
		cpu, mem := newPlainOracle(bytes, 0x8000)
		cpu.SetIX(0x9000)
		mem.Write(0x9003, 0x00)
		cpu.Step()
		if got := mem.Read(0x9003); got != 0x80 {
			t.Errorf("zen80 memory[IX+3] = %#02x, want 0x80 (bit 7 set)", got)
		}
		if cpu.D != 0x80 {
			t.Errorf("zen80 D (undocumented copy target) = %#02x, want 0x80", cpu.D)
		}
	})

	t.Run("BIT has no copy target", func(t *testing.T) {
		// x==1 (BIT) has no memory write and no copy at all -- z!=6 here
		// is an undocumented duplicate opcode with no observable
		// behavioural difference (per decode_ddfd.go's own comment).
		// Confirms zendis doesn't emit a spurious copy-target operand,
		// and zen80 doesn't touch any register for it.
		bytes := []byte{0xDD, 0xCB, 0x03, 0x41} // x=1(BIT) y=0 z=1(C)
		in, err := Decode(bytes, 0, 0x8000)
		if err != nil {
			t.Fatalf("zendis decode: %v", err)
		}
		if in.Text() != "BIT 0,(IX+0x03)" {
			t.Fatalf("zendis decoded %q, want \"BIT 0,(IX+0x03)\" (no copy-target operand)", in.Text())
		}
		cpu, mem := newPlainOracle(bytes, 0x8000)
		cpu.SetIX(0x9000)
		cpu.C = 0xAA
		mem.Write(0x9003, 0x01)
		cpu.Step()
		if cpu.C != 0xAA {
			t.Errorf("zen80 C changed to %#02x under BIT -- should be untouched", cpu.C)
		}
		if got := mem.Read(0x9003); got != 0x01 {
			t.Errorf("zen80 memory[IX+3] changed to %#02x under BIT -- should be untouched (flags-only)", got)
		}
	})

	t.Run("FD/IY same mechanism", func(t *testing.T) {
		bytes := []byte{0xFD, 0xCB, 0x02, 0x04} // x=0(RLC) y=0 z=4(H) under IY
		in, err := Decode(bytes, 0, 0x8000)
		if err != nil {
			t.Fatalf("zendis decode: %v", err)
		}
		if in.Text() != "RLC (IY+0x02),H" || in.Length != 4 {
			t.Fatalf("zendis decoded %q len %d, want \"RLC (IY+0x02),H\" len 4", in.Text(), in.Length)
		}
		cpu, mem := newPlainOracle(bytes, 0x8000)
		cpu.SetIY(0x9000)
		mem.Write(0x9002, 0x40) // 0100_0000 -> RLC -> 1000_0000
		cpu.Step()
		if got := mem.Read(0x9002); got != 0x80 {
			t.Errorf("zen80 memory[IY+2] = %#02x, want 0x80", got)
		}
		if cpu.H != 0x80 {
			t.Errorf("zen80 H (undocumented copy target) = %#02x, want 0x80", cpu.H)
		}
	})
}
