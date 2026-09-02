// Command zendis disassembles Z80/Z80N machine code from a binary
// file, a tape image (.tap/.tzx/.pzx), a snapshot (.sna/.z80/.szx), or
// an RZX recording. This is a thin CLI wrapper: flag parsing, file
// reading, and process exit codes live here; the actual pipeline
// (resolving an input source into code+org, then decoding and
// rendering it) is pkg/api's own Disassemble, exported directly for a
// caller embedding zendis as a library rather than shelling out to
// this binary.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/ha1tch/zendis/pkg/api"
	"github.com/ha1tch/zendis/pkg/disasm"
	"github.com/ha1tch/zendis/pkg/render"
)

func main() {
	orgStr := flag.String("org", "", "load address (decimal, or 0x-prefixed hex). Raw-binary mode defaults to 0 if omitted; --tape/--snapshot mode uses the source's own load address/PC if omitted, or overrides it if given")
	targetName := flag.String("target", "z80", "CPU target: z80 (Z80N-only opcodes stay undefined) or z80n")
	warnZ80N := flag.Bool("warn-z80n", true, "under --target=z80, warn on stderr when an undefined opcode matches a Z80N instruction")
	labels := flag.Bool("labels", true, "replace JP/JR/CALL/DJNZ target addresses with generated labels (Lxxxx) where no hints-file name overrides them")
	hintsPath := flag.String("hints", "", "path to a hints file (label/comment/data/entry directives -- see README)")
	format := flag.String("format", "listing", "output format: listing (default), plain/zenas, pasmo, sjasmplus, snasm (all four re-assemblable), or json")
	symPath := flag.String("sym", "", "write a symbol file (format matches --format: pasmo's/zenas's shape for plain/zenas/pasmo, sjasmplus's shape for sjasmplus) -- requires --format to be one of those four; not available for snasm, see README")

	tapePath := flag.String("tape", "", "load code from a tape image (.tap, .tzx, or .pzx) instead of a raw binary file")
	snapshotPath := flag.String("snapshot", "", "load code from a snapshot (.sna, .z80, or .szx, 48K or 128K-family) instead of a raw binary file; see --bank for 128K-family bank selection")
	recordingPath := flag.String("recording", "", "load code from the first embedded snapshot in an RZX input-recording file instead of a raw binary file; see --bank for 128K-family bank selection")
	blockStr := flag.String("block", "", "--tape only: select a block by its 0-based position in the tape's own block list (not a header+data pair count -- some tapes have bare data blocks with no header at all); default is the first Code-type block found")
	blockName := flag.String("block-name", "", "--tape only: select a block by its exact header name, overriding --block")
	lengthStr := flag.String("length", "", "limit disassembly to N bytes from the resolved org (decimal, or 0x-prefixed hex); default is everything the source itself provides (the whole file, the whole tape block, or up to the top of the 64K address space for a snapshot)")
	bankStr := flag.String("bank", "", "--snapshot/--recording only, 128K-family snapshots: select a specific RAM bank (0-7) to appear at 0xC000-0xFFFF, overriding the snapshot's own paging state -- for inspecting a bank that is not currently paged in. Not meaningful for a 48K snapshot")

	flag.Parse()

	target, err := disasm.ParseTarget(*targetName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "zendis: %v\n", err)
		os.Exit(2)
	}
	switch *format {
	case "listing", "plain", "zenas", "pasmo", "sjasmplus", "snasm", "json":
	default:
		fmt.Fprintf(os.Stderr, "zendis: unknown --format %q (want listing, plain, zenas, pasmo, sjasmplus, snasm, or json)\n", *format)
		os.Exit(2)
	}
	if *symPath != "" {
		switch *format {
		case "plain", "zenas", "pasmo", "sjasmplus":
		case "snasm":
			fmt.Fprintln(os.Stderr, "zendis: --sym is not available for --format=snasm -- SNASM's own -sym/-vice flags produce no output in the tested release (V3.2.3.0), so there's no working format to match; its -map output is a different kind of artifact (source-line debug info, not just a symbol table) and isn't implemented here")
			os.Exit(2)
		default:
			fmt.Fprintf(os.Stderr, "zendis: --sym requires --format to be plain, zenas, pasmo, or sjasmplus (got %q)\n", *format)
			os.Exit(2)
		}
	}

	// Exactly one input source: a positional raw-binary path, --tape,
	// --snapshot, or --recording.
	sources := 0
	if flag.NArg() == 1 {
		sources++
	}
	if *tapePath != "" {
		sources++
	}
	if *snapshotPath != "" {
		sources++
	}
	if *recordingPath != "" {
		sources++
	}
	if sources != 1 {
		fmt.Fprintln(os.Stderr, "usage: zendis [flags] <file.bin>\n   or: zendis [flags] --tape=FILE.tap|FILE.tzx|FILE.pzx\n   or: zendis [flags] --snapshot=FILE.sna|FILE.z80|FILE.szx\n   or: zendis [flags] --recording=FILE.rzx\nexactly one input source is required")
		os.Exit(2)
	}
	if *blockName != "" || *blockStr != "" {
		if *tapePath == "" {
			fmt.Fprintln(os.Stderr, "zendis: --block/--block-name only apply with --tape")
			os.Exit(2)
		}
	}
	if *bankStr != "" && *snapshotPath == "" && *recordingPath == "" {
		fmt.Fprintln(os.Stderr, "zendis: --bank only applies with --snapshot or --recording")
		os.Exit(2)
	}

	orgOverride, err := parseOptionalU16(*orgStr, "--org")
	if err != nil {
		fmt.Fprintf(os.Stderr, "zendis: %v\n", err)
		os.Exit(2)
	}
	lengthOverride, err := parseOptionalLength(*lengthStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "zendis: %v\n", err)
		os.Exit(2)
	}
	var bankOverride *int
	if *bankStr != "" {
		b, err := strconv.Atoi(*bankStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "zendis: invalid --bank %q: %v\n", *bankStr, err)
			os.Exit(2)
		}
		bankOverride = &b
	}
	var blockIndexOverride *int
	if *blockStr != "" {
		idx, err := strconv.Atoi(*blockStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "zendis: invalid --block %q: %v\n", *blockStr, err)
			os.Exit(2)
		}
		blockIndexOverride = &idx
	}

	hints := &disasm.Hints{}
	if *hintsPath != "" {
		f, err := os.Open(*hintsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "zendis: %v\n", err)
			os.Exit(1)
		}
		h, err := disasm.ParseHints(f)
		f.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "zendis: %v\n", err)
			os.Exit(1)
		}
		hints = h
	}

	kind, path := api.SourceRaw, flagArgOrEmpty()
	switch {
	case *tapePath != "":
		kind, path = api.SourceTape, *tapePath
	case *snapshotPath != "":
		kind, path = api.SourceSnapshot, *snapshotPath
	case *recordingPath != "":
		kind, path = api.SourceRecording, *recordingPath
	}
	image, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "zendis: %v\n", err)
		os.Exit(1)
	}

	d, err := api.Disassemble(api.Options{
		Kind:       kind,
		Image:      image,
		Target:     target,
		Hints:      hints,
		AutoLabels: *labels,
		Org:        orgOverride,
		Length:     lengthOverride,
		BlockIndex: blockIndexOverride,
		BlockName:  *blockName,
		Bank:       bankOverride,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "zendis: %v\n", err)
		os.Exit(1)
	}

	exitCode := 0
	if d.TruncatedTail() {
		exitCode = 1
	}

	switch *format {
	case "listing":
		d.WriteListing(os.Stdout, *warnZ80N, os.Stderr)
	case "plain", "zenas", "pasmo", "sjasmplus", "snasm":
		d.WriteAssemblySource(os.Stdout, render.Dialect(*format), os.Stderr)
	case "json":
		if err := d.WriteJSON(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "zendis: %v\n", err)
			os.Exit(1)
		}
	}

	if *symPath != "" {
		symbols := d.Symbols()
		f, err := os.Create(*symPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "zendis: %v\n", err)
			os.Exit(1)
		}
		var werr error
		if *format == "sjasmplus" {
			werr = render.WriteSjasmplusSym(f, symbols)
		} else {
			werr = render.WritePasmoSym(f, symbols)
		}
		cerr := f.Close()
		if werr != nil {
			fmt.Fprintf(os.Stderr, "zendis: %v\n", werr)
			os.Exit(1)
		}
		if cerr != nil {
			fmt.Fprintf(os.Stderr, "zendis: %v\n", cerr)
			os.Exit(1)
		}
	}

	os.Exit(exitCode)
}

// flagArgOrEmpty returns flag.Arg(0), or "" if there isn't one -- a
// small helper so the mutual-exclusion logic above (already checked)
// doesn't need repeating here.
func flagArgOrEmpty() string {
	if flag.NArg() == 1 {
		return flag.Arg(0)
	}
	return ""
}

// parseOptionalU16 parses s as a uint16 (decimal or 0x-prefixed hex)
// if non-empty, returning nil if s is empty -- the shared "was this
// explicitly set" pattern --org and --length both need, since 0 is a
// legitimate value for either and can't double as an unset sentinel.
func parseOptionalU16(s, flagName string) (*uint16, error) {
	if s == "" {
		return nil, nil
	}
	v, err := strconv.ParseUint(s, 0, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid %s %q: %w", flagName, s, err)
	}
	u := uint16(v)
	return &u, nil
}

func parseOptionalLength(s string) (*int, error) {
	if s == "" {
		return nil, nil
	}
	v, err := strconv.ParseUint(s, 0, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid --length %q: %w", s, err)
	}
	n := int(v)
	return &n, nil
}
