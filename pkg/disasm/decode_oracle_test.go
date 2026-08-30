package disasm

// Oracle harness: cross-checks zendis's Z80N decode claims against
// zen80 (github.com/ha1tch/zen80), a real Z80N execution core whose
// own test suite exceeds ZEXALL/ZEXDOC and whose Z80N implementation
// is independently cross-checked against ZEsarUX/CSpect (see
// zen80/Z80N_ZESARUX_CROSSCHECK.md). This is a test-only dependency —
// nothing in cmd/ or the non-test parts of pkg/disasm imports zen80.
//
// The check: for each Z80N opcode, zendis's decoded Length should
// equal the number of bytes zen80's own Step() actually consumes when
// executing the identical bytes on a scratch, disposable CPU. This
// exhaustively catches operand-byte-count mistakes (the kind T-1's
// table is most at risk of) against an independently-implemented
// oracle, not just zendis's own hand-written expectations.
//
// Scope, deliberately: this sweep proves decode Length is right. It
// does not attempt to replicate zen80's own execution-correctness
// testing (flag behaviour, exact register semantics) — that's what
// zen80's own extensive test suite is for. A few targeted semantic
// checks are added below only where they're nearly free and where a
// decode mistake is otherwise easy to miss: PUSH's big-endian operand
// (the single such case in the ISA) and NEXTREG's port-write pair.
//
// Two opcodes need special handling, not the plain length-delta check:
//   - JP (C) (0x98) computes a new PC from an I/O read; it doesn't
//     "advance past itself" the way every other instruction does, so
//     there's no length-delta invariant to check. Its Length (2, no
//     operand bytes at all) is verified directly instead.
//   - LDIRX/LDPIRX/LDDRX (0xB4/0xB7/0xBC) rewind PC by 2 and repeat
//     when BC != 0 after decrementing, exactly like classic LDIR. Seeding
//     BC=1 before stepping takes the non-repeating path, matching a
//     disassembler's model of one static instruction at a fixed length.

import (
	"testing"

	zenio "github.com/ha1tch/zen80/io"
	"github.com/ha1tch/zen80/memory"
	"github.com/ha1tch/zen80/z80"
)

// oracleResult is what a single oracle Step() exposes back to a test.
type oracleResult struct {
	cpu    *z80.Z80
	mem    *memory.RAM
	pcBase uint16
	writes []struct {
		port  uint16
		value uint8
	}
}

// newOracle builds a scratch, disposable zen80 CPU with Z80N enabled,
// RAM loaded with code at addr, PC set to addr, and an I/O implementation
// that records every port write (needed for NEXTREG's port-write proof;
// harmless no-op bookkeeping for everything else).
func newOracle(code []byte, addr uint16) *oracleResult {
	mem := memory.NewRAM()
	mem.Load(addr, code)

	res := &oracleResult{mem: mem, pcBase: addr}
	mio := zenio.NewMappedIO()
	// Per-port handlers, not RegisterWriteRange(0x0000, 0xFFFF, ...):
	// that method loops "for port := start; port <= end; port++" with
	// port as uint16, which never terminates when end is 0xFFFF (port++
	// wraps 0xFFFF to 0, and 0 <= 0xFFFF stays true). Confirmed by
	// hanging this harness outright. A real bug in zen80's io package,
	// separate from T-1 -- flagged, not patched here. The only ports
	// this harness needs recorded are NEXTREG's two, so registering
	// them individually sidesteps the bug entirely.
	recordWrite := func(port uint16, value uint8) {
		res.writes = append(res.writes, struct {
			port  uint16
			value uint8
		}{port, value})
	}
	mio.RegisterWriteHandler(0x243B, recordWrite) // NEXTREG register-select port
	mio.RegisterWriteHandler(0x253B, recordWrite) // NEXTREG data port

	cpu := z80.New(mem, mio)
	cpu.Z80N = true
	cpu.PC = addr
	res.cpu = cpu
	return res
}

// TestZ80N_OracleLengthAgreement sweeps every Z80N opcode except JP (C)
// (handled separately below) and cross-checks zendis's decoded Length
// against zen80's own PC advance for the identical bytes.
func TestZ80N_OracleLengthAgreement(t *testing.T) {
	for _, tc := range z80nCases {
		if tc.text == "JP (C)" {
			continue // no length-delta invariant holds for this one; see below
		}
		t.Run(tc.text, func(t *testing.T) {
			in, err := DecodeTarget(tc.bytes, 0, 0x8000, TargetZ80N)
			if err != nil {
				t.Fatalf("zendis decode: %v", err)
			}

			const addr = 0x8000
			o := newOracle(tc.bytes, addr)
			switch tc.text {
			case "LDIRX", "LDPIRX", "LDDRX":
				// Seed BC=1 so the repeat condition (BC != 0 after the
				// decrement inside the op) is false: one pass, no rewind.
				o.cpu.SetBC(1)
			}
			o.cpu.Step()

			gotDelta := int(o.cpu.PC - addr)
			if gotDelta != in.Length {
				t.Errorf("zen80 advanced PC by %d, zendis decoded Length %d (bytes % X)",
					gotDelta, in.Length, tc.bytes)
			}
		})
	}
}

// TestZ80N_OracleJPC_LengthOnly covers the one opcode excluded from the
// sweep above: JP (C) takes no operand bytes at all (its target comes
// from an I/O read at execution time, not from the instruction stream),
// so there's no length-delta invariant to check against zen80 — only
// that zendis agrees it's a bare 2-byte instruction.
func TestZ80N_OracleJPC_LengthOnly(t *testing.T) {
	in, err := DecodeTarget([]byte{0xED, 0x98}, 0, 0x8000, TargetZ80N)
	if err != nil {
		t.Fatalf("zendis decode: %v", err)
	}
	if in.Length != 2 {
		t.Errorf("JP (C) Length = %d, want 2 (ED 98, no operand bytes)", in.Length)
	}
}

