package load

import (
	"errors"
	"testing"

	"github.com/ha1tch/zentools/pkg/tap"
	"github.com/ha1tch/zentools/pkg/tzx"
)

func intPtr(n int) *int       { return &n }
func u16Ptr(n uint16) *uint16 { return &n }

func TestFromTAP_Basic(t *testing.T) {
	code := []byte{0x3E, 0x2A, 0xC9} // LD A,0x2A; RET
	image := tap.EncodeCode("TESTCODE", code, 0x8000)

	gotCode, gotOrg, err := FromTAP(image, TapeOptions{})
	if err != nil {
		t.Fatalf("FromTAP: %v", err)
	}
	if gotOrg != 0x8000 {
		t.Errorf("org = %#04x, want 0x8000 (the header's own Param1)", gotOrg)
	}
	if string(gotCode) != string(code) {
		t.Errorf("code = % X, want % X", gotCode, code)
	}
}

func TestFromTAP_OrgOverride(t *testing.T) {
	code := []byte{0x00, 0x00, 0x00}
	image := tap.EncodeCode("T", code, 0x8000)

	_, gotOrg, err := FromTAP(image, TapeOptions{Options: Options{Org: u16Ptr(0x9000)}})
	if err != nil {
		t.Fatalf("FromTAP: %v", err)
	}
	if gotOrg != 0x9000 {
		t.Errorf("org = %#04x, want 0x9000 (explicit override, not the header's 0x8000)", gotOrg)
	}
}

func TestFromTAP_LengthOverride(t *testing.T) {
	code := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	image := tap.EncodeCode("T", code, 0x8000)

	gotCode, _, err := FromTAP(image, TapeOptions{Options: Options{Length: intPtr(3)}})
	if err != nil {
		t.Fatalf("FromTAP: %v", err)
	}
	if len(gotCode) != 3 || string(gotCode) != string(code[:3]) {
		t.Errorf("code = % X, want the first 3 bytes % X", gotCode, code[:3])
	}
}

func TestFromTAP_LengthExceedsAvailable(t *testing.T) {
	code := []byte{0x01, 0x02, 0x03}
	image := tap.EncodeCode("T", code, 0x8000)

	_, _, err := FromTAP(image, TapeOptions{Options: Options{Length: intPtr(100)}})
	if err == nil {
		t.Fatal("expected an error requesting 100 bytes from a 3-byte block, got nil")
	}
}

func TestFromTAP_BlockSelection(t *testing.T) {
	var image []byte
	image = append(image, tap.EncodeCode("FIRST", []byte{0x01}, 0x8000)...)
	image = append(image, tap.EncodeCode("SECOND", []byte{0x02, 0x02}, 0x9000)...)

	// By index: block 2 is the second header (0=header,1=data,2=header,3=data).
	code, org, err := FromTAP(image, TapeOptions{BlockIndex: intPtr(2)})
	if err != nil {
		t.Fatalf("FromTAP by index: %v", err)
	}
	if org != 0x9000 || string(code) != "\x02\x02" {
		t.Errorf("by index: got org=%#04x code=% X, want org=0x9000 code=02 02", org, code)
	}

	// By name.
	code, org, err = FromTAP(image, TapeOptions{BlockName: "FIRST"})
	if err != nil {
		t.Fatalf("FromTAP by name: %v", err)
	}
	if org != 0x8000 || string(code) != "\x01" {
		t.Errorf("by name: got org=%#04x code=% X, want org=0x8000 code=01", org, code)
	}

	// Unknown name.
	_, _, err = FromTAP(image, TapeOptions{BlockName: "NOSUCH"})
	if !errors.Is(err, ErrBlockNotFound) {
		t.Errorf("unknown name: err = %v, want ErrBlockNotFound", err)
	}
}

func TestFromTAP_BareDataBlock(t *testing.T) {
	// Confirmed against a real commercial tape (zenzx's own chase.tap):
	// a single header for a BASIC loader, then several bare data blocks
	// with no headers at all, for a custom loader that already knows
	// their addresses. Build the same shape synthetically: one header
	// block, then a bare data block appended directly (bypassing
	// EncodeCode, which always writes a header).
	header := tap.EncodeCode("LOADER", []byte{0xAA}, 0x5C00) // a small header+data pair
	bareData := []byte{
		0x05, 0x00, // block length = 5 (flag + 3 data + checksum)
		0xFF,             // data flag
		0x10, 0x20, 0x30, // 3 "code" bytes
		0xFF ^ 0x10 ^ 0x20 ^ 0x30, // checksum
	}
	image := append(append([]byte{}, header...), bareData...)

	blocks, err := tap.Decode(image)
	if err != nil {
		t.Fatalf("sanity check: tap.Decode: %v", err)
	}
	if len(blocks) != 3 || blocks[2].IsHeader {
		t.Fatalf("test construction bug: want 3 blocks with block 2 a bare data block, got %+v", blocks)
	}

	// Without an explicit Org, a bare data block has no load address to
	// default to -- must error, not silently default to 0.
	_, _, err = FromTAP(image, TapeOptions{BlockIndex: intPtr(2)})
	if err == nil {
		t.Fatal("selecting a bare data block with no Org set should error, got nil")
	}

	// With an explicit Org, it works.
	code, org, err := FromTAP(image, TapeOptions{BlockIndex: intPtr(2), Options: Options{Org: u16Ptr(0x9000)}})
	if err != nil {
		t.Fatalf("FromTAP with explicit Org on a bare data block: %v", err)
	}
	if org != 0x9000 || string(code) != "\x10\x20\x30" {
		t.Errorf("got org=%#04x code=% X, want org=0x9000 code=10 20 30", org, code)
	}
}

