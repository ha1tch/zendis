package load

import (
	"testing"

	"github.com/ha1tch/zentools/pkg/pzx"
	"github.com/ha1tch/zentools/pkg/tap"
)

// rawTAPBlockBytes returns the raw flag+payload+checksum bytes for a
// tap.Block, the shape a PZX DataBlock's own payload needs to be.
func rawTAPBlockBytes(b tap.Block) []byte {
	raw := make([]byte, 0, len(b.Data)+2)
	raw = append(raw, b.Flag)
	raw = append(raw, b.Data...)
	raw = append(raw, b.Checksum)
	return raw
}

// pzxFromTAPBlocks builds a real PZX file from a slice of raw TAP
// blocks, using standard Spectrum ROM timing -- mirroring
// encodePZXFromTAPBlocks in the sibling zentools project's own zx pzx
// make command, since pkg/pzx itself has no such convenience and this
// package's tests shouldn't need to duplicate that logic beyond what's
// needed for a fixture.
func pzxFromTAPBlocks(t *testing.T, blocks []tap.Block) []byte {
	t.Helper()
	f := &pzx.File{Blocks: []pzx.Block{pzx.HeaderBlock{Major: 1, Minor: 0}}}
	for _, b := range blocks {
		raw := rawTAPBlockBytes(b)
		f.Blocks = append(f.Blocks,
			pzx.PulseBlock{Pulses: []pzx.Pulse{
				{Count: 3223, Duration: 2168},
				{Count: 1, Duration: 667},
				{Count: 1, Duration: 735},
			}},
			pzx.DataBlock{
				InitialLevelHigh: true,
				BitCount:         uint32(len(raw)) * 8,
				Tail:             945,
				S0:               []uint16{855, 855},
				S1:               []uint16{1710, 1710},
				Data:             raw,
			},
		)
	}
	img, err := pzx.Encode(f)
	if err != nil {
		t.Fatalf("pzx.Encode: %v", err)
	}
	return img
}

func TestFromPZX_Basic(t *testing.T) {
	code := []byte{0x3E, 0x2A, 0xC9}
	tapImage := tap.EncodeCode("TESTCODE", code, 0x8000)
	blocks, err := tap.Decode(tapImage)
	if err != nil {
		t.Fatal(err)
	}
	pzxImage := pzxFromTAPBlocks(t, blocks)

	gotCode, gotOrg, err := FromPZX(pzxImage, TapeOptions{})
	if err != nil {
		t.Fatalf("FromPZX: %v", err)
	}
	if gotOrg != 0x8000 {
		t.Errorf("org = %#04x, want 0x8000", gotOrg)
	}
	if string(gotCode) != string(code) {
		t.Errorf("code = % X, want % X", gotCode, code)
	}
}

func TestFromPZX_BlockSelection(t *testing.T) {
	tapImage := tap.EncodeCode("LOADER", []byte{1, 2}, 0x8000)
	tapImage2 := tap.EncodeCode("MAIN", []byte{3, 4, 5}, 0x9000)
	blocks1, _ := tap.Decode(tapImage)
	blocks2, _ := tap.Decode(tapImage2)
	all := append(append([]tap.Block{}, blocks1...), blocks2...)
	pzxImage := pzxFromTAPBlocks(t, all)

	code, org, err := FromPZX(pzxImage, TapeOptions{BlockName: "MAIN"})
	if err != nil {
		t.Fatalf("FromPZX by name: %v", err)
	}
	if org != 0x9000 || string(code) != "\x03\x04\x05" {
		t.Errorf("got org=%#04x code=% X, want org=0x9000 code=03 04 05", org, code)
	}
}

func TestFromPZX_NonStandardTiming(t *testing.T) {
	// The core value proposition: extraction must not depend on the
	// source using standard ROM timing. A real, well-formed turbo-loader
	// capture is exactly as extractable as a standard-speed one, since
	// PZX's own decoder already resolves DATA block bits into real bytes
	// regardless of what pulse durations represented them physically.
	code := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	tapImage := tap.EncodeCode("TURBO", code, 0x9000)
	blocks, _ := tap.Decode(tapImage)
	raw := rawTAPBlockBytes(blocks[1]) // the data block

	f := &pzx.File{Blocks: []pzx.Block{
		pzx.HeaderBlock{Major: 1, Minor: 0},
		pzx.PulseBlock{Pulses: []pzx.Pulse{
			{Count: 2000, Duration: 800}, // deliberately non-standard
			{Count: 1, Duration: 300},
			{Count: 1, Duration: 350},
		}},
		pzx.DataBlock{
			InitialLevelHigh: true,
			BitCount:         uint32(len(raw)) * 8,
			Tail:             400,
			S0:               []uint16{300, 300},
			S1:               []uint16{600, 600},
			Data:             raw,
		},
	}}
	img, err := pzx.Encode(f)
	if err != nil {
		t.Fatal(err)
	}

	gotCode, _, err := FromPZX(img, TapeOptions{Options: Options{Org: u16Ptr(0x9000)}, BlockIndex: intPtr(0)})
	if err != nil {
		t.Fatalf("FromPZX on non-standard timing: %v", err)
	}
	if string(gotCode) != string(code) {
		t.Errorf("code = % X, want % X", gotCode, code)
	}
}

func TestFromPZX_NoDataBlocks(t *testing.T) {
	f := &pzx.File{Blocks: []pzx.Block{pzx.HeaderBlock{Major: 1, Minor: 0}}}
	img, err := pzx.Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = FromPZX(img, TapeOptions{})
	if err == nil {
		t.Fatal("expected an error for a PZX with no DATA blocks, got nil")
	}
}

func TestFromTape_DetectsPZX(t *testing.T) {
	code := []byte{0x11, 0x22}
	tapImage := tap.EncodeCode("X", code, 0x8000)
	blocks, _ := tap.Decode(tapImage)
	pzxImage := pzxFromTAPBlocks(t, blocks)

	gotCode, gotOrg, err := FromTape(pzxImage, TapeOptions{})
	if err != nil {
		t.Fatalf("FromTape on a PZX image: %v", err)
	}
	if gotOrg != 0x8000 || string(gotCode) != string(code) {
		t.Errorf("got org=%#04x code=% X, want org=0x8000 code=% X", gotOrg, gotCode, code)
	}
}
