package load

import (
	"errors"
	"testing"

	"github.com/ha1tch/zentools/pkg/snapshot"
)

// state128K builds a minimal, valid 128K-family MachineState for test
// fixtures. Each bank gets a distinctive first byte (its own number)
// so which bank actually ended up at 0xC000 can be checked precisely.
func state128K(pc uint16, port7FFD uint8) *snapshot.MachineState {
	s := &snapshot.MachineState{Model: snapshot.Model128K}
	s.CPU.PC = pc
	s.CPU.SP = 0xFF00
	s.Paging.Port7FFD = port7FFD
	for b := 0; b < 8; b++ {
		s.Memory.RAM[b][0] = byte(0xB0 + b) // 0xB0..0xB7, one per bank
	}
	return s
}

func TestSnapshot128_CurrentView(t *testing.T) {
	// Port7FFD low 3 bits = 3: bank 3 should end up at 0xC000.
	s := state128K(0xC000, 0x03)
	image, err := snapshot.EncodeSNA128(s)
	if err != nil {
		t.Fatalf("EncodeSNA128: %v", err)
	}

	code, org, err := FromSNA128(image, SnapshotOptions{})
	if err != nil {
		t.Fatalf("FromSNA128: %v", err)
	}
	if org != 0xC000 {
		t.Errorf("org = %#04x, want 0xC000 (the snapshot's own PC)", org)
	}
	if len(code) == 0 || code[0] != 0xB3 {
		t.Errorf("code[0] = %#02x, want 0xB3 (bank 3, selected by Port7FFD)", code[0])
	}
}

func TestSnapshot128_FixedBanks(t *testing.T) {
	// Confirmed against every source consulted for ErrSpecialPagingMode's
	// own doc comment: banks 5 and 2 are always at 0x4000/0x8000 in every
	// 128K/+2/+2A/+3 normal-mode configuration, regardless of Port7FFD.
	s := state128K(0x4000, 0x00)
	image, _ := snapshot.EncodeSNA128(s)

	code, _, err := FromSNA128(image, SnapshotOptions{Options: Options{Org: u16Ptr(0x4000)}})
	if err != nil {
		t.Fatalf("FromSNA128 at 0x4000: %v", err)
	}
	if code[0] != 0xB5 {
		t.Errorf("bank at 0x4000 = %#02x, want 0xB5 (bank 5, always fixed there)", code[0])
	}

	code, _, err = FromSNA128(image, SnapshotOptions{Options: Options{Org: u16Ptr(0x8000)}})
	if err != nil {
		t.Fatalf("FromSNA128 at 0x8000: %v", err)
	}
	if code[0] != 0xB2 {
		t.Errorf("bank at 0x8000 = %#02x, want 0xB2 (bank 2, always fixed there)", code[0])
	}
}

func TestSnapshot128_ExplicitBankOverride(t *testing.T) {
	// Port7FFD selects bank 3 for 0xC000 -- deliberately NOT bank 2 or 5.
	// Confirmed as a real bug in zentools's own EncodeSNA128, isolated and
	// verified directly (not assumed): when the paged bank aliases one of
	// the two always-fixed banks (a genuine, real, valid hardware
	// scenario -- Port7FFD's bank selection is not restricted to avoid
	// 2/5), the encoder writes that bank's content twice (once as the
	// fixed bank, once as "the paged bank") and produces an image exactly
	// one 16K bank too large (147487 bytes instead of 131103), which
	// DecodeSNA128's own strict length check then correctly rejects.
	// Sidestepped here rather than triggering it, since this test's job
	// is exercising --bank override, not zentools's own encoder.
	s := state128K(0xC000, 0x03)
	image, _ := snapshot.EncodeSNA128(s)

	// Without Bank: current view, bank 3 at 0xC000.
	code, _, err := FromSNA128(image, SnapshotOptions{})
	if err != nil {
		t.Fatalf("FromSNA128 current view: %v", err)
	}
	if code[0] != 0xB3 {
		t.Errorf("current view code[0] = %#02x, want 0xB3", code[0])
	}

	// With Bank=6: bank 6 at 0xC000 instead, overriding Port7FFD entirely,
	// and org defaults to 0xC000 (not the snapshot's PC) since PC may not
	// even fall within the explicitly-selected bank's window.
	bank := 6
	code, org, err := FromSNA128(image, SnapshotOptions{Bank: &bank})
	if err != nil {
		t.Fatalf("FromSNA128 with Bank=6: %v", err)
	}
	if org != 0xC000 {
		t.Errorf("org = %#04x, want 0xC000 (Bank given, no explicit Org)", org)
	}
	if code[0] != 0xB6 {
		t.Errorf("code[0] = %#02x, want 0xB6 (bank 6, explicit override)", code[0])
	}
}

func TestSnapshot128_BankRequiresC000Range(t *testing.T) {
	s := state128K(0xC000, 0x00)
	image, _ := snapshot.EncodeSNA128(s)
	bank := 4

	// Org below 0xC000 makes no sense with an explicit Bank: a bank can
	// only ever appear at 0xC000-0xFFFF in the real address space.
	_, _, err := FromSNA128(image, SnapshotOptions{Bank: &bank, Options: Options{Org: u16Ptr(0x8000)}})
	if err == nil {
		t.Fatal("expected an error for Bank + org below 0xC000, got nil")
	}
}

