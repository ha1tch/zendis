// Command roundtrip verifies zendis's decoder against an independent
// reference the way T-3 originally envisioned: assemble Z80/Z80N source
// with zenas, disassemble the resulting binary with zendis, reassemble
// zendis's own output with zenas again, and confirm the two binaries are
// byte-identical.
//
// This is a different verification axis from decode_oracle_test.go and
// decode_ddfd_oracle_test.go (which check zendis's decode claims against
// zen80's execution). Those never touch zenas's parser at all; this tool
// specifically proves zendis's output text is valid, round-trippable
// zenas input -- catching a class of bug the execution oracle can't (a
// decode that is semantically right but syntactically malformed or
// differently-valued from what it should re-encode to).
//
// Deliberately a separate module (own go.mod) with its own zenas/zentools
// dependency tree, not part of `go test ./...` from the zendis module
// root. Run explicitly:
//
//	cd tools/roundtrip && go run .
//
// Known, confirmed gaps in zenas's current encoder (not zendis bugs --
// verified directly against assembler/encoder.go's instruction table)
// mean two instruction families can't be round-tripped through zenas at
// all right now, and are excluded from the corpus rather than failed:
//   - DD/FD-prefixed CB (rotate/BIT/RES/SET on (IX+d)/(IY+d), including
//     the undocumented copy-to-register forms T-4 covers): zenas only
//     registers these mnemonics for a plain register or (HL) operand,
//     never an indexed one.
//   - EX (SP),IX / EX (SP),IY: zenas's EX encoder is hardcoded to
//     (SP),HL only.
//
// If zenas ever adds these, the excluded cases below are exactly what to
// re-enable.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ha1tch/zenas/assembler"
	"github.com/ha1tch/zendis/pkg/disasm"
)

type testCase struct {
	name   string
	source string // zenas source producing the starting binary
	target disasm.Target
	z80n   bool // EnableZ80N() on the zenas assembler
}

var cases = []testCase{
	{
		name:   "unprefixed mix (LD/ALU/branch/stack)",
		target: disasm.TargetZ80,
		source: `ORG 0x8000
    LD A,0x2A
    LD B,0x05
LOOP:
    ADD A,B
    DEC B
    JR NZ,LOOP
    LD HL,0x9000
    LD (HL),A
    LD DE,0x1234
    LD BC,0x5678
    PUSH DE
    POP BC
    CALL SUB
    JP DONE
SUB:
    XOR A
    RET
DONE:
    HALT
`,
	},
	{
		name:   "CB rotates and bit ops",
		target: disasm.TargetZ80,
		source: `ORG 0x8000
    RLC B
    RRC C
    RL D
    RR E
    SLA H
    SRA L
    SRL A
    BIT 7,A
    RES 0,B
    SET 3,C
    RLC (HL)
    BIT 4,(HL)
`,
	},
	{
		name:   "ED classic (block ops, 16-bit ADC/SBC, IN/OUT)",
		target: disasm.TargetZ80,
		source: `ORG 0x8000
    LD HL,0x9000
    LD DE,0xA000
    LD BC,0x0010
    LDIR
    LDDR
    CPIR
    ADC HL,DE
    SBC HL,BC
    IN A,(C)
    OUT (C),A
    NEG
    IM 1
    RETI
`,
	},
	{
		name:   "Z80N extensions",
		target: disasm.TargetZ80N,
		z80n:   true,
		source: `ORG 0x8000
    SWAPNIB
    MIRROR A
    TEST 0x2A
    BSLA DE,B
    BSRA DE,B
    BSRL DE,B
    BSRF DE,B
    BRLC DE,B
    MUL D,E
    ADD HL,A
    ADD DE,A
    ADD BC,A
    ADD HL,0x1234
    PUSH 0x1234
    OUTINB
    NEXTREG 0x10,0x05
    NEXTREG 0x10,A
    PIXELDN
    PIXELAD
    SETAE
    JP (C)
    LDIX
    LDWS
    LDDX
    LDIRX
    LDPIRX
    LDDRX
`,
	},
	{
		name:   "DD/FD substitution: exceptions, half-registers, indexed memory, 16-bit forms",
		target: disasm.TargetZ80,
		source: `ORG 0x8000
    HALT
    EX DE,HL
    LD H,(IX+0x02)
    LD L,(IY+0x02)
    INC IXh
    DEC IXl
    LD IXh,0x05
    LD (IX+0x05),0x7A
    LD IX,0x1234
    LD IY,0x5678
    ADD IX,DE
    ADD IY,BC
    PUSH IX
    POP IX
    PUSH IY
    POP IY
    JP (IX)
    JP (IY)
    LD SP,IX
    ADD A,(IX+0x04)
    CP (IX+0x01)
    SUB (IY+0x02)
    AND (IX+0x03)
`,
	},
	{
		name:   "IXh/IXl and IYh/IYl half-register matrix (undocumented but real)",
		target: disasm.TargetZ80,
		source: `ORG 0x8000
    ADD A,IXh
    ADC A,IXh
    SUB IXh
    SBC A,IXh
    AND IXh
    XOR IXh
    OR IXh
    CP IXh
    ADD A,IXl
    SUB IXl
    CP IXl
    LD B,IXh
    LD C,IXl
    LD D,IXh
    LD E,IXl
    LD A,IXh
    LD IXh,B
    LD IXl,C
    LD IXh,A
    LD IXh,IXl
    LD IXl,IXh
    ADD A,IYh
    LD IYh,IYl
    LD D,IYh
`,
	},
	{
		name:   "rare-but-documented ED forms",
		target: disasm.TargetZ80,
		source: `ORG 0x8000
    RLD
    RRD
    LD I,A
    LD A,I
    LD R,A
    LD A,R
`,
	},
}

