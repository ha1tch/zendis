// Ten adversarial cases beyond the "clean, illustrative" corpus in
// main.go: real, unedited third-party machine code, deliberately
// awkward boundary conditions, and instruction forms this project has
// already found real per-dialect gaps in (so a regression shows up
// immediately, not just "looks plausible").
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// zexallSlice is bytes 0x0100-0x0139 of zexall.com (Ian Bartholomew's
// Z80 instruction exerciser, via zen80's own test ROM set) -- real,
// unedited machine code, not written for this project. Bounded to end
// just before the string-literal data that follows in the real file
// (confirmed by disassembling the whole thing first and finding where
// it stops looking like code): CP/M header dispatch, a BDOS string
// print via register C=9, and the exerciser's own startup loop. Dense
// and varied by construction -- it's real code doing real work, not
// a clean textbook example.
var zexallSlice = []byte{
	0xC3, 0x13, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x2A, 0x06, 0x00, 0xF9, 0x11,
	0xDA, 0x1D, 0x0E, 0x09, 0xCD, 0xCE, 0x1D, 0x21, 0x3A, 0x01, 0x7E, 0x23,
	0xB6, 0xCA, 0x2F, 0x01, 0x2B, 0xCD, 0xE2, 0x1A, 0xC3, 0x22, 0x01, 0x11,
	0xF6, 0x1D, 0x0E, 0x09, 0xCD, 0xCE, 0x1D, 0xC3, 0x00, 0x00,
}

// spectrum48kROMSlice is bytes 0x0000-0x002F of the real Sinclair ZX
// Spectrum 48K ROM (via zen80's own ROM set): the reset routine and
// the RST vector table (RST 0x38 filling unused slots -- the
// maskable-interrupt entry point, a famous, extensively documented
// piece of the machine). Shipped, production firmware, not a test
// fixture written for this project.
var spectrum48kROMSlice = []byte{
	0xF3, 0xAF, 0x11, 0xFF, 0xFF, 0xC3, 0xCB, 0x11, 0x2A, 0x5D, 0x5C, 0x22,
	0x5F, 0x5C, 0x18, 0x43, 0xC3, 0xF2, 0x15, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0x2A, 0x5D, 0x5C, 0x7E, 0xCD, 0x7D, 0x00, 0xD0, 0xCD, 0x74, 0x00, 0x18,
	0xF7, 0xFF, 0xFF, 0xFF, 0xC3, 0x5B, 0x33, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
}

var adversarialCases = []testCase{
	{
		name:     "1. real-world: zexall.com code slice (Z80 exerciser, unedited)",
		rawBytes: zexallSlice,
		rawOrg:   0x0100,
		noLabels: true, // calls routines outside the slice -- see noLabels doc
	},
	{
		name:     "2. real-world: Spectrum 48K ROM reset + RST vector table (unedited)",
		rawBytes: spectrum48kROMSlice,
		rawOrg:   0x0000,
		noLabels: true,
	},
	{
		name: "3. keyword-colliding label (\"END\", a real directive name in both dialects)",
		source: `ORG 0x8000
    JP End
    NOP
End:
    HALT
`,
		hints: "label 0x8005 End\n",
	},
	{
		name: "4. IX/IY displacement at the extreme signed-byte boundary (-128, +127)",
		source: `ORG 0x8000
    LD A,(IX-128)
    LD A,(IX+127)
    LD B,(IY-128)
    LD (IY+127),C
`,
	},
	{
		// Bytes decoded and confirmed against zendis's own oracle-tested
		// decoder before use here (not hand-derived and trusted blind):
		// SLL B, SLL (HL), SLL (IX+0x02), SLL (IY-0x02).
		name:     "6. SLL undocumented shift (supported by both real assemblers, unlike zenas)",
		rawBytes: []byte{0xCB, 0x30, 0xCB, 0x36, 0xDD, 0xCB, 0x02, 0x36, 0xFD, 0xCB, 0xFE, 0x36},
		rawOrg:   0x8000,
	},
	{
		// Bytes independently confirmed against the real pasmo and
		// sjasmplus binaries directly, by hand, before this harness
		// existed: RLC (IX+0x03), BIT 0,(IX+0x03), RES 1,(IY+0x02),
		// SET 7,(IX+0x00).
		name:     "7. DDCB/FDCB plain rotate and bit forms (no copy target)",
		rawBytes: []byte{0xDD, 0xCB, 0x03, 0x06, 0xDD, 0xCB, 0x03, 0x46, 0xFD, 0xCB, 0x02, 0x8E, 0xDD, 0xCB, 0x00, 0xFE},
		rawOrg:   0x8000,
	},
}