func TestSnapshot128_ROMRejected(t *testing.T) {
	s := state128K(0xC000, 0x00)
	image, _ := snapshot.EncodeSNA128(s)

	_, _, err := FromSNA128(image, SnapshotOptions{Options: Options{Org: u16Ptr(0x1000)}})
	if err == nil {
		t.Fatal("expected an error for org in ROM (below 0x4000), got nil")
	}
}

func TestSnapshot128_BankOutOfRange(t *testing.T) {
	s := state128K(0xC000, 0x00)
	image, _ := snapshot.EncodeSNA128(s)
	bad := 8

	_, _, err := FromSNA128(image, SnapshotOptions{Bank: &bad})
	if err == nil {
		t.Fatal("expected an error for Bank=8 (valid range is 0-7), got nil")
	}
}

func TestSnapshot128_SpecialPagingModeRejected(t *testing.T) {
	// Confirmed directly against real +3 hardware documentation before
	// relying on it (zentools itself carries Port1FFD undecoded): bit 0
	// set means special all-RAM mode is active. Must use .z80, not .sna:
	// confirmed by reading zentools own sna.go, the 128K .sna trailer is
	// PC+Port7FFD+TR-DOS flag only -- there is no Port1FFD field in the
	// format at all, so DecodeSNA128 never sets it and this check could
	// never fire through that path no matter what the real snapshot had.
	s := state128K(0xC000, 0x00)
	s.Paging.Port1FFD = 0x01 // special mode bit set
	image, err := snapshot.EncodeZ80v3(s)
	if err != nil {
		t.Fatalf("EncodeZ80v3: %v", err)
	}

	_, _, err = FromZ80128(image, SnapshotOptions{})
	if !errors.Is(err, ErrSpecialPagingMode) {
		t.Errorf("err = %v, want ErrSpecialPagingMode", err)
	}
}

func TestFromZ80128_V3(t *testing.T) {
	// 0x8000, not 0x9000: the marker byte state128K sets is at offset 0
	// within each bank, and bank 2's fixed window starts at 0x8000 -- an
	// org of 0x9000 would read offset 0x1000 into the bank instead, which
	// is zero, not the marker (an address-arithmetic mistake caught by
	// running this and checking the actual byte, not by inspection).
	s := state128K(0x8000, 0x05)
	image, err := snapshot.EncodeZ80v3(s)
	if err != nil {
		t.Fatalf("EncodeZ80v3: %v", err)
	}

	code, org, err := FromZ80128(image, SnapshotOptions{})
	if err != nil {
		t.Fatalf("FromZ80128: %v", err)
	}
	if org != 0x8000 || code[0] != 0xB2 { // 0x8000 is bank 2's fixed window start
		t.Errorf("got org=%#04x code[0]=%#02x, want org=0x8000 code[0]=0xB2", org, code[0])
	}
}

func TestFromZ80128_RejectsV1(t *testing.T) {
	// A v1 .z80 is always 48K -- there is no 128K v1 variant at all.
	s := state48K(0x8000)
	image, err := snapshot.EncodeZ80(s)
	if err != nil {
		t.Fatalf("EncodeZ80: %v", err)
	}
	_, _, err = FromZ80128(image, SnapshotOptions{})
	if err == nil {
		t.Fatal("expected an error decoding a 48K v1 .z80 via FromZ80128, got nil")
	}
}

func TestFromSnapshot128K_TriesBothFormats(t *testing.T) {
	sSNA := state128K(0xC000, 0x01)
	snaImage, _ := snapshot.EncodeSNA128(sSNA)
	if code, _, err := FromSnapshot128K(snaImage, SnapshotOptions{}); err != nil || code[0] != 0xB1 {
		t.Errorf("SNA128 via FromSnapshot128K: code[0]=%#02x err=%v, want 0xB1 err=nil", code[0], err)
	}

	sZ80 := state128K(0xC000, 0x02)
	z80Image, _ := snapshot.EncodeZ80v3(sZ80)
	if code, _, err := FromSnapshot128K(z80Image, SnapshotOptions{}); err != nil || code[0] != 0xB2 {
		t.Errorf("Z80 via FromSnapshot128K: code[0]=%#02x err=%v, want 0xB2 err=nil", code[0], err)
	}
}

func TestFromSnapshot_AutoDetectsSizeAndModel(t *testing.T) {
	// The unified entry point: 48K SNA, 128K SNA, and 128K Z80 all route
	// correctly with no caller-supplied hint about which shape it is.
	s48, _ := snapshot.EncodeSNA(state48K(0x8000))
	if _, org, err := FromSnapshot(s48, SnapshotOptions{}); err != nil || org != 0x8000 {
		t.Errorf("48K SNA via FromSnapshot: org=%#04x err=%v, want org=0x8000 err=nil", org, err)
	}

	s128sna, _ := snapshot.EncodeSNA128(state128K(0xC000, 0x00))
	if code, _, err := FromSnapshot(s128sna, SnapshotOptions{}); err != nil || code[0] != 0xB0 {
		t.Errorf("128K SNA via FromSnapshot: code[0]=%#02x err=%v, want 0xB0 err=nil", code[0], err)
	}

	s128z80, _ := snapshot.EncodeZ80v3(state128K(0xC000, 0x07))
	if code, _, err := FromSnapshot(s128z80, SnapshotOptions{}); err != nil || code[0] != 0xB7 {
		t.Errorf("128K Z80 via FromSnapshot: code[0]=%#02x err=%v, want 0xB7 err=nil", code[0], err)
	}
}
