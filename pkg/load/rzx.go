package load

import (
	"fmt"
	"strings"

	"github.com/ha1tch/zentools/pkg/rzx"
	"github.com/ha1tch/zentools/pkg/snapshot"
	"github.com/ha1tch/zentools/pkg/szx"
)

// rzxSignature is the 4-byte magic RZX files open with.
const rzxSignature = "RZX!"

// FromRZX extracts code from the first embedded snapshot in an RZX
// input-recording file, applying the same snapshot loading (48K or
// 128K-family, Bank/Org/Length via SnapshotOptions) as FromSnapshot
// already gives a standalone .sna/.z80/.szx file.
//
// An RZX's own defining content -- the frame-by-frame log of what
// every CPU IN instruction returned during an actual recorded session
// -- is deliberately not this function's concern at all: this package
// exists to get code in front of a disassembler, and the code a
// recording's own embedded snapshot captures is exactly as
// disassemblable as if it had been extracted from that snapshot
// directly. A multiload recording can carry more than one embedded
// Snapshot block (a fresh one marking each tape-reload point); the
// first is used, the same "first plausible entry point" convention
// FromTape's own default-to-first-Code-block already follows.
//
// An External snapshot block (a filename reference rather than
// embedded bytes -- see the sibling zentools project's own
// SnapshotBlock.External doc) has nothing to decode here and is
// rejected with a clear error rather than silently producing an
// empty result.
func FromRZX(image []byte, opts SnapshotOptions) (code []byte, org uint16, err error) {
	f, err := rzx.Decode(image)
	if err != nil {
		return nil, 0, fmt.Errorf("load: decoding RZX: %w", err)
	}
	for _, block := range f.Blocks {
		sb, ok := block.(rzx.SnapshotBlock)
		if !ok {
			continue
		}
		if sb.External {
			return nil, 0, fmt.Errorf("load: this RZX's snapshot block is external (a filename reference, not embedded data) -- nothing to load")
		}

		var s *snapshot.MachineState
		switch strings.ToLower(sb.Extension) {
		case "sna":
			if len(sb.Data) == sna128ExactLen {
				s, err = snapshot.DecodeSNA128(sb.Data)
			} else {
				s, err = snapshot.DecodeSNA(sb.Data)
			}
		case "z80":
			s, err = snapshot.DecodeZ80(sb.Data)
		case "szx":
			s, err = szx.Decode(sb.Data)
		default:
			return nil, 0, fmt.Errorf("load: embedded snapshot has extension %q, not a format this package decodes", sb.Extension)
		}
		if err != nil {
			return nil, 0, fmt.Errorf("load: decoding RZX's embedded %s snapshot: %w", strings.ToUpper(sb.Extension), err)
		}

		if s.Model.Is128KFamily() {
			return snapshot128ToCode(s, opts)
		}
		return snapshot48KToCode(s, opts)
	}
	return nil, 0, fmt.Errorf("load: this RZX has no embedded snapshot block")
}