// case8DDCBCopy and case9InOutC are run separately from
// adversarialCases (below, in checkAdversarial) rather than through
// the shared pasmo/sjasmplus loop: pasmo is expected to hit its own
// new DB fallback here (isRepresentable is false for exactly these
// forms), while sjasmplus is expected to keep them as real mnemonics
// -- an asymmetric expectation the generic sharedCases loop doesn't
// express.
// case8DDCBCopy bytes: LD A,0x2A; RLC (IX+0x03),B; RES 1,(IY+0x02),C;
// HALT -- decoded and confirmed against zendis's own oracle-tested
// decoder before use (zenas cannot assemble the DDCB copy-to-register
// form at all -- a separate, already-documented zenas gap, not
// something to route around here).
var case8DDCBCopy = testCase{
	name:     "8. DDCB/FDCB undocumented copy-to-register form (pasmo fallback expected)",
	rawBytes: []byte{0x3E, 0x2A, 0xDD, 0xCB, 0x03, 0x00, 0xFD, 0xCB, 0x02, 0x89, 0x76},
	rawOrg:   0x8000,
}

// case9InOutC bytes: LD A,0x2A; IN (C); OUT (C),0x00; HALT -- the IN/OUT
// bytes independently confirmed against real sjasmplus by hand before
// this harness existed (ED 70 / ED 71); zenas cannot assemble either
// undocumented form.
var case9InOutC = testCase{
	name:     "9. undocumented bare IN (C) / OUT (C),0 (pasmo fallback expected)",
	rawBytes: []byte{0x3E, 0x2A, 0xED, 0x70, 0xED, 0x71, 0x76},
	rawOrg:   0x8000,
}

// case10Z80NIdiom is synthesized, not extracted -- there's no single
// open-source file this project could point at -- but every construct
// in it is a well-established, real ZX Spectrum Next programming
// idiom: NEXTREG writes to set the border colour and configure Layer
// 2 (a real Next graphics mode), and MUL for a fast sprite-row-offset
// calculation (row * width), the canonical real-world use of Z80N's
// hardware multiply. Mixed with ordinary classic instructions, so
// pasmo's fallback and its surrounding classic code are exercised in
// the same realistic sequence, not in isolation.
var case10Z80NIdiom = testCase{
	name: "10. realistic Z80N idiom: Next palette/Layer 2 setup + MUL sprite offset (pasmo fallback expected)",
	z80n: true,
	source: `ORG 0x8000
    NEXTREG 0x11,0x02
    NEXTREG 0x12,0x00
    NEXTREG 0x13,0x00
    LD A,0x07
    NEXTREG 0x40,A
    LD D,0x0A
    LD E,0xC0
    MUL D,E
    LD (0x9000),DE
    HALT
`,
}

// case11SidewizePoll and case12SidewizeCaller: real Z80 code from
// Sidewize, by Steve Whetherill, shared for technical discussion by
// MartianGirl (author of the SpecIDE emulator) in a Mastodon thread
// (https://mas.to/@FromTheFourthPlanet/117185231157817130), used here
// with attribution. The bytes were reconstructed from the shared
// disassembly listing and independently cross-checked against a 2007
// ZX Spectrum clone-design writeup that reverse-engineered the same
// routine address-for-address
// (http://www.zxdesign.info/sidewizeTest.shtml) -- not a first
// disclosure, a well-documented piece of a well-known game's
// behaviour. Verified against zendis's own decoder before use here
// (every address and mnemonic matches both sources exactly).
//
// The routine is Sidewize's floating-bus poll -- LD A,R and IN A,(C)
// with BC=$40FF read the ULA's floating bus (the byte the display
// hardware is currently fetching from video memory), spinning until it
// matches a target value in E, to synchronise precisely with the
// raster beam. Two entry points share one loop body with a different
// target pre-loaded ($40 vs $00) -- a genuinely elegant piece of real
// code reuse. Real, hand-optimised timing code is a different flavour
// of complexity from the ROM/exerciser extracts above: it deliberately
// discards read results for their timing side effects alone.
var case11SidewizePoll = testCase{
	name:     "11. Sidewize (Steve Whetherill): floating-bus poll routine, two entry points sharing one loop",
	rawBytes: []byte{0x01, 0xFF, 0x40, 0x1E, 0x40, 0xED, 0x5F, 0xED, 0x78, 0xBB, 0xC2, 0xF9, 0x9C, 0xC9, 0x01, 0xFF, 0x40, 0x1E, 0x00, 0xC3, 0xF9, 0x9C},
	rawOrg:   0x9CF4,
}

