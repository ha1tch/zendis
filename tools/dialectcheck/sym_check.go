package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// checkSymFiles verifies zendis's writePasmoSym and writeSjasmplusSym
// against each real assembler's own .sym output, for a case with one
// resolved label -- confirms byte-for-byte identity, not just
// structural similarity.
func checkSymFiles(zendisBin, pasmoBin, sjasmplusBin string) (ok bool, detail string) {
	tc := testCase{
		name: "sym check",
		source: `ORG 0x8000
    JP Main
    NOP
Main:
    HALT
`,
	}
	original, org, err := assembleWithZenas(tc)
	if err != nil {
		return false, "  " + err.Error()
	}

	work, err := os.MkdirTemp("", "dialectcheck-sym-*")
	if err != nil {
		return false, "  " + err.Error()
	}
	defer os.RemoveAll(work)

	binPath := filepath.Join(work, "input.bin")
	if err := os.WriteFile(binPath, original, 0644); err != nil {
		return false, "  " + err.Error()
	}
	orgFlag := fmt.Sprintf("--org=0x%04X", org)

	// pasmo
	zendisSymPasmo := filepath.Join(work, "zendis_pasmo.sym")
	asmPasmo := filepath.Join(work, "out_pasmo.asm")
	out, err := exec.Command(zendisBin, orgFlag, "--format=pasmo", "--sym="+zendisSymPasmo, binPath).Output()
	if err != nil {
		return false, "  zendis --format=pasmo run: " + err.Error()
	}
	if err := os.WriteFile(asmPasmo, out, 0644); err != nil {
		return false, "  " + err.Error()
	}
	realSymPasmo := filepath.Join(work, "real_pasmo.sym")
	outBinPasmo := filepath.Join(work, "real_pasmo.bin")
	if cout, err := exec.Command(pasmoBin, asmPasmo, outBinPasmo, realSymPasmo).CombinedOutput(); err != nil {
		return false, fmt.Sprintf("  real pasmo run: %v\n%s", err, cout)
	}
	zendisData, err := os.ReadFile(zendisSymPasmo)
	if err != nil {
		return false, "  reading zendis pasmo .sym: " + err.Error()
	}
	realData, err := os.ReadFile(realSymPasmo)
	if err != nil {
		return false, "  reading real pasmo .sym: " + err.Error()
	}
	if !bytes.Equal(zendisData, realData) {
		return false, fmt.Sprintf("  pasmo .sym mismatch:\n    zendis: %q\n    real:   %q", zendisData, realData)
	}

	// sjasmplus
	zendisSymSj := filepath.Join(work, "zendis_sj.sym")
	asmSj := filepath.Join(work, "out_sj.asm")
	out, err = exec.Command(zendisBin, orgFlag, "--format=sjasmplus", "--sym="+zendisSymSj, binPath).Output()
	if err != nil {
		return false, "  zendis --format=sjasmplus run: " + err.Error()
	}
	if err := os.WriteFile(asmSj, out, 0644); err != nil {
		return false, "  " + err.Error()
	}
	realSymSj := filepath.Join(work, "real_sj.sym")
	outBinSj := filepath.Join(work, "real_sj.bin")
	if cout, err := exec.Command(sjasmplusBin, "--raw="+outBinSj, "--sym="+realSymSj, asmSj).CombinedOutput(); err != nil {
		return false, fmt.Sprintf("  real sjasmplus run: %v\n%s", err, cout)
	}
	zendisData2, err := os.ReadFile(zendisSymSj)
	if err != nil {
		return false, "  reading zendis sjasmplus .sym: " + err.Error()
	}
	realData2, err := os.ReadFile(realSymSj)
	if err != nil {
		return false, "  reading real sjasmplus .sym: " + err.Error()
	}
	if !bytes.Equal(zendisData2, realData2) {
		return false, fmt.Sprintf("  sjasmplus .sym mismatch:\n    zendis: %q\n    real:   %q", zendisData2, realData2)
	}

	return true, ""
}
