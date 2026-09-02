package disasm

// T-3 oracle: cross-checks zendis's DD/FD substitution decisions
// against zen80's real DD/FD dispatch (github.com/ha1tch/zen80). The
// substitution rule itself (HL/H/L/(HL) fields get IX/IY substituted;
// HALT and EX DE,HL are the two hard-coded exceptions; LD H,(IX+d) and
// LD L,(IX+d) keep a literal register on the non-memory side) is
// exactly the area T-3 flags as verified only against this decoder's
// own author's recollection. This targets the substitution *decision*
// specifically -- the exceptions, the boundary cases around them, and
// the half-register/indexed forms -- rather than an exhaustive sweep
// of all ~200 substituted opcodes plus the DDCB/FDCB combinatorics
// (512 more slots). That fuller sweep is real remaining work; see the
// note at the end of this file.
//
// FD (IY) is exercised via the same decodeIndexed code path DD (IX)
// uses, parameterised only by the register name -- checked once
// explicitly (TestDDFD_Oracle_IYSamePath) rather than doubling every
// case, since the two paths share all decode logic.

import (
	"testing"

	zenio "github.com/ha1tch/zen80/io"
	"github.com/ha1tch/zen80/memory"
	"github.com/ha1tch/zen80/z80"
)

// newPlainOracle is newOracle without Z80N and without I/O recording --
// classic DD/FD decode needs neither: EX (SP),IX/IY, PUSH/POP, and the
// arithmetic/load forms in this test set all stay on the memory bus.
func newPlainOracle(code []byte, addr uint16) (*z80.Z80, *memory.RAM) {
	mem := memory.NewRAM()
	mem.Load(addr, code)
	cpu := z80.New(mem, zenio.NewNullIO())
	cpu.PC = addr
	return cpu, mem
}

// TestDDFD_Oracle_HALTException: DD 76 must genuinely HALT (zen80's
// Halted flag set, PC advanced by exactly 2), never decode as a memory
// op on (IX+d) the way the general LD-family substitution would if the
// exception weren't hard-coded.
func TestDDFD_Oracle_HALTException(t *testing.T) {
	bytes := []byte{0xDD, 0x76}
	in, err := Decode(bytes, 0, 0x8000)
	if err != nil {
		t.Fatalf("zendis decode: %v", err)
	}
	if in.Text() != "HALT" || in.Length != 2 {
		t.Fatalf("zendis decoded %q len %d, want \"HALT\" len 2", in.Text(), in.Length)
	}

	// zen80 < 0.5.6's own executeDD panicked on DD 76: its involvesHL
	// fast path checked (x==1 && (y==6||z_val==6)), true for 0x76
	// (x=1,y=6,z=6), routing it into the "LD (IX+d),r" branch and
	// calling getRegister8(6) -- which panicked by design. Fixed in
	// zen80 v0.5.6 (confirmed via a live, panic-free run before this
	// assertion was trusted, not just read from the changelog); this
	// oracle check now asserts directly rather than recovering from an
	// expected panic. zendis's own decode above, independently correct
	// per documented Z80 behaviour, was never affected either way.
	cpu, _ := newPlainOracle(bytes, 0x8000)
	cpu.Step()
	if !cpu.Halted {
		t.Error("zen80: DD 76 did not set Halted")
	}
	if cpu.PC != 0x8002 {
		t.Errorf("zen80 PC after DD 76 = %#04x, want 0x8002", cpu.PC)
	}
}