// Also confirmed rejected by zenas's current encoder (same
// assembler/encoder.go check as the DD/FD-CB and EX (SP),IX gaps in
// this file's header comment), so excluded from the corpus above:
//   - SLL r / SLL (HL): the undocumented "shift left, set bit 0" CB
//     rotate (zendis's rotNames[6]). zenas doesn't register the SLL
//     mnemonic at all: "unknown instruction: SLL".
//   - IN (C): the undocumented flags-only-read form (no destination
//     register). zenas: "no matching encoding for IN with operands:
//     [indirect:C]".
//   - OUT (C),0x00: the undocumented no-source-register form (writes a
//     literal 0). zenas: "no matching encoding for OUT with operands:
//     [indirect:C, imm16:0]".

// lossyCase is a known-undocumented-duplicate opcode encoding that
// cannot round-trip through mnemonic text at all -- not because zendis
// decodes it wrong, but because the encoding itself is a redundant
// alias with no distinct standard syntax. Starting bytes are given
// directly (not produced by an initial zenas assembly step) since
// zenas's source syntax has no way to independently express "the
// undocumented duplicate form" in the first place.
type lossyCase struct {
	name   string
	bytes  []byte
	org    uint16
	target disasm.Target
	note   string // why the mismatch is expected, not a bug
}

var lossyCases = []lossyCase{
	{
		name:   "undocumented ED NOP duplicate (ED 77)",
		bytes:  []byte{0xED, 0x77},
		org:    0x8000,
		target: disasm.TargetZ80,
		note: "zendis renders this as plain \"NOP\" -- text indistinguishable " +
			"from the real 1-byte 0x00 NOP. zenas reassembles \"NOP\" as that " +
			"canonical 1-byte form, so the round-trip is expected to shrink " +
			"2 bytes to 1.",
	},
	{
		name:   "undocumented IM 0/1 duplicate (ED 4E)",
		bytes:  []byte{0xED, 0x4E},
		org:    0x8000,
		target: disasm.TargetZ80,
		note: "zendis renders this as \"IM 0/1\", a human-readable ambiguity " +
			"note, not real syntax. zenas's expression parser accepts it " +
			"anyway, evaluating \"0/1\" as arithmetic division (=0) and " +
			"silently producing the CANONICAL IM 0 encoding (ED 46) -- a " +
			"byte mismatch zenas never flags as an error. This is exactly " +
			"the kind of silent-wrong-bytes case comparing output bytes " +
			"catches that checking \"did it assemble\" alone would miss.",
	},
}

func main() {
	failures := 0
	for _, tc := range cases {
		ok, detail := roundTrip(tc)
		if ok {
			fmt.Printf("PASS  %s\n", tc.name)
			continue
		}
		failures++
		fmt.Printf("FAIL  %s\n%s\n", tc.name, detail)
	}

	if ok, detail := checkCLIPlainFormat(); ok {
		fmt.Println("PASS  CLI --hints + --format=plain (real binary, not the library)")
	} else {
		failures++
		fmt.Printf("FAIL  CLI --hints + --format=plain\n%s\n", detail)
	}

	fmt.Println()
	total := len(cases) + 1
	if failures > 0 {
		fmt.Printf("%d/%d round-trips failed\n", failures, total)
		os.Exit(1)
	}
	fmt.Printf("all %d round-trips passed\n\n", total)

	fmt.Println("Known-lossy encodings (expected mismatch, not a failure):")
	for _, lc := range lossyCases {
		matched, binB, detail := checkLossy(lc)
		status := "MISMATCH (expected)"
		if matched {
			status = "MATCHED (unexpectedly -- worth a second look)"
		}
		fmt.Printf("  %-45s %s\n    original:  % X\n    round-trip: % X\n    %s\n%s\n",
			lc.name, status, lc.bytes, binB, lc.note, detail)
	}
}

// checkLossy decodes lc.bytes with zendis, reassembles the resulting
// text with zenas, and reports whether the result matches the
// original -- without treating a mismatch as failure the way
// roundTrip does.
func checkLossy(lc lossyCase) (matched bool, binB []byte, detail string) {
	in, err := disasm.DecodeTarget(lc.bytes, 0, lc.org, lc.target)
	if err != nil {
		return false, nil, fmt.Sprintf("    zendis failed to decode: %v", err)
	}
	source := fmt.Sprintf("ORG 0x%04X\n%s\n", lc.org, in.Text())
	result, err := newAssembler(false).AssembleString(source)
	if err != nil || len(result.Errors) > 0 {
		return false, nil, fmt.Sprintf("    zenas rejected %q: err=%v errors=%v", in.Text(), err, result.Errors)
	}
	return bytes.Equal(lc.bytes, result.MachineCode), result.MachineCode, ""
}