// TestZ80N_OraclePushBigEndian is the highest-risk case in the table:
// PUSH's operand is the only big-endian one in the Z80/Z80N instruction
// set. zendis decodes "ED 8A 12 34" as PUSH 0x1234; this confirms
// zen80's own push (SP--, write high, SP--, write low) agrees that the
// pushed value is 0x1234, not 0x3412.
func TestZ80N_OraclePushBigEndian(t *testing.T) {
	bytes := []byte{0xED, 0x8A, 0x12, 0x34}
	in, err := DecodeTarget(bytes, 0, 0x8000, TargetZ80N)
	if err != nil {
		t.Fatalf("zendis decode: %v", err)
	}
	if in.Text() != "PUSH 0x1234" {
		t.Fatalf("zendis decoded %q, want \"PUSH 0x1234\"", in.Text())
	}

	const addr = 0x8000
	o := newOracle(bytes, addr)
	beforeSP := o.cpu.SP
	o.cpu.Step()

	if o.cpu.SP != beforeSP-2 {
		t.Fatalf("SP after PUSH = %#04x, want %#04x", o.cpu.SP, beforeSP-2)
	}
	lo := o.mem.Read(o.cpu.SP)
	hi := o.mem.Read(o.cpu.SP + 1)
	pushed := uint16(hi)<<8 | uint16(lo)
	if pushed != 0x1234 {
		t.Errorf("zen80 pushed %#04x, want 0x1234 (a byte-order mistake would show 0x3412)", pushed)
	}
}

// TestZ80N_OracleNextreg confirms both NEXTREG forms' decoded operand
// bytes match the register-select/data port writes zen80 actually
// issues (0x243B/0x253B), proving the decoded reg/value pair -- not
// just the instruction length -- is right.
func TestZ80N_OracleNextreg(t *testing.T) {
	const selectPort, dataPort = 0x243B, 0x253B

	t.Run("NEXTREG n,n", func(t *testing.T) {
		bytes := []byte{0xED, 0x91, 0x10, 0x05}
		in, err := DecodeTarget(bytes, 0, 0x8000, TargetZ80N)
		if err != nil {
			t.Fatalf("zendis decode: %v", err)
		}
		if in.Text() != "NEXTREG 0x10,0x05" {
			t.Fatalf("zendis decoded %q, want \"NEXTREG 0x10,0x05\"", in.Text())
		}
		o := newOracle(bytes, 0x8000)
		o.cpu.Step()
		if len(o.writes) != 2 || o.writes[0].port != selectPort || o.writes[0].value != 0x10 ||
			o.writes[1].port != dataPort || o.writes[1].value != 0x05 {
			t.Errorf("zen80 port writes = %+v, want [{%#04x 0x10} {%#04x 0x05}]",
				o.writes, selectPort, dataPort)
		}
	})

	t.Run("NEXTREG n,A", func(t *testing.T) {
		bytes := []byte{0xED, 0x92, 0x10}
		in, err := DecodeTarget(bytes, 0, 0x8000, TargetZ80N)
		if err != nil {
			t.Fatalf("zendis decode: %v", err)
		}
		if in.Text() != "NEXTREG 0x10,A" {
			t.Fatalf("zendis decoded %q, want \"NEXTREG 0x10,A\"", in.Text())
		}
		o := newOracle(bytes, 0x8000)
		o.cpu.A = 0x77
		o.cpu.Step()
		if len(o.writes) != 2 || o.writes[0].port != selectPort || o.writes[0].value != 0x10 ||
			o.writes[1].port != dataPort || o.writes[1].value != 0x77 {
			t.Errorf("zen80 port writes = %+v, want [{%#04x 0x10} {%#04x 0x77}]",
				o.writes, selectPort, dataPort)
		}
	})
}

// TestZ80N_OracleAddWordImmediate confirms the little-endian word decode
// for the three ADD rr,nn forms by zeroing the target pair and checking
// it equals the decoded immediate after ADD-from-zero.
func TestZ80N_OracleAddWordImmediate(t *testing.T) {
	cases := []struct {
		bytes []byte
		text  string
		zero  func(cpu *z80.Z80)
		get   func(cpu *z80.Z80) uint16
	}{
		{[]byte{0xED, 0x34, 0x34, 0x12}, "ADD HL,0x1234",
			func(c *z80.Z80) { c.SetHL(0) }, func(c *z80.Z80) uint16 { return c.HL() }},
		{[]byte{0xED, 0x35, 0x34, 0x12}, "ADD DE,0x1234",
			func(c *z80.Z80) { c.SetDE(0) }, func(c *z80.Z80) uint16 { return c.DE() }},
		{[]byte{0xED, 0x36, 0x34, 0x12}, "ADD BC,0x1234",
			func(c *z80.Z80) { c.SetBC(0) }, func(c *z80.Z80) uint16 { return c.BC() }},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			in, err := DecodeTarget(tc.bytes, 0, 0x8000, TargetZ80N)
			if err != nil {
				t.Fatalf("zendis decode: %v", err)
			}
			if in.Text() != tc.text {
				t.Fatalf("zendis decoded %q, want %q", in.Text(), tc.text)
			}
			o := newOracle(tc.bytes, 0x8000)
			tc.zero(o.cpu)
			o.cpu.Step()
			if got := tc.get(o.cpu); got != 0x1234 {
				t.Errorf("zen80 result = %#04x, want 0x1234", got)
			}
		})
	}
}
