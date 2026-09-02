package load

import (
	"errors"
	"testing"

	"github.com/ha1tch/zentools/pkg/snapshot"
)

// state48K builds a minimal, valid 48K MachineState for test fixtures,
// with a distinctive byte pattern at a known address so extraction can
// be checked precisely.
func state48K(pc uint16) *snapshot.MachineState {
	s := &snapshot.MachineState{Model: snapshot.Model48K}
	s.CPU.PC = pc
	s.CPU.SP = 0xFF00
	// A recognizable pattern at 0xC000 (bank 0, the region FromSnapshot48K
	// will read from if org lands there).
	copy(s.Memory.RAM[0][:], []byte{0x11, 0x22, 0x33, 0x44, 0x55})
	return s
}

func TestFromSNA48K_Basic(t *testing.T) {
	s := state48K(0xC000)
	image, err := snapshot.EncodeSNA(s)
	if err != nil {
		t.Fatalf("EncodeSNA: %v", err)
	}

	code, org, err := FromSNA48K(image, SnapshotOptions{})
	if err != nil {
		t.Fatalf("FromSNA48K: %v", err)
	}
	if org != 0xC000 {
		t.Errorf("org = %#04x, want 0xC000 (the snapshot's own resolved PC)", org)
	}
	if len(code) < 5 || string(code[:5]) != "\x11\x22\x33\x44\x55" {
		t.Errorf("code[:5] = % X, want 11 22 33 44 55", code[:5])
	}
	if len(code) != 0x10000-0xC000 {
		t.Errorf("len(code) = %d, want %d (default: everything from org to the top of the 64K space)", len(code), 0x10000-0xC000)
	}
}

func TestFromSNA48K_OrgOverride(t *testing.T) {
	s := state48K(0xC000) // snapshot's own PC is 0xC000
	image, _ := snapshot.EncodeSNA(s)

	_, org, err := FromSNA48K(image, SnapshotOptions{Options: Options{Org: u16Ptr(0x8000)}})
	if err != nil {
		t.Fatalf("FromSNA48K: %v", err)
	}
	if org != 0x8000 {
		t.Errorf("org = %#04x, want 0x8000 (explicit override, not the snapshot's own PC 0xC000)", org)
	}
}

func TestFromSNA48K_LengthOverride(t *testing.T) {
	s := state48K(0xC000)
	image, _ := snapshot.EncodeSNA(s)

	code, _, err := FromSNA48K(image, SnapshotOptions{Options: Options{Length: intPtr(3)}})
	if err != nil {
		t.Fatalf("FromSNA48K: %v", err)
	}
	if len(code) != 3 || string(code) != "\x11\x22\x33" {
		t.Errorf("code = % X, want the first 3 bytes 11 22 33", code)
	}
}

func TestFromZ80_V1(t *testing.T) {
	s := state48K(0x8500)
	image, err := snapshot.EncodeZ80(s) // v1
	if err != nil {
		t.Fatalf("EncodeZ80: %v", err)
	}

	_, org, err := FromZ80(image, SnapshotOptions{})
	if err != nil {
		t.Fatalf("FromZ80 (v1): %v", err)
	}
	if org != 0x8500 {
		t.Errorf("org = %#04x, want 0x8500", org)
	}
}

func TestFromZ80_V3(t *testing.T) {
	// Confirmed directly from zentools's own source before relying on
	// it: DecodeZ80's doc comment says v2/v3 "is not yet supported",
	// but decodeZ80Extended is fully implemented and DecodeZ80 already
	// dispatches to it whenever the v1 header's PC field is zero -- the
	// comment is stale, not the behaviour. This test is exactly that
	// claim, checked directly rather than trusted from a comment.
	s := state48K(0x9200)
	image, err := snapshot.EncodeZ80v3(s)
	if err != nil {
		t.Fatalf("EncodeZ80v3: %v", err)
	}

	_, org, err := FromZ80(image, SnapshotOptions{})
	if err != nil {
		t.Fatalf("FromZ80 (v3): %v", err)
	}
	if org != 0x9200 {
		t.Errorf("org = %#04x, want 0x9200", org)
	}
}

func TestFromSnapshot48K_TriesBothFormats(t *testing.T) {
	sSNA := state48K(0x8000)
	snaImage, _ := snapshot.EncodeSNA(sSNA)
	if _, org, err := FromSnapshot48K(snaImage, SnapshotOptions{}); err != nil || org != 0x8000 {
		t.Errorf("SNA via FromSnapshot48K: org=%#04x err=%v, want org=0x8000 err=nil", org, err)
	}

	sZ80 := state48K(0x8100)
	z80Image, _ := snapshot.EncodeZ80v3(sZ80)
	if _, org, err := FromSnapshot48K(z80Image, SnapshotOptions{}); err != nil || org != 0x8100 {
		t.Errorf("Z80 via FromSnapshot48K: org=%#04x err=%v, want org=0x8100 err=nil", org, err)
	}
}

func TestFromSnapshot48K_128KRejected(t *testing.T) {
	s128 := &snapshot.MachineState{Model: snapshot.Model128K}
	s128.CPU.PC = 0x8000
	s128.Paging.Port7FFD = 0x03

	// Via .sna (exact-size detection path).
	snaImage, err := snapshot.EncodeSNA128(s128)
	if err != nil {
		t.Fatalf("EncodeSNA128: %v", err)
	}
	if _, _, err := FromSnapshot48K(snaImage, SnapshotOptions{}); !errors.Is(err, ErrSnapshot128K) {
		t.Errorf("128K .sna: err = %v, want ErrSnapshot128K", err)
	}

	// Via .z80 v3 (Is128KFamily() check inside snapshot48KToCode).
	z80Image, err := snapshot.EncodeZ80v3(s128)
	if err != nil {
		t.Fatalf("EncodeZ80v3 (128K): %v", err)
	}
	if _, _, err := FromSnapshot48K(z80Image, SnapshotOptions{}); !errors.Is(err, ErrSnapshot128K) {
		t.Errorf("128K .z80: err = %v, want ErrSnapshot128K", err)
	}
}

func TestFromSnapshot48K_NeitherFormat(t *testing.T) {
	garbage := []byte("this is not a snapshot of any kind")
	_, _, err := FromSnapshot48K(garbage, SnapshotOptions{})
	if err == nil {
		t.Fatal("expected an error for unrecognized input, got nil")
	}
	if errors.Is(err, ErrSnapshot128K) {
		t.Error("garbage input should not be classified as a 128K snapshot")
	}
}