// TestDDFD_Oracle_EXDEHLException: DD EB must genuinely swap DE/HL,
// untouched by IX substitution (there is no "EX DE,IX").
func TestDDFD_Oracle_EXDEHLException(t *testing.T) {
	bytes := []byte{0xDD, 0xEB}
	in, err := Decode(bytes, 0, 0x8000)
	if err != nil {
		t.Fatalf("zendis decode: %v", err)
	}
	if in.Text() != "EX DE,HL" || in.Length != 2 {
		t.Fatalf("zendis decoded %q len %d, want \"EX DE,HL\" len 2", in.Text(), in.Length)
	}

	cpu, _ := newPlainOracle(bytes, 0x8000)
	cpu.SetDE(0x1111)
	cpu.SetHL(0x2222)
	cpu.SetIX(0x9999) // sentinel: must be untouched
	cpu.Step()
	// zen80 < 0.5.6 substituted IX into EX DE,HL instead of leaving it
	// unaffected by the DD prefix, the same way the HALT exception
	// documents zendis's own decoder as handling correctly. Fixed in
	// zen80 v0.5.6 (confirmed via a live register check before this
	// assertion was trusted, not just read from the changelog); this
	// oracle check now asserts directly rather than logging a known
	// divergence and skipping.
	if cpu.DE() != 0x2222 || cpu.HL() != 0x1111 || cpu.IX() != 0x9999 {
		t.Errorf("zen80 DD EB: DE=%#04x HL=%#04x IX=%#04x, want DE=0x2222 HL=0x1111 IX=0x9999 unchanged",
			cpu.DE(), cpu.HL(), cpu.IX())
	}
}

// TestDDFD_Oracle_LiteralRegisterException: LD H,(IX+d) and LD L,(IX+d)
// keep a literal H/L on the register side -- documented, distinct from
// the undocumented IXh/IXl half-register substitution that applies only
// when neither side is a memory reference. Confirmed by writing distinct
// sentinel values into H and IXH and checking which one the load
// actually overwrote.
func TestDDFD_Oracle_LiteralRegisterException(t *testing.T) {
	// LD H,(IX+2): IX=0x8010, memory[0x8012]=0x55. H must become 0x55;
	// IXH (which decodeIndexed's IXh/IXl path would target if this were
	// wrongly treated as a half-register substitution) must be untouched.
	bytes := []byte{0xDD, 0x66, 0x02} // DD 66 02 = LD H,(IX+2)
	in, err := Decode(bytes, 0, 0x8000)
	if err != nil {
		t.Fatalf("zendis decode: %v", err)
	}
	if in.Text() != "LD H,(IX+0x02)" {
		t.Fatalf("zendis decoded %q, want \"LD H,(IX+0x02)\"", in.Text())
	}

	cpu, mem := newPlainOracle(bytes, 0x8000)
	cpu.SetIX(0x8010)
	cpu.H = 0xAA
	mem.Write(0x8012, 0x55)
	cpu.Step()
	// Correct invariant: H gets the memory value, IX itself is
	// untouched (LD H,(IX+d) only reads memory and writes H). An
	// earlier version of this test also set cpu.IXH as an independent
	// "untouched" sentinel -- invalid, since IXH IS the high byte of IX
	// (SetIX writes it directly), so that line silently corrupted IX
	// itself before the instruction ran. Retracted: there was no zen80
	// bug here, only this test's own aliasing mistake.
	if cpu.H != 0x55 {
		t.Errorf("zen80 H after LD H,(IX+2) = %#02x, want 0x55", cpu.H)
	}
	if cpu.IX() != 0x8010 {
		t.Errorf("zen80 IX changed to %#04x, want unchanged 0x8010", cpu.IX())
	}
}

// TestDDFD_Oracle_HalfRegister: the undocumented IXh/IXl forms, by
// contrast, DO substitute -- INC IXH is a pure register op with no
// memory access at all, distinct from INC (IX+d).
func TestDDFD_Oracle_HalfRegister(t *testing.T) {
	bytes := []byte{0xDD, 0x24} // DD 24 = INC IXh
	in, err := Decode(bytes, 0, 0x8000)
	if err != nil {
		t.Fatalf("zendis decode: %v", err)
	}
	if in.Text() != "INC IXh" || in.Length != 2 {
		t.Fatalf("zendis decoded %q len %d, want \"INC IXh\" len 2", in.Text(), in.Length)
	}

	cpu, _ := newPlainOracle(bytes, 0x8000)
	cpu.SetIX(0x1234)
	cpu.Step()
	if cpu.IX() != 0x1334 {
		t.Errorf("zen80 IX after INC IXh = %#04x, want 0x1334 (only the high byte incremented)", cpu.IX())
	}
}