func newAssembler(z80n bool) *assembler.Assembler {
	a := assembler.New()
	if z80n {
		a.EnableZ80N()
	}
	return a
}

func roundTrip(tc testCase) (ok bool, detail string) {
	result, err := newAssembler(tc.z80n).AssembleString(tc.source)
	if err != nil || len(result.Errors) > 0 {
		return false, fmt.Sprintf("  initial assembly failed: err=%v errors=%v", err, result.Errors)
	}
	binA := result.MachineCode
	org := result.Origin

	var lines []string
	lines = append(lines, fmt.Sprintf("ORG 0x%04X", org))
	offset := 0
	addr := org
	for offset < len(binA) {
		in, err := disasm.DecodeTarget(binA, offset, addr, tc.target)
		if err != nil {
			return false, fmt.Sprintf("  zendis failed to decode at offset %d: %v", offset, err)
		}
		if in.Undefined {
			return false, fmt.Sprintf("  zendis left %04X undefined (%s) -- can't round-trip", in.Address, in.Text())
		}
		lines = append(lines, in.Text())
		addr += uint16(in.Length)
		offset += in.Length
	}
	reasmSource := strings.Join(lines, "\n")

	result2, err := newAssembler(tc.z80n).AssembleString(reasmSource)
	if err != nil || len(result2.Errors) > 0 {
		return false, fmt.Sprintf("  zenas rejected zendis's own output: err=%v errors=%v\n  reassembled source:\n%s",
			err, result2.Errors, indent(reasmSource))
	}
	binB := result2.MachineCode

	if !bytes.Equal(binA, binB) {
		return false, fmt.Sprintf("  byte mismatch:\n    original:      % X\n    round-tripped: % X\n  reassembled source:\n%s",
			binA, binB, indent(reasmSource))
	}
	return true, ""
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "    " + l
	}
	return strings.Join(lines, "\n")
}

// checkCLIPlainFormat builds the real zendis CLI binary (not the
// library, and not a reimplementation of its logic) and runs it with a
// hints file exercising labels, a comment, and a data range, against a
// small test binary. Confirms the resulting --format=plain output
// reassembles through zenas to the exact same bytes -- proof that
// hints-driven label declarations, comment lines, and data-range DB
// output are all genuinely round-trippable, not just that they look
// plausible. This exercises a layer the roundTrip cases above don't:
// those call disasm.DecodeTarget directly, never touching main.go's
// hints parsing, label/comment resolution, or format dispatch.
func checkCLIPlainFormat() (ok bool, detail string) {
	zendisBin := filepath.Join(os.TempDir(), "zendis_roundtrip_check")
	build := exec.Command("go", "build", "-o", zendisBin, "github.com/ha1tch/zendis/cmd/zendis")
	if out, err := build.CombinedOutput(); err != nil {
		return false, fmt.Sprintf("  failed to build the zendis CLI: %v\n%s", err, out)
	}
	defer os.Remove(zendisBin)

	// JP 0x800B; 8 bytes of data; HALT at 0x800B -- same shape as the
	// hand-verified case this test is standing in for permanently.
	original := []byte{0xC3, 0x0B, 0x80, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x76}
	binPath := filepath.Join(os.TempDir(), "zendis_roundtrip_check.bin")
	if err := os.WriteFile(binPath, original, 0644); err != nil {
		return false, fmt.Sprintf("  failed to write the test binary: %v", err)
	}
	defer os.Remove(binPath)

	hintsContent := "label 0x800B Main\ncomment 0x800B program entry point\ndata 0x8003 8\n"
	hintsPath := filepath.Join(os.TempDir(), "zendis_roundtrip_check.hints")
	if err := os.WriteFile(hintsPath, []byte(hintsContent), 0644); err != nil {
		return false, fmt.Sprintf("  failed to write the hints file: %v", err)
	}
	defer os.Remove(hintsPath)

	out, err := exec.Command(zendisBin, "--org=0x8000", "--hints="+hintsPath, "--format=plain", binPath).Output()
	if err != nil {
		return false, fmt.Sprintf("  zendis CLI run failed: %v", err)
	}

	result, err := newAssembler(false).AssembleString(string(out))
	if err != nil || len(result.Errors) > 0 {
		return false, fmt.Sprintf("  zenas rejected zendis's --format=plain output: err=%v errors=%v\n  output:\n%s",
			err, result.Errors, indent(string(out)))
	}
	if !bytes.Equal(original, result.MachineCode) {
		return false, fmt.Sprintf("  byte mismatch:\n    original:      % X\n    round-tripped: % X\n  --format=plain output:\n%s",
			original, result.MachineCode, indent(string(out)))
	}
	return true, ""
}
