// Command zendis disassembles Z80/Z80N machine code from a binary
// file, a tape image (.tap/.tzx/.pzx), a snapshot (.sna/.z80/.szx), an
// RZX recording, or a Spectrum Next executable (.nex). This is a thin CLI wrapper: flag parsing, file
// reading, and process exit codes live here; the actual pipeline
// (resolving an input source into code+org, then decoding and
// rendering it) is pkg/api's own Disassemble, exported directly for a
// caller embedding zendis as a library rather than shelling out to
// this binary.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ha1tch/zendis/pkg/api"
	"github.com/ha1tch/zendis/pkg/disasm"
	"github.com/ha1tch/zendis/pkg/render"
	"github.com/ha1tch/zentools/pkg/symfile"
)

// pathList is a flag that can be given more than once.
type pathList []string

func (p *pathList) String() string     { return fmt.Sprint([]string(*p)) }
func (p *pathList) Set(v string) error { *p = append(*p, v); return nil }

// dataList is the --data flag: ADDR:LENGTH or ADDR:LENGTH:KIND, any
// number of times.
type dataList []disasm.DataRange

func (d *dataList) String() string { return fmt.Sprint([]disasm.DataRange(*d)) }
func (d *dataList) Set(v string) error {
	r, err := disasm.ParseDataRange(v)
	if err != nil {
		return err
	}
	*d = append(*d, r)
	return nil
}

