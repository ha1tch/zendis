package load

import (
	"errors"
	"fmt"

	"github.com/ha1tch/zentools/pkg/snapshot"
)

// sna128ExactLen is the exact byte length of a 128K .sna image (27-byte
// header + 3*16384 window banks + 4-byte trailer + 5*16384 remaining
// banks -- zentools's own sna128Len, unexported there, recomputed here
// so a 128K .sna can be recognized and given a clear error before
// falling through to a confusing Z80-decode failure).
const sna128ExactLen = 27 + 8*16384 + 4

// SnapshotOptions controls snapshot loading: the org/length overrides
// shared with tape loading (Options), plus Bank for a 128K-family
// snapshot.
type SnapshotOptions struct {
	Options

	// Bank, if non-nil, selects a specific RAM bank (0-7) to appear at
	// 0xC000-0xFFFF for a 128K-family snapshot, overriding its own
	// Port7FFD-selected bank -- for inspecting a bank that is not
	// currently paged in. Meaningless for a 48K snapshot, which has no
	// bank concept at all; FromSNA48K/FromZ80/FromSnapshot48K never
	// look at it.
	Bank *int
}

// ErrSnapshot128K is returned by every loader in this file when the
// decoded snapshot targets a 128K-family model. 128K paged-memory
// loading is a deliberately separate, harder problem -- see the
// project's own register item T-12/Wave 7 -- not an oversight here.
var ErrSnapshot128K = errors.New("load: snapshot targets a 128K-family model; 128K paged-memory loading isn't implemented yet (see T-12)")

// snapshot48KToCode extracts a 48K MachineState's memory as a flat,
// contiguous buffer starting at org (the snapshot's own resolved PC,
// unless opts.Org overrides it), honoring opts.Length.
func snapshot48KToCode(s *snapshot.MachineState, opts SnapshotOptions) (code []byte, org uint16, err error) {
	if s.Model.Is128KFamily() {
		return nil, 0, ErrSnapshot128K
	}

	org = s.CPU.PC
	if opts.Org != nil {
		org = *opts.Org
	}

	// Reconstruct the flat 64K view: banks 5, 2, 0 at 0x4000, 0x8000,
	// 0xC000 respectively -- 48K's fixed mapping (zentools's own
	// state.go: "48K memory is banks 5 (0x4000-0x7FFF), 2
	// (0x8000-0xBFFF) and 0 (0xC000-0xFFFF)"), never needing Paging.
	var flat [0x10000]byte
	copy(flat[0x4000:0x8000], s.Memory.RAM[5][:])
	copy(flat[0x8000:0xC000], s.Memory.RAM[2][:])
	copy(flat[0xC000:0x10000], s.Memory.RAM[0][:])

	available := 0x10000 - int(org)
	if available < 0 {
		available = 0
	}
	n, err := opts.clampLength(available)
	if err != nil {
		return nil, 0, err
	}
	return flat[int(org) : int(org)+n], org, nil
}

// FromSNA48K loads a 48K .sna snapshot.
func FromSNA48K(image []byte, opts SnapshotOptions) (code []byte, org uint16, err error) {
	s, err := snapshot.DecodeSNA(image)
	if err != nil {
		return nil, 0, fmt.Errorf("load: decoding SNA: %w", err)
	}
	return snapshot48KToCode(s, opts)
}

// FromZ80 loads a .z80 snapshot -- v1, v2, and v3 are all handled by
// zentools's own DecodeZ80 transparently (v2/v3 detected via the v1
// header's PC field being zero, dispatching to the extended-header
// path, which covers 128K paging -- exactly why the ErrSnapshot128K
// check in snapshot48KToCode matters here and not just for
// FromSNA48K).
func FromZ80(image []byte, opts SnapshotOptions) (code []byte, org uint16, err error) {
	s, err := snapshot.DecodeZ80(image)
	if err != nil {
		return nil, 0, fmt.Errorf("load: decoding Z80: %w", err)
	}
	return snapshot48KToCode(s, opts)
}

// FromSnapshot48K loads a 48K snapshot in either .sna or .z80 format,
// detected by trying each decoder in turn rather than a file
// extension or a magic byte signature -- neither .sna nor .z80 has one
// the way TZX does ("ZXTape!"): .sna is a fixed-size raw register
// dump, and .z80's own "signature" is really just its first field
// (register A) with no reserved magic value. DecodeSNA's own strict
// length check (exactly 49179 bytes for 48K) makes it a safe first
// try -- it rejects anything the wrong size immediately rather than
// misreading it, so falling through to DecodeZ80 on that failure is
// safe. A 128K .sna (exactly 131103 bytes) is checked for explicitly
// first, since DecodeSNA would otherwise reject it as "not a 48K
// snapshot" only for the fallback to then fail confusingly against
// DecodeZ80 too -- ErrSnapshot128K is a much clearer answer.
func FromSnapshot48K(image []byte, opts SnapshotOptions) (code []byte, org uint16, err error) {
	if len(image) == sna128ExactLen {
		return nil, 0, ErrSnapshot128K
	}
	code, org, snaErr := FromSNA48K(image, opts)
	if snaErr == nil {
		return code, org, nil
	}
	if errors.Is(snaErr, ErrSnapshot128K) {
		return nil, 0, snaErr // a real 128K result, not a format mismatch -- don't mask it by falling through
	}
	code, org, z80Err := FromZ80(image, opts)
	if z80Err == nil {
		return code, org, nil
	}
	if errors.Is(z80Err, ErrSnapshot128K) {
		return nil, 0, z80Err
	}
	return nil, 0, fmt.Errorf("load: not a recognized 48K snapshot -- as SNA: %v; as Z80: %v", snaErr, z80Err)
}