func TestFromTAP_NoCodeBlock(t *testing.T) {
	// A Program (BASIC) block only -- auto-select must not guess.
	image := tap.EncodeProgram("BASIC", []byte{0x00}, 10)
	_, _, err := FromTAP(image, TapeOptions{})
	if !errors.Is(err, ErrNoCodeBlock) {
		t.Errorf("err = %v, want ErrNoCodeBlock", err)
	}
}

func TestFromTZX_Basic(t *testing.T) {
	code := []byte{0x21, 0x00, 0x80, 0xC9} // LD HL,0x8000; RET
	tapImage := tap.EncodeCode("TZXTEST", code, 0x8500)
	tzxImage, err := tzx.EncodeFromTAP(tapImage, tzx.EncodeOptions{})
	if err != nil {
		t.Fatalf("tzx.EncodeFromTAP: %v", err)
	}

	gotCode, gotOrg, err := FromTZX(tzxImage, TapeOptions{})
	if err != nil {
		t.Fatalf("FromTZX: %v", err)
	}
	if gotOrg != 0x8500 || string(gotCode) != string(code) {
		t.Errorf("got org=%#04x code=% X, want org=0x8500 code=% X", gotOrg, gotCode, code)
	}
}

func TestFromTape_Sniffing(t *testing.T) {
	code := []byte{0x00}
	tapImage := tap.EncodeCode("T", code, 0x8000)
	tzxImage, err := tzx.EncodeFromTAP(tapImage, tzx.EncodeOptions{})
	if err != nil {
		t.Fatalf("tzx.EncodeFromTAP: %v", err)
	}

	if _, _, err := FromTape(tapImage, TapeOptions{}); err != nil {
		t.Errorf("FromTape on a TAP image: %v", err)
	}
	if _, _, err := FromTape(tzxImage, TapeOptions{}); err != nil {
		t.Errorf("FromTape on a TZX image: %v", err)
	}
}

func TestFromTZX_NoStandardDataBlocks(t *testing.T) {
	// A TZX with only pulse-level blocks (no 0x10 at all) must be
	// distinguished from "0x10 blocks existed but none had a Code
	// header" -- there was nothing to even search here.
	image := []byte("ZXTape!\x1a\x01\x14") // signature + version 1.20, no blocks at all
	_, _, err := FromTZX(image, TapeOptions{})
	if !errors.Is(err, ErrNoStandardDataBlocks) {
		t.Errorf("err = %v, want ErrNoStandardDataBlocks", err)
	}
}

func TestFromTZX_TurboSpeedDataBlock(t *testing.T) {
	// An earlier version of FromTZX kept only 0x10, silently dropping
	// every turbo-loaded block on any real TZX using one -- a large
	// fraction of real commercial tapes, which is the whole reason
	// turbo loaders exist. Fixed for the identical reason in the
	// sibling zentools project's own TZX -> tape conversion path first;
	// this test is the same fix applied to zendis's own loading.
	code := []byte{0xCA, 0xFE, 0xBA, 0xBE}
	tapImage := tap.EncodeCode("TURBO", code, 0x9000)
	blocks, err := tap.Decode(tapImage)
	if err != nil {
		t.Fatal(err)
	}
	dataBlock := blocks[1]
	raw := append([]byte{dataBlock.Flag}, dataBlock.Data...)
	raw = append(raw, dataBlock.Checksum)

	image := append([]byte("ZXTape!"), 0x1A, 1, 20)
	u16 := func(v uint16) []byte { return []byte{byte(v), byte(v >> 8)} }
	var block []byte
	block = append(block, 0x11)         // Turbo Speed Data
	block = append(block, u16(1200)...) // pilotPulse
	block = append(block, u16(400)...)  // syncPulse1
	block = append(block, u16(450)...)  // syncPulse2
	block = append(block, u16(500)...)  // dataPulse0
	block = append(block, u16(1000)...) // dataPulse1
	block = append(block, u16(1500)...) // pilotLength
	block = append(block, 8)            // bitsInLastByte
	block = append(block, u16(0)...)    // pause
	n := len(raw)
	block = append(block, byte(n), byte(n>>8), byte(n>>16))
	block = append(block, raw...)
	image = append(image, block...)

	gotCode, gotOrg, err := FromTZX(image, TapeOptions{Options: Options{Org: u16Ptr(0x9000)}, BlockIndex: intPtr(0)})
	if err != nil {
		t.Fatalf("FromTZX on a 0x11 block: %v", err)
	}
	if gotOrg != 0x9000 || string(gotCode) != string(code) {
		t.Errorf("got org=%#04x code=% X, want org=0x9000 code=% X", gotOrg, gotCode, code)
	}
}
