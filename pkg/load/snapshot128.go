package load

import (
	"errors"
	"fmt"

	"github.com/ha1tch/zentools/pkg/snapshot"
)

// ErrSpecialPagingMode is returned when a snapshot has the +3/+2A's
// special all-RAM paging mode active (port 0x1FFD bit 0 set). In that
// mode all four 16K address regions hold RAM in one of four fixed
// configurations (confirmed against real +3 hardware documentation --
// bits 1-2 of 0x1FFD select which; source: Sinclair Wiki's Memory
// paging and ZX Spectrum +3 pages, and the MrKWatkins/ZXSpectrumNextTests
// port reference, all independently describing the same bit 0
// normal/special switch), not the fixed-bank-5/bank-2-plus-one-paged-
// bank layout every other 128K/+2/+2A/+3 mode uses. zentools carries
// Port1FFD as a raw, undecoded byte (confirmed by reading its source:
// the field exists in state.go, but nothing anywhere in the package
// interprets its bits), so there is no foundation here to build a
// correct reconstruction of the special-mode layout on top of --
// refusing cleanly is safer than guessing at a memory map this
// package cannot verify.
var ErrSpecialPagingMode = errors.New("load: snapshot has the +3's special all-RAM paging mode active (port 0x1FFD bit 0 set); not supported")

// sna128ExactLen is defined in snapshot.go and reused here.

// Historical note, no longer an open issue: zentools's own
// EncodeSNA128, as of v0.6.0, used to silently produce a malformed,
// oversized image whenever Paging.Port7FFD's selected bank equalled 2
// or 5 -- a genuine, real hardware aliasing scenario (nothing prevents
// the 128K paging port from selecting either always-fixed bank for the
// 0xC000 slot too). Verified fixed in the currently-depended-on
// zentools v0.8.6, by direct testing against the real published
// module, not assumed: EncodeSNA128 now refuses this exact case
// outright with a clear error rather than producing bad output, since
// the 128K .sna format genuinely has no lossless way to represent it
// -- a real format limitation, not something to work around here.
// DecodeSNA128 itself, decoding a real, correctly-formed 128K .sna
// produced by an actual emulator, was never affected either way.

// snapshot128ToCode extracts a 128K-family MachineState's memory as a
// flat, contiguous buffer starting at org, honoring opts.Bank and
// opts.Length.
//
// Bank 5 and bank 2 are always mapped at 0x4000 and 0x8000
// respectively in every 128K/+2/+2A/+3 *normal* paging mode -- this is
// not model-specific, confirmed across every source consulted for
// ErrSpecialPagingMode's own doc comment. Only the top quarter,
// 0xC000-0xFFFF, varies: opts.Bank (0-7) if given, otherwise the
// snapshot's own Port7FFD-selected bank (bits 0-2).
//
// Org defaults to the snapshot's own CPU.PC when opts.Bank is nil
// (matching FromSnapshot48K's convention exactly) -- but PC may not
// even fall within whichever bank opts.Bank names, so when Bank is
// given org instead defaults to 0xC000 (the start of that bank's only
// possible window) unless opts.Org overrides either default. An org
// below 0x4000 is always rejected (ROM, never captured in a
// snapshot); an org below 0xC000 is rejected specifically when Bank is
// given, since a bank can only ever appear in the address space at
// 0xC000-0xFFFF -- addressing it anywhere else would not correspond to
// anything a real machine could ever show.
func snapshot128ToCode(s *snapshot.MachineState, opts SnapshotOptions) (code []byte, org uint16, err error) {
	if !s.Model.Is128KFamily() {
		return nil, 0, fmt.Errorf("load: snapshot is not 128K-family (its Model is 48K) -- use FromSnapshot48K or plain FromSnapshot instead")
	}
	if s.Paging.Port1FFD&0x01 != 0 {
		return nil, 0, ErrSpecialPagingMode
	}

	pagedBank := int(s.Paging.Port7FFD & 0x07)
	if opts.Bank != nil {
		if *opts.Bank < 0 || *opts.Bank > 7 {
			return nil, 0, fmt.Errorf("load: Bank must be 0-7, got %d", *opts.Bank)
		}
		pagedBank = *opts.Bank
	}

	switch {
	case opts.Org != nil:
		org = *opts.Org
	case opts.Bank != nil:
		org = 0xC000
	default:
		org = s.CPU.PC
	}

	if org < 0x4000 {
		return nil, 0, fmt.Errorf("load: org %#04x is in ROM (0x0000-0x3FFF), which is never captured in a snapshot", org)
	}
	if opts.Bank != nil && org < 0xC000 {
		return nil, 0, fmt.Errorf("load: org %#04x is outside 0xC000-0xFFFF, the only range an explicitly selected Bank can occupy", org)
	}

	var flat [0x10000]byte
	copy(flat[0x4000:0x8000], s.Memory.RAM[5][:])
	copy(flat[0x8000:0xC000], s.Memory.RAM[2][:])
	copy(flat[0xC000:0x10000], s.Memory.RAM[pagedBank][:])

	available := 0x10000 - int(org)
	n, err := opts.clampLength(available)
	if err != nil {
		return nil, 0, err
	}
	return flat[int(org) : int(org)+n], org, nil
}

