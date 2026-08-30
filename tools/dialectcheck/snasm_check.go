package main

import (
	"fmt"
	"os"
	"os/exec"
)

// roundTripSnasm disassembles tc's fixture with --format=snasm and
// confirms real SNasm.exe (run via mono) reassembles it to the exact
// same bytes.
func roundTripSnasm(zendisBin, monoPath, snasmBin string, tc testCase) (ok bool, detail string) {
	return roundTrip(zendisBin, tc, "snasm", func(asmPath string) ([]byte, error) {
		outBin := asmPath + ".bin"
		cmd := exec.Command(monoPath, snasmBin, asmPath, outBin)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("snasm: %v\n%s", err, out)
		}
		return os.ReadFile(outBin)
	})
}

// snasmMulCase exercises the one Z80N form confirmed to need its own
// text fix: SNASM's MUL has no operand syntax at all (fixed on
// D*E->DE), and "MUL D,E" -- zendis's normal rendering, correct for
// zenas/pasmo/sjasmplus -- fails against the real binary even under
// "opt illegal". Every other Z80N form checked (SWAPNIB, OUTINB,
// PIXELDN, PIXELAD, SETAE, LDIX/LDWS/LDDX/LDIRX/LDDRX, MIRROR A, the
// ADD HL/DE/BC forms, PUSH imm16, TEST imm, the five barrel-shift
// instructions, NEXTREG, JP (C)) matched zendis's existing rendering
// exactly with no fix needed, so this is deliberately the one
// mnemonic singled out here rather than a broader Z80N sweep.
var snasmMulCase = testCase{
	name: "SNASM: MUL text fix (bare MUL, not \"MUL D,E\")",
	z80n: true,
	source: `ORG 0x8000
    LD A,0x2A
    MUL D,E
    LD B,A
    HALT
`,
}

// runSnasmChecks runs the shared classic corpus, the adversarial
// cases, and the SNASM-specific MUL case against the real binary.
// Kept as one self-contained block (rather than threading SNASM into
// every existing pasmo/sjasmplus loop) since it's the one dialect
// that's conditionally skipped, not a peer with unconditional
// coverage.
func runSnasmChecks(zendisBin, monoPath, snasmBin string) (failures, total int) {
	for _, tc := range sharedCases {
		total++
		if ok, detail := roundTripSnasm(zendisBin, monoPath, snasmBin, tc); ok {
			fmt.Printf("PASS  [snasm]      %s\n", tc.name)
		} else {
			failures++
			fmt.Printf("FAIL  [snasm]      %s\n%s\n", tc.name, detail)
		}
	}

	for _, tc := range adversarialCases {
		total++
		if ok, detail := roundTripSnasm(zendisBin, monoPath, snasmBin, tc); ok {
			fmt.Printf("PASS  [snasm]      %s\n", tc.name)
		} else {
			failures++
			fmt.Printf("FAIL  [snasm]      %s\n%s\n", tc.name, detail)
		}
	}

	for _, tc := range []testCase{case11SidewizePoll, case12SidewizeCaller} {
		total++
		if ok, detail := roundTripSnasm(zendisBin, monoPath, snasmBin, tc); ok {
			fmt.Printf("PASS  [snasm]      %s\n", tc.name)
		} else {
			failures++
			fmt.Printf("FAIL  [snasm]      %s\n%s\n", tc.name, detail)
		}
	}

	// case8/case9's DDCB-copy-to-register and bare IN/OUT forms are
	// exactly what snasmUnrepresentable exists for -- confirms the
	// fallback triggers and the fallback DB line's bytes are
	// correctly $-formatted, not just that ordinary code round-trips.
	for _, tc := range []testCase{case8DDCBCopy, case9InOutC} {
		total++
		if ok, detail := roundTripSnasm(zendisBin, monoPath, snasmBin, tc); ok {
			fmt.Printf("PASS  [snasm]      %s\n", tc.name)
		} else {
			failures++
			fmt.Printf("FAIL  [snasm]      %s\n%s\n", tc.name, detail)
		}
	}

	total++
	if ok, detail := roundTripSnasm(zendisBin, monoPath, snasmBin, snasmMulCase); ok {
		fmt.Printf("PASS  [snasm]      %s\n", snasmMulCase.name)
	} else {
		failures++
		fmt.Printf("FAIL  [snasm]      %s\n%s\n", snasmMulCase.name, detail)
	}

	return failures, total
}
