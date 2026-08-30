// Command dialectcheck verifies zendis's --format=pasmo and
// --format=sjasmplus output against the real pasmo and sjasmplus
// binaries -- the same property tools/roundtrip verifies for zenas
// (a Go dependency, always available), but for two assemblers that
// have no Go bindings at all. bootstrap.go finds them on PATH, in a
// cache, or downloads and builds them from source.
//
// Deliberately a separate module and a separate tool from
// tools/roundtrip: this one has real, environment-dependent
// prerequisites (git, g++, and for sjasmplus, make) that the
// zenas-based harness doesn't. Not part of `go test ./...` from the
// zendis module root. Run explicitly:
//
//	cd tools/dialectcheck && go run .
//
// zenas is used here only to assemble each case's starting binary from
// readable source -- its own instruction encoding is independently
// confirmed identical to real pasmo's and sjasmplus's (see
// docs/DIALECT_COMPATIBILITY.md and this tool's own findings), so it's
// a reliable fixture generator, not part of what's being verified.
// What's actually under test is zendis's dialect-specific output,
// fed to the real pasmo/sjasmplus binaries, never to zenas.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ha1tch/zenas/assembler"
)

type testCase struct {
	name   string
	source string
	z80n   bool // assemble (and disassemble) under Z80N

	// rawBytes/rawOrg, when set, bypass zenas fixture assembly entirely
	// and use these bytes directly as the starting binary -- for cases
	// whose fixture is a real, pre-existing binary (a ROM image, a test
	// exerciser) rather than hand-written source.
	rawBytes []byte
	rawOrg   uint16

	// hints, when non-empty, is written to a temp file and passed as
	// zendis's --hints for this case (label/comment/data directives).
	hints string

	// noLabels passes --labels=false. For a slice extracted from a
	// larger real program, a JP/CALL target routinely falls outside
	// the slice -- there's no requirement that everything a real
	// program calls lives in whatever window was extracted from it.
	// Raw-address rendering is the correct, honest choice there:
	// generating a label for an address the listing never defines
	// would produce text no assembler could resolve, for a reason
	// that has nothing to do with dialect correctness.
	noLabels bool
}

// fixture returns tc's starting binary and load address: rawBytes/rawOrg
// directly if set, otherwise assembled from source via zenas.
func fixture(tc testCase) (code []byte, org uint16, err error) {
	if tc.rawBytes != nil {
		return tc.rawBytes, tc.rawOrg, nil
	}
	return assembleWithZenas(tc)
}

// sharedCases assemble and disassemble identically under pasmo and
// sjasmplus -- the classic instruction set both real assemblers were
// independently confirmed to accept in this project's own
// investigation (zenas's cross-checking plus direct source reading of
// both real assemblers).
var sharedCases = []testCase{
	{
		name: "unprefixed mix, label, comment",
		source: `ORG 0x8000
    JP Main
    LD A,0x2A
    ADD A,B
    LD B,0x05
Main:
    HALT
`,
	},
	{
		name: "CB rotates and bit ops",
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
		name: "ED classic",
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
		name: "DD/FD substitution: exceptions, half-registers, indexed memory, 16-bit forms",
		source: `ORG 0x8000
    HALT
    EX DE,HL
    LD H,(IX+0x02)
    LD L,(IY+0x02)
    INC IXh
    DEC IXl
    LD (IX+0x05),0x7A
    LD IX,0x1234
    ADD IX,DE
    PUSH IX
    POP IX
    JP (IX)
    LD SP,IX
    ADD A,(IX+0x04)
    CP (IX+0x01)
`,
	},
	{
		name: "IXh/IXl full ALU matrix",
		source: `ORG 0x8000
    ADD A,IXh
    ADC A,IXh
    SUB IXh
    SBC A,IXh
    AND IXh
    XOR IXh
    OR IXh
    CP IXh
    LD B,IXh
    LD IXh,A
    LD IXh,IXl
`,
	},
}

// sjasmplusOnlyCases need Z80N, which only sjasmplus (of the two real
// binaries here) supports at all -- pasmo has none, confirmed by
// grepping its source for nextreg/z80n/zxnext (zero matches).
var sjasmplusOnlyCases = []testCase{
	{
		name: "Z80N extensions",
		z80n: true,
		source: `ORG 0x8000
    SWAPNIB
    MIRROR A
    TEST 0x2A
    BSLA DE,B
    MUL D,E
    ADD HL,0x1234
    PUSH 0x1234
    OUTINB
    NEXTREG 0x10,0x05
    NEXTREG 0x10,A
    PIXELDN
    JP (C)
    LDIX
    LDWS
    LDDX
    LDIRX
    LDPIRX
    LDDRX
`,
	},
}

