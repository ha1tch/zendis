package load

import (
	"errors"
	"testing"

	"github.com/ha1tch/zentools/pkg/snapshot"
	"github.com/ha1tch/zentools/pkg/szx"
)

func TestFromSZX_48K(t *testing.T) {
	// state48K's marker lives in bank 0, which maps to 0xC000-0xFFFF --
	// PC must land there to read it back, not 0x8000 (bank 2's region).
	// Caught by running this test and getting 0x00 back, not by
	// re-reading state48K's own doc comment closely enough the first
	// time.
	s := state48K(0xC000)
	img, err := szx.Encode(s)
	if err != nil {
		t.Fatal(err)
	}

	code, org, err := FromSZX(img, SnapshotOptions{})
	if err != nil {
		t.Fatalf("FromSZX: %v", err)
	}
	if org != 0xC000 {
		t.Errorf("org = %#04x, want 0xC000", org)
	}
	if code[0] != 0x11 {
		t.Errorf("code[0] = %#02x, want 0x11", code[0])
	}
}

func TestFromSZX_128KCurrentView(t *testing.T) {
	s := state128K(0xC000, 0x03) // bank 3 paged
	img, err := szx.Encode(s)
	if err != nil {
		t.Fatal(err)
	}

	code, org, err := FromSZX(img, SnapshotOptions{})
	if err != nil {
		t.Fatalf("FromSZX: %v", err)
	}
	if org != 0xC000 {
		t.Errorf("org = %#04x, want 0xC000", org)
	}
	if code[0] != 0xB3 {
		t.Errorf("code[0] = %#02x, want 0xB3 (bank 3, the paged bank)", code[0])
	}
}

func TestFromSZX_128KExplicitBank(t *testing.T) {
	s := state128K(0xC000, 0x02) // bank 2 currently paged
	img, err := szx.Encode(s)
	if err != nil {
		t.Fatal(err)
	}

	bank := 6
	code, org, err := FromSZX(img, SnapshotOptions{Bank: &bank})
	if err != nil {
		t.Fatalf("FromSZX with Bank=6: %v", err)
	}
	if org != 0xC000 {
		t.Errorf("org = %#04x, want 0xC000 (Bank given, no explicit Org)", org)
	}
	if code[0] != 0xB6 {
		t.Errorf("code[0] = %#02x, want 0xB6 (the explicitly selected bank, not the paged one)", code[0])
	}
}

func TestFromSZX_SpecialPagingModeRejected(t *testing.T) {
	// zentools's own szx.Decode doesn't reject this itself (confirmed by
	// reading it directly: it stores Port1FFD's raw byte, no
	// interpretation) -- the rejection has to come from this package's
	// own snapshot128ToCode, the same ErrSpecialPagingMode check a
	// Z80/SNA source in that state already gets. Verified here, not
	// assumed from the szx.go doc comment alone.
	s := state128K(0xC000, 0x00)
	s.Paging.Port1FFD = 0x01 // special mode bit set
	img, err := szx.Encode(s)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = FromSZX(img, SnapshotOptions{})
	if !errors.Is(err, ErrSpecialPagingMode) {
		t.Errorf("err = %v, want ErrSpecialPagingMode", err)
	}
}

func TestFromSZX_InvalidImage(t *testing.T) {
	_, _, err := FromSZX([]byte("not an szx file"), SnapshotOptions{})
	if err == nil {
		t.Fatal("expected an error for a non-SZX image, got nil")
	}
}

func TestFromSnapshot_DetectsSZX(t *testing.T) {
	cases := map[string]*snapshot.MachineState{
		"48K":  state48K(0x9000),
		"128K": state128K(0xC000, 0x04),
	}
	for name, s := range cases {
		t.Run(name, func(t *testing.T) {
			img, err := szx.Encode(s)
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = FromSnapshot(img, SnapshotOptions{})
			if err != nil {
				t.Fatalf("FromSnapshot on an SZX image: %v", err)
			}
		})
	}
}