// FromSNA128 loads a 128K-family .sna snapshot.
func FromSNA128(image []byte, opts SnapshotOptions) (code []byte, org uint16, err error) {
	s, err := snapshot.DecodeSNA128(image)
	if err != nil {
		return nil, 0, fmt.Errorf("load: decoding SNA128: %w", err)
	}
	return snapshot128ToCode(s, opts)
}

// FromZ80128 loads a 128K-family .z80 snapshot. Only v2/v3 (the
// extended-header form) ever describes a 128K machine -- v1's header
// has no hardware-mode field at all, so a v1 .z80 is always 48K; use
// FromZ80 for that case.
func FromZ80128(image []byte, opts SnapshotOptions) (code []byte, org uint16, err error) {
	s, err := snapshot.DecodeZ80(image)
	if err != nil {
		return nil, 0, fmt.Errorf("load: decoding Z80: %w", err)
	}
	return snapshot128ToCode(s, opts)
}

// FromSnapshot128K loads a 128K-family snapshot in either .sna or
// .z80 format, detected the same way FromSnapshot48K detects its own
// pair: DecodeSNA128's exact-length check (131103 bytes) makes it a
// safe first try, falling through to DecodeZ80 -- which, unlike the
// 48K case, needs no separate "is this secretly the other size"
// special-case, since DecodeZ80 itself reports the model and
// snapshot128ToCode already rejects a 48K result with a clear error.
func FromSnapshot128K(image []byte, opts SnapshotOptions) (code []byte, org uint16, err error) {
	code, org, snaErr := FromSNA128(image, opts)
	if snaErr == nil {
		return code, org, nil
	}
	if errors.Is(snaErr, ErrSpecialPagingMode) {
		return nil, 0, snaErr // a real diagnosis, not a format mismatch -- do not mask it by falling through
	}
	code, org, z80Err := FromZ80128(image, opts)
	if z80Err == nil {
		return code, org, nil
	}
	if errors.Is(z80Err, ErrSpecialPagingMode) {
		return nil, 0, z80Err
	}
	return nil, 0, fmt.Errorf("load: not a recognized 128K-family snapshot -- as SNA128: %v; as Z80: %v", snaErr, z80Err)
}

// FromSnapshot loads a snapshot in .sna, .z80, or .szx format, 48K or
// 128K-family, entirely auto-detected from the image's own bytes and
// decoded Model -- a caller does not need to already know which shape
// their snapshot has, unlike FromSnapshot48K/FromSnapshot128K, which
// require picking the right one up front. This is what --snapshot
// uses in the CLI.
func FromSnapshot(image []byte, opts SnapshotOptions) (code []byte, org uint16, err error) {
	if len(image) >= len(szxSignature) && string(image[:len(szxSignature)]) == szxSignature {
		return FromSZX(image, opts)
	}
	if len(image) == sna128ExactLen {
		return FromSNA128(image, opts)
	}
	if code, org, err := FromSNA48K(image, opts); err == nil {
		return code, org, nil
	}
	// Not a recognized .sna of either size: try .z80, which reports its
	// own Model regardless of 48K or 128K, then dispatch on that.
	s, z80Err := snapshot.DecodeZ80(image)
	if z80Err != nil {
		return nil, 0, fmt.Errorf("load: not a recognized snapshot -- as SNA: wrong size; as SZX: no ZXST signature; as Z80: %v", z80Err)
	}
	if s.Model.Is128KFamily() {
		return snapshot128ToCode(s, opts)
	}
	return snapshot48KToCode(s, opts)
}