// pasmoFallbackCase mixes classic instructions with one Z80N
// instruction. Under --format=pasmo this must fall back to a DB line
// for the Z80N instruction specifically -- pasmo can't represent it as
// a mnemonic -- while the classic instructions around it stay as
// mnemonics. The whole thing must still round-trip byte-for-byte
// through real pasmo, proving the fallback is byte-correct even where
// it isn't mnemonic-readable.
var pasmoFallbackCase = testCase{
	name: "pasmo Z80N fallback (classic + one Z80N instruction)",
	z80n: true,
	source: `ORG 0x8000
    LD A,0x2A
    MUL D,E
    LD B,A
    HALT
`,
}

func main() {
	fmt.Println("dialectcheck: locating zendis, pasmo, sjasmplus...")
	zendisBin, err := buildZendisBinary()
	if err != nil {
		fmt.Println("FATAL: building zendis:", err)
		os.Exit(1)
	}
	pasmoBin, err := findOrBuildPasmo()
	if err != nil {
		fmt.Println("FATAL: pasmo unavailable:", err)
		os.Exit(1)
	}
	sjasmplusBin, err := findOrBuildSjasmplus()
	if err != nil {
		fmt.Println("FATAL: sjasmplus unavailable:", err)
		os.Exit(1)
	}
	fmt.Printf("zendis: %s\npasmo: %s\nsjasmplus: %s\n\n", zendisBin, pasmoBin, sjasmplusBin)

	// SNASM is treated as optional, not fatal, unlike pasmo/sjasmplus
	// above: it cannot be auto-fetched (not open source, no public
	// repo -- see findSnasm's own doc comment), so its absence is
	// expected in most environments, not an error in this harness.
	monoPath, snasmBin, snasmErr := findSnasm()
	if snasmErr != nil {
		fmt.Println("SNASM checks skipped:", snasmErr)
		fmt.Println()
	} else {
		fmt.Printf("snasm: %s (via %s)\n\n", snasmBin, monoPath)
	}

	failures := 0
	total := 0

	for _, tc := range sharedCases {
		total++
		if ok, detail := roundTripPasmo(zendisBin, pasmoBin, tc); ok {
			fmt.Printf("PASS  [pasmo]      %s\n", tc.name)
		} else {
			failures++
			fmt.Printf("FAIL  [pasmo]      %s\n%s\n", tc.name, detail)
		}

		total++
		if ok, detail := roundTripSjasmplus(zendisBin, sjasmplusBin, tc); ok {
			fmt.Printf("PASS  [sjasmplus]  %s\n", tc.name)
		} else {
			failures++
			fmt.Printf("FAIL  [sjasmplus]  %s\n%s\n", tc.name, detail)
		}
	}

	for _, tc := range sjasmplusOnlyCases {
		total++
		if ok, detail := roundTripSjasmplus(zendisBin, sjasmplusBin, tc); ok {
			fmt.Printf("PASS  [sjasmplus]  %s\n", tc.name)
		} else {
			failures++
			fmt.Printf("FAIL  [sjasmplus]  %s\n%s\n", tc.name, detail)
		}
	}

	total++
	if ok, detail := roundTripPasmo(zendisBin, pasmoBin, pasmoFallbackCase); ok {
		fmt.Printf("PASS  [pasmo]      %s\n", pasmoFallbackCase.name)
	} else {
		failures++
		fmt.Printf("FAIL  [pasmo]      %s\n%s\n", pasmoFallbackCase.name, detail)
	}

	fmt.Println()
	total++
	if ok, detail := checkSymFiles(zendisBin, pasmoBin, sjasmplusBin); ok {
		fmt.Println("PASS  [.sym]       pasmo and sjasmplus symbol files match each real assembler's own output")
	} else {
		failures++
		fmt.Printf("FAIL  [.sym]       %s\n", detail)
	}

	fmt.Println()
	fmt.Println("--- adversarial cases ---")
	advFailures, advTotal := checkAdversarial(zendisBin, pasmoBin, sjasmplusBin)
	failures += advFailures
	total += advTotal

	if snasmErr == nil {
		fmt.Println()
		fmt.Println("--- snasm (optional, skipped if unavailable) ---")
		snasmFailures, snasmTotal := runSnasmChecks(zendisBin, monoPath, snasmBin)
		failures += snasmFailures
		total += snasmTotal
	}

	fmt.Println()
	if failures > 0 {
		fmt.Printf("%d/%d checks failed\n", failures, total)
		os.Exit(1)
	}
	fmt.Printf("all %d checks passed\n", total)
}