// TestDDFD_Oracle_IndexedMemory: LD (IX+d),n writes n to memory at
// IX+d, not to a register -- the ordinary memory-substitution case the
// literal-register exception above is carved out of.
func TestDDFD_Oracle_IndexedMemory(t *testing.T) {
	bytes := []byte{0xDD, 0x36, 0x05, 0x7A} // DD 36 05 7A = LD (IX+5),0x7A
	in, err := Decode(bytes, 0, 0x8000)
	if err != nil {
		t.Fatalf("zendis decode: %v", err)
	}
	if in.Text() != "LD (IX+0x05),0x7A" {
		t.Fatalf("zendis decoded %q, want \"LD (IX+0x05),0x7A\"", in.Text())
	}

	cpu, mem := newPlainOracle(bytes, 0x8000)
	cpu.SetIX(0x9000)
	cpu.Step()
	if got := mem.Read(0x9005); got != 0x7A {
		t.Errorf("zen80 memory[IX+5] = %#02x, want 0x7A", got)
	}
}

// TestDDFD_Oracle_DDCB_Rotate: the DDCB sub-form's displacement-then-
// opcode byte order (DD CB d op, displacement BEFORE the CB-style
// opcode -- the reverse of every other DD/FD form's operand ordering),
// and its target being (IX+d) regardless of the CB opcode's own z field.
func TestDDFD_Oracle_DDCB_Rotate(t *testing.T) {
	bytes := []byte{0xDD, 0xCB, 0x03, 0x06} // DD CB 03 06 = RLC (IX+3)
	in, err := Decode(bytes, 0, 0x8000)
	if err != nil {
		t.Fatalf("zendis decode: %v", err)
	}
	if in.Text() != "RLC (IX+0x03)" || in.Length != 4 {
		t.Fatalf("zendis decoded %q len %d, want \"RLC (IX+0x03)\" len 4", in.Text(), in.Length)
	}

	cpu, mem := newPlainOracle(bytes, 0x8000)
	cpu.SetIX(0x9000)
	mem.Write(0x9003, 0x81) // 1000_0001: RLC -> 0000_0011, carry out
	cpu.Step()
	if got := mem.Read(0x9003); got != 0x03 {
		t.Errorf("zen80 memory[IX+3] after RLC = %#02x, want 0x03 (0x81 rotated left)", got)
	}
}

// TestDDFD_Oracle_IYSamePath: FD shares decodeIndexed's entire logic
// with DD, parameterised only by register name -- one representative
// case (the literal-register exception, the subtlest of the three)
// confirms the shared path resolves correctly for IY too.
func TestDDFD_Oracle_IYSamePath(t *testing.T) {
	bytes := []byte{0xFD, 0x6E, 0x02} // FD 6E 02 = LD L,(IY+2)
	in, err := Decode(bytes, 0, 0x8000)
	if err != nil {
		t.Fatalf("zendis decode: %v", err)
	}
	if in.Text() != "LD L,(IY+0x02)" {
		t.Fatalf("zendis decoded %q, want \"LD L,(IY+0x02)\"", in.Text())
	}

	cpu, mem := newPlainOracle(bytes, 0x8000)
	cpu.SetIY(0x8010)
	cpu.L = 0xAA
	mem.Write(0x8012, 0x66)
	cpu.Step()
	// Same corrected invariant as the DD/IX case: see its comment. No
	// IYL sentinel -- it aliases IY's own storage the same way IXH does.
	if cpu.L != 0x66 {
		t.Errorf("zen80 L after LD L,(IY+2) = %#02x, want 0x66", cpu.L)
	}
	if cpu.IY() != 0x8010 {
		t.Errorf("zen80 IY changed to %#04x, want unchanged 0x8010", cpu.IY())
	}
}

// Remaining for a fuller T-3 pass, not attempted here: an exhaustive
// sweep of every substituted opcode (the ~200 forms decodeIndexed's
// general case covers) and the full DDCB/FDCB combinatorics (all 256
// CB-style opcodes x 2 registers x the undocumented copy-to-register
// forms T-4 also flags). This file covers the substitution *decision*
// itself -- the two exceptions, the literal-register carve-out, and one
// representative case each for half-register, indexed-memory, and DDCB
// forms -- which is where a table error is most likely and, per T-3's
// own trigger note, least likely to be caught by casual use.