// case12SidewizeCaller calls into case11SidewizePoll's routine (JP
// NZ,0x9CF4) if a check fails, and otherwise falls through to its own
// timing trick: reading contended screen memory (0x5800, in the
// attribute area) purely to burn contended-memory T-states before
// polling an I/O port with the immediate-operand IN A,(n) form -- a
// different opcode from case9InOutC's IN A,(C). noLabels is set since
// its branch target lives in case11's block, outside this one's own
// window -- there's no requirement that everything a real routine
// calls lives in whatever slice was extracted from around it; raw
// address rendering is the honest choice, not a label with nothing to
// declare it.
var case12SidewizeCaller = testCase{
	name:     "12. Sidewize (Steve Whetherill): caller using contended-memory timing + immediate-port IN A,(n)",
	rawBytes: []byte{0x3A, 0x53, 0x0B, 0xFE, 0x7E, 0xC2, 0xF4, 0x9C, 0x11, 0x0F, 0x41, 0x3A, 0x00, 0x58, 0x7B, 0xDB, 0xFD, 0xBA, 0xC2, 0xF5, 0xFD, 0xC9},
	rawOrg:   0xFDEA,
	noLabels: true,
}

// checkAdversarial runs all ten adversarial cases and returns the
// failure count, printing PASS/FAIL for each the same way main's own
// corpus loop does.
func checkAdversarial(zendisBin, pasmoBin, sjasmplusBin string) (failures, total int) {
	for _, tc := range adversarialCases {
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

	// 5. Long label names at pasmo's exact tab-padding boundaries
	// (tablabel: 2 tabs under 8 chars, 1 tab for 8-15, a space at
	// 16+) -- a dedicated .sym check, not an instruction round-trip.
	total++
	if ok, detail := checkLongLabelSymPadding(zendisBin, pasmoBin); ok {
		fmt.Println("PASS  [pasmo .sym] 5. long label names at pasmo's tab-padding boundaries (7/8/15/16 chars)")
	} else {
		failures++
		fmt.Printf("FAIL  [pasmo .sym] 5. long label names at pasmo's tab-padding boundaries\n%s\n", detail)
	}

	// 8: pasmo must fall back (DB), sjasmplus must keep the mnemonics.
	total++
	if ok, detail := roundTripPasmo(zendisBin, pasmoBin, case8DDCBCopy); ok {
		fmt.Printf("PASS  [pasmo]      %s\n", case8DDCBCopy.name)
	} else {
		failures++
		fmt.Printf("FAIL  [pasmo]      %s\n%s\n", case8DDCBCopy.name, detail)
	}
	total++
	if ok, detail := roundTripSjasmplus(zendisBin, sjasmplusBin, case8DDCBCopy); ok {
		fmt.Printf("PASS  [sjasmplus]  %s\n", case8DDCBCopy.name)
	} else {
		failures++
		fmt.Printf("FAIL  [sjasmplus]  %s\n%s\n", case8DDCBCopy.name, detail)
	}

	// 9: same asymmetric expectation.
	total++
	if ok, detail := roundTripPasmo(zendisBin, pasmoBin, case9InOutC); ok {
		fmt.Printf("PASS  [pasmo]      %s\n", case9InOutC.name)
	} else {
		failures++
		fmt.Printf("FAIL  [pasmo]      %s\n%s\n", case9InOutC.name, detail)
	}
	total++
	if ok, detail := roundTripSjasmplus(zendisBin, sjasmplusBin, case9InOutC); ok {
		fmt.Printf("PASS  [sjasmplus]  %s\n", case9InOutC.name)
	} else {
		failures++
		fmt.Printf("FAIL  [sjasmplus]  %s\n%s\n", case9InOutC.name, detail)
	}

	// 10: pasmo falls back for the Z80N lines, keeps the classic lines
	// as mnemonics; sjasmplus keeps everything as mnemonics.
	total++
	if ok, detail := roundTripPasmo(zendisBin, pasmoBin, case10Z80NIdiom); ok {
		fmt.Printf("PASS  [pasmo]      %s\n", case10Z80NIdiom.name)
	} else {
		failures++
		fmt.Printf("FAIL  [pasmo]      %s\n%s\n", case10Z80NIdiom.name, detail)
	}
	total++
	if ok, detail := roundTripSjasmplus(zendisBin, sjasmplusBin, case10Z80NIdiom); ok {
		fmt.Printf("PASS  [sjasmplus]  %s\n", case10Z80NIdiom.name)
	} else {
		failures++
		fmt.Printf("FAIL  [sjasmplus]  %s\n%s\n", case10Z80NIdiom.name, detail)
	}

	// 11 and 12: real Sidewize code, run through the same generic
	// pasmo+sjasmplus loop as adversarialCases -- both are ordinary
	// (no fallback expected on either dialect).
	for _, tc := range []testCase{case11SidewizePoll, case12SidewizeCaller} {
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

	return failures, total
}

// checkLongLabelSymPadding verifies zendis's pasmo .sym writer matches
// real pasmo's own variable tab-padding (tablabel in asm.cxx: 2 tabs
// under 8 chars, 1 tab for 8-15, a single space at 16+) at all four
// boundary lengths, not just the short-name case the earlier sym_check
// already covers. Label lengths are asserted with len(), not counted
// by eye, given this project's own track record of off-by-one mistakes
// in hand-constructed test data earlier this session.
func checkLongLabelSymPadding(zendisBin, pasmoBin string) (ok bool, detail string) {
	type namedAddr struct {
		name    string
		wantLen int
	}
	labels := []namedAddr{
		{"Label07", 7},
		{"Label008", 8},
		{"Label0000000015", 15},
		{"Label00000000016", 16},
	}
	for _, l := range labels {
		if len(l.name) != l.wantLen {
			return false, fmt.Sprintf("  test bug: %q is %d chars, want %d", l.name, len(l.name), l.wantLen)
		}
	}

	tc := testCase{
		name:   "long label padding fixture",
		source: "ORG 0x8000\n    NOP\n    NOP\n    NOP\n    NOP\n",
	}
	original, org, err := fixture(tc)
	if err != nil {
		return false, "  " + err.Error()
	}

	hintsLines := ""
	for i, l := range labels {
		hintsLines += fmt.Sprintf("label 0x%04X %s\n", int(org)+i, l.name)
	}

	work, err := os.MkdirTemp("", "dialectcheck-longlabel-*")
	if err != nil {
		return false, "  " + err.Error()
	}
	defer os.RemoveAll(work)

	binPath := filepath.Join(work, "input.bin")
	if err := os.WriteFile(binPath, original, 0644); err != nil {
		return false, "  " + err.Error()
	}
	hintsPath := filepath.Join(work, "hints.txt")
	if err := os.WriteFile(hintsPath, []byte(hintsLines), 0644); err != nil {
		return false, "  " + err.Error()
	}

	zendisSym := filepath.Join(work, "zendis.sym")
	asmPath := filepath.Join(work, "out.asm")
	out, err := exec.Command(zendisBin, fmt.Sprintf("--org=0x%04X", org), "--hints="+hintsPath, "--format=pasmo", "--sym="+zendisSym, binPath).Output()
	if err != nil {
		return false, "  zendis run: " + err.Error()
	}
	if err := os.WriteFile(asmPath, out, 0644); err != nil {
		return false, "  " + err.Error()
	}

	realSym := filepath.Join(work, "real.sym")
	realBin := filepath.Join(work, "real.bin")
	if cout, err := exec.Command(pasmoBin, asmPath, realBin, realSym).CombinedOutput(); err != nil {
		return false, fmt.Sprintf("  real pasmo run: %v\n%s", err, cout)
	}

	zendisData, err := os.ReadFile(zendisSym)
	if err != nil {
		return false, "  " + err.Error()
	}
	realData, err := os.ReadFile(realSym)
	if err != nil {
		return false, "  " + err.Error()
	}
	if !bytes.Equal(zendisData, realData) {
		return false, fmt.Sprintf("  .sym mismatch:\n    zendis: %q\n    real:   %q", zendisData, realData)
	}
	return true, ""
}