// buildZendisBinary builds the real zendis CLI (not a reimplementation
// of its logic) to a temp path.
func buildZendisBinary() (string, error) {
	bin := filepath.Join(os.TempDir(), "zendis_dialectcheck")
	// Built from zendis's own module context (cmd.Dir = the zendis repo
	// root, a relative package path within it), not resolved as a
	// dependency of this module -- dialectcheck never imports zendis's
	// Go package (it only shells out to the compiled binary), so nothing
	// anchors a require for it here the way tools/roundtrip has one
	// (it does import pkg/disasm directly, for its library-level cases).
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/zendis")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go build: %v\n%s", err, out)
	}
	return bin, nil
}

// assembleWithZenas produces tc's starting binary from its source.
// zenas is a fixture generator here, not part of what's under test.
func assembleWithZenas(tc testCase) (code []byte, org uint16, err error) {
	a := assembler.New()
	if tc.z80n {
		a.EnableZ80N()
	}
	result, err := a.AssembleString(tc.source)
	if err != nil || len(result.Errors) > 0 {
		return nil, 0, fmt.Errorf("zenas fixture assembly failed: err=%v errors=%v", err, result.Errors)
	}
	return result.MachineCode, result.Origin, nil
}

// roundTripPasmo disassembles tc's fixture with --format=pasmo and
// confirms real pasmo reassembles it to the exact same bytes.
func roundTripPasmo(zendisBin, pasmoBin string, tc testCase) (ok bool, detail string) {
	return roundTrip(zendisBin, tc, "pasmo", func(asmPath string) ([]byte, error) {
		outBin := asmPath + ".bin"
		outSym := asmPath + ".sym"
		cmd := exec.Command(pasmoBin, asmPath, outBin, outSym)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("pasmo: %v\n%s", err, out)
		}
		return os.ReadFile(outBin)
	})
}

// roundTripSjasmplus disassembles tc's fixture with --format=sjasmplus
// and confirms real sjasmplus reassembles it to the exact same bytes.
func roundTripSjasmplus(zendisBin, sjasmplusBin string, tc testCase) (ok bool, detail string) {
	return roundTrip(zendisBin, tc, "sjasmplus", func(asmPath string) ([]byte, error) {
		outBin := asmPath + ".bin"
		args := []string{"--raw=" + outBin, asmPath}
		if tc.z80n {
			args = append([]string{"--zxnext"}, args...)
		}
		cmd := exec.Command(sjasmplusBin, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("sjasmplus: %v\n%s", err, out)
		}
		return os.ReadFile(outBin)
	})
}

// roundTrip is the shared shape: assemble the fixture with zenas,
// disassemble it with the real zendis binary under the given format,
// reassemble that output with assembleFn (the real target assembler),
// and compare bytes.
func roundTrip(zendisBin string, tc testCase, format string, assembleFn func(asmPath string) ([]byte, error)) (ok bool, detail string) {
	original, org, err := fixture(tc)
	if err != nil {
		return false, "  " + err.Error()
	}

	work, err := os.MkdirTemp("", "dialectcheck-*")
	if err != nil {
		return false, "  " + err.Error()
	}
	defer os.RemoveAll(work)

	binPath := filepath.Join(work, "input.bin")
	if err := os.WriteFile(binPath, original, 0644); err != nil {
		return false, "  " + err.Error()
	}

	target := "z80"
	if tc.z80n {
		target = "z80n"
	}
	args := []string{
		fmt.Sprintf("--org=0x%04X", org),
		"--target=" + target,
		"--format=" + format,
	}
	if tc.noLabels {
		args = append(args, "--labels=false")
	}
	if tc.hints != "" {
		hintsPath := filepath.Join(work, "hints.txt")
		if err := os.WriteFile(hintsPath, []byte(tc.hints), 0644); err != nil {
			return false, "  " + err.Error()
		}
		args = append(args, "--hints="+hintsPath)
	}
	args = append(args, binPath)
	cmd := exec.Command(zendisBin, args...)
	asmOut, err := cmd.Output()
	if err != nil {
		return false, fmt.Sprintf("  zendis run failed: %v", err)
	}

	asmPath := filepath.Join(work, "out.asm")
	if err := os.WriteFile(asmPath, asmOut, 0644); err != nil {
		return false, "  " + err.Error()
	}

	reassembled, err := assembleFn(asmPath)
	if err != nil {
		return false, fmt.Sprintf("  %v\n  zendis output:\n%s", err, indent(string(asmOut)))
	}

	if !bytes.Equal(original, reassembled) {
		return false, fmt.Sprintf("  byte mismatch:\n    original:      % X\n    round-tripped: % X\n  zendis output:\n%s",
			original, reassembled, indent(string(asmOut)))
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
