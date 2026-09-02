package load

import (
	"testing"

	"github.com/ha1tch/zentools/pkg/rzx"
	"github.com/ha1tch/zentools/pkg/snapshot"
	"github.com/ha1tch/zentools/pkg/szx"
)

func rzxWithSnapshot(t *testing.T, ext string, data []byte) []byte {
	t.Helper()
	f := &rzx.File{
		MajorVersion: 0, MinorVersion: 13,
		Blocks: []rzx.Block{
			rzx.CreatorBlock{ID: "test", Major: 1, Minor: 0},
			rzx.SnapshotBlock{Extension: ext, Data: data},
		},
	}
	img, err := rzx.Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestFromRZX_EmbeddedSNA(t *testing.T) {
	s := state48K(0xC000)
	data, err := snapshot.EncodeSNA(s)
	if err != nil {
		t.Fatal(err)
	}
	img := rzxWithSnapshot(t, "SNA", data)

	code, org, err := FromRZX(img, SnapshotOptions{})
	if err != nil {
		t.Fatalf("FromRZX: %v", err)
	}
	if org != 0xC000 || code[0] != 0x11 {
		t.Errorf("got org=%#04x code[0]=%#02x, want org=0xC000 code[0]=0x11", org, code[0])
	}
}

func TestFromRZX_EmbeddedZ80(t *testing.T) {
	s := state48K(0xC000)
	data, err := snapshot.EncodeZ80(s)
	if err != nil {
		t.Fatal(err)
	}
	img := rzxWithSnapshot(t, "Z80", data)

	code, org, err := FromRZX(img, SnapshotOptions{})
	if err != nil {
		t.Fatalf("FromRZX: %v", err)
	}
	if org != 0xC000 || code[0] != 0x11 {
		t.Errorf("got org=%#04x code[0]=%#02x, want org=0xC000 code[0]=0x11", org, code[0])
	}
}

func TestFromRZX_EmbeddedSZX128K(t *testing.T) {
	s := state128K(0xC000, 0x05)
	data, err := szx.Encode(s)
	if err != nil {
		t.Fatal(err)
	}
	// Lowercase extension, deliberately -- extension matching must be
	// case-insensitive, the same as the sibling zentools project's own
	// convertRZXToSnap.
	img := rzxWithSnapshot(t, "szx", data)

	code, org, err := FromRZX(img, SnapshotOptions{})
	if err != nil {
		t.Fatalf("FromRZX: %v", err)
	}
	if org != 0xC000 || code[0] != 0xB5 {
		t.Errorf("got org=%#04x code[0]=%#02x, want org=0xC000 code[0]=0xB5 (bank 5, the paged bank)", org, code[0])
	}
}

func TestFromRZX_128KBankSelection(t *testing.T) {
	s := state128K(0xC000, 0x02)
	data, err := szx.Encode(s)
	if err != nil {
		t.Fatal(err)
	}
	img := rzxWithSnapshot(t, "SZX", data)

	bank := 7
	code, org, err := FromRZX(img, SnapshotOptions{Bank: &bank})
	if err != nil {
		t.Fatalf("FromRZX with Bank=7: %v", err)
	}
	if org != 0xC000 || code[0] != 0xB7 {
		t.Errorf("got org=%#04x code[0]=%#02x, want org=0xC000 code[0]=0xB7 (explicitly selected bank)", org, code[0])
	}
}

func TestFromRZX_ExternalSnapshotRejected(t *testing.T) {
	f := &rzx.File{Blocks: []rzx.Block{
		rzx.SnapshotBlock{External: true, Extension: "SNA", Data: []byte("some-filename-reference")},
	}}
	img, err := rzx.Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = FromRZX(img, SnapshotOptions{})
	if err == nil {
		t.Fatal("expected an error for an external (filename-reference) snapshot block, got nil")
	}
}

func TestFromRZX_NoSnapshotBlock(t *testing.T) {
	f := &rzx.File{Blocks: []rzx.Block{
		rzx.CreatorBlock{ID: "test", Major: 1, Minor: 0},
		rzx.RecordingBlock{Frames: []rzx.Frame{{FetchCount: 1000, PortReads: []byte{0xFF}}}},
	}}
	img, err := rzx.Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = FromRZX(img, SnapshotOptions{})
	if err == nil {
		t.Fatal("expected an error for an RZX with no embedded snapshot, got nil")
	}
}

func TestFromRZX_UnrecognizedExtension(t *testing.T) {
	img := rzxWithSnapshot(t, "EXE", []byte{1, 2, 3, 4})
	_, _, err := FromRZX(img, SnapshotOptions{})
	if err == nil {
		t.Fatal("expected an error for an unrecognized embedded snapshot extension, got nil")
	}
}

func TestFromRZX_InvalidImage(t *testing.T) {
	_, _, err := FromRZX([]byte("not an rzx file"), SnapshotOptions{})
	if err == nil {
		t.Fatal("expected an error for a non-RZX image, got nil")
	}
}