func main() {
	orgStr := flag.String("org", "", "load address (decimal, or 0x-prefixed hex). Raw-binary mode defaults to 0 if omitted; --tape/--snapshot mode uses the source's own load address/PC if omitted, or overrides it if given")
	targetName := flag.String("target", "z80", "CPU target: z80 (Z80N-only opcodes stay undefined) or z80n. With --nex the default is z80n, a NEX file being a Next's")
	warnZ80N := flag.Bool("warn-z80n", true, "under --target=z80, warn on stderr when an undefined opcode matches a Z80N instruction")
	labels := flag.Bool("labels", true, "replace JP/JR/CALL/DJNZ target addresses with generated labels (Lxxxx) where no hints-file name overrides them")
	hintsPath := flag.String("hints", "", "path to a hints file (label/comment/data/entry directives -- see docs/USAGE.md)")
	var dataRanges dataList
	flag.Var(&dataRanges, "data", "write a range as data and not as instructions: ADDR:LENGTH or ADDR:LENGTH:KIND, where KIND is bytes (DB of numbers, the default), hex (zenas's compact DB 0x 3E 01), words (DW), text (DB of quoted strings) or fill (DS for runs of equal bytes). May be given more than once; the same as a hints file's data directive")
	asKind := flag.String("as", "code", "what everything not marked otherwise is: code (instructions), or a data kind (bytes, hex, words, text, fill) to write the whole of it as data -- for a block that is a table, a screen or text")
	listBlocks := flag.Bool("blocks", false, "list the parts of the input and stop: a tape's blocks with the numbers --block takes, a NEX file's banks with where each is and how much of it is used")
	format := flag.String("format", "listing", "output format: listing (default), plain/zenas, pasmo, sjasmplus, snasm (all four re-assemblable), or json")
	symPath := flag.String("sym", "", "write a symbol file (format matches --format: pasmo's/zenas's shape for plain/zenas/pasmo, sjasmplus's shape for sjasmplus) -- requires --format to be one of those four; not available for snasm, see docs/USAGE.md")

	var symbolPaths pathList
	flag.Var(&symbolPaths, "symbols", "read names for addresses from a symbol file: a CSpect map (SNASM's, or sjasmplus's CSPECTMAP), a pasmo or zenas .sym, an sjasmplus --sym file or a z88dk .map. May be given more than once; a hints-file label for the same address wins, then the first file given -- see docs/USAGE.md")
	symbolsFormat := flag.String("symbols-format", "auto", "the format of the --symbols files: auto (from each file's first line), cspect, pasmo, zenas, sjasmplus or z88dk")
	mapPath := flag.String("map", "", "write the labels as a CSpect map file, for its debugger's -map option; works with every --format")

	tapePath := flag.String("tape", "", "load code from a tape image (.tap, .tzx, or .pzx) instead of a raw binary file")
	snapshotPath := flag.String("snapshot", "", "load code from a snapshot (.sna, .z80, or .szx, 48K or 128K-family) instead of a raw binary file; see --bank for 128K-family bank selection")
	recordingPath := flag.String("recording", "", "load code from the first embedded snapshot in an RZX input-recording file instead of a raw binary file; see --bank for 128K-family bank selection")
	nexPath := flag.String("nex", "", "load code from a Spectrum Next executable (.nex) instead of a raw binary file: what is used of the banks in the address space when the program starts (5 at 0x4000, 2 at 0x8000, the entry bank at 0xC000), from the bank the entry point is in. --bank or --page takes another part; --blocks lists them")
	pageStr := flag.String("page", "", "--nex only: take one 8K page (0-223) alone, at --bank-at; page 2n and 2n+1 are the halves of bank n")
	bankAtStr := flag.String("bank-at", "", "--nex only, with --bank or --page: the address of the first byte of the bank or page, a multiple of 0x2000 (default 0xC000)")
	blockStr := flag.String("block", "", "--tape only: select a block by its 0-based position in the tape's own block list (not a header+data pair count -- some tapes have bare data blocks with no header at all); default is the first Code-type block found")
	blockName := flag.String("block-name", "", "--tape only: select a block by its exact header name, overriding --block")
	lengthStr := flag.String("length", "", "limit disassembly to N bytes from the resolved org (decimal, or 0x-prefixed hex); default is everything the source itself provides (the whole file, the whole tape block, or up to the top of the 64K address space for a snapshot)")
	bankStr := flag.String("bank", "", "--snapshot/--recording, 128K-family snapshots: select a specific RAM bank (0-7) to appear at 0xC000-0xFFFF, overriding the snapshot's own paging state -- for inspecting a bank that is not currently paged in. Not meaningful for a 48K snapshot. --nex: take one 16K bank (0-111) alone, at --bank-at")

	flag.Parse()

	targetGiven := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "target" {
			targetGiven = true
		}
	})
	target, err := disasm.ParseTarget(*targetName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "zendis: %v\n", err)
		os.Exit(2)
	}
	if *nexPath != "" && !targetGiven {
		target = api.DefaultTarget(api.SourceNEX)
	}
	var asData *disasm.DataKind
	if !strings.EqualFold(*asKind, "code") {
		k, err := disasm.ParseDataKind(*asKind)
		if err != nil {
			fmt.Fprintf(os.Stderr, "zendis: --as: unknown %q (want code, bytes, hex, words, text or fill)\n", *asKind)
			os.Exit(2)
		}
		asData = &k
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
			fmt.Fprintln(os.Stderr, "zendis: --sym is not available for --format=snasm -- SNASM's own -sym/-vice flags produce no output in the tested release (V3.2.3.0), so there's no working format to match; use --map, which writes the symbols as the map file SNASM's -map gives and CSpect loads")
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
	if *nexPath != "" {
		sources++
	}
	if sources != 1 {
		fmt.Fprintln(os.Stderr, "usage: zendis [flags] <file.bin>\n   or: zendis [flags] --tape=FILE.tap|FILE.tzx|FILE.pzx\n   or: zendis [flags] --snapshot=FILE.sna|FILE.z80|FILE.szx\n   or: zendis [flags] --recording=FILE.rzx\n   or: zendis [flags] --nex=FILE.nex\nexactly one input source is required")
		os.Exit(2)
	}
	if *blockName != "" || *blockStr != "" {
		if *tapePath == "" {
			fmt.Fprintln(os.Stderr, "zendis: --block/--block-name only apply with --tape")
			os.Exit(2)
		}
	}
	if *bankStr != "" && *snapshotPath == "" && *recordingPath == "" && *nexPath == "" {
		fmt.Fprintln(os.Stderr, "zendis: --bank only applies with --snapshot, --recording or --nex")
		os.Exit(2)
	}
	if (*pageStr != "" || *bankAtStr != "") && *nexPath == "" {
		fmt.Fprintln(os.Stderr, "zendis: --page and --bank-at only apply with --nex")
		os.Exit(2)
	}
	if *bankAtStr != "" && *bankStr == "" && *pageStr == "" {
		fmt.Fprintln(os.Stderr, "zendis: --bank-at says where a --bank or a --page is; give one of them")
		os.Exit(2)
	}
	if *bankStr != "" && *pageStr != "" {
		fmt.Fprintln(os.Stderr, "zendis: --bank and --page are not given together")
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
	var pageOverride *int
	if *pageStr != "" {
		p, err := strconv.Atoi(*pageStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "zendis: invalid --page %q: %v\n", *pageStr, err)
			os.Exit(2)
		}
		pageOverride = &p
	}
	bankAt, err := parseOptionalU16(*bankAtStr, "--bank-at")
	if err != nil {
		fmt.Fprintf(os.Stderr, "zendis: %v\n", err)
		os.Exit(2)
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
	// --data ranges go ahead of the file's: the command line is the more
	// specific of the two.
	if len(dataRanges) > 0 {
		hints.Data = append(append([]disasm.DataRange{}, dataRanges...), hints.Data...)
	}
	var symbols *symfile.Table
	if len(symbolPaths) > 0 {
		symFormat, err := symfile.ParseFormat(*symbolsFormat)
		if err != nil {
			fmt.Fprintf(os.Stderr, "zendis: --symbols-format: %v\n", err)
			os.Exit(2)
		}
		symbols = symfile.NewTable()
		for _, path := range symbolPaths {
			f, err := os.Open(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "zendis: %v\n", err)
				os.Exit(1)
			}
			_, err = symbols.AddFile(f, symFormat)
			f.Close()
			if err != nil {
				fmt.Fprintf(os.Stderr, "zendis: %s: %v\n", path, err)
				os.Exit(1)
			}
		}
	}

	kind, path := api.SourceRaw, flagArgOrEmpty()
	switch {
	case *tapePath != "":
		kind, path = api.SourceTape, *tapePath
	case *snapshotPath != "":
		kind, path = api.SourceSnapshot, *snapshotPath
	case *recordingPath != "":
		kind, path = api.SourceRecording, *recordingPath
	case *nexPath != "":
		kind, path = api.SourceNEX, *nexPath
	}
	image, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "zendis: %v\n", err)
		os.Exit(1)
	}

	if *listBlocks {
		lerr := api.WriteParts(os.Stdout, kind, image)
		if errors.Is(lerr, api.ErrOnePart) {
			fmt.Fprintln(os.Stderr, "zendis: --blocks lists a tape's blocks (--tape) or a NEX file's banks (--nex); the other inputs are one block, chosen with --org and --length (and --bank for a 128K snapshot)")
			os.Exit(2)
		}
		if lerr != nil {
			fmt.Fprintf(os.Stderr, "zendis: %v\n", lerr)
			os.Exit(1)
		}
		os.Exit(0)
	}

	d, err := api.Disassemble(api.Options{
		Kind:       kind,
		Image:      image,
		Target:     target,
		Hints:      hints,
		AutoLabels: *labels,
		Symbols:    symbols,
		Org:        orgOverride,
		Length:     lengthOverride,
		BlockIndex: blockIndexOverride,
		BlockName:  *blockName,
		Bank:       bankOverride,
		Page:       pageOverride,
		BankAt:     bankAt,
		AsData:     asData,
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

	if *mapPath != "" {
		f, err := os.Create(*mapPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "zendis: %v\n", err)
			os.Exit(1)
		}
		werr := d.WriteCSpectMap(f)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			fmt.Fprintf(os.Stderr, "zendis: %v\n", werr)
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
